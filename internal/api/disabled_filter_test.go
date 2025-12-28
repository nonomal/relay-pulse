package api

import (
	"testing"

	"monitor/internal/config"
)

// TestFilterMonitorsDisabled 测试 filterMonitors 对 disabled 的过滤
func TestFilterMonitorsDisabled(t *testing.T) {
	h := &Handler{
		config: &config.AppConfig{},
	}

	monitors := []config.ServiceConfig{
		{Provider: "active-provider", Service: "cc", Board: "hot", Disabled: false, Hidden: false},
		{Provider: "disabled-provider", Service: "cc", Board: "hot", Disabled: true, Hidden: true},
		{Provider: "hidden-provider", Service: "cc", Board: "hot", Disabled: false, Hidden: true},
	}

	t.Run("默认模式：只返回活跃的", func(t *testing.T) {
		result := h.filterMonitors(monitors, "all", "all", "all", false, false)
		if len(result) != 1 {
			t.Errorf("期望返回 1 个监测项，实际返回 %d 个", len(result))
		}
		if len(result) > 0 && result[0].Provider != "active-provider" {
			t.Errorf("期望返回 active-provider，实际返回 %s", result[0].Provider)
		}
	})

	t.Run("include_hidden=true：返回活跃和隐藏的，但不包括禁用的", func(t *testing.T) {
		result := h.filterMonitors(monitors, "all", "all", "all", false, true)
		if len(result) != 2 {
			t.Errorf("期望返回 2 个监测项，实际返回 %d 个", len(result))
		}

		// 确认禁用的不在结果中
		for _, m := range result {
			if m.Disabled {
				t.Errorf("禁用的监测项不应该出现在结果中: %s", m.Provider)
			}
		}
	})

	t.Run("按 provider 过滤", func(t *testing.T) {
		// 尝试获取禁用的 provider，应该返回空
		result := h.filterMonitors(monitors, "disabled-provider", "all", "all", false, true)
		if len(result) != 0 {
			t.Errorf("禁用的 provider 应该返回空列表，实际返回 %d 个", len(result))
		}

		// 获取隐藏但未禁用的 provider
		result = h.filterMonitors(monitors, "hidden-provider", "all", "all", false, true)
		if len(result) != 1 {
			t.Errorf("隐藏的 provider 应该返回 1 个，实际返回 %d 个", len(result))
		}
	})
}

// TestExtractUniqueProviderSlugsDisabled 测试 extractUniqueProviderSlugs 对 disabled 的过滤
func TestExtractUniqueProviderSlugsDisabled(t *testing.T) {
	h := &Handler{}

	monitors := []config.ServiceConfig{
		{Provider: "active-provider", ProviderSlug: "active", Disabled: false, Hidden: false},
		{Provider: "disabled-provider", ProviderSlug: "disabled", Disabled: true, Hidden: true},
		{Provider: "hidden-provider", ProviderSlug: "hidden", Disabled: false, Hidden: true},
	}

	slugs := h.extractUniqueProviderSlugs(monitors)

	// 应该只返回活跃的 provider slug
	if len(slugs) != 1 {
		t.Errorf("期望返回 1 个 slug，实际返回 %d 个: %v", len(slugs), slugs)
	}

	if len(slugs) > 0 && slugs[0] != "active" {
		t.Errorf("期望返回 'active'，实际返回 %s", slugs[0])
	}

	// 确认禁用和隐藏的 slug 不在结果中
	for _, slug := range slugs {
		if slug == "disabled" {
			t.Errorf("禁用的 slug 不应该出现在结果中")
		}
		if slug == "hidden" {
			t.Errorf("隐藏的 slug 不应该出现在结果中")
		}
	}
}

// TestFilterMonitorsDedupe 测试 filterMonitors 去重逻辑
func TestFilterMonitorsDedupe(t *testing.T) {
	h := &Handler{}

	monitors := []config.ServiceConfig{
		{Provider: "provider-a", Service: "cc", Channel: "ch1", Board: "hot", Disabled: false, Hidden: false},
		{Provider: "provider-a", Service: "cc", Channel: "ch1", Board: "hot", Disabled: false, Hidden: false}, // 重复
		{Provider: "provider-a", Service: "cc", Channel: "ch2", Board: "hot", Disabled: false, Hidden: false}, // 不同 channel，不重复
	}

	result := h.filterMonitors(monitors, "all", "all", "all", false, false)
	if len(result) != 2 {
		t.Errorf("期望返回 2 个监测项（去重后），实际返回 %d 个", len(result))
	}
}

// TestFilterMonitorsBoard 测试 filterMonitors 对 board 的过滤
func TestFilterMonitorsBoard(t *testing.T) {
	h := &Handler{}

	monitors := []config.ServiceConfig{
		{Provider: "hot-provider", Service: "cc", Board: "hot", Disabled: false, Hidden: false},
		{Provider: "cold-provider", Service: "cc", Board: "cold", ColdReason: "测试冷板原因", Disabled: false, Hidden: false},
	}

	t.Run("boards未启用：返回全部", func(t *testing.T) {
		result := h.filterMonitors(monitors, "all", "all", "hot", false, false)
		if len(result) != 2 {
			t.Errorf("boards未启用时应返回全部，期望 2 个，实际返回 %d 个", len(result))
		}
	})

	t.Run("boards启用+hot：只返回热板", func(t *testing.T) {
		result := h.filterMonitors(monitors, "all", "all", "hot", true, false)
		if len(result) != 1 {
			t.Errorf("期望返回 1 个热板监测项，实际返回 %d 个", len(result))
		}
		if len(result) > 0 && result[0].Provider != "hot-provider" {
			t.Errorf("期望返回 hot-provider，实际返回 %s", result[0].Provider)
		}
	})

	t.Run("boards启用+cold：只返回冷板", func(t *testing.T) {
		result := h.filterMonitors(monitors, "all", "all", "cold", true, false)
		if len(result) != 1 {
			t.Errorf("期望返回 1 个冷板监测项，实际返回 %d 个", len(result))
		}
		if len(result) > 0 && result[0].Provider != "cold-provider" {
			t.Errorf("期望返回 cold-provider，实际返回 %s", result[0].Provider)
		}
		if len(result) > 0 && result[0].ColdReason != "测试冷板原因" {
			t.Errorf("期望 cold_reason 为 '测试冷板原因'，实际返回 %s", result[0].ColdReason)
		}
	})

	t.Run("boards启用+all：返回全部", func(t *testing.T) {
		result := h.filterMonitors(monitors, "all", "all", "all", true, false)
		if len(result) != 2 {
			t.Errorf("board=all时应返回全部，期望 2 个，实际返回 %d 个", len(result))
		}
	})
}
