package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestResolveBodyIncludes(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	dataDir := filepath.Join(configDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("创建 data 目录失败: %v", err)
	}

	expected := `{"hello":"world"}`
	payloadPath := filepath.Join(dataDir, "payload.json")
	if err := os.WriteFile(payloadPath, []byte(expected), 0o644); err != nil {
		t.Fatalf("写入 payload 失败: %v", err)
	}

	cfg := AppConfig{
		Monitors: []ServiceConfig{
			{
				Provider: "demo",
				Service:  "codex",
				Body:     "!include data/payload.json",
			},
		},
	}

	if err := cfg.ResolveBodyIncludes(configDir); err != nil {
		t.Fatalf("解析 include 失败: %v", err)
	}

	if cfg.Monitors[0].Body != expected {
		t.Fatalf("body 解析结果不符合预期，got=%s", cfg.Monitors[0].Body)
	}
}

func TestResolveBodyIncludesRejectsOutsideData(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	cfg := AppConfig{
		Monitors: []ServiceConfig{
			{
				Provider: "demo",
				Service:  "codex",
				Body:     "!include ../secret.json",
			},
		},
	}

	if err := cfg.ResolveBodyIncludes(configDir); err == nil {
		t.Fatalf("期望 include 非 data 目录时报错")
	}
}

// Test consecutive hyphens in slug
func TestConsecutiveHyphensSlug(t *testing.T) {
	tests := []struct {
		name      string
		slug      string
		shouldErr bool
	}{
		{"单连字符", "easy-chat", false},
		{"连续两个连字符", "easy--chat", true},
		{"连续三个连字符", "easy---chat", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProviderSlug(tt.slug)
			if tt.shouldErr && err == nil {
				t.Errorf("validateProviderSlug(%q) should return error", tt.slug)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("validateProviderSlug(%q) should not return error, got: %v", tt.slug, err)
			}
		})
	}
}

// Test baseURL normalization
func TestBaseURLNormalization(t *testing.T) {
	tests := []struct {
		name        string
		inputURL    string
		expectedURL string
		shouldErr   bool
	}{
		{"正常 HTTPS URL", "https://relaypulse.top", "https://relaypulse.top", false},
		{"带尾随斜杠", "https://relaypulse.top/", "https://relaypulse.top", false},
		{"多个尾随斜杠", "https://relaypulse.top///", "https://relaypulse.top", false},
		{"HTTP URL（警告）", "http://example.com", "http://example.com", false},
		{"无效协议", "ftp://example.com", "", true},
		{"缺少协议", "example.com", "", true},
		{"缺少主机", "https://", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AppConfig{PublicBaseURL: tt.inputURL}
			err := cfg.Normalize()

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Normalize() should return error for %q", tt.inputURL)
				}
			} else {
				if err != nil {
					t.Errorf("Normalize() should not return error for %q, got: %v", tt.inputURL, err)
				}
				if cfg.PublicBaseURL != tt.expectedURL {
					t.Errorf("Normalize() URL = %q, want %q", cfg.PublicBaseURL, tt.expectedURL)
				}
			}
		})
	}
}

// TestBadgeRefUnmarshalYAML tests BadgeRef YAML parsing (string and object formats)
func TestBadgeRefUnmarshalYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		yamlInput       string
		expectedID      string
		expectedTooltip string
		shouldErr       bool
	}{
		{
			name:            "字符串格式",
			yamlInput:       `"api_key_official"`,
			expectedID:      "api_key_official",
			expectedTooltip: "",
			shouldErr:       false,
		},
		{
			name:            "对象格式无 tooltip",
			yamlInput:       `id: "api_key_user"`,
			expectedID:      "api_key_user",
			expectedTooltip: "",
			shouldErr:       false,
		},
		{
			name:            "对象格式带 tooltip",
			yamlInput:       "id: \"api_key_user\"\ntooltip_override: \"由 @zhangsan 贡献\"",
			expectedID:      "api_key_user",
			expectedTooltip: "由 @zhangsan 贡献",
			shouldErr:       false,
		},
		{
			name:            "字符串格式带空格",
			yamlInput:       `"  api_key_official  "`,
			expectedID:      "api_key_official",
			expectedTooltip: "",
			shouldErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ref BadgeRef
			err := yaml.Unmarshal([]byte(tt.yamlInput), &ref)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Unmarshal should return error for %q", tt.yamlInput)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if ref.ID != tt.expectedID {
				t.Errorf("ID = %q, want %q", ref.ID, tt.expectedID)
			}
			if ref.Tooltip != tt.expectedTooltip {
				t.Errorf("Tooltip = %q, want %q", ref.Tooltip, tt.expectedTooltip)
			}
		})
	}
}

