package middleware

import (
	"log/slog"
	"strings"
	"time"

	"Pulsemon/pkg/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// NewCORSMiddleware creates a CORS middleware configured from the application config.
func NewCORSMiddleware(cfg config.Config) gin.HandlerFunc {
	allowedOrigins := []string{"http://localhost:3000"}

	if cfg.AllowedOrigins != "" {
		parts := strings.Split(cfg.AllowedOrigins, ",")
		origins := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
		if len(origins) > 0 {
			allowedOrigins = origins
		}
	}

	if cfg.AppEnv == "production" && cfg.AllowedOrigins == "" {
		slog.Warn("ALLOWED_ORIGINS not set in production",
			"defaulting_to", "http://localhost:3000")
	}

	slog.Info("CORS configured",
		"allowed_origins", allowedOrigins)

	return cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
		ExposeHeaders:    []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
