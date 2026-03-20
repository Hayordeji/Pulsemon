package middleware

import (
	"log/slog"
	"net/http"

	"Pulsemon/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// VerifiedMiddleware ensures the user has completed email verification.
// It assumes that AuthMiddleware has already run and set 'userID' in the context.
func VerifiedMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userID")
		if !exists {
			slog.Warn("verified middleware called without user id context",
				"path", c.Request.URL.Path)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var user models.User
		err := db.WithContext(c.Request.Context()).Select("is_verified").Where("id = ?", userID).First(&user).Error
		if err != nil {
			slog.Error("failed to verify user status in middleware",
				"user_id", userID,
				"error", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		if !user.IsVerified {
			slog.Warn("unverified user attempted to access protected route",
				"user_id", userID,
				"path", c.Request.URL.Path)
			c.AbortWithStatusJSON(http.StatusForbidden, models.ApiResponse{
				Success: false,
				Message: "Email verification required",
				Error:   "Please verify your email address to access this resource",
			})
			return
		}

		c.Next()
	}
}
