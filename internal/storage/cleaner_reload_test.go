package storage

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"monitor/internal/config"
)

// mockCleanerStorage 实现 RetentionStorage
type mockCleanerStorage struct {
	purgeCount atomic.Int64
}

func (m *mockCleanerStorage) PurgeOldRecords(_ context.Context, _ time.Time, _ int) (int64, error) {
	m.purgeCount.Add(1)
	return 0, nil // 返回 0 表示无更多数据，cleaner 完成本轮后回到等待
}

func boolPtr(v bool) *bool { return &v }

func enabledRetention(interval string, days int) *config.RetentionConfig {
	cfg := &config.RetentionConfig{
		Enabled:          boolPtr(true),
		Days:             days,
		CleanupInterval:  interval,
		BatchSize:        100,
		MaxBatchesPerRun: 10,
		StartupDelay:     "0s", // 测试中不等待
		Jitter:           0,    // 关闭随机抖动，确保 timer 精确
	}
	// 手动解析 duration（生产代码由 normalize 完成）
	cfg.CleanupIntervalDuration, _ = time.ParseDuration(interval)
	cfg.StartupDelayDuration = 0
	return cfg
}

// TestCleanerUpdateConfigWakesTimer 验证 UpdateRetentionConfig 唤醒 reloadCh 并使新配置生效
func TestCleanerUpdateConfigWakesTimer(t *testing.T) {
	store := &mockCleanerStorage{}

	// 初始配置：interval 很长，不会自然触发
	cfg := enabledRetention("1h", 30)
	cleaner := NewCleaner(store, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})
	go func() {
		close(started)
		cleaner.Start(ctx)
	}()
	<-started
	time.Sleep(20 * time.Millisecond) // 让 Start 进入主循环

	// 更新配置：interval 很短，立即触发
	newCfg := enabledRetention("10ms", 15)
	cleaner.UpdateRetentionConfig(newCfg)

	// 等待 purge 被调用（证明 reloadCh 唤醒了 timer）
	deadline := time.After(2 * time.Second)
	for {
		if store.purgeCount.Load() > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("UpdateRetentionConfig 后 2 秒内未触发 PurgeOldRecords")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// 验证新配置的 Days 已被采用
	got := cleaner.currentConfig()
	if got.Days != 15 {
		t.Errorf("Days = %d, want 15", got.Days)
	}

	cancel()
}

// TestCleanerReloadChNonBlocking 验证 reloadCh 是非阻塞的，连续发送不会死锁
func TestCleanerReloadChNonBlocking(t *testing.T) {
	store := &mockCleanerStorage{}
	cfg := enabledRetention("1h", 30)
	cleaner := NewCleaner(store, cfg)

	// 不启动 Start，无人消费 reloadCh，连续发送应不阻塞
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			cleaner.UpdateRetentionConfig(cfg)
		}
		close(done)
	}()

	select {
	case <-done:
		// 成功
	case <-time.After(1 * time.Second):
		t.Fatal("连续 UpdateRetentionConfig 死锁")
	}
}

// TestCleanerStopBlocksUntilExit 验证 Stop 阻塞直到 Start goroutine 退出
func TestCleanerStopBlocksUntilExit(t *testing.T) {
	store := &mockCleanerStorage{}
	cfg := enabledRetention("1h", 30)
	cleaner := NewCleaner(store, cfg)

	ctx := context.Background()
	go cleaner.Start(ctx)
	time.Sleep(20 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		cleaner.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Stop 正常返回
	case <-time.After(2 * time.Second):
		t.Fatal("Stop 超时，可能 Start goroutine 未退出")
	}
}

// TestCleanerDisabledConfigSkips 验证禁用的 retention 配置不会触发清理
func TestCleanerDisabledConfigSkips(t *testing.T) {
	store := &mockCleanerStorage{}
	cfg := &config.RetentionConfig{
		Enabled: boolPtr(false),
	}
	cleaner := NewCleaner(store, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		cleaner.Start(ctx)
		close(done)
	}()

	// Start 应在发现禁用后立即返回
	select {
	case <-done:
		// 正确
	case <-time.After(1 * time.Second):
		t.Fatal("禁用的 Cleaner 应该立即返回")
	}

	if store.purgeCount.Load() != 0 {
		t.Errorf("purgeCount = %d, want 0", store.purgeCount.Load())
	}
}

// TestCleanerUpdateToDisabled 验证运行中切换到禁用配置后 cleaner 退出
func TestCleanerUpdateToDisabled(t *testing.T) {
	store := &mockCleanerStorage{}
	cfg := enabledRetention("1h", 30)
	cleaner := NewCleaner(store, cfg)

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		cleaner.Start(ctx)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)

	// 切换到禁用
	disabledCfg := &config.RetentionConfig{
		Enabled:                 boolPtr(false),
		CleanupIntervalDuration: time.Hour,
	}
	cleaner.UpdateRetentionConfig(disabledCfg)

	select {
	case <-done:
		// Start 检测到禁用后正常退出
	case <-time.After(2 * time.Second):
		t.Fatal("切换到禁用后 cleaner 未退出")
	}
}
