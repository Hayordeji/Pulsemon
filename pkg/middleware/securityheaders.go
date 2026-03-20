package middleware

import (
	"Pulsemon/pkg/config"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders sets strict security headers on all responses to harden the API.
func SecurityHeaders(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "0")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Header("Permissions-Policy", "geolocation=(), camera=(), microphone=()")

		if cfg.AppEnv == "production" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}
