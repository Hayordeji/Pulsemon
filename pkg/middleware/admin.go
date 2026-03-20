package middleware

import (
	"log/slog"
	"net/http"

	"Pulsemon/pkg/models"
	"Pulsemon/pkg/roles"

	"github.com/gin-gonic/gin"
)

func AdminOnly(registry *roles.RoleRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleID := c.GetString("roleID")

		if !registry.IsAdmin(roleID) {
			slog.Warn("admin access denied",
				"request_id", c.GetString("requestID"),
				"role_id", roleID,
				"path", c.Request.URL.Path,
				"ip", c.ClientIP())

			c.AbortWithStatusJSON(http.StatusForbidden, models.ApiResponse{
				Success: false,
				Message: "Forbidden",
				Error:   "admin access required",
			})
			return
		}

		c.Next()
	}
}
