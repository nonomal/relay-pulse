package scheduler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"monitor/internal/config"
	"monitor/internal/storage"
)

func boolPtr(v bool) *bool { return &v }

func newTestStore(t *testing.T) *storage.SQLiteStorage {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "scheduler_test.db")
	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	if err := store.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func mkMonitor(provider, service, channel, model, url string, interval time.Duration) config.ServiceConfig {
	return config.ServiceConfig{
		Provider:               provider,
		Service:                service,
		Channel:                channel,
		Model:                  model,
		URL:                    url,
		Method:                 http.MethodGet,
		SlowLatencyDuration:    5 * time.Second,
		TimeoutDuration:        5 * time.Second,
		IntervalDuration:       interval,
		RetryCount:             0,
		RetryBaseDelayDuration: 200 * time.Millisecond,
		RetryMaxDelayDuration:  2 * time.Second,
	}
}

func mkKey(m config.ServiceConfig) storage.MonitorKey {
	return storage.MonitorKey{
		Provider: m.Provider,
		Service:  m.Service,
		Channel:  m.Channel,
		Model:    m.Model,
	}
}

func waitRecords(t *testing.T, store storage.Storage, key storage.MonitorKey, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		recs, err := store.GetHistory(key.Provider, key.Service, key.Channel, key.Model, time.Unix(0, 0))
		if err != nil {
			t.Fatalf("GetHistory: %v", err)
		}
		if len(recs) >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %d records, have %d", want, len(recs))
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// --- lifecycle ---

func TestStartStop(t *testing.T) {
	store := newTestStore(t)
	s := NewScheduler(store, 30*time.Second)

	cfg := &config.AppConfig{
		IntervalDuration: 30 * time.Second,
		StaggerProbes:    boolPtr(false),
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s.Start(ctx, cfg)

	s.mu.Lock()
	running := s.running
	schedCtx := s.ctx
	s.mu.Unlock()

	if !running {
		t.Fatal("scheduler not running after Start")
	}
	if schedCtx == nil {
		t.Fatal("scheduler context is nil after Start")
	}

	s.Stop()

	s.mu.Lock()
	running = s.running
	timer := s.timer
	s.mu.Unlock()

	if running {
		t.Error("scheduler still running after Stop")
	}
	if timer != nil {
		t.Error("timer not cleared after Stop")
	}

	select {
	case <-schedCtx.Done():
	case <-time.After(2 * time.Second):
		t.Error("context not cancelled after Stop")
	}
}

func TestStartStop_DoubleStart(t *testing.T) {
	store := newTestStore(t)
	s := NewScheduler(store, 30*time.Second)

	cfg := &config.AppConfig{
		IntervalDuration: 30 * time.Second,
		StaggerProbes:    boolPtr(false),
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s.Start(ctx, cfg)
	s.Start(ctx, cfg) // second start is no-op
	t.Cleanup(s.Stop)

	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	if !running {
		t.Fatal("scheduler not running after double Start")
	}
}

func TestStartStop_DoubleStop(t *testing.T) {
	store := newTestStore(t)
	s := NewScheduler(store, 30*time.Second)

	cfg := &config.AppConfig{
		IntervalDuration: 30 * time.Second,
		StaggerProbes:    boolPtr(false),
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s.Start(ctx, cfg)
	s.Stop()
	s.Stop() // second stop is no-op, should not panic
}

// --- TriggerNow ---

func TestTriggerNow(t *testing.T) {
	store := newTestStore(t)
	s := NewScheduler(store, 30*time.Second)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	mon := mkMonitor("trig", "svc", "ch", "m", srv.URL, 30*time.Second)
	cfg := &config.AppConfig{
		IntervalDuration: 30 * time.Second,
		MaxConcurrency:   1,
		StaggerProbes:    boolPtr(false),
		Monitors:         []config.ServiceConfig{mon},
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s.Start(ctx, cfg)
	t.Cleanup(s.Stop)

	key := mkKey(mon)
	// Wait for initial probe
	waitRecords(t, store, key, 1, 5*time.Second)

	// Trigger immediate re-probe
	s.TriggerNow()
	waitRecords(t, store, key, 2, 5*time.Second)
}

func TestTriggerNow_NotRunning(t *testing.T) {
	store := newTestStore(t)
	s := NewScheduler(store, 30*time.Second)
	// Should not panic when called on a stopped scheduler
	s.TriggerNow()
}

// --- UpdateConfig ---

func TestUpdateConfig(t *testing.T) {
	store := newTestStore(t)
	s := NewScheduler(store, 30*time.Second)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	cfgA := &config.AppConfig{
		IntervalDuration: 30 * time.Second,
		MaxConcurrency:   1,
		StaggerProbes:    boolPtr(false),
		Monitors: []config.ServiceConfig{
			mkMonitor("a", "svc", "ch", "m1", srv.URL, 30*time.Second),
		},
	}

	cfgB := &config.AppConfig{
		IntervalDuration: 45 * time.Second,
		MaxConcurrency:   2,
		StaggerProbes:    boolPtr(false),
		Monitors: []config.ServiceConfig{
			mkMonitor("b", "svc", "ch", "m2", srv.URL, 45*time.Second),
			mkMonitor("c", "svc", "ch2", "m3", srv.URL, 45*time.Second),
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s.Start(ctx, cfgA)
	t.Cleanup(s.Stop)

	s.UpdateConfig(cfgB)

	s.mu.Lock()
	taskCount := len(s.tasks)
	semCap := 0
	if s.sem != nil {
		semCap = cap(s.sem)
	}
	s.mu.Unlock()

	if taskCount != len(cfgB.Monitors) {
		t.Errorf("task count: want %d, got %d", len(cfgB.Monitors), taskCount)
	}
	if semCap != cfgB.MaxConcurrency {
		t.Errorf("semaphore cap: want %d, got %d", cfgB.MaxConcurrency, semCap)
	}
}

func TestUpdateConfig_EmptyMonitors(t *testing.T) {
	store := newTestStore(t)
	s := NewScheduler(store, 30*time.Second)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	cfgA := &config.AppConfig{
		IntervalDuration: 30 * time.Second,
		MaxConcurrency:   1,
		StaggerProbes:    boolPtr(false),
		Monitors: []config.ServiceConfig{
			mkMonitor("a", "svc", "ch", "m", srv.URL, 30*time.Second),
		},
	}
	cfgEmpty := &config.AppConfig{
		IntervalDuration: 30 * time.Second,
		StaggerProbes:    boolPtr(false),
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s.Start(ctx, cfgA)
	t.Cleanup(s.Stop)

	s.UpdateConfig(cfgEmpty)

	s.mu.Lock()
	taskCount := len(s.tasks)
	s.mu.Unlock()

	if taskCount != 0 {
		t.Errorf("task count after empty config: want 0, got %d", taskCount)
	}
}

// --- MaxConcurrency ---

func TestMaxConcurrency(t *testing.T) {
	store := newTestStore(t)
	s := NewScheduler(store, time.Minute)

	const maxConc = 2
	const monCount = 4

	releaseCh := make(chan struct{})
	var releaseOnce sync.Once
	closeRelease := func() { releaseOnce.Do(func() { close(releaseCh) }) }

	startedCh := make(chan struct{}, monCount)
	var active, maxActive int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cur := atomic.AddInt32(&active, 1)
		for {
			m := atomic.LoadInt32(&maxActive)
			if cur <= m || atomic.CompareAndSwapInt32(&maxActive, m, cur) {
				break
			}
		}
		startedCh <- struct{}{}
		<-releaseCh
		atomic.AddInt32(&active, -1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	cfg := &config.AppConfig{
		IntervalDuration: time.Minute,
		MaxConcurrency:   maxConc,
		StaggerProbes:    boolPtr(false),
	}
	for i := 0; i < monCount; i++ {
		cfg.Monitors = append(cfg.Monitors, mkMonitor(
			fmt.Sprintf("p%d", i), "svc", fmt.Sprintf("ch%d", i), fmt.Sprintf("m%d", i),
			srv.URL, time.Minute,
		))
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s.Start(ctx, cfg)
	t.Cleanup(s.Stop)
	t.Cleanup(closeRelease)

	// Wait for maxConc probes to start (they will block on releaseCh)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for i := 0; i < maxConc; i++ {
		select {
		case <-startedCh:
		case <-timer.C:
			t.Fatalf("timeout waiting for probe %d to start", i)
		}
	}

	// Give a moment for any extra probes to start (they shouldn't)
	time.Sleep(100 * time.Millisecond)

	if peak := atomic.LoadInt32(&maxActive); peak > int32(maxConc) {
		t.Fatalf("peak concurrency %d exceeds limit %d", peak, maxConc)
	}

	// Release blocked probes
	closeRelease()

	// Remaining probes should now execute
	for i := 0; i < monCount-maxConc; i++ {
		select {
		case <-startedCh:
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for remaining probe %d", i)
		}
	}
}
