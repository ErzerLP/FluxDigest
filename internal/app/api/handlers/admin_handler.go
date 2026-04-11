package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"rss-platform/internal/domain/profile"
	"rss-platform/internal/service"
)

var errAdminStatusReaderRequired = errors.New("admin status reader is not configured")
var errAdminConfigReaderRequired = errors.New("admin config reader is not configured")
var errAdminLLMUpdaterRequired = errors.New("admin llm updater is not configured")
var errAdminLLMTesterRequired = errors.New("admin llm tester is not configured")
var errAdminJobReaderRequired = errors.New("admin job reader is not configured")

// AdminStatusReader 定义 dashboard 状态读取能力。
type AdminStatusReader interface {
	GetStatus(ctx context.Context) (service.AdminStatusView, error)
}

// AdminConfigReader 定义管理员配置读取能力。
type AdminConfigReader interface {
	GetSnapshot(ctx context.Context) (service.AdminConfigSnapshot, error)
}

// AdminLLMUpdater 定义管理员 LLM 配置更新能力。
type AdminLLMUpdater interface {
	UpdateLLM(ctx context.Context, input service.UpdateLLMConfigInput) (profile.Version, error)
}

// AdminLLMTester 定义管理员 LLM 连通性测试能力。
type AdminLLMTester interface {
	TestLLM(ctx context.Context, draft service.LLMTestDraft) (service.ConnectivityTestResult, error)
}

// AdminJobReader 定义管理员作业列表读取能力。
type AdminJobReader interface {
	ListLatest(ctx context.Context, filter service.JobRunListFilter) ([]service.JobRunRecord, error)
}

// AdminDeps 定义 admin handler 依赖。
type AdminDeps struct {
	Status     AdminStatusReader
	Configs    AdminConfigReader
	LLMUpdater AdminLLMUpdater
	LLMTester  AdminLLMTester
	Jobs       AdminJobReader
}

type profileVersionResponse struct {
	ID          string `json:"id"`
	ProfileType string `json:"profile_type"`
	Name        string `json:"name"`
	Version     int    `json:"version"`
	IsActive    bool   `json:"is_active"`
}

// RegisterAdminRoutes 注册管理后台接口。
func RegisterAdminRoutes(group *gin.RouterGroup, deps AdminDeps) {
	admin := group.Group("/admin")
	admin.GET("/status", func(c *gin.Context) {
		if deps.Status == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": errAdminStatusReaderRequired.Error()})
			return
		}

		status, err := deps.Status.GetStatus(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, status)
	})

	admin.GET("/configs", func(c *gin.Context) {
		if deps.Configs == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": errAdminConfigReaderRequired.Error()})
			return
		}

		snapshot, err := deps.Configs.GetSnapshot(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, snapshot)
	})

	admin.PUT("/configs/llm", func(c *gin.Context) {
		if deps.LLMUpdater == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": errAdminLLMUpdaterRequired.Error()})
			return
		}

		var req service.UpdateLLMConfigInput
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		version, err := deps.LLMUpdater.UpdateLLM(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, toProfileVersionResponse(version))
	})

	admin.POST("/test/llm", func(c *gin.Context) {
		if deps.LLMTester == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": errAdminLLMTesterRequired.Error()})
			return
		}

		var req service.LLMTestDraft
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		result, err := deps.LLMTester.TestLLM(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "result": result})
			return
		}

		c.JSON(http.StatusOK, result)
	})

	admin.GET("/jobs", func(c *gin.Context) {
		if deps.Jobs == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": errAdminJobReaderRequired.Error()})
			return
		}

		limit := 20
		if value := c.Query("limit"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
				return
			}
			if parsed <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
				return
			}
			limit = parsed
		}

		runs, err := deps.Jobs.ListLatest(c.Request.Context(), service.JobRunListFilter{Limit: limit})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"items": runs})
	})
}

func toProfileVersionResponse(version profile.Version) profileVersionResponse {
	return profileVersionResponse{
		ID:          version.ID,
		ProfileType: version.ProfileType,
		Name:        version.Name,
		Version:     version.Version,
		IsActive:    version.IsActive,
	}
}
