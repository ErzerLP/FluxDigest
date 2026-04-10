package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ProfileReader 定义活动配置读取所需的最小能力。
type ProfileReader interface {
	ActiveProfile(profileType string) map[string]any
}

// RegisterProfileRoutes 注册配置查询接口。
func RegisterProfileRoutes(group *gin.RouterGroup, svc ProfileReader) {
	group.GET("/profiles/:profileType/active", func(c *gin.Context) {
		profileType := c.Param("profileType")
		payload := gin.H{"profile_type": profileType}
		if svc != nil {
			payload = gin.H(svc.ActiveProfile(profileType))
		}

		c.JSON(http.StatusOK, payload)
	})
}
