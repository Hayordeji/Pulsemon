package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"Pulsemon/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HealthHandler serves the public health check endpoint.
type HealthHandler struct {
	db        *gorm.DB
	startTime time.Time
}

// NewHealthHandler creates a HealthHandler that tracks uptime from now.
func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{
		db:        db,
		startTime: time.Now(),
	}
}

// RegisterRoutes wires up the health check route.
func (h *HealthHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/health", h.Check)
}

// Check handles GET /health.
func (h *HealthHandler) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	dbStatus := "ok"

	sqlDB, err := h.db.DB()
	if err != nil {
		slog.Warn("health check: database ping failed",
			"error", err)
		dbStatus = "unavailable"
	} else if err = sqlDB.PingContext(ctx); err != nil {
		slog.Warn("health check: database ping failed",
			"error", err)
		dbStatus = "unavailable"
	}

	uptime := time.Since(h.startTime).Round(time.Second).String()

	status := "ok"
	if dbStatus != "ok" {
		status = "degraded"
	}

	httpStatus := http.StatusOK
	if status != "ok" {
		httpStatus = http.StatusServiceUnavailable
	}

	slog.Info("health check completed",
		"status", status,
		"database", dbStatus,
		"uptime", uptime)

	c.JSON(httpStatus, models.ApiResponse{
		Success: status == "ok",
		Message: "health check",
		Data: gin.H{
			"status":   status,
			"database": dbStatus,
			"uptime":   uptime,
		},
	})
}
