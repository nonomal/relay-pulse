package api

import (
	"net/http"
	"testing"
	"time"
)

func TestIPRateLimiter_BasicAllow(t *testing.T) {
	limiter := newIPRateLimiter(3, time.Minute)
	now := time.Now()

	// 前 3 次应该允许（桶容量 3）
	for i := 0; i < 3; i++ {
		allowed, _ := limiter.Allow("1.2.3.4", now)
		if !allowed {
			t.Fatalf("request %d: want allowed, got denied", i+1)
		}
	}

	// 第 4 次应该拒绝
	allowed, retryAfter := limiter.Allow("1.2.3.4", now)
	if allowed {
		t.Fatal("request 4: want denied, got allowed")
	}
	if retryAfter < time.Second {
		t.Fatalf("retryAfter = %v, want >= 1s", retryAfter)
	}
}

func TestIPRateLimiter_DifferentIPs(t *testing.T) {
	limiter := newIPRateLimiter(1, time.Minute)
	now := time.Now()

	// IP-A 用完配额
	allowed, _ := limiter.Allow("1.1.1.1", now)
	if !allowed {
		t.Fatal("IP-A first request should be allowed")
	}
	allowed, _ = limiter.Allow("1.1.1.1", now)
	if allowed {
		t.Fatal("IP-A second request should be denied")
	}

	// IP-B 不受影响
	allowed, _ = limiter.Allow("2.2.2.2", now)
	if !allowed {
		t.Fatal("IP-B first request should be allowed")
	}
}

func TestIPRateLimiter_Refill(t *testing.T) {
	limiter := newIPRateLimiter(2, time.Minute)
	now := time.Now()

	// 用完配额
	limiter.Allow("1.2.3.4", now)
	limiter.Allow("1.2.3.4", now)
	allowed, _ := limiter.Allow("1.2.3.4", now)
	if allowed {
		t.Fatal("should be denied after exhausting bucket")
	}

	// 过了足够时间后应该回填
	later := now.Add(time.Minute + time.Second)
	allowed, _ = limiter.Allow("1.2.3.4", later)
	if !allowed {
		t.Fatal("should be allowed after full refill window")
	}
}

func TestIPRateLimiter_Cleanup(t *testing.T) {
	limiter := newIPRateLimiter(5, time.Minute)
	now := time.Now()

	limiter.Allow("old-ip", now)
	limiter.Allow("new-ip", now.Add(3*time.Minute))

	// old-ip 应该被清理（TTL = 2 * window = 2min）
	limiter.mu.Lock()
	_, hasOld := limiter.buckets["old-ip"]
	_, hasNew := limiter.buckets["new-ip"]
	limiter.mu.Unlock()

	if hasOld {
		t.Error("old-ip should have been cleaned up")
	}
	if !hasNew {
		t.Error("new-ip should still exist")
	}
}

func TestClientIP_TrustProxy(t *testing.T) {
	tests := []struct {
		name       string
		trust      bool
		xff        string
		realIP     string
		remoteAddr string
		want       string
	}{
		{
			name:       "trust=false ignores XFF",
			trust:      false,
			xff:        "1.1.1.1",
			remoteAddr: "2.2.2.2:12345",
			want:       "2.2.2.2",
		},
		{
			name:       "trust=true uses XFF",
			trust:      true,
			xff:        "1.1.1.1, 3.3.3.3",
			remoteAddr: "2.2.2.2:12345",
			want:       "1.1.1.1",
		},
		{
			name:       "trust=true uses X-Real-IP when no XFF",
			trust:      true,
			realIP:     "4.4.4.4",
			remoteAddr: "2.2.2.2:12345",
			want:       "4.4.4.4",
		},
		{
			name:       "trust=false uses RemoteAddr",
			trust:      false,
			remoteAddr: "10.0.0.1:8080",
			want:       "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := http.NewRequest("GET", "/", nil)
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.realIP != "" {
				r.Header.Set("X-Real-IP", tt.realIP)
			}
			r.RemoteAddr = tt.remoteAddr

			got := clientIP(r, tt.trust)
			if got != tt.want {
				t.Errorf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveCORSOrigin(t *testing.T) {
	tests := []struct {
		name        string
		origin      string
		allowed     []string
		wantOrigin  string
		wantAllowed bool
	}{
		{"wildcard", "https://example.com", []string{"*"}, "*", true},
		{"empty allowed = wildcard", "https://example.com", nil, "*", true},
		{"match", "https://example.com", []string{"https://example.com"}, "https://example.com", true},
		{"no match", "https://evil.com", []string{"https://example.com"}, "", false},
		{"no origin header", "", []string{"https://example.com"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOrigin, gotAllowed := resolveCORSOrigin(tt.origin, tt.allowed)
			if gotOrigin != tt.wantOrigin {
				t.Errorf("origin = %q, want %q", gotOrigin, tt.wantOrigin)
			}
			if gotAllowed != tt.wantAllowed {
				t.Errorf("allowed = %v, want %v", gotAllowed, tt.wantAllowed)
			}
		})
	}
}
