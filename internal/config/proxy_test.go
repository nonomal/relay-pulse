package config

import (
	"strings"
	"testing"
)

func TestValidateProxyURL(t *testing.T) {
	tests := []struct {
		name    string
		proxy   string
		wantErr bool
		errMsg  string // 部分匹配
	}{
		// 有效配置
		{name: "空字符串", proxy: "", wantErr: false},
		{name: "仅空格", proxy: "   ", wantErr: false},
		{name: "HTTP 代理", proxy: "http://proxy.example.com:8080", wantErr: false},
		{name: "HTTPS 代理", proxy: "https://proxy.example.com:8080", wantErr: false},
		{name: "HTTP 代理无端口", proxy: "http://proxy.example.com", wantErr: false}, // HTTP 允许无端口
		{name: "SOCKS5 代理", proxy: "socks5://proxy.example.com:1080", wantErr: false},
		{name: "SOCKS 代理（别名）", proxy: "socks://proxy.example.com:1080", wantErr: false},
		{name: "带认证的 SOCKS5", proxy: "socks5://user:pass@proxy.example.com:1080", wantErr: false},
		{name: "带特殊字符密码", proxy: "socks5://user:%23%23%23@proxy.example.com:1080", wantErr: false},
		{name: "尾部斜杠", proxy: "http://proxy.example.com:8080/", wantErr: false},

		// 无效配置 - 协议问题
		// 注意：url.Parse("proxy.example.com:8080") 会将 "proxy.example.com" 解析为 scheme
		// 所以实际触发的是"协议无效"而不是"缺少协议"，这是 Go URL 解析的行为
		{name: "缺少协议", proxy: "proxy.example.com:8080", wantErr: true, errMsg: "协议无效"},
		{name: "无效协议", proxy: "ftp://proxy.example.com:8080", wantErr: true, errMsg: "协议无效"},
		{name: "ws 协议", proxy: "ws://proxy.example.com:8080", wantErr: true, errMsg: "协议无效"},

		// 无效配置 - 主机问题
		{name: "缺少主机", proxy: "http://", wantErr: true, errMsg: "缺少主机"},
		{name: "空主机", proxy: "socks5://:1080", wantErr: true, errMsg: "缺少主机名"},

		// 无效配置 - SOCKS5 端口问题
		{name: "SOCKS5 无端口", proxy: "socks5://proxy.example.com", wantErr: true, errMsg: "必须指定端口"},
		{name: "SOCKS 无端口", proxy: "socks://proxy.example.com", wantErr: true, errMsg: "必须指定端口"},

		// 无效配置 - 路径/查询参数问题
		{name: "带路径", proxy: "http://proxy.example.com:8080/path", wantErr: true, errMsg: "不支持路径"},
		{name: "带查询参数", proxy: "http://proxy.example.com:8080?key=value", wantErr: true, errMsg: "不支持 query"},
		{name: "带 fragment", proxy: "http://proxy.example.com:8080#section", wantErr: true, errMsg: "不支持 query/fragment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProxyURL(tt.proxy)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateProxyURL(%q) 应该返回错误", tt.proxy)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateProxyURL(%q) 错误信息 = %q, 应包含 %q", tt.proxy, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateProxyURL(%q) 不应返回错误, got %v", tt.proxy, err)
				}
			}
		})
	}
}

func TestProxyNormalize(t *testing.T) {
	tests := []struct {
		name     string
		proxy    string
		expected string
	}{
		{name: "空字符串", proxy: "", expected: ""},
		{name: "仅空格", proxy: "   ", expected: ""},
		{name: "首尾空格", proxy: "  http://proxy:8080  ", expected: "http://proxy:8080"},
		{name: "大写 scheme", proxy: "HTTP://proxy:8080", expected: "http://proxy:8080"},
		{name: "混合大小写", proxy: "SoCkS5://proxy:1080", expected: "socks5://proxy:1080"},
		{name: "尾部斜杠", proxy: "http://proxy:8080/", expected: "http://proxy:8080"},
		{name: "已规范化", proxy: "socks5://user:pass@proxy:1080", expected: "socks5://user:pass@proxy:1080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AppConfig{
				Monitors: []ServiceConfig{
					{
						Provider: "test",
						Service:  "test",
						Channel:  "test",
						Category: "commercial",
						URL:      "http://test.com",
						Method:   "GET",
						Proxy:    tt.proxy,
					},
				},
			}

			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate() 失败: %v", err)
			}
			if err := cfg.Normalize(); err != nil {
				t.Fatalf("Normalize() 失败: %v", err)
			}

			if cfg.Monitors[0].Proxy != tt.expected {
				t.Errorf("Normalize() Proxy = %q, want %q", cfg.Monitors[0].Proxy, tt.expected)
			}
		})
	}
}

func TestProxyInheritance(t *testing.T) {
	tests := []struct {
		name          string
		parentProxy   string
		childProxy    string
		expectedProxy string
	}{
		{name: "子未配置时继承父", parentProxy: "socks5://proxy:1080", childProxy: "", expectedProxy: "socks5://proxy:1080"},
		{name: "子为空格时继承父", parentProxy: "socks5://proxy:1080", childProxy: "   ", expectedProxy: "socks5://proxy:1080"},
		{name: "子配置时覆盖父", parentProxy: "socks5://proxy:1080", childProxy: "http://other:8080", expectedProxy: "http://other:8080"},
		{name: "父子都未配置", parentProxy: "", childProxy: "", expectedProxy: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AppConfig{
				Monitors: []ServiceConfig{
					// 父通道
					{
						Provider: "test",
						Service:  "cc",
						Channel:  "main",
						Model:    "gpt-4",
						Category: "commercial",
						URL:      "http://test.com",
						Method:   "POST",
						Proxy:    tt.parentProxy,
					},
					// 子通道
					{
						Parent: "test/cc/main",
						Model:  "gpt-4-turbo",
						Proxy:  tt.childProxy,
					},
				},
			}

			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate() 失败: %v", err)
			}
			if err := cfg.Normalize(); err != nil {
				t.Fatalf("Normalize() 失败: %v", err)
			}

			childProxy := cfg.Monitors[1].Proxy
			if childProxy != tt.expectedProxy {
				t.Errorf("子通道 Proxy = %q, want %q", childProxy, tt.expectedProxy)
			}
		})
	}
}
