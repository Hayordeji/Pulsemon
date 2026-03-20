package dashboard

import (
	"Pulsemon/pkg/middleware"
	"Pulsemon/pkg/models"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// DashboardHandler handles HTTP requests for the /dashboard endpoints.
type DashboardHandler struct {
	repo *DashboardRepository
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(repo *DashboardRepository) *DashboardHandler {
	return &DashboardHandler{repo: repo}
}

// RegisterRoutes wires up all dashboard-related routes on the given router.
func (h *DashboardHandler) RegisterRoutes(router *gin.RouterGroup, rateLimiter *middleware.RateLimiter) {
	router.GET("/dashboard/:service_id", rateLimiter.Read(), h.GetDashboard)
	router.GET("/dashboard/:service_id/alerts", rateLimiter.Read(), h.GetServiceAlerts)
}

// getUserID extracts the user identity from the X-User-ID header.
func getUserID(c *gin.Context) string {
	userID, exists := c.Get("userID")
	if !exists {
		return ""
	}
	return userID.(string)
}

// GetDashboard handles GET /dashboard/:service_id.
// @Summary      Get service health dashboard
// @Description  Returns full health overview including latency, SLA, SSL and recent probes
// @Tags         dashboard
// @Produce      json
// @Security     BearerAuth
// @Param        service_id  path      string  true  "Service ID"
// @Success      200         {object}  DashboardResponse
// @Failure      401         {object}  map[string]string
// @Failure      404         {object}  map[string]string
// @Router       /dashboard/{service_id} [get]
func (h *DashboardHandler) GetDashboard(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-User-ID header"})
		return
	}

	res := models.ApiResponse{
		Message: "Get Dashboard Failed",
		Success: false,
		Error:   "",
		Data:    nil,
	}

	serviceID := c.Param("service_id")
	pagination := ParsePaginationParams(c)

	// Verify service exists and belongs to this user.
	service, err := h.repo.FindServiceByIDAndUser(FindServiceInput{
		ServiceID: serviceID,
		UserID:    userID,
	})
	if err != nil {
		slog.Error("failed to fetch service for dashboard",
			"service_id", serviceID,
			"error", err)
		res.Error = err.Error()
		c.JSON(http.StatusInternalServerError, gin.H{"response": res})
		return
	}
	if service == nil {
		res.Error = "Service not found"
		c.JSON(http.StatusNotFound, gin.H{"response": res})
		return
	}

	// Fetch recent probe results with cursor pagination.
	results, err := h.repo.FindRecentProbeResults(FindProbeResultsInput{
		ServiceID: serviceID,
		Params:    pagination,
	})
	if err != nil {
		slog.Error("failed to fetch probe results for dashboard",
			"service_id", serviceID,
			"error", err)
		res.Error = err.Error()
		c.JSON(http.StatusInternalServerError, gin.H{"response": res})
		return
	}

	res.Message = "Dashboard retrieved successfully"
	res.Data = ToDashboardResponse(*service, results, pagination.Limit)
	res.Success = true
	c.JSON(http.StatusOK, gin.H{"response": res})
}

// GetServiceAlerts handles GET /dashboard/:service_id/alerts.
// @Summary      Get service alert history
// @Description  Returns all alerts sent for a specific service
// @Tags         dashboard
// @Produce      json
// @Security     BearerAuth
// @Param        service_id  path      string  true  "Service ID"
// @Success      200         {object}  ServiceAlertsResponse
// @Failure      401         {object}  map[string]string
// @Failure      404         {object}  map[string]string
// @Router       /dashboard/{service_id}/alerts [get]
func (h *DashboardHandler) GetServiceAlerts(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-User-ID header"})
		return
	}

	res := models.ApiResponse{
		Message: "Get Service Alerts Failed",
		Success: false,
		Error:   "",
		Data:    nil,
	}

	serviceID := c.Param("service_id")
	pagination := ParsePaginationParams(c)

	// Verify service exists and belongs to this user.
	service, err := h.repo.FindServiceByIDAndUser(FindServiceInput{
		ServiceID: serviceID,
		UserID:    userID,
	})
	if err != nil {
		slog.Error("failed to fetch service for alerts",
			"service_id", serviceID,
			"error", err)
		res.Error = err.Error()
		c.JSON(http.StatusInternalServerError, gin.H{"response": res})
		return
	}
	if service == nil {
		res.Error = "Service not found"
		c.JSON(http.StatusNotFound, gin.H{"response": res})
		return
	}

	// Fetch alerts with cursor pagination.
	alertsList, err := h.repo.FindAlertsByService(FindAlertsInput{
		ServiceID: serviceID,
		UserID:    userID,
		Params:    pagination,
	})
	if err != nil {
		slog.Error("failed to fetch alerts for service",
			"service_id", serviceID,
			"error", err)
		res.Error = err.Error()
		c.JSON(http.StatusInternalServerError, gin.H{"response": res})
		return
	}

	res.Message = "Service alerts retrieved successfully"
	res.Data = ToServiceAlertsResponse(serviceID, alertsList, pagination.Limit)
	res.Success = true
	c.JSON(http.StatusOK, gin.H{"response": res})
}
