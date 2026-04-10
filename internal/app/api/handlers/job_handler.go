package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var errJobTriggerRequired = errors.New("job trigger is required")

// JobTrigger 定义手动触发日报任务所需的最小能力。
type JobTrigger interface {
	TriggerDailyDigest(ctx context.Context, now time.Time) error
}

type triggerDailyDigestRequest struct {
	TriggerAt string `json:"trigger_at"`
}

// RegisterJobRoutes 注册任务触发接口。
func RegisterJobRoutes(group *gin.RouterGroup, svc JobTrigger) {
	group.POST("/jobs/daily-digest", func(c *gin.Context) {
		if svc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": errJobTriggerRequired.Error()})
			return
		}

		triggerAt := time.Now()
		if c.Request.ContentLength > 0 {
			var req triggerDailyDigestRequest
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

		if err := svc.TriggerDailyDigest(c.Request.Context(), triggerAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"status":     "accepted",
			"trigger_at": triggerAt.Format(time.RFC3339),
		})
	})
}
