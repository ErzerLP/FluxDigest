package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"rss-platform/internal/app/api/middleware"
	"rss-platform/internal/service"
)

const adminAuthUnavailableMessage = "authentication service unavailable"

type AdminAuthenticator interface {
	Login(ctx context.Context, input service.LoginInput) (service.LoginResult, string, error)
	Logout(ctx context.Context, sessionID string) error
	CurrentUser(ctx context.Context, sessionID string) (service.LoginResult, error)
	SessionTTL() time.Duration
}

type AdminAuthDeps struct {
	Auth         AdminAuthenticator
	CookieName   string
	CookiePath   string
	CookieDomain string
	CookieSecure bool
	SessionAuth  gin.HandlerFunc
}

func RegisterAdminAuthRoutes(group *gin.RouterGroup, deps AdminAuthDeps) {
	auth := adminAuthRouteGroup(group)
	cookieName := deps.CookieName
	if cookieName == "" {
		cookieName = service.DefaultAdminSessionCookieName
	}
	cookiePath := deps.CookiePath
	if cookiePath == "" {
		cookiePath = "/"
	}

	auth.POST("/login", func(c *gin.Context) {
		if deps.Auth == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": adminAuthUnavailableMessage})
			return
		}

		var input service.LoginInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		result, sessionID, err := deps.Auth.Login(c.Request.Context(), input)
		if err != nil {
			if errors.Is(err, service.ErrInvalidAdminCredentials) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": service.ErrInvalidAdminCredentials.Error()})
				return
			}
			log.Printf("admin login failed: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": adminAuthUnavailableMessage})
			return
		}

		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(cookieName, sessionID, int(deps.Auth.SessionTTL().Seconds()), cookiePath, deps.CookieDomain, shouldUseSecureCookie(c.Request, deps.CookieSecure), true)
		c.JSON(http.StatusOK, result)
	})

	protected := auth.Group("")
	if deps.SessionAuth != nil {
		protected.Use(deps.SessionAuth)
	}

	protected.GET("/me", func(c *gin.Context) {
		if user, ok := middleware.CurrentAdminUser(c); ok {
			c.JSON(http.StatusOK, user)
			return
		}
		current, err := currentAdminFromCookie(c, deps.Auth, cookieName)
		if err != nil {
			respondAdminAuthError(c, err)
			return
		}
		c.JSON(http.StatusOK, current)
	})

	protected.POST("/logout", func(c *gin.Context) {
		if deps.Auth == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": adminAuthUnavailableMessage})
			return
		}

		sessionID, _ := c.Cookie(cookieName)
		if err := deps.Auth.Logout(c.Request.Context(), sessionID); err != nil {
			log.Printf("admin logout failed: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": adminAuthUnavailableMessage})
			return
		}

		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(cookieName, "", -1, cookiePath, deps.CookieDomain, shouldUseSecureCookie(c.Request, deps.CookieSecure), true)
		c.Status(http.StatusNoContent)
	})
}

func currentAdminFromCookie(c *gin.Context, auth AdminAuthenticator, cookieName string) (service.LoginResult, error) {
	if auth == nil {
		return service.LoginResult{}, errors.New("admin auth service is not configured")
	}
	sessionID, err := c.Cookie(cookieName)
	if err != nil {
		return service.LoginResult{}, service.ErrAdminSessionNotFound
	}
	return auth.CurrentUser(c.Request.Context(), sessionID)
}

func respondAdminAuthError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrAdminSessionNotFound) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "admin session required"})
		return
	}
	log.Printf("admin auth route failed: %v", err)
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": adminAuthUnavailableMessage})
}

func adminAuthRouteGroup(group *gin.RouterGroup) *gin.RouterGroup {
	if strings.HasSuffix(group.BasePath(), "/admin/auth") {
		return group
	}
	return group.Group("/admin/auth")
}

func shouldUseSecureCookie(req *http.Request, force bool) bool {
	if force {
		return true
	}
	if req == nil {
		return false
	}
	if req.TLS != nil {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(req.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(req.Header.Get("X-Forwarded-Scheme")), "https") {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(req.Header.Get("X-Forwarded-Ssl")), "on")
}
