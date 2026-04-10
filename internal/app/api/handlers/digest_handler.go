package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DigestReader 定义最新日报读取所需的最小能力。
type DigestReader interface {
	LatestDigest() map[string]any
}

// RegisterDigestRoutes 注册日报查询接口。
func RegisterDigestRoutes(group *gin.RouterGroup, svc DigestReader) {
	group.GET("/digests/latest", func(c *gin.Context) {
		payload := gin.H{}
		if svc != nil {
			payload = gin.H(svc.LatestDigest())
		}

		c.JSON(http.StatusOK, payload)
	})
}
