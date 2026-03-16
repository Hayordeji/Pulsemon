package alerts

import (
	"context"
	"fmt"
	"log/slog"
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
	subject := composeSubject(input.AlertType, service.Name)
	body := fmt.Sprintf("Service: %s\nAlert Type: %s\n\n%s", service.Name, input.AlertType, input.Message)

	// --- Step D: Send email via Resend ---
	// Fetch user email.
	var user models.User
	if err := e.db.WithContext(ctx).Where("id = ?", input.UserID).First(&user).Error; err != nil {
		slog.Error("failed to fetch user for alert",
			"user_id", input.UserID,
			"error", err)
		return fmt.Errorf("failed to fetch user: %w", err)
	}

	// client := resend.NewClient(e.resendAPIKey)
	params := &resend.SendEmailRequest{
		From:    e.fromEmail,
		To:      []string{user.Email},
		Subject: subject,
		Text:    body,
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
		"alert_type", input.AlertType)

	return nil
}

// composeSubject returns the email subject line based on alert type.
func composeSubject(alertType string, serviceName string) string {
	switch alertType {
	case "failure_streak":
		return "Service Down: " + serviceName
	case "sla_breach":
		return "SLA Breach: " + serviceName
	case "ssl_expiry":
		return "SSL Expiry Warning: " + serviceName
	case "recovery":
		return "Service Recovered: " + serviceName
	default:
		return "Alert: " + serviceName
	}
}
