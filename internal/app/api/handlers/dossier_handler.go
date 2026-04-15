package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"rss-platform/internal/service"
)

var errDossierReaderRequired = errors.New("dossier reader is not configured")

// DossierReader 定义 dossier 查询所需的最小能力。
type DossierReader interface {
	ListDossiers(ctx context.Context, filter service.DossierListFilter) ([]service.DossierListItem, error)
	GetDossier(ctx context.Context, dossierID string) (service.DossierDetail, error)
}

// RegisterDossierRoutes 注册 dossier 查询接口。
func RegisterDossierRoutes(group *gin.RouterGroup, svc DossierReader) {
	group.GET("/dossiers", func(c *gin.Context) {
		if svc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": errDossierReaderRequired.Error()})
			return
		}

		limit := 20
		if value := c.Query("limit"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 || parsed > 100 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
				return
			}
			limit = parsed
		}

		items, err := svc.ListDossiers(c.Request.Context(), service.DossierListFilter{Limit: limit})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"items": items})
	})

	group.GET("/dossiers/:id", func(c *gin.Context) {
		if svc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": errDossierReaderRequired.Error()})
			return
		}

		item, err := svc.GetDossier(c.Request.Context(), c.Param("id"))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "dossier not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, item)
	})
}
