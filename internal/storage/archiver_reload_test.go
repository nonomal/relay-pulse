package storage

import (
	"context"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"monitor/internal/config"
)

// mockArchiverStorage 实现 Storage + ArchiveStorage
type mockArchiverStorage struct {
	exportCount atomic.Int64
	panicStorage
}

func (m *mockArchiverStorage) ExportDayToWriter(_ context.Context, _, _ int64, _ io.Writer) (int64, error) {
	m.exportCount.Add(1)
	return 0, nil
}

func enabledArchive(t *testing.T, archiveDays int) *config.ArchiveConfig {
	t.Helper()
	return &config.ArchiveConfig{
		Enabled:      boolPtr(true),
		ArchiveDays:  archiveDays,
		OutputDir:    t.TempDir(),
		Format:       "csv",
		BackfillDays: 1,
	}
}

// TestArchiverUpdateConfigWakesTimer 验证 UpdateArchiveConfig 唤醒调度循环
//
// 策略：Archiver 首次启动后计算的 nextArchiveTime 通常是几小时后的整点。
// 更新配置后调用 Stop()，如果 reloadCh 未被正确消费（timer 未被唤醒），
// Stop() 会挂在长 timer 上。因此 Stop() 在 200ms 内返回 = 证明 timer 被唤醒。
func TestArchiverUpdateConfigWakesTimer(t *testing.T) {
	store := &mockArchiverStorage{}
	cfg := enabledArchive(t, 30)
	archiver := NewArchiver(store, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})
	go func() {
		close(started)
		archiver.Start(ctx)
	}()
	<-started
	time.Sleep(50 * time.Millisecond) // 让 Start 进入主循环（执行首次归档 + 进入 timer 等待）

	// 记录首次启动时的 exportCount（Start 会立即执行一次 runArchive）
	initialExports := store.exportCount.Load()

	// 更新配置 — reloadCh 应唤醒 timer
	newCfg := enabledArchive(t, 10)
	archiver.UpdateArchiveConfig(newCfg)

	// 等待循环重新执行（reload 后会 recalculate next archive time，
	// 然后再进入新 timer，期间不会再次 runArchive，
	// 但循环已经重新读取了新配置）
	time.Sleep(50 * time.Millisecond)

	// 验证配置已更新
	if got := archiver.currentConfig(); got.ArchiveDays != 10 {
		t.Errorf("更新后 ArchiveDays = %d, want 10", got.ArchiveDays)
	}

	// 关键断言：Stop 必须在短时间内返回
	// 如果 reloadCh 没有唤醒 timer，archiver 会挂在几小时后的 timer 上，Stop 会超时
	done := make(chan struct{})
	go func() {
		archiver.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Stop 在合理时间内返回 = reloadCh 机制工作正常
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Stop 超时 — reloadCh 可能未唤醒 timer")
	}

	// 验证至少有首次归档执行
	if initialExports < 1 {
		t.Errorf("首次归档未执行: exportCount = %d", initialExports)
	}
}

// TestArchiverReloadChNonBlocking 验证 reloadCh 非阻塞，连续更新不死锁
func TestArchiverReloadChNonBlocking(t *testing.T) {
	store := &mockArchiverStorage{}
	cfg := enabledArchive(t, 30)
	archiver := NewArchiver(store, cfg)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			archiver.UpdateArchiveConfig(cfg)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("连续 UpdateArchiveConfig 死锁")
	}
}

// TestArchiverStopBlocksUntilExit 验证 Stop 阻塞直到 Start goroutine 退出
func TestArchiverStopBlocksUntilExit(t *testing.T) {
	store := &mockArchiverStorage{}
	cfg := enabledArchive(t, 30)
	archiver := NewArchiver(store, cfg)

	ctx := context.Background()
	go archiver.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		archiver.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop 超时")
	}
}

// TestArchiverDisabledSkips 验证禁用配置不启动归档
func TestArchiverDisabledSkips(t *testing.T) {
	store := &mockArchiverStorage{}
	cfg := &config.ArchiveConfig{Enabled: boolPtr(false)}
	archiver := NewArchiver(store, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		archiver.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("禁用的 Archiver 应立即返回")
	}

	if store.exportCount.Load() != 0 {
		t.Errorf("exportCount = %d, want 0", store.exportCount.Load())
	}
}

// TestArchiverUpdateToDisabled 验证运行中切换到禁用配置后 archiver 退出
func TestArchiverUpdateToDisabled(t *testing.T) {
	store := &mockArchiverStorage{}
	cfg := enabledArchive(t, 30)
	archiver := NewArchiver(store, cfg)

	ctx := context.Background()
	exitCh := make(chan struct{})
	go func() {
		archiver.Start(ctx)
		close(exitCh)
	}()
	time.Sleep(50 * time.Millisecond)

	// 切换到禁用
	disabledCfg := &config.ArchiveConfig{Enabled: boolPtr(false)}
	archiver.UpdateArchiveConfig(disabledCfg)

	select {
	case <-exitCh:
		// Start 检测到禁用后正常退出
	case <-time.After(2 * time.Second):
		t.Fatal("切换到禁用后 archiver 未退出")
	}
}
