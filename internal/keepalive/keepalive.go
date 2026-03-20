package keepalive

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"Pulsemon/pkg/config"
)

type KeepAlive struct {
	url      string
	interval time.Duration
	client   *http.Client
}

func NewKeepAlive(cfg config.Config) *KeepAlive {
	return &KeepAlive{
		url:      cfg.AppBaseURL + "/api/v1/health",
		interval: 12 * time.Minute,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (ka *KeepAlive) Start(ctx context.Context) {
	slog.Info("keep-alive started",
		"url", ka.url,
		"interval", ka.interval.String())

	ticker := time.NewTicker(ka.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ka.ping(ctx)
		case <-ctx.Done():
			slog.Info("keep-alive stopped")
			return
		}
	}
}

func (ka *KeepAlive) ping(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ka.url, nil)
	if err != nil {
		slog.Error("keep-alive: failed to build request",
			"error", err)
		return
	}

	resp, err := ka.client.Do(req)
	if err != nil {
		slog.Error("keep-alive: ping failed",
			"url", ka.url,
			"error", err)
		return
	}
	defer resp.Body.Close()

	slog.Info("keep-alive: ping successful",
		"url", ka.url,
		"status", resp.StatusCode)
}
