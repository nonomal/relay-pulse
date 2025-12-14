package selftest

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ipLimiterEntry 包含限流器和最后访问时间
type ipLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPLimiter 基于 IP 的速率限制器（使用 token bucket 算法）
type IPLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*ipLimiterEntry // IP -> entry
	rateVal  rate.Limit                 // 每秒请求数（例如 10/60 表示每分钟 10 次）
	burst    int                        // 突发容量
	ttl      time.Duration              // 清理 TTL（防止内存泄漏）

	stopCh   chan struct{}
	stopOnce sync.Once // Ensure Stop is called only once
	wg       sync.WaitGroup
}

// NewIPLimiter 创建新的 IP 速率限制器
// perMinute: 每个 IP 每分钟允许的请求数
// burst: 突发容量（通常与 perMinute 相同）
func NewIPLimiter(perMinute int, burst int) *IPLimiter {
	// 转换为每秒速率
	ratePerSecond := rate.Limit(float64(perMinute) / 60.0)

	limiter := &IPLimiter{
		limiters: make(map[string]*ipLimiterEntry),
		rateVal:  ratePerSecond,
		burst:    burst,
		ttl:      5 * time.Minute, // 5 分钟未使用则回收
		stopCh:   make(chan struct{}),
	}

	// 启动清理 goroutine
	limiter.wg.Add(1)
	go limiter.cleanupWorker()

	return limiter
}

// Allow 检查来自给定 IP 的请求是否被允许
// 返回 true 表示允许，false 表示超出限流
func (l *IPLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, exists := l.limiters[ip]
	if !exists {
		// 为该 IP 创建新的限流器
		entry = &ipLimiterEntry{
			limiter:  rate.NewLimiter(l.rateVal, l.burst),
			lastSeen: time.Now(),
		}
		l.limiters[ip] = entry
	} else {
		entry.lastSeen = time.Now()
	}

	return entry.limiter.Allow()
}

// GetLimiter 返回指定 IP 的限流器（用于测试/调试）
func (l *IPLimiter) GetLimiter(ip string) *rate.Limiter {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if entry, ok := l.limiters[ip]; ok {
		return entry.limiter
	}
	return nil
}

// cleanupWorker 定期清理旧的限流器以防止内存泄漏
func (l *IPLimiter) cleanupWorker() {
	defer l.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.cleanup()
		case <-l.stopCh:
			return
		}
	}
}

// cleanup 清理长时间未使用的限流器
func (l *IPLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for ip, entry := range l.limiters {
		if now.Sub(entry.lastSeen) > l.ttl {
			delete(l.limiters, ip)
		}
	}
}

// Stop 停止清理 goroutine（用于优雅退出，幂等安全）
func (l *IPLimiter) Stop() {
	l.stopOnce.Do(func() {
		close(l.stopCh)
		l.wg.Wait()
	})
}

// Reset 清空所有速率限制器（用于测试）
func (l *IPLimiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.limiters = make(map[string]*ipLimiterEntry)
}

// Count 返回当前跟踪的 IP 数量
func (l *IPLimiter) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.limiters)
}
