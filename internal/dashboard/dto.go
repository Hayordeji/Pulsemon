package dashboard

import (
	"math"
	"strconv"
	"strings"
	"time"

	"Pulsemon/pkg/models"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

// PaginationParams holds the parsed cursor and limit from query parameters.
type PaginationParams struct {
	Cursor *time.Time
	Limit  int
}

// PaginationMeta is included in paginated API responses.
type PaginationMeta struct {
	NextCursor *time.Time `json:"next_cursor"`
	HasMore    bool       `json:"has_more"`
}

const (
	defaultLimit = 20
	maxLimit     = 100
)

// ParsePaginationParams extracts cursor and limit from the Gin query string.
// Cursor is expected as an RFC 3339 timestamp. Limit defaults to 20, max 100.
func ParsePaginationParams(c *gin.Context) PaginationParams {
	params := PaginationParams{Limit: defaultLimit}

	cursorStr := c.Query("cursor")

	if cursorStr != "" {
	}
	if t, err := time.Parse(time.RFC3339, cursorStr); err == nil {
		params.Cursor = &t
	}

	limitStr := c.Query("limit")
	if limitStr == "" {

	}
	n, err := strconv.Atoi(limitStr)

	if err == nil && n > 0 {
		params.Limit = n
	}

	if params.Limit > maxLimit {
		params.Limit = maxLimit
	}

	return params
}

// ---------------------------------------------------------------------------
// Response DTOs
// ---------------------------------------------------------------------------

// SSLStatusResponse is the nested SSL object in DashboardResponse.
type SSLStatusResponse struct {
	CertValid     bool       `json:"cert_valid"`
	CertExpiry    *time.Time `json:"cert_expiry"`
	DaysRemaining *int       `json:"days_remaining"`
	ExpiryWarning bool       `json:"expiry_warning"`
}

// RecentResultResponse represents a single probe result entry.
type RecentResultResponse struct {
	CheckedAt  time.Time `json:"checked_at"`
	StatusCode int       `json:"status_code"`
	LatencyMs  float64   `json:"latency_ms"`
	IsSuccess  bool      `json:"is_success"`
}

// DashboardResponse is returned by GET /dashboard/:service_id.
type DashboardResponse struct {
	ServiceID        string                 `json:"service_id"`
	Name             string                 `json:"name"`
	CurrentStatus    string                 `json:"current_status"`
	UptimePercentage float64                `json:"uptime_percentage"`
	SLATarget        float64                `json:"sla_target"`
	SLABreached      bool                   `json:"sla_breached"`
	FailureStreak    int                    `json:"failure_streak"`
	AvgLatencyMs     float64                `json:"avg_latency_ms"`
	P95LatencyMs     float64                `json:"p95_latency_ms"`
	P99LatencyMs     float64                `json:"p99_latency_ms"`
	LastCheckedAt    *time.Time             `json:"last_checked_at"`
	SSL              *SSLStatusResponse     `json:"ssl"`
	RecentResults    []RecentResultResponse `json:"recent_results"`
	Pagination       PaginationMeta         `json:"pagination"`
}

// AlertResponse represents a single alert entry.
type AlertResponse struct {
	ID        string    `json:"id"`
	AlertType string    `json:"alert_type"`
	Message   string    `json:"message"`
	SentAt    time.Time `json:"sent_at"`
}

// ServiceAlertsResponse is returned by GET /dashboard/:service_id/alerts.
type ServiceAlertsResponse struct {
	ServiceID  string          `json:"service_id"`
	Alerts     []AlertResponse `json:"alerts"`
	Pagination PaginationMeta  `json:"pagination"`
}

// ---------------------------------------------------------------------------
// Mapper functions — all business logic lives here
// ---------------------------------------------------------------------------

// ToDashboardResponse maps a models.Service and its probe results to a
// DashboardResponse, including SSL status and cursor pagination metadata.
func ToDashboardResponse(service models.Service, results []models.ProbeResult, limit int) DashboardResponse {
	resp := DashboardResponse{
		ServiceID:        service.ID.String(),
		Name:             service.Name,
		CurrentStatus:    service.CurrentStatus,
		UptimePercentage: math.Ceil(service.SLAPercentage*100) / 100,
		SLATarget:        service.SLATarget,
		SLABreached:      service.SLAPercentage < service.SLATarget,
		FailureStreak:    service.FailureStreak,
		AvgLatencyMs:     math.Ceil(service.AvgLatencyMs*100) / 100,
		P95LatencyMs:     math.Ceil(service.P95LatencyMs*100) / 100,
		P99LatencyMs:     math.Ceil(service.P99LatencyMs*100) / 100,
		LastCheckedAt:    service.LastCheckedAt,
	}

	// SSL: populate only for HTTPS services with cert data.
	if strings.HasPrefix(service.URL, "https://") && service.SSLCertValid != nil {
		ssl := &SSLStatusResponse{
			CertValid:     *service.SSLCertValid,
			CertExpiry:    service.SSLCertExpiry,
			DaysRemaining: service.SSLDaysRemaining,
		}
		if service.SSLDaysRemaining != nil && *service.SSLDaysRemaining < 30 {
			ssl.ExpiryWarning = true
		}
		resp.SSL = ssl
	}

	// Determine pagination: repo fetched limit+1 rows.
	hasMore := len(results) > limit
	if hasMore {
		results = results[:limit]
	}

	recentResults := make([]RecentResultResponse, len(results))
	for i, r := range results {
		recentResults[i] = RecentResultResponse{
			CheckedAt:  r.CheckedAt,
			StatusCode: r.StatusCode,
			LatencyMs:  math.Ceil(r.LatencyMs*100) / 100,
			IsSuccess:  r.IsSuccess,
		}
	}
	resp.RecentResults = recentResults

	// Build pagination meta.
	resp.Pagination = PaginationMeta{HasMore: hasMore}
	if hasMore && len(recentResults) > 0 {
		last := recentResults[len(recentResults)-1].CheckedAt
		resp.Pagination.NextCursor = &last
	}

	return resp
}

// ToAlertResponse maps a models.Alert to an AlertResponse.
func ToAlertResponse(alert models.Alert) AlertResponse {
	return AlertResponse{
		ID:        alert.ID.String(),
		AlertType: alert.AlertType,
		Message:   alert.Message,
		SentAt:    alert.SentAt,
	}
}

// ToServiceAlertsResponse maps a service ID and its alerts to a
// ServiceAlertsResponse, including cursor pagination metadata.
func ToServiceAlertsResponse(serviceID string, alertsList []models.Alert, limit int) ServiceAlertsResponse {
	hasMore := len(alertsList) > limit
	if hasMore {
		alertsList = alertsList[:limit]
	}

	items := make([]AlertResponse, len(alertsList))
	for i, a := range alertsList {
		items[i] = ToAlertResponse(a)
	}

	pagination := PaginationMeta{HasMore: hasMore}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1].SentAt
		pagination.NextCursor = &last
	}

	return ServiceAlertsResponse{
		ServiceID:  serviceID,
		Alerts:     items,
		Pagination: pagination,
	}
}
