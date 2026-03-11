package validator

import (
	"slices"
	"strings"
	"testing"
)

func TestDeriveStatusQueryURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        string
		wantErrPart string
	}{
		{
			name:  "标准 /api/events",
			input: "https://relaypulse.example/api/events",
			want:  "https://relaypulse.example/api/status/query",
		},
		{
			name:  "带尾部 slash",
			input: "https://relaypulse.example/api/events/",
			want:  "https://relaypulse.example/api/status/query",
		},
		{
			name:  "自定义路径前缀",
			input: "https://relaypulse.example/custom/prefix/api/events",
			want:  "https://relaypulse.example/custom/prefix/api/status/query",
		},
		{
			name:  "非标准路径回退根",
			input: "https://relaypulse.example/v2/events",
			want:  "https://relaypulse.example/api/status/query",
		},
		{
			name:  "查询参数被清除",
			input: "https://relaypulse.example/api/events?token=abc",
			want:  "https://relaypulse.example/api/status/query",
		},
		{
			name:  "带空格自动 trim",
			input: "  https://relaypulse.example/api/events  ",
			want:  "https://relaypulse.example/api/status/query",
		},
		{
			name:  "fragment 被清除",
			input: "https://relaypulse.example/api/events#section",
			want:  "https://relaypulse.example/api/status/query",
		},
		{
			name:        "缺少 scheme 和 host",
			input:       "/api/events",
			wantErrPart: "缺少 scheme 或 host",
		},
		{
			name:        "非法 URL 触发解析错误",
			input:       "://invalid",
			wantErrPart: "events_url 无效",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := deriveStatusQueryURL(tt.input)
			if tt.wantErrPart == "" {
				if err != nil {
					t.Fatalf("deriveStatusQueryURL(%q) error = %v", tt.input, err)
				}
				if got != tt.want {
					t.Fatalf("deriveStatusQueryURL(%q) = %q, want %q", tt.input, got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("deriveStatusQueryURL(%q) error = nil, want contains %q", tt.input, tt.wantErrPart)
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Fatalf("deriveStatusQueryURL(%q) error = %q, want contains %q", tt.input, err.Error(), tt.wantErrPart)
			}
		})
	}
}

func TestFormatCandidates(t *testing.T) {
	tests := []struct {
		name       string
		candidates []string
		kind       string
		want       string
	}{
		{
			name: "空列表",
			kind: "service",
			want: "",
		},
		{
			name:       "单元素带空白",
			candidates: []string{"  openai  "},
			kind:       "service",
			want:       "openai",
		},
		{
			name:       "多元素",
			candidates: []string{"openai", "anthropic", "azure"},
			kind:       "service",
			want:       "openai、anthropic、azure",
		},
		{
			name:       "空 channel 显示 default",
			candidates: []string{"stable", "", "beta"},
			kind:       "channel",
			want:       "stable、default、beta",
		},
		{
			name:       "空 service 不显示 default",
			candidates: []string{"openai", "", "azure"},
			kind:       "service",
			want:       "openai、azure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCandidates(tt.candidates, tt.kind)
			if got != tt.want {
				t.Fatalf("FormatCandidates(%#v, %q) = %q, want %q", tt.candidates, tt.kind, got, tt.want)
			}
		})
	}
}

func TestParseNotFoundLevel(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want NotFoundLevel
	}{
		{name: "精确匹配 provider", msg: "provider 不存在", want: NotFoundProvider},
		{name: "精确匹配 service", msg: "service 不存在", want: NotFoundService},
		{name: "精确匹配 channel", msg: "channel 不存在", want: NotFoundChannel},
		{name: "关键词回退 provider", msg: "provider foo missing", want: NotFoundProvider},
		{name: "关键词回退 channel", msg: "CHANNEL foo missing", want: NotFoundChannel},
		{name: "默认回退 service", msg: "unknown target", want: NotFoundService},
		{name: "带空白 trim", msg: "  provider 不存在  ", want: NotFoundProvider},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseNotFoundLevel(tt.msg)
			if got != tt.want {
				t.Fatalf("parseNotFoundLevel(%q) = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}

func TestLimitSlice(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		max   int
		want  []string
	}{
		{
			name:  "不截断",
			input: []string{"a", "b"},
			max:   3,
			want:  []string{"a", "b"},
		},
		{
			name:  "恰好等于 max",
			input: []string{"a", "b", "c"},
			max:   3,
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "截断",
			input: []string{"a", "b", "c", "d"},
			max:   2,
			want:  []string{"a", "b"},
		},
		{
			name:  "空切片",
			input: []string{},
			max:   2,
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := limitSlice(tt.input, tt.max)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("limitSlice(%#v, %d) = %#v, want %#v", tt.input, tt.max, got, tt.want)
			}
		})
	}
}
