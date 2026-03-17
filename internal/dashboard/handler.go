package dashboard

import (
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
func (h *DashboardHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/dashboard/:service_id", h.GetDashboard)
	router.GET("/dashboard/:service_id/alerts", h.GetServiceAlerts)
}

// getUserID extracts the user identity from the X-User-ID header.
func getUserID(c *gin.Context) string {
	return c.GetHeader("X-User-ID")
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if service == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, ToDashboardResponse(*service, results, pagination.Limit))
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if service == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, ToServiceAlertsResponse(serviceID, alertsList, pagination.Limit))
}
