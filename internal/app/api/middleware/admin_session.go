package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"rss-platform/internal/service"
)

const adminSessionContextKey = "admin_session_user"
const adminAuthUnavailableMessage = "authentication service unavailable"

type AdminSessionReader interface {
	CurrentUser(ctx context.Context, sessionID string) (service.LoginResult, error)
}

type AdminSessionOptions struct {
	CookieName string
}

func RequireAdminSession(reader AdminSessionReader, opts AdminSessionOptions) gin.HandlerFunc {
	cookieName := opts.CookieName
	if cookieName == "" {
		cookieName = service.DefaultAdminSessionCookieName
	}

	return func(c *gin.Context) {
		if reader == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": adminAuthUnavailableMessage})
			return
		}

		sessionID, err := c.Cookie(cookieName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "admin session required"})
			return
		}

		user, err := reader.CurrentUser(c.Request.Context(), sessionID)
		if err != nil {
			if errors.Is(err, service.ErrAdminSessionNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "admin session required"})
				return
			}
			log.Printf("admin session lookup failed: %v", err)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": adminAuthUnavailableMessage})
			return
		}

		c.Set(adminSessionContextKey, user)
		c.Next()
	}
}

func CurrentAdminUser(c *gin.Context) (service.LoginResult, bool) {
	if c == nil {
		return service.LoginResult{}, false
	}
	value, ok := c.Get(adminSessionContextKey)
	if !ok {
		return service.LoginResult{}, false
	}
	user, ok := value.(service.LoginResult)
	return user, ok
}
