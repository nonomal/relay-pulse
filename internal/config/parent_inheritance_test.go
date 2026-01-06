package config

import (
	"strings"
	"testing"
)

func TestValidateMonitorsUniqueByQuadruple(t *testing.T) {
	cfg := &AppConfig{
		Monitors: []ServiceConfig{
			{
				Provider: "demo",
				Service:  "cc",
				Channel:  "vip",
				Model:    "gpt-4o",
				URL:      "https://example.com",
				Method:   "POST",
				Category: "public",
			},
			{
				Provider: "demo",
				Service:  "cc",
				Channel:  "vip",
				Model:    "gpt-4o",
				URL:      "https://example.com/2",
				Method:   "POST",
				Category: "public",
			},
		},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "重复的监测项") {
		t.Fatalf("期望重复的监测项错误, got=%v", err)
	}
}

func TestValidateParentRequiresModel(t *testing.T) {
	cfg := &AppConfig{
		Monitors: []ServiceConfig{
			{
				Provider: "demo",
				Service:  "cc",
				Channel:  "vip",
				Model:    "base",
				URL:      "https://example.com",
				Method:   "POST",
				Category: "public",
			},
			{
				Provider: "demo",
				Service:  "cc",
				Channel:  "vip",
				Parent:   "demo/cc/vip",
				Category: "public",
			},
		},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "有 parent 但缺少 model") {
		t.Fatalf("期望子通道缺少 model 报错, got=%v", err)
	}
}

func TestValidateReferencedParentRequiresModel(t *testing.T) {
	cfg := &AppConfig{
		Monitors: []ServiceConfig{
			{
				Provider: "demo",
				Service:  "cc",
				Channel:  "vip",
				URL:      "https://example.com",
				Method:   "POST",
				Category: "public",
			},
			{
				Provider: "demo",
				Service:  "cc",
				Channel:  "vip",
				Model:    "child",
				Parent:   "demo/cc/vip",
				Category: "public",
			},
		},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "被引用为父") {
		t.Fatalf("期望父通道缺少 model 报错, got=%v", err)
	}
}

func TestValidateParentFormatError(t *testing.T) {
	cfg := &AppConfig{
		Monitors: []ServiceConfig{
			{
				Provider: "demo",
				Service:  "cc",
				Channel:  "vip",
				Model:    "base",
				URL:      "https://example.com",
				Method:   "POST",
				Category: "public",
			},
			{
				Provider: "demo",
				Service:  "cc",
				Channel:  "vip",
				Model:    "child",
				Parent:   "bad-format",
				Category: "public",
			},
		},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "parent 格式错误") {
		t.Fatalf("期望 parent 格式错误, got=%v", err)
	}
}

func TestValidateParentNotFound(t *testing.T) {
	cfg := &AppConfig{
		Monitors: []ServiceConfig{
			{
				Provider: "demo",
				Service:  "cc",
				Channel:  "vip",
				Model:    "child",
				Parent:   "demo/cc/vip",
				Category: "public",
			},
		},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "找不到父通道") {
		t.Fatalf("期望找不到父通道错误, got=%v", err)
	}
}

func TestValidateParentMultipleDefinitions(t *testing.T) {
	cfg := &AppConfig{
		Monitors: []ServiceConfig{
			{
				Provider: "demo",
				Service:  "cc",
				Channel:  "vip",
				Model:    "base-a",
				URL:      "https://example.com",
				Method:   "POST",
				Category: "public",
			},
			{
				Provider: "demo",
				Service:  "cc",
				Channel:  "vip",
				Model:    "base-b",
				URL:      "https://example.com",
				Method:   "POST",
				Category: "public",
			},
			{
				Provider: "demo",
				Service:  "cc",
				Channel:  "vip",
				Model:    "child",
				Parent:   "demo/cc/vip",
				Category: "public",
			},
		},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "存在多个定义") {
		t.Fatalf("期望父通道多定义报错, got=%v", err)
	}
}

func TestNormalizeAppliesParentInheritanceAndTemplates(t *testing.T) {
	cfg := &AppConfig{
		Monitors: []ServiceConfig{
			{
				Provider:        "demo",
				Service:         "cc",
				Channel:         "vip",
				Model:           "base",
				URL:             "https://example.com",
				Method:          "POST",
				Category:        "public",
				APIKey:          "k",
				Headers:         map[string]string{"Authorization": "Bearer {{API_KEY}}", "X-Model": "{{MODEL}}"},
				Body:            `{"model":"{{MODEL}}","api_key":"{{API_KEY}}"}`,
				SuccessContains: "ok",
			},
			{
				Provider: "demo",
				Service:  "cc",
				Channel:  "vip",
				Model:    "child",
				Parent:   "demo/cc/vip",
				Category: "public",
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("Normalize() failed: %v", err)
	}

	child := &cfg.Monitors[1]
	if child.APIKey != "k" {
		t.Fatalf("child.APIKey = %q, want %q", child.APIKey, "k")
	}
	if child.URL != "https://example.com" {
		t.Fatalf("child.URL = %q, want %q", child.URL, "https://example.com")
	}
	if child.Method != "POST" {
		t.Fatalf("child.Method = %q, want %q", child.Method, "POST")
	}
	if child.SuccessContains != "ok" {
		t.Fatalf("child.SuccessContains = %q, want %q", child.SuccessContains, "ok")
	}
	if child.Body != `{"model":"{{MODEL}}","api_key":"{{API_KEY}}"}` {
		t.Fatalf("child.Body = %q, want inherited body", child.Body)
	}
	if child.Headers == nil || child.Headers["Authorization"] != "Bearer {{API_KEY}}" {
		t.Fatalf("child.Headers not inherited as expected, got=%v", child.Headers)
	}

	// 确保 headers 深拷贝（修改子不影响父）
	child.Headers["X-Test"] = "x"
	if _, exists := cfg.Monitors[0].Headers["X-Test"]; exists {
		t.Fatalf("期望 child.Headers 为深拷贝，但父 headers 被污染")
	}

	child.ProcessPlaceholders()
	if child.Headers["Authorization"] != "Bearer k" {
		t.Fatalf("Authorization placeholder not replaced, got=%q", child.Headers["Authorization"])
	}
	if child.Headers["X-Model"] != "child" {
		t.Fatalf("MODEL placeholder not replaced, got=%q", child.Headers["X-Model"])
	}
	if child.Body != `{"model":"child","api_key":"k"}` {
		t.Fatalf("Body placeholders not replaced, got=%q", child.Body)
	}
}
