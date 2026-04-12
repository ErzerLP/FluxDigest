package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"rss-platform/internal/service"
)

var errJobTriggerRequired = errors.New("job trigger is required")
var errJobForceTriggerUnsupported = errors.New("force daily digest trigger is not supported")

// JobTrigger 定义手动触发日报任务所需的最小能力。
type JobTrigger interface {
	TriggerDailyDigest(ctx context.Context, now time.Time) (service.JobTriggerResult, error)
	TriggerArticleReprocess(ctx context.Context, articleID string, force bool) (service.JobTriggerResult, error)
}

// DailyDigestForceTrigger 定义支持 force 触发日报任务的可选能力。
type DailyDigestForceTrigger interface {
	TriggerDailyDigestWithOptions(ctx context.Context, now time.Time, opts service.DailyDigestTriggerOptions) (service.JobTriggerResult, error)
}

type triggerDailyDigestRequest struct {
	TriggerAt string `json:"trigger_at"`
	Force     bool   `json:"force"`
}

type triggerArticleReprocessRequest struct {
	ArticleID string `json:"article_id"`
	Force     bool   `json:"force"`
}

// RegisterJobRoutes 注册任务触发接口。
func RegisterJobRoutes(group *gin.RouterGroup, svc JobTrigger) {
	group.POST("/jobs/daily-digest", func(c *gin.Context) {
		if svc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": errJobTriggerRequired.Error()})
			return
		}

		triggerAt := time.Now()
		req := triggerDailyDigestRequest{}
		if c.Request.ContentLength > 0 {
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if req.TriggerAt != "" {
				parsed, err := time.Parse(time.RFC3339, req.TriggerAt)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid trigger_at"})
					return
				}
				triggerAt = parsed
			}
		}

		var (
			result service.JobTriggerResult
			err    error
		)
		if req.Force {
			forceTrigger, ok := svc.(DailyDigestForceTrigger)
			if !ok {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": errJobForceTriggerUnsupported.Error()})
				return
			}
			result, err = forceTrigger.TriggerDailyDigestWithOptions(c.Request.Context(), triggerAt, service.DailyDigestTriggerOptions{Force: true})
		} else {
			result, err = svc.TriggerDailyDigest(c.Request.Context(), triggerAt)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		statusCode := http.StatusAccepted
		if result.Status == "skipped" {
			statusCode = http.StatusOK
		}
		if result.Status == "" {
			result.Status = "accepted"
		}

		c.JSON(statusCode, gin.H{
			"digest_date": result.DigestDate,
			"status":      result.Status,
			"trigger_at":  triggerAt.Format(time.RFC3339),
			"force":       req.Force,
		})
	})

	group.POST("/jobs/article-reprocess", func(c *gin.Context) {
		if svc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": errJobTriggerRequired.Error()})
			return
		}

		var req triggerArticleReprocessRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.ArticleID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "article_id is required"})
			return
		}

		result, err := svc.TriggerArticleReprocess(c.Request.Context(), req.ArticleID, req.Force)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if result.Status == "" {
			result.Status = "accepted"
		}

		c.JSON(http.StatusAccepted, gin.H{
			"article_id": result.ArticleID,
			"status":     result.Status,
			"force":      req.Force,
		})
	})
}
