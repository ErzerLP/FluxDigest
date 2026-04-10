package scheduler

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
)

// Trigger 定义调度器触发日报任务所需的最小能力。
type Trigger interface {
	TriggerDailyDigest(ctx context.Context, now time.Time) error
}

// Start 启动每天 Asia/Shanghai 07:00 的日报调度。
func Start(job Trigger) *cron.Cron {
	location := shanghaiLocation()
	scheduler := cron.New(cron.WithLocation(location))
	_, _ = scheduler.AddFunc("0 7 * * *", func() {
		if job == nil {
			return
		}
		_ = job.TriggerDailyDigest(context.Background(), time.Now().In(location))
	})
	scheduler.Start()
	return scheduler
}

func shanghaiLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return location
}
