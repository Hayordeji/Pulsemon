package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"Pulsemon/pkg/config"
	"Pulsemon/pkg/models"

	"github.com/google/uuid"
	"github.com/resendlabs/resend-go"
	"gorm.io/gorm"
)

// AlertEngine implements the processor.Alerter interface.
// It checks cooldown windows, composes emails, sends them via
// the Resend API, and records sent alerts in the database.
type AlertEngine struct {
	db              *gorm.DB
	client          *resend.Client
	resendAPIKey    string
	fromEmail       string
	cooldownMinutes int
}

type SendAlertInput struct {
	ServiceID string
	UserID    string
	AlertType string
	Message   string
}

// NewAlertEngine creates a new AlertEngine from the database connection
// and application configuration.
func NewAlertEngine(db *gorm.DB, cfg config.Config) *AlertEngine {
	return &AlertEngine{
		db:              db,
		client:          resend.NewClient(cfg.ResendAPIKey),
		resendAPIKey:    cfg.ResendAPIKey,
		fromEmail:       cfg.ResendFromEmail,
		cooldownMinutes: 30,
	}
}

// SendAlert checks cooldown, composes and sends an email alert via
// Resend, and records the alert in the database.
func (e *AlertEngine) SendAlert(ctx context.Context, input SendAlertInput) error {
	// --- Step A: Cooldown check ---
	var lastAlert models.Alert
	err := e.db.WithContext(ctx).
		Where("service_id = ? AND alert_type = ?", input.ServiceID, input.AlertType).
		Order("sent_at DESC").
		Limit(1).
		First(&lastAlert).Error

	if err == nil {
		// A previous alert exists — check if it's within the cooldown window.
		cooldownCutoff := time.Now().Add(-time.Duration(e.cooldownMinutes) * time.Minute)
		if lastAlert.SentAt.After(cooldownCutoff) {
			slog.Info("alert suppressed by cooldown",
				"service_id", input.ServiceID,
				"alert_type", input.AlertType)
			return nil
		}
	}
	// If err is gorm.ErrRecordNotFound, no previous alert exists — proceed.
	// Any other DB error is unexpected but we still proceed to send the alert.
	if err != nil && err != gorm.ErrRecordNotFound {
		slog.Error("failed to query cooldown alert",
			"service_id", input.ServiceID,
			"alert_type", input.AlertType,
			"error", err)
	}

	// --- Step B: Fetch service name ---
	var service models.Service
	if err := e.db.WithContext(ctx).Where("id = ?", input.ServiceID).First(&service).Error; err != nil {
		slog.Error("failed to fetch service for alert",
			"service_id", input.ServiceID,
			"error", err)
		return fmt.Errorf("failed to fetch service: %w", err)
	}

	// --- Step C: Compose email ---
	subject, alertType := composeSubject(input.AlertType, service.Name)
	// body := fmt.Sprintf("Service: %s\nAlert Type: %s\n\n%s", service.Name, input.AlertType, input.Message)

	// --- Step D: Send email via Resend ---
	// Fetch user email.
	var user models.User
	if err := e.db.WithContext(ctx).Where("id = ?", input.UserID).First(&user).Error; err != nil {
		slog.Error("failed to fetch user for alert",
			"user_id", input.UserID,
			"error", err)
		return fmt.Errorf("failed to fetch user: %w", err)
	}

	dashboardUrl := "Will do it later"
	htmlBody := returnHtmlBody(&service.Name, &alertType, &input.Message, &dashboardUrl)

	// client := resend.NewClient(e.resendAPIKey)
	params := &resend.SendEmailRequest{
		From:    e.fromEmail,
		To:      []string{user.Email},
		Subject: subject,
		// Text:    body,
		Html: htmlBody,
	}
	_, err = e.client.Emails.Send(params)
	if err != nil {
		slog.Error("failed to send alert email",
			"service_id", input.ServiceID,
			"alert_type", input.AlertType,
			"error", err)
		return err
	}

	// --- Step E: Record the alert ---
	parsedServiceID, err := uuid.Parse(input.ServiceID)
	if err != nil {
		slog.Error("invalid service ID for alert record",
			"service_id", input.ServiceID,
			"error", err)
		return fmt.Errorf("invalid service ID: %w", err)
	}
	parsedUserID, err := uuid.Parse(input.UserID)
	if err != nil {
		slog.Error("invalid user ID for alert record",
			"user_id", input.UserID,
			"error", err)
		return fmt.Errorf("invalid user ID: %w", err)
	}

	alert := models.Alert{
		ServiceID: parsedServiceID,
		UserID:    parsedUserID,
		AlertType: input.AlertType,
		Message:   input.Message,
		SentAt:    time.Now(),
	}
	if err := e.db.WithContext(ctx).Create(&alert).Error; err != nil {
		slog.Error("failed to record alert",
			"service_id", input.ServiceID,
			"alert_type", input.AlertType,
			"error", err)
		return fmt.Errorf("failed to record alert: %w", err)
	}

	slog.Info("alert sent successfully",
		"service_id", input.ServiceID,
		"alert_type", input.AlertType,
		"email", user.Email)

	return nil
}

