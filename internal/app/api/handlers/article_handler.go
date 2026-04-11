package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"rss-platform/internal/service"
)

var errArticleReaderRequired = errors.New("article reader is not configured")

// ArticleReader 定义文章列表读取所需的最小能力。
type ArticleReader interface {
	ListArticles(ctx context.Context) ([]service.ArticleView, error)
}

// RegisterArticleRoutes 注册文章查询接口。
func RegisterArticleRoutes(group *gin.RouterGroup, svc ArticleReader) {
	group.GET("/articles", func(c *gin.Context) {
		if svc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": errArticleReaderRequired.Error()})
			return
		}

		items := []service.ArticleView{}
		loaded, err := svc.ListArticles(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		items = loaded

		c.JSON(http.StatusOK, gin.H{"items": items})
	})
}
