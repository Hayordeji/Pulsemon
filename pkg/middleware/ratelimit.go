package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"Pulsemon/pkg/models"

	"github.com/gin-gonic/gin"
	limiter "github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// RateLimiter provides tiered rate limiting middleware for Gin routes.
type RateLimiter struct {
	store limiter.Store
}

// NewRateLimiter creates a RateLimiter backed by an in-memory store.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		store: memory.NewStore(),
	}
}

// newMiddleware creates a Gin middleware for the given rate.
func (rl *RateLimiter) newMiddleware(rate limiter.Rate) gin.HandlerFunc {
	instance := limiter.New(rl.store, rate)
	middleware := mgin.NewMiddleware(instance,
		mgin.WithLimitReachedHandler(func(c *gin.Context) {
			slog.Warn("rate limit exceeded",
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path)

			c.JSON(http.StatusTooManyRequests, models.ApiResponse{
				Success: false,
				Message: "Too many requests",
				Error:   "Rate limit exceeded, please try again later",
			})
			c.Abort()
		}),
	)
	return middleware
}

// AuthStrict returns middleware limiting auth endpoints to 10 req/min per IP.
func (rl *RateLimiter) AuthStrict() gin.HandlerFunc {
	return rl.newMiddleware(limiter.Rate{Period: 1 * time.Minute, Limit: 10})
}

// Write returns middleware limiting write endpoints to 30 req/min per IP.
func (rl *RateLimiter) Write() gin.HandlerFunc {
	return rl.newMiddleware(limiter.Rate{Period: 1 * time.Minute, Limit: 30})
}

// Read returns middleware limiting read endpoints to 60 req/min per IP.
func (rl *RateLimiter) Read() gin.HandlerFunc {
	return rl.newMiddleware(limiter.Rate{Period: 1 * time.Minute, Limit: 60})
}

// Global returns middleware limiting all endpoints to 100 req/min per IP.
func (rl *RateLimiter) Global() gin.HandlerFunc {
	return rl.newMiddleware(limiter.Rate{Period: 1 * time.Minute, Limit: 100})
}
