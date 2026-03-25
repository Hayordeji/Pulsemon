package services

import (
	"Pulsemon/pkg/middleware"
	"Pulsemon/pkg/models"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ServiceHandler handles HTTP requests for the /services endpoints.
type ServiceHandler struct {
	svc *ServiceService
}

// NewServiceHandler creates a new ServiceHandler.
func NewServiceHandler(svc *ServiceService) *ServiceHandler {
	return &ServiceHandler{svc: svc}
}

// getUserID extracts the user identity from the Gin context (set by JWT middleware).
// Returns an empty string if the user identity is missing.
func getUserID(c *gin.Context) string {
	userID, exists := c.Get("userID")
	if !exists {
		return ""
	}
	return userID.(string)
}

// RegisterRoutes wires up all service-related routes on the given router.
func (h *ServiceHandler) RegisterRoutes(router *gin.RouterGroup, rateLimiter *middleware.RateLimiter) {
	router.GET("/services", rateLimiter.Read(), h.ListServices)
	router.POST("/services", rateLimiter.Write(), h.CreateService)
	router.GET("/services/:id", rateLimiter.Read(), h.GetService)
	router.PUT("/services/:id", rateLimiter.Write(), h.UpdateService)
	router.DELETE("/services/:id", rateLimiter.Write(), h.DeleteService)
}

// CreateService handles POST /services.
// @Summary      Create a new service
// @Description  Registers a new HTTP/HTTPS endpoint to monitor
// @Tags         services
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body CreateServiceInput true "Service configuration"
// @Success      201  {object}  map[string]CreateServiceResponse
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      429  {object}  map[string]string
// @Router       /services [post]
func (h *ServiceHandler) CreateService(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-User-ID header"})
		return
	}

	res := models.ApiResponse{
		Message: "Create Service Failed",
		Success: false,
		Error:   "",
		Data:    nil,
	}

	var input CreateServiceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		res.Error = err.Error()
		c.JSON(http.StatusBadRequest, gin.H{"response": res})
		return
	}

	service, err := h.svc.CreateService(userID, input)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "already exists"):
			c.JSON(http.StatusConflict, models.ApiResponse{
				Success: false,
				Message: "Conflict",
				Error:   err.Error(),
			})
		case errors.Is(err, ErrServiceLimitReached):
			res.Error = err.Error()
			c.JSON(http.StatusTooManyRequests, gin.H{"response": res})
		case errors.Is(err, ErrInvalidURL), errors.Is(err, ErrInvalidInterval):
			res.Error = err.Error()
			c.JSON(http.StatusBadRequest, gin.H{"response": res})
		default:
			res.Error = err.Error()
			c.JSON(http.StatusInternalServerError, gin.H{"response": res})
		}
		return
	}

	res.Message = "Service created successfully"
	res.Success = true
	res.Data = &service
	c.JSON(http.StatusCreated, gin.H{"response": res})
}

// ListServices handles GET /services.
// @Summary      List all services
// @Description  Returns a summary list of all services for the logged-in user
// @Tags         services
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  map[string][]ServiceSummaryResponse
// @Failure      401  {object}  map[string]string
// @Router       /services [get]
func (h *ServiceHandler) ListServices(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-User-ID header"})
		return
	}

	res := models.ApiResponse{
		Message: "Get Services Failed",
		Success: false,
		Error:   "",
		Data:    nil,
	}

	services, err := h.svc.GetServices(userID)
	if err != nil {
		res.Error = err.Error()
		c.JSON(http.StatusInternalServerError, gin.H{"response": res})
		return
	}

	// summaries := make([]ServiceSummaryResponse, len(services))
	// for i, s := range services {
	// 	summaries[i] = ToServiceSummaryResponse(s)
	// }

	res.Success = true
	res.Message = "Services retrieved successfully"
	res.Data = services
	c.JSON(http.StatusOK, gin.H{"response": res})
}

// GetService handles GET /services/:id.
// @Summary      Get service details
// @Description  Returns full details of a single service including latency and SSL info
// @Tags         services
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Service ID"
// @Success      200  {object}  map[string]ServiceDetailResponse
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /services/{id} [get]
func (h *ServiceHandler) GetService(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	res := models.ApiResponse{
		Message: "Get Service Failed",
		Success: false,
		Error:   "",
		Data:    nil,
	}

	serviceID := c.Param("id")

	service, err := h.svc.GetServiceByID(serviceID, userID)
	if err != nil {
		if errors.Is(err, ErrServiceNotFound) {
			res.Error = err.Error()
			c.JSON(http.StatusNotFound, gin.H{"response": res})
			return
		}
		res.Error = err.Error()
		c.JSON(http.StatusInternalServerError, gin.H{"response": res})
		return
	}

	res.Success = true
	res.Message = "Service retrieved successfully"
	res.Data = ToServiceDetailResponse(*service)
	c.JSON(http.StatusOK, gin.H{"response": res})
}

// UpdateService handles PUT /services/:id.
// @Summary      Update a service
// @Description  Updates service configuration. URL cannot be changed.
// @Tags         services
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string             true  "Service ID"
// @Param        body body      UpdateServiceInput true  "Updated configuration"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /services/{id} [put]
func (h *ServiceHandler) UpdateService(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-User-ID header"})
		return
	}

	res := models.ApiResponse{
		Message: "Update Service Failed",
		Success: false,
		Error:   "",
		Data:    nil,
	}

	serviceID := c.Param("id")

	var input UpdateServiceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		res.Error = err.Error()
		c.JSON(http.StatusBadRequest, gin.H{"response": res})
		return
	}

	err := h.svc.UpdateService(serviceID, userID, input)
	if err != nil {
		switch {
		case errors.Is(err, ErrServiceNotFound):
			res.Error = err.Error()
			c.JSON(http.StatusNotFound, gin.H{"response": res})
		case errors.Is(err, ErrInvalidInterval):
			res.Error = err.Error()
			c.JSON(http.StatusBadRequest, gin.H{"response": res})
		default:
			res.Error = err.Error()
			c.JSON(http.StatusInternalServerError, gin.H{"response": res})
		}
		return
	}

	res.Message = "Service updated successfully"
	res.Success = true
	c.JSON(http.StatusOK, gin.H{"response": res})
}

// DeleteService handles DELETE /services/:id.
// @Summary      Delete a service
// @Description  Permanently deletes a service and all its probe history
// @Tags         services
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Service ID"
// @Success      200  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /services/{id} [delete]
func (h *ServiceHandler) DeleteService(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-User-ID header"})
		return
	}

	res := models.ApiResponse{
		Message: "Delete Service Failed",
		Success: false,
		Error:   "",
		Data:    nil,
	}

	serviceID := c.Param("id")

	err := h.svc.DeleteService(serviceID, userID)
	if err != nil {
		if errors.Is(err, ErrServiceNotFound) {
			res.Error = err.Error()
			c.JSON(http.StatusNotFound, gin.H{"response": res})
			return
		}
		res.Error = err.Error()
		c.JSON(http.StatusInternalServerError, gin.H{"response": res})
		return
	}

	res.Message = "Service deleted successfully"
	res.Success = true
	c.JSON(http.StatusOK, gin.H{"response": res})
}
