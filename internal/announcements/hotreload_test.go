package announcements

import (
	"context"
	"testing"
	"time"

	"monitor/internal/config"
)

func testAnnouncementsConfig() config.AnnouncementsConfig {
	return config.AnnouncementsConfig{
		Owner:                "test",
		Repo:                 "test",
		CategoryName:         "Announcements",
		PollInterval:         "10m",
		PollIntervalDuration: 10 * time.Minute,
		WindowHours:          48,
		MaxItems:             5,
		APIMaxAge:            60,
	}
}

func testGitHubConfig() config.GitHubConfig {
	return config.GitHubConfig{
		Token:           "", // 无 token，fetcher 不会实际请求
		TimeoutDuration: 10 * time.Second,
	}
}

// TestServiceStopBlocksUntilExit 验证 Stop 阻塞直到内部 goroutine 退出
func TestServiceStopBlocksUntilExit(t *testing.T) {
	svc, err := NewService(testAnnouncementsConfig(), testGitHubConfig())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.Start(ctx)

	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Stop 正常返回
	case <-time.After(2 * time.Second):
		t.Fatal("Stop 超时")
	}
}

// TestServiceStopIdempotent 验证 Stop 幂等
func TestServiceStopIdempotent(t *testing.T) {
	svc, err := NewService(testAnnouncementsConfig(), testGitHubConfig())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.Start(ctx)
	svc.Stop()

	// 第二次调用 Stop 不应 panic 或阻塞
	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("第二次 Stop 超时")
	}
}

// TestServiceRecreateAfterStop 模拟热更新：停止旧实例 → 创建新实例 → 启动新实例
func TestServiceRecreateAfterStop(t *testing.T) {
	cfg := testAnnouncementsConfig()
	ghCfg := testGitHubConfig()

	// 创建并启动旧实例
	old, err := NewService(cfg, ghCfg)
	if err != nil {
		t.Fatalf("NewService old: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	old.Start(ctx)

	// 创建新实例（模拟新配置）
	newCfg := cfg
	newCfg.WindowHours = 72

	newSvc, err := NewService(newCfg, ghCfg)
	if err != nil {
		t.Fatalf("NewService new: %v", err)
	}

	// 启动新实例
	newSvc.Start(ctx)

	// 停止旧实例
	old.Stop()

	// 验证新实例独立运行
	if newSvc.cfg.WindowHours != 72 {
		t.Errorf("新实例 WindowHours = %d, want 72", newSvc.cfg.WindowHours)
	}

	newSvc.Stop()
}

// TestServiceStartWithoutStop 验证 context 取消时 Start goroutine 退出
func TestServiceStartWithoutStop(t *testing.T) {
	svc, err := NewService(testAnnouncementsConfig(), testGitHubConfig())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	svc.Start(ctx)

	// 通过 context 取消而非 Stop
	cancel()

	// 等待 stoppedC 关闭
	select {
	case <-svc.stoppedC:
	case <-time.After(2 * time.Second):
		t.Fatal("context 取消后 goroutine 未退出")
	}
}
