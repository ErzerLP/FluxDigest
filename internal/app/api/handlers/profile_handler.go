package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"rss-platform/internal/service"
)

var errProfileReaderRequired = errors.New("profile reader is not configured")

// ProfileReader 定义活动配置读取所需的最小能力。
type ProfileReader interface {
	ActiveProfile(ctx context.Context, profileType string) (service.ProfileView, error)
}

// RegisterProfileRoutes 注册配置查询接口。
func RegisterProfileRoutes(group *gin.RouterGroup, svc ProfileReader) {
	group.GET("/profiles/:profileType/active", func(c *gin.Context) {
		if svc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": errProfileReaderRequired.Error()})
			return
		}

		profileType := c.Param("profileType")
		payload, err := svc.ActiveProfile(c.Request.Context(), profileType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, payload)
	})
}
