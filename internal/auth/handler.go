package auth

import (
	"Pulsemon/pkg/middleware"
	"Pulsemon/pkg/models"
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
func (h *AuthHandler) RegisterRoutes(router *gin.RouterGroup, rateLimiter *middleware.RateLimiter, jwtSecret string) {
	router.POST("/auth/register", rateLimiter.AuthStrict(), h.Register)
	router.POST("/auth/login", rateLimiter.AuthStrict(), h.Login)
	router.POST("/auth/verify", rateLimiter.AuthStrict(), h.VerifyEmail)
	router.POST("/auth/resend-verify", rateLimiter.Global(), h.ResendVerification)
	router.POST("/auth/forgot-password", h.ForgotPassword)
	router.POST("/auth/reset-password", h.ResetPassword)
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

	res := models.ApiResponse{
		Message: "Registration failed",
		Success: false,
		Error:   "",
		Data:    nil,
	}

	err := h.svc.Register(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyExists) {
			res.Error = err.Error()
			c.JSON(http.StatusConflict, gin.H{"response": res})
			return
		}
		if errors.Is(err, ErrUsernameAlreadyExists) {
			res.Error = err.Error()
			c.JSON(http.StatusConflict, gin.H{"response": res})
			return
		}
		if err.Error() == "invalid email format" {
			res.Error = err.Error()
			c.JSON(http.StatusBadRequest, gin.H{"response": res})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	res.Success = true
	res.Message = "Registration successful"
	c.JSON(http.StatusCreated, gin.H{"response": res})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	res := models.ApiResponse{
		Message: "Login failed",
		Success: false,
		Error:   "",
		Data:    nil,
	}

	token, err := h.svc.Login(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			res.Error = err.Error()
			c.JSON(http.StatusUnauthorized, res)
			return
		}
		if errors.Is(err, ErrUserIsNotVerified) {
			res.Error = err.Error()
			c.JSON(http.StatusUnauthorized, res)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	res.Success = true
	res.Message = "Login successful"
	res.Data = token
	c.JSON(http.StatusOK, res)
}

// VerifyEmail handles POST /auth/verify.
// @Summary      Verify user email
// @Description  Verifies a user's email using the token sent to their inbox
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        token query string false "Verification token"
// @Param        body body VerifyEmailInput false "Verification token (alternative to query param)"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}
// @Router       /auth/verify [post]
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	user_id := c.Query("user_id")

	if token == "" && user_id == "" {
		var input VerifyEmailInput
		if err := c.ShouldBindJSON(&input); err == nil {
			token = input.Token
			user_id = input.UserID
		}
	}

	if token == "" && user_id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user_id or verification token"})
		return
	}

	res := models.ApiResponse{
		Message: "Verification failed",
		Success: false,
	}

	err := h.svc.VerifyEmail(c.Request.Context(), VerifyEmailInput{Token: token})
	if err != nil {
		if errors.Is(err, ErrInvalidOrExpiredToken) {
			res.Error = err.Error()
			c.JSON(http.StatusBadRequest, gin.H{"response": res})
			return
		}
		if errors.Is(err, ErrAlreadyVerified) {
			res.Error = err.Error()
			res.Message = "Email is already verified"
			c.JSON(http.StatusConflict, gin.H{"response": res})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	res.Success = true
	res.Message = "Email verified successfully"
	c.JSON(http.StatusOK, gin.H{"response": res})
}

// ResendVerification handles POST /auth/resend-verify.
// @Summary      Resend verification email
// @Description  Generates a new verification token and resends the email
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Param		body ResendVerificationInput true "Resend verification email"
// @Success      200  {object}  map[string]interface{}
// @Failure      401  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}
// @Router       /auth/resend-verify [post]
func (h *AuthHandler) ResendVerification(c *gin.Context) {
	var input ResendVerificationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	res := models.ApiResponse{
		Message: "Failed to resend verification email",
		Success: false,
	}

	err := h.svc.ResendVerification(c.Request.Context(), ResendVerificationInput{
		Email: input.Email,
	})

	if err != nil {
		if errors.Is(err, ErrAlreadyVerified) {
			res.Message = "Email is already verified"
			res.Success = true
			c.JSON(http.StatusOK, gin.H{"response": res})
			return
		}
		if err.Error() == "user not found" {
			res.Error = err.Error()
			c.JSON(http.StatusUnauthorized, gin.H{"response": res})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	res.Success = true
	res.Message = "Verification email sent successfully"
	c.JSON(http.StatusOK, gin.H{"response": res})
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ForgotPassword handles POST /auth/forgot-password.
// @Summary      Request password reset
// @Description  Sends a password reset email if the account exists
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body ForgotPasswordRequest true "Forgot Password details"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	err := h.svc.ForgotPassword(c.Request.Context(), ForgotPasswordInput{Email: req.Email})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, models.ApiResponse{
		Success: true,
		Message: "If that email is registered you will receive a reset link shortly",
	})
}

type ResetPasswordRequest struct {
	UserID      string `json:"user_id"`
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// ResetPassword handles POST /auth/reset-password.
// @Summary      Reset password
// @Description  Resets a user password using a valid token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        token query string false "Reset token"
// @Param        user_id query string false "User ID"
// @Param        body body ResetPasswordRequest true "Reset Password details"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	token := c.Query("token")
	userID := c.Query("user_id")

	var req ResetPasswordRequest

	res := models.ApiResponse{
		Success: false,
		Message: "Invalid Request",
	}

	if err := c.ShouldBindJSON(&req); err == nil {
		if token == "" {
			token = req.Token
		}
		if userID == "" {
			userID = req.UserID
		}
	} else if token == "" || userID == "" {
		res.Error = "invalid request body"
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if token == "" || userID == "" {
		res.Error = "token and user_id are required"
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if req.NewPassword == "" {
		res.Error = "new_password is required"
		c.JSON(http.StatusBadRequest, res)
		return
	}

	err := h.svc.ResetPassword(c.Request.Context(), ResetPasswordInput{
		UserID:      userID,
		Token:       token,
		NewPassword: req.NewPassword,
	})

	if err != nil {
		if errors.Is(err, ErrInvalidOrExpiredResetToken) || errors.Is(err, ErrInvalidNewPassword) {
			c.JSON(http.StatusBadRequest, models.ApiResponse{
				Success: false,
				Message: "Password reset failed",
				Error:   err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, models.ApiResponse{
		Success: true,
		Message: "Password reset successfully",
	})
}
