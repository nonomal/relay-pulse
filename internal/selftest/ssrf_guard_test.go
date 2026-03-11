package selftest

import (
	"net"
	"strings"
	"testing"
)

func TestSSRFGuardValidateURLRejectsUnsafeInputs(t *testing.T) {
	guard := NewSSRFGuard()

	cases := []struct {
		name    string
		rawURL  string
		wantErr string
	}{
		{name: "http_scheme", rawURL: "http://example.com/v1/chat/completions", wantErr: "only HTTPS is allowed"},
		{name: "ftp_scheme", rawURL: "ftp://example.com/file", wantErr: "only HTTPS is allowed"},
		{name: "missing_scheme", rawURL: "//example.com/v1/chat/completions", wantErr: "only HTTPS is allowed"},
		{name: "userinfo", rawURL: "https://user:pass@example.com/v1/chat/completions", wantErr: "userinfo not allowed"},
		{name: "missing_hostname", rawURL: "https:///v1/chat/completions", wantErr: "missing hostname"},
		{name: "ipv4_literal", rawURL: "https://127.0.0.1/v1/chat/completions", wantErr: "IP addresses not allowed"},
		{name: "ipv4_public", rawURL: "https://8.8.8.8/api", wantErr: "IP addresses not allowed"},
		{name: "ipv6_literal", rawURL: "https://[::1]/v1/chat/completions", wantErr: "IP addresses not allowed"},
		{name: "metadata_ip", rawURL: "https://169.254.169.254/latest/meta-data", wantErr: "IP addresses not allowed"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := guard.ValidateURL(tc.rawURL)
			if err == nil {
				t.Fatalf("ValidateURL(%q) unexpectedly succeeded", tc.rawURL)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ValidateURL(%q) error = %q, want substring %q", tc.rawURL, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestSSRFGuardValidateURLRejectsLocalhostAfterResolution(t *testing.T) {
	guard := NewSSRFGuard()

	err := guard.ValidateURL("https://localhost/v1/chat/completions")
	if err == nil {
		t.Fatal("ValidateURL(localhost) unexpectedly succeeded")
	}
	// localhost 解析取决于环境——某些 CI 可能没有 localhost 记录
	if strings.Contains(err.Error(), "DNS lookup failed") {
		t.Skipf("localhost did not resolve in this environment: %v", err)
	}
	if !strings.Contains(err.Error(), "private IP") {
		t.Fatalf("ValidateURL(localhost) error = %q, want private IP rejection", err.Error())
	}
}

func TestSSRFGuardIsIPAddress(t *testing.T) {
	guard := NewSSRFGuard()

	cases := []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"::1", true},
		{"2001:db8::1", true},
		{"999.999.999.999", true}, // regex 匹配但无效 IP，仍被认为是 IP 格式
		{"example.com", false},
		{"api.example.com", false},
		{"my-service", false},
	}

	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			if got := guard.isIPAddress(tc.host); got != tc.want {
				t.Fatalf("isIPAddress(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

func TestSSRFGuardIsPrivateIP(t *testing.T) {
	guard := NewSSRFGuard()

	cases := []struct {
		name string
		ip   string
		want bool
	}{
		// 私有 IPv4
		{"private_10", "10.1.2.3", true},
		{"private_10_boundary", "10.255.255.255", true},
		{"private_172", "172.16.5.10", true},
		{"private_172_boundary", "172.31.255.255", true},
		{"private_192", "192.168.1.20", true},
		// 特殊地址
		{"loopback_v4", "127.0.0.1", true},
		{"loopback_v4_other", "127.0.0.2", true},
		{"link_local_v4", "169.254.169.254", true},
		{"this_network", "0.0.0.0", true},
		// IPv6 私有/特殊
		{"loopback_v6", "::1", true},
		{"ula_v6", "fc00::1", true},
		{"ula_v6_fd", "fd12:3456::1", true},
		{"link_local_v6", "fe80::1", true},
		// 公网
		{"public_v4", "8.8.8.8", false},
		{"public_v4_2", "1.1.1.1", false},
		{"public_v6", "2001:4860:4860::8888", false},
		{"public_v6_2", "2607:f8b0:4004:800::200e", false},
		// 边界情况
		{"not_private_172", "172.32.0.1", false},
		{"not_private_172_2", "172.15.255.255", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("ParseIP(%q) returned nil", tc.ip)
			}
			if got := guard.isPrivateIP(ip); got != tc.want {
				t.Fatalf("isPrivateIP(%q) = %v, want %v", tc.ip, got, tc.want)
			}
		})
	}
}
