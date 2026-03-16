package worker

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"Pulsemon/internal/scheduler"
)

// ProbeResult carries everything the Result Processor needs after a probe.
type ProbeResult struct {
	ServiceID     string
	UserID        string
	StatusCode    int
	LatencyMs     float64
	IsSuccess     bool
	ErrorMessage  *string
	CheckedAt     time.Time
	CertExpiry    *time.Time // HTTPS only
	CertValid     *bool      // HTTPS only
	CertIssuer    *string    // HTTPS only
	DaysRemaining *int       // HTTPS only
}

// Prober defines the contract for executing a single probe.
type Prober interface {
	Probe(ctx context.Context, job scheduler.ProbeJob) ProbeResult
}

// HTTPProber implements Prober using a shared *http.Client.
type HTTPProber struct {
	client *http.Client
}

// NewHTTPProber creates an HTTPProber with a reusable HTTP client.
func NewHTTPProber() *HTTPProber {
	return &HTTPProber{
		client: &http.Client{
			Transport: &http.Transport{},
		},
	}
}

// Probe executes a single HTTP/HTTPS probe and returns the result.
func (h *HTTPProber) Probe(ctx context.Context, job scheduler.ProbeJob) ProbeResult {
	start := time.Now()
	checkedAt := start

	// Per-probe timeout derived from the job configuration.
	probeCtx, cancel := context.WithTimeout(ctx, time.Duration(job.TimeoutSeconds)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, job.URL, nil)
	if err != nil {
		errMsg := err.Error()
		return ProbeResult{
			ServiceID:    job.ServiceID,
			UserID:       job.UserID,
			StatusCode:   0,
			LatencyMs:    time.Since(start).Seconds() * 1000,
			IsSuccess:    false,
			ErrorMessage: &errMsg,
			CheckedAt:    checkedAt,
		}
	}

	resp, err := h.client.Do(req)
	latencyMs := time.Since(start).Seconds() * 1000

	if err != nil {
		errMsg := err.Error()
		return ProbeResult{
			ServiceID:    job.ServiceID,
			UserID:       job.UserID,
			StatusCode:   0,
			LatencyMs:    latencyMs,
			IsSuccess:    false,
			ErrorMessage: &errMsg,
			CheckedAt:    checkedAt,
		}
	}
	defer resp.Body.Close()

	isSuccess := resp.StatusCode == job.ExpectedStatus

	result := ProbeResult{
		ServiceID:  job.ServiceID,
		UserID:     job.UserID,
		StatusCode: resp.StatusCode,
		LatencyMs:  latencyMs,
		IsSuccess:  isSuccess,
		CheckedAt:  checkedAt,
	}

	// Set error message if status code does not match expected.
	if !isSuccess {
		errMsg := fmt.Sprintf("unexpected status code: got %d, expected %d",
			resp.StatusCode, job.ExpectedStatus)
		result.ErrorMessage = &errMsg
	}

	// Extract TLS certificate info for HTTPS services.
	if strings.HasPrefix(job.URL, "https://") &&
		resp.TLS != nil &&
		len(resp.TLS.PeerCertificates) > 0 {

		cert := resp.TLS.PeerCertificates[0]

		certExpiry := cert.NotAfter
		result.CertExpiry = &certExpiry

		certValid := time.Now().Before(cert.NotAfter)
		result.CertValid = &certValid

		certIssuer := cert.Issuer.CommonName
		result.CertIssuer = &certIssuer

		daysRemaining := int(time.Until(cert.NotAfter).Hours() / 24)
		result.DaysRemaining = &daysRemaining
	}

	return result
}
