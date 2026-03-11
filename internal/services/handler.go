package services

import (
	"errors"
	"net/http"

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

// getUserID extracts the user identity from the X-User-ID header.
// Returns an empty string if the header is missing.
func getUserID(c *gin.Context) string {
	return c.GetHeader("X-User-ID")
}

// RegisterRoutes wires up all service-related routes on the given router.
func (h *ServiceHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/services", h.ListServices)
	router.POST("/services", h.CreateService)
	router.GET("/services/:id", h.GetService)
	router.PUT("/services/:id", h.UpdateService)
	router.DELETE("/services/:id", h.DeleteService)
}

// CreateService handles POST /services.
func (h *ServiceHandler) CreateService(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-User-ID header"})
		return
	}

	var input CreateServiceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	service, err := h.svc.CreateService(userID, input)
	if err != nil {
		switch {
		case errors.Is(err, ErrServiceLimitReached):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		case errors.Is(err, ErrInvalidURL), errors.Is(err, ErrInvalidInterval):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"service": ToCreateServiceResponse(*service)})
}

// ListServices handles GET /services.
func (h *ServiceHandler) ListServices(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-User-ID header"})
		return
	}

	services, err := h.svc.GetServices(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	summaries := make([]ServiceSummaryResponse, len(services))
	for i, s := range services {
		summaries[i] = ToServiceSummaryResponse(s)
	}

	c.JSON(http.StatusOK, gin.H{"services": summaries})
}

// GetService handles GET /services/:id.
func (h *ServiceHandler) GetService(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-User-ID header"})
		return
	}

	serviceID := c.Param("id")

	service, err := h.svc.GetServiceByID(serviceID, userID)
	if err != nil {
		if errors.Is(err, ErrServiceNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"service": ToServiceDetailResponse(*service)})
}

// UpdateService handles PUT /services/:id.
func (h *ServiceHandler) UpdateService(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-User-ID header"})
		return
	}

	serviceID := c.Param("id")

	var input UpdateServiceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.svc.UpdateService(serviceID, userID, input)
	if err != nil {
		switch {
		case errors.Is(err, ErrServiceNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrInvalidInterval):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Service updated successfully"})
}

// DeleteService handles DELETE /services/:id.
func (h *ServiceHandler) DeleteService(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-User-ID header"})
		return
	}

	serviceID := c.Param("id")

	err := h.svc.DeleteService(serviceID, userID)
	if err != nil {
		if errors.Is(err, ErrServiceNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Service deleted successfully"})
}
