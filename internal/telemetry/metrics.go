package telemetry

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// Metrics 保存最小 Prometheus 计数器。
type Metrics struct {
	dailyDigestTriggered atomic.Uint64
	dailyDigestSkipped   atomic.Uint64
}

// NewMetrics 创建最小 metrics 导出器。
func NewMetrics() *Metrics {
	return &Metrics{}
}

// IncDailyDigestTriggered 记录日报触发次数。
func (m *Metrics) IncDailyDigestTriggered() {
	if m == nil {
		return
	}
	m.dailyDigestTriggered.Add(1)
}

// IncDailyDigestSkipped 记录日报幂等跳过次数。
func (m *Metrics) IncDailyDigestSkipped() {
	if m == nil {
		return
	}
	m.dailyDigestSkipped.Add(1)
}

// Handler 导出最小 Prometheus 文本格式指标。
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		triggered := uint64(0)
		skipped := uint64(0)
		if m != nil {
			triggered = m.dailyDigestTriggered.Load()
			skipped = m.dailyDigestSkipped.Load()
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = fmt.Fprintf(w, "# TYPE rss_daily_digest_triggered_total counter\nrss_daily_digest_triggered_total %d\n", triggered)
		_, _ = fmt.Fprintf(w, "# TYPE rss_daily_digest_skipped_total counter\nrss_daily_digest_skipped_total %d\n", skipped)
	})
}