// composeSubject returns the email subject line based on alert type.
func composeSubject(alertType string, serviceName string) (string, string) {
	switch alertType {
	case string(models.AlertTypeFailureStreak):
		return "Service Down: " + serviceName, "Failure Streak"
	case string(models.AlertTypeSLA_Breach):
		return "SLA Breach: " + serviceName, "SLA Breach"
	case string(models.AlertTypeSSL_Expiry):
		return "SSL Expiry Warning: " + serviceName, "SSL Expiry"
	case string(models.AlertTypeRecovery):
		return "Service Recovered: " + serviceName, "Recovery"
	default:
		return "Alert: " + serviceName, "Alert"
	}
}

func returnHtmlBody(serviceName *string, alertType *string, message *string, url *string) string {
	body := `
				<!DOCTYPE html>
		<html lang="en" xmlns="http://www.w3.org/1999/xhtml">
		<head>
		<meta charset="UTF-8" />
		<meta name="viewport" content="width=device-width, initial-scale=1.0" />
		<meta http-equiv="X-UA-Compatible" content="IE=edge" />
		<title>Pulsemon Alert</title>
		<style>
			@media only screen and (max-width: 600px) {
			.email-wrapper  { padding: 16px !important; }
			.email-card     { border-radius: 12px !important; }
			.card-body      { padding: 24px 20px !important; }
			.banner-cell    { padding: 14px 20px !important; }
			.stat-table td  { display: block !important; width: 100% !important; padding-bottom: 16px !important; }
			.cta-btn        { width: 100% !important; text-align: center !important; display: block !important; }
			.footer-cell    { padding: 24px 0 0 0 !important; }
			}
		</style>
		</head>
		<body style="margin:0;padding:0;background-color:#0a0c12;-webkit-text-size-adjust:100%;moz-text-size-adjust:100%;">

		<!-- Outer wrapper -->
		<table role="presentation" width="100%" cellpadding="0" cellspacing="0"
				style="background-color:#0a0c12;min-height:100vh;">
			<tr>
			<td class="email-wrapper" align="center" style="padding:40px 16px;">

				<!-- Max-width container -->
				<table role="presentation" width="100%" cellpadding="0" cellspacing="0"
					style="max-width:580px;width:100%;">

				<!-- ─── HEADER ─── -->
				<tr>
					<td style="padding:0 0 28px 0;">
					<table role="presentation" width="100%" cellpadding="0" cellspacing="0">
						<tr>
						<td style="vertical-align:middle;">
							<table role="presentation" cellpadding="0" cellspacing="0">
							<tr>
								<td style="vertical-align:middle;padding-right:10px;">
								<svg width="20" height="20" viewBox="0 0 24 24" fill="none"
									xmlns="http://www.w3.org/2000/svg">
									<path d="M13 2L4.5 13.5H11L10 22L19.5 10H13L13 2Z"
										fill="#3b82f6" stroke="#3b82f6"
										stroke-width="1.5" stroke-linejoin="round"/>
								</svg>
								</td>
								<td style="vertical-align:middle;">
								<span style="color:#ffffff;font-size:17px;font-weight:700;
											font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',
											Helvetica,Arial,sans-serif;letter-spacing:-0.4px;">
									Pulsemon
								</span>
								</td>
							</tr>
							</table>
						</td>
						<td align="right" style="vertical-align:middle;">
							<span style="color:#4a4e6a;font-size:12px;
										font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',
										Helvetica,Arial,sans-serif;">
							Service Alert
							</span>
						</td>
						</tr>
					</table>
					</td>
				</tr>

				<!-- ─── MAIN CARD ─── -->
				<tr>
					<td class="email-card"
						style="background-color:#12151f;border-radius:16px;
							border:1px solid #1e2235;overflow:hidden;">

					<!-- Top accent banner — always blue -->
					<table role="presentation" width="100%" cellpadding="0" cellspacing="0">
						<tr>
						<td class="banner-cell"
							style="background-color:#1d2a42;border-bottom:1px solid #1e2235;
									padding:14px 28px;">
							<table role="presentation" cellpadding="0" cellspacing="0">
							<tr>
								<td style="vertical-align:middle;padding-right:10px;">
								<!-- Pulse dot -->
								<div style="width:8px;height:8px;border-radius:50%;
											background-color:#3b82f6;display:inline-block;">
								</div>
								</td>
								<td style="vertical-align:middle;">
								<span style="color:#93c5fd;font-size:12px;font-weight:600;
											letter-spacing:1px;text-transform:uppercase;
											font-family:-apple-system,BlinkMacSystemFont,
											'Segoe UI',Helvetica,Arial,sans-serif;">
									{{ALERT_BADGE}}
								</span>
								</td>
							</tr>
							</table>
						</td>
						</tr>
					</table>

					<!-- Card content -->
					<table role="presentation" width="100%" cellpadding="0" cellspacing="0">
						<tr>
						<td class="card-body" style="padding:32px 28px 28px 28px;">

							<!-- Service name -->
							<p style="margin:0 0 4px 0;color:#6b7280;font-size:11px;
									font-weight:600;letter-spacing:1.2px;
									text-transform:uppercase;
									font-family:-apple-system,BlinkMacSystemFont,
									'Segoe UI',Helvetica,Arial,sans-serif;">
							Affected Service
							</p>
							<p style="margin:0 0 28px 0;color:#ffffff;font-size:24px;
									font-weight:700;letter-spacing:-0.5px;line-height:1.2;
									font-family:-apple-system,BlinkMacSystemFont,
									'Segoe UI',Helvetica,Arial,sans-serif;">
							{{SERVICE_NAME}}
							</p>

							<!-- Divider -->
							<table role="presentation" width="100%" cellpadding="0" cellspacing="0"
								style="margin-bottom:28px;">
							<tr>
								<td style="border-top:1px solid #1e2235;font-size:0;line-height:0;">
								&nbsp;
								</td>
							</tr>
							</table>

							<!-- Stat row -->
							<table role="presentation" width="100%" cellpadding="0" cellspacing="0"
								class="stat-table" style="margin-bottom:28px;">
							<tr>
								<td width="50%" style="vertical-align:top;padding-right:16px;">
								<p style="margin:0 0 6px 0;color:#6b7280;font-size:11px;
											font-weight:600;letter-spacing:1.2px;
											text-transform:uppercase;
											font-family:-apple-system,BlinkMacSystemFont,
											'Segoe UI',Helvetica,Arial,sans-serif;">
									Alert Type
								</p>
								<span style="display:inline-block;background-color:#1d2a42;
											color:#93c5fd;font-size:12px;font-weight:600;
											padding:4px 12px;border-radius:100px;
											border:1px solid #2a3f5f;
											font-family:-apple-system,BlinkMacSystemFont,
											'Segoe UI',Helvetica,Arial,sans-serif;">
									{{ALERT_TYPE_LABEL}}
								</span>
								</td>
								<td width="50%" style="vertical-align:top;">
								<p style="margin:0 0 6px 0;color:#6b7280;font-size:11px;
											font-weight:600;letter-spacing:1.2px;
											text-transform:uppercase;
											font-family:-apple-system,BlinkMacSystemFont,
											'Segoe UI',Helvetica,Arial,sans-serif;">
									Timestamp
								</p>
								<p style="margin:0;color:#d1d5db;font-size:13px;font-weight:500;
											line-height:1.4;
											font-family:-apple-system,BlinkMacSystemFont,
											'Segoe UI',Helvetica,Arial,sans-serif;">
									{{TIMESTAMP}}
								</p>
								</td>
							</tr>
							</table>

							<!-- Message box -->
							<table role="presentation" width="100%" cellpadding="0" cellspacing="0"
								style="margin-bottom:32px;">
							<tr>
								<td style="background-color:#0a0c12;border-radius:10px;
										border:1px solid #1e2235;
										border-left:3px solid #3b82f6;
										padding:16px 20px;">
								<p style="margin:0 0 6px 0;color:#6b7280;font-size:11px;
											font-weight:600;letter-spacing:1.2px;
											text-transform:uppercase;
											font-family:-apple-system,BlinkMacSystemFont,
											'Segoe UI',Helvetica,Arial,sans-serif;">
									Details
								</p>
								<p style="margin:0;color:#e5e7eb;font-size:14px;
											line-height:1.7;
											font-family:-apple-system,BlinkMacSystemFont,
											'Segoe UI',Helvetica,Arial,sans-serif;">
									{{MESSAGE}}
								</p>
								</td>
							</tr>
							</table>

							<!-- CTA button -->
							<table role="presentation" cellpadding="0" cellspacing="0">
							<tr>
								<td style="border-radius:8px;background-color:#3b82f6;">
								<a href="{{DASHBOARD_URL}}" class="cta-btn"
									style="display:inline-block;padding:13px 28px;
											color:#ffffff;font-size:14px;font-weight:600;
											text-decoration:none;letter-spacing:0.1px;
											font-family:-apple-system,BlinkMacSystemFont,
											'Segoe UI',Helvetica,Arial,sans-serif;">
									View Service Dashboard &rarr;
								</a>
								</td>
							</tr>
							</table>

						</td>
						</tr>
					</table>

					</td>
				</tr>

				<!-- ─── FOOTER ─── -->
				<tr>
					<td class="footer-cell" style="padding:28px 4px 0 4px;">

					<table role="presentation" width="100%" cellpadding="0" cellspacing="0"
							style="margin-bottom:20px;">
						<tr>
						<td style="border-top:1px solid #1a1d2e;font-size:0;line-height:0;">
							&nbsp;
						</td>
						</tr>
					</table>

					<p style="margin:0 0 8px 0;color:#4a4e6a;font-size:12px;line-height:1.6;
								font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',
								Helvetica,Arial,sans-serif;">
						You are receiving this because
						<strong style="color:#6b7280;">{{SERVICE_NAME}}</strong>
						is registered on Pulsemon. To stop alerts for this service,
						delete it from your dashboard.
					</p>
					<p style="margin:0;color:#323650;font-size:11px;
								font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',
								Helvetica,Arial,sans-serif;">
						&copy; 2026 Pulsemon &nbsp;&middot;&nbsp; 
					</p>

					</td>
				</tr>

				</table>
			</td>
			</tr>
		</table>

		</body>
		</html>
	`

	body = strings.ReplaceAll(body, "{{SERVICE_NAME}}", *serviceName)
	body = strings.ReplaceAll(body, "{{MESSAGE}}", *message)
	body = strings.ReplaceAll(body, "{{ALERT_BADGE}}", *alertType)
	body = strings.ReplaceAll(body, "{{ALERT_TYPE_LABEL}}", *alertType)
	body = strings.ReplaceAll(body, "{{TIMESTAMP}}", time.Now().Format("Jan 2, 2006 · 15:04 MST"))
	body = strings.ReplaceAll(body, "{{DASHBOARD_URL}}", *url)

	return body
}
