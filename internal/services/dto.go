package services

import (
	"strings"
	"time"

	"Pulsemon/pkg/models"
)

// SSLInfoResponse is the nested SSL object in ServiceDetailResponse.
// Populated only for HTTPS services; nil for HTTP.
type SSLInfoResponse struct {
	CertValid     bool       `json:"cert_valid"`
	CertExpiry    *time.Time `json:"cert_expiry"`
	CertIssuer    *string    `json:"cert_issuer"`
	DaysRemaining *int       `json:"days_remaining"`
	ExpiryWarning bool       `json:"expiry_warning"`
}

// ServiceSummaryResponse is returned by the list endpoint (GET /services).
type ServiceSummaryResponse struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	URL           string     `json:"url"`
	Interval      string     `json:"interval"`
	CurrentStatus string     `json:"current_status"`
	SLAPercentage float64    `json:"sla_percentage"`
	LastCheckedAt *time.Time `json:"last_checked_at"`
}

// ServiceDetailResponse is returned by the get endpoint (GET /services/:id).
type ServiceDetailResponse struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	URL              string           `json:"url"`
	Interval         string           `json:"interval"`
	TimeoutSeconds   int              `json:"timeout_seconds"`
	ExpectedStatus   int              `json:"expected_status"`
	SLATarget        float64          `json:"sla_target"`
	IsActive         bool             `json:"is_active"`
	CurrentStatus    string           `json:"current_status"`
	LastCheckedAt    *time.Time       `json:"last_checked_at"`
	FailureStreak    int              `json:"failure_streak"`
	AvgLatencyMs     float64          `json:"avg_latency_ms"`
	P95LatencyMs     float64          `json:"p95_latency_ms"`
	P99LatencyMs     float64          `json:"p99_latency_ms"`
	SLAPercentage    float64          `json:"sla_percentage"`
	TotalChecks      int              `json:"total_checks"`
	SuccessfulChecks int              `json:"successful_checks"`
	SSL              *SSLInfoResponse `json:"ssl"`
	CreatedAt        time.Time        `json:"created_at"`
}

// CreateServiceResponse is returned by the create endpoint (POST /services).
type CreateServiceResponse struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	URL            string    `json:"url"`
	Interval       string    `json:"interval"`
	TimeoutSeconds int       `json:"timeout_seconds"`
	ExpectedStatus int       `json:"expected_status"`
	SLATarget      float64   `json:"sla_target"`
	IsActive       bool      `json:"is_active"`
	CurrentStatus  string    `json:"current_status"`
	CreatedAt      time.Time `json:"created_at"`
}

// ---------------------------------------------------------------------------
// Mapper functions
// ---------------------------------------------------------------------------

// ToServiceSummaryResponse maps a models.Service to a ServiceSummaryResponse.
func ToServiceSummaryResponse(s models.Service) ServiceSummaryResponse {
	return ServiceSummaryResponse{
		ID:            s.ID.String(),
		Name:          s.Name,
		URL:           s.URL,
		Interval:      s.Interval,
		CurrentStatus: s.CurrentStatus,
		SLAPercentage: s.SLAPercentage,
		LastCheckedAt: s.LastCheckedAt,
	}
}

// ToServiceDetailResponse maps a models.Service to a ServiceDetailResponse,
// including the nested SSL block for HTTPS services.
func ToServiceDetailResponse(s models.Service) ServiceDetailResponse {
	resp := ServiceDetailResponse{
		ID:               s.ID.String(),
		Name:             s.Name,
		URL:              s.URL,
		Interval:         s.Interval,
		TimeoutSeconds:   s.TimeoutSeconds,
		ExpectedStatus:   s.ExpectedStatus,
		SLATarget:        s.SLATarget,
		IsActive:         s.IsActive,
		CurrentStatus:    s.CurrentStatus,
		LastCheckedAt:    s.LastCheckedAt,
		FailureStreak:    s.FailureStreak,
		AvgLatencyMs:     s.AvgLatencyMs,
		P95LatencyMs:     s.P95LatencyMs,
		P99LatencyMs:     s.P99LatencyMs,
		SLAPercentage:    s.SLAPercentage,
		TotalChecks:      s.TotalChecks,
		SuccessfulChecks: s.SuccessfulChecks,
		CreatedAt:        s.CreatedAt,
	}

	// Populate SSL info only for HTTPS services with cert data.
	if strings.HasPrefix(s.URL, "https://") && s.SSLCertValid != nil {
		ssl := &SSLInfoResponse{
			CertValid:     *s.SSLCertValid,
			CertExpiry:    s.SSLCertExpiry,
			CertIssuer:    s.SSLCertIssuer,
			DaysRemaining: s.SSLDaysRemaining,
		}
		if s.SSLDaysRemaining != nil && *s.SSLDaysRemaining < 30 {
			ssl.ExpiryWarning = true
		}
		resp.SSL = ssl
	}

	return resp
}

// ToCreateServiceResponse maps a models.Service to a CreateServiceResponse.
func ToCreateServiceResponse(s models.Service) CreateServiceResponse {
	return CreateServiceResponse{
		ID:             s.ID.String(),
		Name:           s.Name,
		URL:            s.URL,
		Interval:       s.Interval,
		TimeoutSeconds: s.TimeoutSeconds,
		ExpectedStatus: s.ExpectedStatus,
		SLATarget:      s.SLATarget,
		IsActive:       s.IsActive,
		CurrentStatus:  s.CurrentStatus,
		CreatedAt:      s.CreatedAt,
	}
}
