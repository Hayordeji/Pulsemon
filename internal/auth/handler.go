package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthHandler handles HTTP requests for the /auth endpoints.
type AuthHandler struct {
	svc *AuthService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(svc *AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// RegisterRoutes wires up all auth-related routes on the given router.
func (h *AuthHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/auth/register", h.Register)
	router.POST("/auth/login", h.Login)
}

// Register handles POST /auth/register.
// @Summary      Register a new user account
// @Description  Creates a new user account with email and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body RegisterInput true "Registration details"
// @Success      201  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {

	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	err := h.svc.Register(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "invalid email format" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Registration successful"})
}

// Login handles POST /auth/login.
// @Summary      Login
// @Description  Authenticates a user and returns a JWT token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body LoginInput true "Login credentials"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	token, err := h.svc.Login(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}
