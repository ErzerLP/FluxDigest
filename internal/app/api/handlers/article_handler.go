package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ArticleReader 定义文章列表读取所需的最小能力。
type ArticleReader interface {
	ListArticles() []map[string]any
}

// RegisterArticleRoutes 注册文章查询接口。
func RegisterArticleRoutes(group *gin.RouterGroup, svc ArticleReader) {
	group.GET("/articles", func(c *gin.Context) {
		items := []map[string]any{}
		if svc != nil {
			items = svc.ListArticles()
		}

		c.JSON(http.StatusOK, gin.H{"items": items})
	})
}
