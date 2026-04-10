package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequireAPIKey 校验 X-API-Key，未配置期望值时直接放行。
func RequireAPIKey(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(expected) == "" {
			c.Next()
			return
		}

		if c.GetHeader("X-API-Key") != expected {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			return
		}

		c.Next()
	}
}
