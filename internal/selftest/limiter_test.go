package selftest

import (
	"testing"
	"time"
)

func TestIPLimiterAllowBurstAndRefill(t *testing.T) {
	// 120/min = 2/s, burst=2
	limiter := NewIPLimiter(120, 2)
	defer limiter.Stop()

	ip := "203.0.113.10"
	if !limiter.Allow(ip) {
		t.Fatal("first request should be allowed")
	}
	if !limiter.Allow(ip) {
		t.Fatal("second request within burst should be allowed")
	}
	// 第 3 次超出 burst
	if limiter.Allow(ip) {
		t.Fatal("third immediate request should be rate limited")
	}

	// 等待令牌回填：2/s → 1 token 需要 0.5s，给 1s 余量
	time.Sleep(1 * time.Second)

	if !limiter.Allow(ip) {
		t.Fatal("request after refill should be allowed")
	}
}

func TestIPLimiterDifferentIPsIndependent(t *testing.T) {
	limiter := NewIPLimiter(60, 1)
	defer limiter.Stop()

	// IP-A 用完配额
	if !limiter.Allow("203.0.113.1") {
		t.Fatal("IP-A first request should be allowed")
	}
	if limiter.Allow("203.0.113.1") {
		t.Fatal("IP-A second request should be denied")
	}

	// IP-B 不受 IP-A 影响
	if !limiter.Allow("203.0.113.2") {
		t.Fatal("IP-B first request should be allowed")
	}
}

func TestIPLimiterCountAndReset(t *testing.T) {
	limiter := NewIPLimiter(60, 1)
	defer limiter.Stop()

	if got := limiter.Count(); got != 0 {
		t.Fatalf("initial Count() = %d, want 0", got)
	}

	limiter.Allow("203.0.113.1")
	limiter.Allow("203.0.113.2")

	if got := limiter.Count(); got != 2 {
		t.Fatalf("Count() = %d, want 2", got)
	}
	if limiter.GetLimiter("203.0.113.1") == nil {
		t.Fatal("GetLimiter returned nil for tracked IP")
	}

	limiter.Reset()

	if got := limiter.Count(); got != 0 {
		t.Fatalf("Count() after Reset() = %d, want 0", got)
	}
	if limiter.GetLimiter("203.0.113.1") != nil {
		t.Fatal("GetLimiter should return nil after Reset()")
	}
}

func TestIPLimiterCleanupRemovesExpiredEntries(t *testing.T) {
	limiter := NewIPLimiter(60, 1)
	defer limiter.Stop()

	limiter.Allow("203.0.113.1")
	limiter.Allow("203.0.113.2")

	// 手动调整 TTL 和 lastSeen 以触发清理
	limiter.mu.Lock()
	limiter.ttl = 100 * time.Millisecond
	limiter.limiters["203.0.113.1"].lastSeen = time.Now().Add(-time.Second)
	limiter.limiters["203.0.113.2"].lastSeen = time.Now()
	limiter.mu.Unlock()

	limiter.cleanup()

	if got := limiter.Count(); got != 1 {
		t.Fatalf("Count() after cleanup = %d, want 1", got)
	}
	if limiter.GetLimiter("203.0.113.1") != nil {
		t.Fatal("expired entry should be removed by cleanup")
	}
	if limiter.GetLimiter("203.0.113.2") == nil {
		t.Fatal("recent entry should be kept by cleanup")
	}
}

func TestIPLimiterStopIdempotent(t *testing.T) {
	limiter := NewIPLimiter(60, 1)

	done := make(chan struct{})
	go func() {
		limiter.Stop()
		limiter.Stop() // 二次调用不应阻塞
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return promptly on repeated calls")
	}
}
