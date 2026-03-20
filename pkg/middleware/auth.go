package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"Pulsemon/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware validates JWT tokens from the Authorization header.
func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			slog.Warn("unauthorized request",
				"request_id", c.GetString(RequestIDKey),
				"path", c.Request.URL.Path,
				"ip", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			slog.Warn("unauthorized request",
				"request_id", c.GetString(RequestIDKey),
				"path", c.Request.URL.Path,
				"ip", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			return
		}

		tokenString := parts[1]

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Validate signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			slog.Warn("unauthorized request",
				"request_id", c.GetString(RequestIDKey),
				"path", c.Request.URL.Path,
				"ip", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			slog.Warn("unauthorized request",
				"request_id", c.GetString(RequestIDKey),
				"path", c.Request.URL.Path,
				"ip", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		userID, ok1 := claims["userID"].(string)
		email, ok2 := claims["email"].(string)
		roleID, ok3 := claims["roleID"].(string)

		if !ok1 || !ok2 || !ok3 {
			slog.Warn("unauthorized request",
				"request_id", c.GetString("requestID"), // Fix missing constant if needed
				"path", c.Request.URL.Path,
				"ip", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ApiResponse{
				Success: false,
				Message: "Unauthorized",
				Error:   "invalid token claims",
			})
			return
		}

		// Set values in Gin context
		c.Set("userID", userID)
		c.Set("email", email)
		c.Set("roleID", roleID)

		c.Next()
	}
}
