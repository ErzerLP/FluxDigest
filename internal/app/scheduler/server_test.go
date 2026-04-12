package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"rss-platform/internal/service"
)

type schedulerConfigReaderStub struct {
	config service.SchedulerRuntimeConfig
	err    error
}

func (s schedulerConfigReaderStub) Scheduler(_ context.Context) (service.SchedulerRuntimeConfig, error) {
	if s.err != nil {
		return service.SchedulerRuntimeConfig{}, s.err
	}
	return s.config, nil
}

type triggerSpy struct {
	mu    sync.Mutex
	calls []time.Time
	ch    chan time.Time
	errs  []error
}

func (s *triggerSpy) TriggerDailyDigest(_ context.Context, now time.Time) error {
	s.mu.Lock()
	s.calls = append(s.calls, now)
	var err error
	if len(s.errs) > 0 {
		err = s.errs[0]
		s.errs = s.errs[1:]
	}
	s.mu.Unlock()
	if s.ch != nil {
		s.ch <- now
	}
	return err
}

func (s *triggerSpy) snapshot() []time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]time.Time, len(s.calls))
	copy(out, s.calls)
	return out
}

func TestSchedulerLoopTriggersOncePerDigestDate(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	ticks := make(chan time.Time, 3)
	trigger := &triggerSpy{ch: make(chan time.Time, 3)}
	server := NewServer(trigger, schedulerConfigReaderStub{config: service.SchedulerRuntimeConfig{
		Enabled:      true,
		ScheduleTime: "07:00",
		Timezone:     "Asia/Shanghai",
	}}, WithTickChannel(ticks), WithLocationLoader(func(name string) (*time.Location, error) {
		if name == "Asia/Shanghai" {
			return loc, nil
		}
		return time.LoadLocation(name)
	}), WithNowFunc(func() time.Time {
		return time.Date(2026, 4, 11, 6, 59, 0, 0, loc)
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- server.Run(ctx)
	}()

	ticks <- time.Date(2026, 4, 11, 7, 0, 0, 0, loc)
	ticks <- time.Date(2026, 4, 11, 7, 0, 30, 0, loc)
	ticks <- time.Date(2026, 4, 12, 7, 0, 0, 0, loc)

	deadline := time.After(2 * time.Second)
	for len(trigger.snapshot()) < 2 {
		select {
		case <-trigger.ch:
		case <-deadline:
			t.Fatalf("timed out waiting for scheduler triggers, got %d", len(trigger.snapshot()))
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop")
	}

	calls := trigger.snapshot()
	if len(calls) != 2 {
		t.Fatalf("want 2 trigger calls got %d", len(calls))
	}
	if got := calls[0].Format("2006-01-02"); got != "2026-04-11" {
		t.Fatalf("want first digest date 2026-04-11 got %s", got)
	}
	if got := calls[1].Format("2006-01-02"); got != "2026-04-12" {
		t.Fatalf("want second digest date 2026-04-12 got %s", got)
	}
}

func TestSchedulerRunTriggersForTodayWhenStartedAfterScheduleTime(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	ticks := make(chan time.Time, 2)
	trigger := &triggerSpy{ch: make(chan time.Time, 2)}
	server := NewServer(trigger, schedulerConfigReaderStub{config: service.SchedulerRuntimeConfig{
		Enabled:      true,
		ScheduleTime: "07:00",
		Timezone:     "Asia/Shanghai",
	}}, WithTickChannel(ticks), WithLocationLoader(func(name string) (*time.Location, error) {
		if name == "Asia/Shanghai" {
			return loc, nil
		}
		return time.LoadLocation(name)
	}), WithNowFunc(func() time.Time {
		return time.Date(2026, 4, 11, 7, 0, 5, 0, loc)
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- server.Run(ctx)
	}()

	select {
	case got := <-trigger.ch:
		if got.Format(time.RFC3339) != "2026-04-11T07:00:05+08:00" {
			t.Fatalf("want immediate trigger at late start got %s", got.Format(time.RFC3339))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected scheduler to trigger immediately on late start")
	}

	ticks <- time.Date(2026, 4, 11, 7, 1, 5, 0, loc)
	time.Sleep(50 * time.Millisecond)
	if len(trigger.snapshot()) != 1 {
		t.Fatalf("want only one trigger for same day got %d", len(trigger.snapshot()))
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop")
	}
}

func TestSchedulerRunContinuesAfterTransientError(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	ticks := make(chan time.Time, 2)
	trigger := &triggerSpy{ch: make(chan time.Time, 2), errs: []error{errors.New("redis down"), nil}}
	server := NewServer(trigger, schedulerConfigReaderStub{config: service.SchedulerRuntimeConfig{
		Enabled:      true,
		ScheduleTime: "07:00",
		Timezone:     "Asia/Shanghai",
	}}, WithTickChannel(ticks), WithLocationLoader(func(name string) (*time.Location, error) {
		if name == "Asia/Shanghai" {
			return loc, nil
		}
		return time.LoadLocation(name)
	}), WithNowFunc(func() time.Time {
		return time.Date(2026, 4, 11, 6, 59, 0, 0, loc)
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- server.Run(ctx)
	}()

	ticks <- time.Date(2026, 4, 11, 7, 0, 0, 0, loc)
	ticks <- time.Date(2026, 4, 12, 7, 0, 0, 0, loc)

	deadline := time.After(2 * time.Second)
	for len(trigger.snapshot()) < 2 {
		select {
		case <-trigger.ch:
		case err := <-done:
			t.Fatalf("scheduler exited after transient error: %v", err)
		case <-deadline:
			t.Fatalf("timed out waiting for scheduler to continue, got %d calls", len(trigger.snapshot()))
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop")
	}

	if len(trigger.snapshot()) != 2 {
		t.Fatalf("want scheduler to continue after transient error, got %d calls", len(trigger.snapshot()))
	}
}
