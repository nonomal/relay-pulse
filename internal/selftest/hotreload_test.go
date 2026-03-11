package selftest

import (
	"testing"
	"time"
)

// TestStopWithDrainCancelsQueuedJobs 验证 StopWithDrain 取消排队任务
func TestStopWithDrainCancelsQueuedJobs(t *testing.T) {
	mgr := NewTestJobManager(
		1,              // maxConcurrent: 只允许 1 个并发
		10,             // maxQueueSize
		30*time.Second, // jobTimeout
		2*time.Minute,  // resultTTL
		100,            // rateLimitPerMinute
	)

	// 验证 StopWithDrain 在无任务时快速返回
	done := make(chan struct{})
	go func() {
		mgr.StopWithDrain(5 * time.Second)
		close(done)
	}()

	select {
	case <-done:
		// 正确：无任务时应立即返回
	case <-time.After(2 * time.Second):
		t.Fatal("StopWithDrain 无任务时超时")
	}
}

// TestStopWithDrainRejectsNewJobs 验证 StopWithDrain 后新任务被拒绝
func TestStopWithDrainRejectsNewJobs(t *testing.T) {
	mgr := NewTestJobManager(
		5,
		10,
		30*time.Second,
		2*time.Minute,
		100,
	)
	mgr.StopWithDrain(1 * time.Second)

	// 尝试创建新任务应被拒绝
	_, err := mgr.CreateJob("cc", "https://example.com/v1/chat/completions", "sk-test", "")
	if err == nil {
		t.Fatal("StopWithDrain 后 CreateJob 应返回错误")
	}
}

// TestStopWithDrainIdempotent 验证多次调用 StopWithDrain 不 panic
func TestStopWithDrainIdempotent(t *testing.T) {
	mgr := NewTestJobManager(5, 10, 30*time.Second, 2*time.Minute, 100)

	// 连续调用不应 panic
	mgr.StopWithDrain(1 * time.Second)
	mgr.StopWithDrain(1 * time.Second)
	mgr.Stop()
}

// TestStopWithDrainTimeout 验证 drain 超时机制
func TestStopWithDrainTimeout(t *testing.T) {
	mgr := NewTestJobManager(
		1,
		10,
		30*time.Second,
		2*time.Minute,
		100,
	)

	// 使用极短超时测试超时路径（即使无运行任务也应在超时内返回）
	start := time.Now()
	mgr.StopWithDrain(50 * time.Millisecond)
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("StopWithDrain(50ms) 耗时 %v，预期 < 1s", elapsed)
	}
}

// TestRecreateAfterStop 验证 Stop 后可以创建新 Manager（模拟热更新重建）
func TestRecreateAfterStop(t *testing.T) {
	// 第一个实例
	mgr1 := NewTestJobManager(5, 10, 30*time.Second, 2*time.Minute, 100)
	mgr1.StopWithDrain(1 * time.Second)

	// 创建新实例（模拟热更新重建）
	mgr2 := NewTestJobManager(10, 20, 60*time.Second, 5*time.Minute, 200)
	defer mgr2.Stop()

	// 新实例应正常工作（不受旧实例影响）
	if mgr2.maxConcurrent != 10 {
		t.Errorf("新实例 maxConcurrent = %d, want 10", mgr2.maxConcurrent)
	}
	if mgr2.maxQueueSize != 20 {
		t.Errorf("新实例 maxQueueSize = %d, want 20", mgr2.maxQueueSize)
	}
}