// TestBadgeKindVariantValidation tests BadgeKind and BadgeVariant IsValid methods
func TestBadgeKindVariantValidation(t *testing.T) {
	t.Parallel()

	kindTests := []struct {
		kind    BadgeKind
		isValid bool
	}{
		{BadgeKindSource, true},
		{BadgeKindInfo, true},
		{BadgeKindFeature, true},
		{BadgeKind("invalid"), false},
		{BadgeKind(""), false},
	}

	for _, tt := range kindTests {
		t.Run("Kind_"+string(tt.kind), func(t *testing.T) {
			if got := tt.kind.IsValid(); got != tt.isValid {
				t.Errorf("BadgeKind(%q).IsValid() = %v, want %v", tt.kind, got, tt.isValid)
			}
		})
	}

	variantTests := []struct {
		variant BadgeVariant
		isValid bool
	}{
		{BadgeVariantDefault, true},
		{BadgeVariantSuccess, true},
		{BadgeVariantWarning, true},
		{BadgeVariantDanger, true},
		{BadgeVariantInfo, true},
		{BadgeVariant("invalid"), false},
		{BadgeVariant(""), false},
	}

	for _, tt := range variantTests {
		t.Run("Variant_"+string(tt.variant), func(t *testing.T) {
			if got := tt.variant.IsValid(); got != tt.isValid {
				t.Errorf("BadgeVariant(%q).IsValid() = %v, want %v", tt.variant, got, tt.isValid)
			}
		})
	}
}

// TestBadgesValidation tests Validate() for badges configuration
func TestBadgesValidation(t *testing.T) {
	t.Parallel()

	baseMonitor := ServiceConfig{
		Provider: "demo",
		Service:  "cc",
		Category: "commercial",
		Sponsor:  "test",
		URL:      "https://example.com",
		Method:   "POST",
	}

	tests := []struct {
		name      string
		config    AppConfig
		shouldErr bool
		errMsg    string
	}{
		{
			name: "有效的 badges 定义",
			config: AppConfig{
				BadgeDefs: map[string]BadgeDef{
					"api_key_user":     {Kind: BadgeKindSource, Variant: BadgeVariantDefault, Weight: 5},
					"api_key_official": {Kind: BadgeKindSource, Variant: BadgeVariantSuccess, Weight: 10},
				},
				Monitors: []ServiceConfig{baseMonitor},
			},
			shouldErr: false,
		},
		{
			name: "badges kind 无效",
			config: AppConfig{
				BadgeDefs: map[string]BadgeDef{
					"test": {Kind: BadgeKind("invalid"), Variant: BadgeVariantDefault},
				},
				Monitors: []ServiceConfig{baseMonitor},
			},
			shouldErr: true,
			errMsg:    "kind",
		},
		{
			name: "badges variant 无效",
			config: AppConfig{
				BadgeDefs: map[string]BadgeDef{
					"test": {Kind: BadgeKindSource, Variant: BadgeVariant("invalid")},
				},
				Monitors: []ServiceConfig{baseMonitor},
			},
			shouldErr: true,
			errMsg:    "variant",
		},
		{
			name: "badges weight 超出范围",
			config: AppConfig{
				BadgeDefs: map[string]BadgeDef{
					"test": {Kind: BadgeKindSource, Variant: BadgeVariantDefault, Weight: 101},
				},
				Monitors: []ServiceConfig{baseMonitor},
			},
			shouldErr: true,
			errMsg:    "weight",
		},
		{
			name: "badge_providers 引用不存在的 badge",
			config: AppConfig{
				BadgeDefs: map[string]BadgeDef{
					"api_key_user": {Kind: BadgeKindSource, Variant: BadgeVariantDefault},
				},
				BadgeProviders: []BadgeProviderConfig{
					{Provider: "demo", Badges: []BadgeRef{{ID: "nonexistent"}}},
				},
				Monitors: []ServiceConfig{baseMonitor},
			},
			shouldErr: true,
			errMsg:    "未找到徽标定义",
		},
		{
			name: "monitors.badges 引用不存在的 badge",
			config: AppConfig{
				BadgeDefs: map[string]BadgeDef{
					"api_key_user": {Kind: BadgeKindSource, Variant: BadgeVariantDefault},
				},
				Monitors: []ServiceConfig{
					{
						Provider: "demo",
						Service:  "cc",
						Category: "commercial",
						Sponsor:  "test",
						URL:      "https://example.com",
						Method:   "POST",
						Badges:   []BadgeRef{{ID: "nonexistent"}},
					},
				},
			},
			shouldErr: true,
			errMsg:    "未找到徽标定义",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Validate() should return error")
				} else if tt.errMsg != "" && !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %q, should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() should not return error, got: %v", err)
				}
			}
		})
	}
}

