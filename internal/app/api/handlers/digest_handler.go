package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"rss-platform/internal/service"
)

var errDigestReaderRequired = errors.New("digest reader is not configured")

// DigestReader 定义最新日报读取所需的最小能力。
type DigestReader interface {
	LatestDigest(ctx context.Context) (service.DigestView, error)
}

// RegisterDigestRoutes 注册日报查询接口。
func RegisterDigestRoutes(group *gin.RouterGroup, svc DigestReader) {
	group.GET("/digests/latest", func(c *gin.Context) {
		if svc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": errDigestReaderRequired.Error()})
			return
		}

		payload, err := svc.LatestDigest(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, payload)
	})
}
