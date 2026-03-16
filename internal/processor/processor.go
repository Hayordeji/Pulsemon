package processor

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"

	"Pulsemon/internal/alerts"
	"Pulsemon/internal/worker"
	"Pulsemon/pkg/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Alerter defines the contract for sending alerts.
type Alerter interface {
	SendAlert(ctx context.Context, input alerts.SendAlertInput) error
}

// Processor consumes ProbeResults from the results channel and:
//   - stores results in the probe_results table
//   - updates failure streaks, latency stats, and SLA on the service record
//   - updates SSL certificate details for HTTPS services
//   - checks alert conditions and delegates to the Alerter
type Processor struct {
	db      *gorm.DB
	results <-chan worker.ProbeResult
	alerter Alerter
}

// NewProcessor creates a new Processor.
func NewProcessor(db *gorm.DB, results <-chan worker.ProbeResult, alerter Alerter) *Processor {
	return &Processor{
		db:      db,
		results: results,
		alerter: alerter,
	}
}

// Start begins consuming results in a blocking loop.
func (p *Processor) Start(ctx context.Context) {
	slog.Info("result processor started")
	for {
		select {
		case result, ok := <-p.results:
			if !ok {
				slog.Info("results channel closed, processor stopping")
				return
			}
			p.process(ctx, result)
		case <-ctx.Done():
			slog.Info("context cancelled, processor stopping")
			return
		}
	}
}

// process handles a single ProbeResult.
func (p *Processor) process(ctx context.Context, result worker.ProbeResult) {
	// Parse string IDs to uuid.UUID.
	serviceID, err := uuid.Parse(result.ServiceID)
	if err != nil {
		slog.Error("processor: invalid service ID",
			"service_id", result.ServiceID,
			"error", err)
		return
	}
	userID, err := uuid.Parse(result.UserID)
	if err != nil {
		slog.Error("processor: invalid user ID",
			"user_id", result.UserID,
			"error", err)
		return
	}

	// --- Step A: Store probe result ---
	probeResult := models.ProbeResult{
		ServiceID:    serviceID,
		UserID:       userID,
		StatusCode:   result.StatusCode,
		LatencyMs:    result.LatencyMs,
		IsSuccess:    result.IsSuccess,
		ErrorMessage: result.ErrorMessage,
		CheckedAt:    result.CheckedAt,
		CertExpiry:   result.CertExpiry,
		CertValid:    result.CertValid,
	}

	alertInput := alerts.SendAlertInput{
		ServiceID: result.ServiceID,
		UserID:    result.UserID,
		AlertType: "ssl_expiry",
		Message:   fmt.Sprintf("SSL certificate expires in %d days", *result.DaysRemaining),
	}

	if err := p.db.Create(&probeResult).Error; err != nil {
		slog.Error("processor: failed to store probe result for service %s: %v", result.ServiceID, err)
		return
	}

	// --- Step B: Fetch the service record ---
	var service models.Service
	if err := p.db.Where("id = ?", serviceID).First(&service).Error; err != nil {
		slog.Error("processor: service %s not found: %v", result.ServiceID, err)
		return
	}

	// --- Step C: Update failure streak ---
	if result.IsSuccess {
		previousStreak := service.FailureStreak
		service.FailureStreak = 0
		service.CurrentStatus = "up"

		if previousStreak >= 3 {
			alertInput.AlertType = "recovery"
			alertInput.Message = "Service has recovered after a failure streak"
			if err := p.alerter.SendAlert(ctx, alertInput); err != nil {
				slog.Error("processor: failed to send recovery alert for service %s: %v", result.ServiceID, err)
			}
		}
	} else {
		service.FailureStreak += 1
		service.CurrentStatus = "down"

		if service.FailureStreak >= 3 {
			alertInput.AlertType = "failure_streak"
			alertInput.Message = fmt.Sprintf("Service has failed %d consecutive times", service.FailureStreak)
			if err := p.alerter.SendAlert(ctx, alertInput); err != nil {
				slog.Error("processor: failed to send failure streak alert for service %s: %v", result.ServiceID, err)
			}
		}
	}

	// --- Step D: Update SLA ---
	service.TotalChecks += 1
	if result.IsSuccess {
		service.SuccessfulChecks += 1
	}

	if service.TotalChecks > 0 {
		service.SLAPercentage = (float64(service.SuccessfulChecks) / float64(service.TotalChecks)) * 100

		if service.SLAPercentage < service.SLATarget {
			alertInput.AlertType = "sla_breach"
			alertInput.Message = fmt.Sprintf("SLA breach: current %.2f%% is below target %.2f%%", service.SLAPercentage, service.SLATarget)

			if err := p.alerter.SendAlert(ctx, alertInput); err != nil {
				slog.Error("processor: failed to send SLA breach alert for service %s: %v", result.ServiceID, err)
			}
		}
	}

	// --- Step E: Update latency stats ---
	var latencies []float64
	if err := p.db.Model(&models.ProbeResult{}).
		Where("service_id = ?", serviceID).
		Order("checked_at DESC").
		Limit(100).
		Pluck("latency_ms", &latencies).Error; err != nil {
		slog.Error("processor: failed to fetch latencies for service %s: %v", result.ServiceID, err)
	} else if len(latencies) > 0 {
		// Calculate average.
		var sum float64
		for _, v := range latencies {
			sum += v
		}
		service.AvgLatencyMs = sum / float64(len(latencies))

		// Sort a copy for percentile calculation.
		sorted := make([]float64, len(latencies))
		copy(sorted, latencies)
		sort.Float64s(sorted)

		p95Idx := int(math.Ceil(0.95*float64(len(sorted)))) - 1
		p99Idx := int(math.Ceil(0.99*float64(len(sorted)))) - 1

		service.P95LatencyMs = sorted[p95Idx]
		service.P99LatencyMs = sorted[p99Idx]
	}

	// --- Step F: Update SSL cert details (HTTPS only) ---
	if result.CertExpiry != nil {
		service.SSLCertExpiry = result.CertExpiry
		service.SSLCertValid = result.CertValid
		service.SSLCertIssuer = result.CertIssuer
		service.SSLDaysRemaining = result.DaysRemaining

		if result.DaysRemaining != nil && *result.DaysRemaining < 30 {
			alertInput.AlertType = "ssl_expiry"
			alertInput.Message = fmt.Sprintf("SSL certificate expires in %d days", *result.DaysRemaining)
			if err := p.alerter.SendAlert(ctx, alertInput); err != nil {
				slog.Error("processor: failed to send SSL expiry alert for service %s: %v", result.ServiceID, err)
			}
		}
	}

	// --- Step G: Update last checked timestamp ---
	checkedAt := result.CheckedAt
	service.LastCheckedAt = &checkedAt

	// --- Step H: Persist all service record changes ---
	if err := p.db.Save(&service).Error; err != nil {
		slog.Error("processor: failed to update service %s: %v", result.ServiceID, err)
	}
}
