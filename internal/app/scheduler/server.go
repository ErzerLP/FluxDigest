package scheduler

import (
	"context"
	"strings"
	"time"

	"rss-platform/internal/service"
)

// Trigger 定义调度器触发日报任务所需的最小能力。
type Trigger interface {
	TriggerDailyDigest(ctx context.Context, now time.Time) error
}

// ConfigReader 定义调度配置读取能力。
type ConfigReader interface {
	Scheduler(ctx context.Context) (service.SchedulerRuntimeConfig, error)
}

type Option func(*Server)

// Server 表示可测试的调度循环。
type Server struct {
	trigger        Trigger
	configs        ConfigReader
	ticks          <-chan time.Time
	stop           func()
	loadLocation   func(name string) (*time.Location, error)
	lastDigestDate string
}

// WithTickChannel 注入测试用 tick channel。
func WithTickChannel(ch <-chan time.Time) Option {
	return func(s *Server) {
		s.ticks = ch
		s.stop = func() {}
	}
}

// WithLocationLoader 注入时区加载器。
func WithLocationLoader(loader func(string) (*time.Location, error)) Option {
	return func(s *Server) {
		if loader != nil {
			s.loadLocation = loader
		}
	}
}

// NewServer 创建调度循环。
func NewServer(trigger Trigger, configs ConfigReader, options ...Option) *Server {
	ticker := time.NewTicker(time.Minute)
	server := &Server{
		trigger:      trigger,
		configs:      configs,
		ticks:        ticker.C,
		stop:         ticker.Stop,
		loadLocation: time.LoadLocation,
	}
	for _, option := range options {
		option(server)
	}
	return server
}

// Close 关闭内部 ticker。
func (s *Server) Close() {
	if s != nil && s.stop != nil {
		s.stop()
	}
}

// Run 启动调度循环。
func (s *Server) Run(ctx context.Context) error {
	if s == nil {
		return nil
	}
	defer s.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		case now, ok := <-s.ticks:
			if !ok {
				return nil
			}
			if err := s.runOnce(ctx, now); err != nil {
				return err
			}
		}
	}
}

func (s *Server) runOnce(ctx context.Context, now time.Time) error {
	if s == nil || s.trigger == nil || s.configs == nil {
		return nil
	}

	cfg, err := s.configs.Scheduler(ctx)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return nil
	}

	location := s.resolveLocation(cfg.Timezone)
	localNow := now.In(location)
	if localNow.Format("15:04") != normalizeScheduleTime(cfg.ScheduleTime) {
		return nil
	}

	digestDate := localNow.Format("2006-01-02")
	if digestDate == s.lastDigestDate {
		return nil
	}

	if err := s.trigger.TriggerDailyDigest(ctx, localNow); err != nil {
		return err
	}

	s.lastDigestDate = digestDate
	return nil
}

func (s *Server) resolveLocation(name string) *time.Location {
	if s == nil || s.loadLocation == nil {
		return shanghaiLocation()
	}

	locationName := strings.TrimSpace(name)
	if locationName == "" {
		locationName = "Asia/Shanghai"
	}

	location, err := s.loadLocation(locationName)
	if err != nil {
		return shanghaiLocation()
	}
	return location
}

func normalizeScheduleTime(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "07:00"
	}

	parsed, err := time.Parse("15:04", trimmed)
	if err != nil {
		return "07:00"
	}

	return parsed.Format("15:04")
}

func shanghaiLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return location
}
