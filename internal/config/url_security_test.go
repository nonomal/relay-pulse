package config

import (
	"net"
	"testing"
)

func TestIsPrivateOrSpecialIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		// 私有 IP
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.0.1", true},
		{"192.168.1.100", true},

		// 回环
		{"127.0.0.1", true},
		{"127.0.0.2", true},

		// 链路本地
		{"169.254.1.1", true},

		// 未指定
		{"0.0.0.0", true},

		// RFC 6598 CGNAT
		{"100.64.0.1", true},
		{"100.127.255.255", true},

		// 公网 IP
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"203.0.113.1", false},
		{"100.63.255.255", false}, // CGNAT 边界外
		{"100.128.0.0", false},    // CGNAT 边界外
		{"172.32.0.1", false},     // 172 边界外
		{"172.15.255.255", false}, // 172 边界外

		// IPv6
		{"::1", true},                       // 回环
		{"fe80::1", true},                   // 链路本地
		{"::", true},                        // 未指定
		{"2001:db8::1", false},              // 文档地址（非 private in Go stdlib）
		{"2607:f8b0:4004:800::200e", false}, // 公网 IPv6
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if ip == nil {
			t.Fatalf("无法解析 IP: %s", tt.ip)
		}
		got := isPrivateOrSpecialIP(ip)
		if got != tt.private {
			t.Errorf("isPrivateOrSpecialIP(%s) = %v, want %v", tt.ip, got, tt.private)
		}
	}
}

func TestValidateNoPrivateIPURL(t *testing.T) {
	tests := []struct {
		url       string
		wantError bool
	}{
		// 公网 URL
		{"https://api.example.com/v1", false},
		{"https://8.8.8.8/api", false},

		// 私网 IP URL
		{"http://10.0.0.1:8080/api", true},
		{"https://192.168.1.100/v1", true},
		{"http://127.0.0.1:3000", true},

		// hostname（不解析，跳过）
		{"https://localhost:3000", false},
		{"https://internal.corp.com", false},

		// 空
		{"", false},
		{"   ", false},
	}

	for _, tt := range tests {
		err := validateNoPrivateIPURL(tt.url, "test_field")
		if tt.wantError && err == nil {
			t.Errorf("validateNoPrivateIPURL(%q) 期望报错但没有", tt.url)
		}
		if !tt.wantError && err != nil {
			t.Errorf("validateNoPrivateIPURL(%q) 不期望报错但得到: %v", tt.url, err)
		}
	}
}

func TestValidateFinalPrivateIPWarning(t *testing.T) {
	t.Run("默认不允许私网 IP", func(t *testing.T) {
		cfg := &AppConfig{
			Interval:    "1m",
			SlowLatency: "5s",
			Monitors: []ServiceConfig{
				{
					Provider:   "test",
					Service:    "cc",
					Channel:    "default",
					BaseURL:    "http://10.0.0.1:8080",
					URLPattern: "{{BASE_URL}}/v1/chat/completions",
					Method:     "POST",
					Category:   "public",
					Sponsor:    "test",
				},
			},
		}
		if err := cfg.normalize(); err != nil {
			t.Fatalf("normalize: %v", err)
		}

		warns := cfg.validateFinal()
		found := false
		for _, w := range warns {
			if w != nil && contains(w.Error(), "私有/特殊网络 IP") {
				found = true
			}
		}
		if !found {
			t.Error("期望私网 IP 告警但没有")
		}
	})

	t.Run("allow_private_networks 关闭告警", func(t *testing.T) {
		cfg := &AppConfig{
			Interval:             "1m",
			SlowLatency:          "5s",
			AllowPrivateNetworks: true,
			Monitors: []ServiceConfig{
				{
					Provider:   "test",
					Service:    "cc",
					Channel:    "default",
					BaseURL:    "http://10.0.0.1:8080",
					URLPattern: "{{BASE_URL}}/v1/chat/completions",
					Method:     "POST",
					Category:   "public",
					Sponsor:    "test",
				},
			},
		}
		if err := cfg.normalize(); err != nil {
			t.Fatalf("normalize: %v", err)
		}

		warns := cfg.validateFinal()
		for _, w := range warns {
			if w != nil && contains(w.Error(), "私有/特殊网络 IP") {
				t.Error("AllowPrivateNetworks=true 时不应有私网 IP 告警")
			}
		}
	})

	t.Run("skip_url_validation 逃生舱", func(t *testing.T) {
		cfg := &AppConfig{
			Interval:    "1m",
			SlowLatency: "5s",
			Monitors: []ServiceConfig{
				{
					Provider:          "test",
					Service:           "cc",
					Channel:           "default",
					BaseURL:           "http://192.168.1.100:8080",
					URLPattern:        "{{BASE_URL}}/v1/chat/completions",
					Method:            "POST",
					Category:          "public",
					Sponsor:           "test",
					SkipURLValidation: true,
				},
			},
		}
		if err := cfg.normalize(); err != nil {
			t.Fatalf("normalize: %v", err)
		}

		warns := cfg.validateFinal()
		for _, w := range warns {
			if w != nil && contains(w.Error(), "私有/特殊网络 IP") {
				t.Error("SkipURLValidation=true 时不应有私网 IP 告警")
			}
		}
	})

	t.Run("绝对 url_pattern 私网 IP 告警", func(t *testing.T) {
		cfg := &AppConfig{
			Interval:    "1m",
			SlowLatency: "5s",
			Monitors: []ServiceConfig{
				{
					Provider:   "test",
					Service:    "cc",
					Channel:    "default",
					BaseURL:    "https://api.example.com",
					URLPattern: "http://10.0.0.5:8080/v1/chat/completions",
					Method:     "POST",
					Category:   "public",
					Sponsor:    "test",
				},
			},
		}
		if err := cfg.normalize(); err != nil {
			t.Fatalf("normalize: %v", err)
		}

		warns := cfg.validateFinal()
		found := false
		for _, w := range warns {
			if w != nil && contains(w.Error(), "url_pattern") && contains(w.Error(), "私有/特殊网络 IP") {
				found = true
			}
		}
		if !found {
			t.Error("绝对 url_pattern 指向私网 IP 时应有告警")
		}
	})

	t.Run("公网 IP 无告警", func(t *testing.T) {
		cfg := &AppConfig{
			Interval:    "1m",
			SlowLatency: "5s",
			Monitors: []ServiceConfig{
				{
					Provider:   "test",
					Service:    "cc",
					Channel:    "default",
					BaseURL:    "https://8.8.8.8",
					URLPattern: "{{BASE_URL}}/v1/chat/completions",
					Method:     "POST",
					Category:   "public",
					Sponsor:    "test",
				},
			},
		}
		if err := cfg.normalize(); err != nil {
			t.Fatalf("normalize: %v", err)
		}

		warns := cfg.validateFinal()
		for _, w := range warns {
			if w != nil && contains(w.Error(), "私有/特殊网络 IP") {
				t.Error("公网 IP 不应有告警")
			}
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