// TestBadgesNormalize tests Normalize() for badge injection and merging
func TestBadgesNormalize(t *testing.T) {
	t.Parallel()

	cfg := &AppConfig{
		BadgeDefs: map[string]BadgeDef{
			"api_key_user":      {Kind: BadgeKindSource, Variant: BadgeVariantDefault, Weight: 5},
			"api_key_official":  {Kind: BadgeKindSource, Variant: BadgeVariantSuccess, Weight: 10},
			"feature_streaming": {Kind: BadgeKindFeature, Variant: BadgeVariantDefault, Weight: 3},
		},
		BadgeProviders: []BadgeProviderConfig{
			{Provider: "demo", Badges: []BadgeRef{{ID: "api_key_official"}}},
		},
		Monitors: []ServiceConfig{
			{
				Provider: "demo",
				Service:  "cc",
				Category: "commercial",
				Sponsor:  "test",
				URL:      "https://example.com",
				Method:   "POST",
				// Monitor 级覆盖 tooltip
				Badges: []BadgeRef{
					{ID: "api_key_user", Tooltip: "由 @zhangsan 贡献"},
					{ID: "feature_streaming"},
				},
			},
			{
				Provider: "other",
				Service:  "chat",
				Category: "public",
				Sponsor:  "community",
				URL:      "https://other.com",
				Method:   "POST",
				// 仅使用 provider 级注入
			},
		},
	}

	if err := cfg.Normalize(); err != nil {
		t.Fatalf("Normalize() failed: %v", err)
	}

	// 验证 demo 监测项的徽标
	demoMonitor := cfg.Monitors[0]
	if len(demoMonitor.ResolvedBadges) != 3 {
		t.Fatalf("demo monitor should have 3 badges, got %d", len(demoMonitor.ResolvedBadges))
	}

	// 验证排序：source kind 在前 (weight desc)，feature 在后
	// 期望顺序：api_key_official (source, weight=10), api_key_user (source, weight=5), feature_streaming (feature, weight=3)
	expectedOrder := []string{"api_key_official", "api_key_user", "feature_streaming"}
	for i, expected := range expectedOrder {
		if demoMonitor.ResolvedBadges[i].ID != expected {
			t.Errorf("Badge[%d].ID = %q, want %q", i, demoMonitor.ResolvedBadges[i].ID, expected)
		}
	}

	// 验证 tooltip 覆盖
	for _, badge := range demoMonitor.ResolvedBadges {
		if badge.ID == "api_key_user" {
			if badge.TooltipOverride != "由 @zhangsan 贡献" {
				t.Errorf("api_key_user TooltipOverride = %q, want %q", badge.TooltipOverride, "由 @zhangsan 贡献")
			}
		}
	}

	// 验证 other 监测项无徽标（其 provider 不在 badge_providers 中）
	otherMonitor := cfg.Monitors[1]
	if len(otherMonitor.ResolvedBadges) != 0 {
		t.Errorf("other monitor should have 0 badges, got %d", len(otherMonitor.ResolvedBadges))
	}
}

// TestBadgesClone tests Clone() for badges deep copy
func TestBadgesClone(t *testing.T) {
	t.Parallel()

	cfg := &AppConfig{
		BadgeDefs: map[string]BadgeDef{
			"test": {Kind: BadgeKindSource, Variant: BadgeVariantDefault},
		},
		BadgeProviders: []BadgeProviderConfig{
			{Provider: "demo", Badges: []BadgeRef{{ID: "test"}}},
		},
		Monitors: []ServiceConfig{
			{
				Provider: "demo",
				Service:  "cc",
				Badges:   []BadgeRef{{ID: "test", Tooltip: "original"}},
				ResolvedBadges: []ResolvedBadge{
					{ID: "test", Kind: BadgeKindSource, Variant: BadgeVariantDefault, TooltipOverride: "original"},
				},
			},
		},
	}

	clone := cfg.Clone()

	// 修改原始配置（map 需要通过临时变量修改）
	modifiedDef := cfg.BadgeDefs["test"]
	modifiedDef.Kind = BadgeKindFeature
	cfg.BadgeDefs["test"] = modifiedDef
	cfg.BadgeProviders[0].Badges[0].ID = "modified"
	cfg.Monitors[0].Badges[0].Tooltip = "modified"
	cfg.Monitors[0].ResolvedBadges[0].TooltipOverride = "modified"

	// 验证克隆不受影响
	if clone.BadgeDefs["test"].Kind != BadgeKindSource {
		t.Errorf("Clone BadgeDefs should not be affected, got Kind = %q", clone.BadgeDefs["test"].Kind)
	}
	if clone.BadgeProviders[0].Badges[0].ID != "test" {
		t.Errorf("Clone BadgeProviders should not be affected, got ID = %q", clone.BadgeProviders[0].Badges[0].ID)
	}
	if clone.Monitors[0].Badges[0].Tooltip != "original" {
		t.Errorf("Clone Monitors.Badges should not be affected, got Tooltip = %q", clone.Monitors[0].Badges[0].Tooltip)
	}
	if clone.Monitors[0].ResolvedBadges[0].TooltipOverride != "original" {
		t.Errorf("Clone Monitors.ResolvedBadges should not be affected, got TooltipOverride = %q", clone.Monitors[0].ResolvedBadges[0].TooltipOverride)
	}
}

// containsSubstring checks if s contains substr
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
