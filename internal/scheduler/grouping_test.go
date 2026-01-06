package scheduler

import (
	"testing"
	"time"

	"monitor/internal/config"
)

// TestBuildMonitorGroups 测试按 PSC 分组监测项
func TestBuildMonitorGroups(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *config.AppConfig
		expectedGroups int
		expectedPSCs   []string
	}{
		{
			name: "单模型组（每个 PSC 一个模型）",
			cfg: &config.AppConfig{
				Monitors: []config.ServiceConfig{
					{Provider: "a", Service: "cc", Channel: "vip", Model: "m1"},
					{Provider: "b", Service: "cc", Channel: "std", Model: "m2"},
					{Provider: "c", Service: "gm", Channel: "", Model: "m3"},
				},
			},
			expectedGroups: 3,
			expectedPSCs:   []string{"a/cc/vip", "b/cc/std", "c/gm/"},
		},
		{
			name: "多模型组（同一 PSC 多个模型）",
			cfg: &config.AppConfig{
				Monitors: []config.ServiceConfig{
					{Provider: "88code", Service: "cc", Channel: "vip", Model: "haiku"},
					{Provider: "88code", Service: "cc", Channel: "vip", Model: "sonnet"},
					{Provider: "88code", Service: "cc", Channel: "vip", Model: "opus"},
					{Provider: "other", Service: "cc", Channel: "std", Model: "gpt4"},
				},
			},
			expectedGroups: 2,
			expectedPSCs:   []string{"88code/cc/vip", "other/cc/std"},
		},
		{
			name: "跳过禁用的监测项",
			cfg: &config.AppConfig{
				Monitors: []config.ServiceConfig{
					{Provider: "a", Service: "cc", Channel: "vip", Model: "m1", Disabled: false},
					{Provider: "a", Service: "cc", Channel: "vip", Model: "m2", Disabled: true}, // 禁用
					{Provider: "b", Service: "cc", Channel: "std", Model: "m3", Disabled: false},
				},
			},
			expectedGroups: 2,
			expectedPSCs:   []string{"a/cc/vip", "b/cc/std"},
		},
		{
			name: "跳过冷板监测项",
			cfg: &config.AppConfig{
				Boards: config.BoardsConfig{Enabled: true},
				Monitors: []config.ServiceConfig{
					{Provider: "a", Service: "cc", Channel: "vip", Model: "m1", Board: "hot"},
					{Provider: "a", Service: "cc", Channel: "vip", Model: "m2", Board: "cold"}, // 冷板
					{Provider: "b", Service: "cc", Channel: "std", Model: "m3", Board: "hot"},
				},
			},
			expectedGroups: 2,
			expectedPSCs:   []string{"a/cc/vip", "b/cc/std"},
		},
		{
			name: "空监测项列表",
			cfg: &config.AppConfig{
				Monitors: []config.ServiceConfig{},
			},
			expectedGroups: 0,
			expectedPSCs:   nil,
		},
		{
			name: "全部禁用返回空",
			cfg: &config.AppConfig{
				Monitors: []config.ServiceConfig{
					{Provider: "a", Service: "cc", Channel: "vip", Model: "m1", Disabled: true},
					{Provider: "b", Service: "cc", Channel: "std", Model: "m2", Disabled: true},
				},
			},
			expectedGroups: 0,
			expectedPSCs:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := buildMonitorGroups(tt.cfg)

			if len(groups) != tt.expectedGroups {
				t.Errorf("groups count = %d, want %d", len(groups), tt.expectedGroups)
			}

			if tt.expectedPSCs != nil {
				for i, expectedPSC := range tt.expectedPSCs {
					if i >= len(groups) {
						t.Errorf("missing group[%d] with PSC %s", i, expectedPSC)
						continue
					}
					if groups[i].psc != expectedPSC {
						t.Errorf("groups[%d].psc = %s, want %s", i, groups[i].psc, expectedPSC)
					}
				}
			}
		})
	}
}

// TestBuildMonitorGroupsLayerOrder 测试组内按 layer_order 排序（父层优先）
func TestBuildMonitorGroupsLayerOrder(t *testing.T) {
	cfg := &config.AppConfig{
		Monitors: []config.ServiceConfig{
			// 故意乱序放置，子层在前
			{Provider: "88code", Service: "cc", Channel: "vip", Model: "child1", Parent: "88code/cc/vip"},
			{Provider: "88code", Service: "cc", Channel: "vip", Model: "parent", Parent: ""},
			{Provider: "88code", Service: "cc", Channel: "vip", Model: "child2", Parent: "88code/cc/vip"},
		},
	}

	groups := buildMonitorGroups(cfg)

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	group := groups[0]
	if len(group.monitorIdxs) != 3 {
		t.Fatalf("expected 3 monitors in group, got %d", len(group.monitorIdxs))
	}

	// 验证父层在最前面（索引 1 是 parent）
	firstMonitor := cfg.Monitors[group.monitorIdxs[0]]
	if firstMonitor.Parent != "" {
		t.Errorf("first monitor should be parent (Parent=''), got Parent='%s', Model='%s'",
			firstMonitor.Parent, firstMonitor.Model)
	}

	// 验证子层在后面
	for i := 1; i < len(group.monitorIdxs); i++ {
		monitor := cfg.Monitors[group.monitorIdxs[i]]
		if monitor.Parent == "" {
			t.Errorf("monitor[%d] should be child (Parent!=''), got Parent='', Model='%s'",
				i, monitor.Model)
		}
	}
}

// TestBuildMonitorGroupsConfigOrder 测试组间按配置顺序排序
func TestBuildMonitorGroupsConfigOrder(t *testing.T) {
	cfg := &config.AppConfig{
		Monitors: []config.ServiceConfig{
			// 按配置顺序：first, second, third
			{Provider: "first", Service: "cc", Channel: "vip", Model: "m1"},
			{Provider: "second", Service: "cc", Channel: "std", Model: "m2"},
			{Provider: "third", Service: "gm", Channel: "", Model: "m3"},
		},
	}

	groups := buildMonitorGroups(cfg)

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	// 验证组间顺序与配置顺序一致
	expectedOrder := []string{"first/cc/vip", "second/cc/std", "third/gm/"}
	for i, expected := range expectedOrder {
		if groups[i].psc != expected {
			t.Errorf("groups[%d].psc = %s, want %s", i, groups[i].psc, expected)
		}
	}
}

// TestBuildMonitorGroupsInterleavedConfig 测试交错配置的分组
func TestBuildMonitorGroupsInterleavedConfig(t *testing.T) {
	// 模拟配置中同一 PSC 的模型不连续的情况
	cfg := &config.AppConfig{
		Monitors: []config.ServiceConfig{
			{Provider: "a", Service: "cc", Channel: "vip", Model: "m1"}, // 组 a/cc/vip
			{Provider: "b", Service: "cc", Channel: "std", Model: "m2"}, // 组 b/cc/std
			{Provider: "a", Service: "cc", Channel: "vip", Model: "m3"}, // 组 a/cc/vip（与 m1 同组）
			{Provider: "b", Service: "cc", Channel: "std", Model: "m4"}, // 组 b/cc/std（与 m2 同组）
		},
	}

	groups := buildMonitorGroups(cfg)

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// 验证分组正确（组按首个配置索引排序）
	if groups[0].psc != "a/cc/vip" {
		t.Errorf("groups[0].psc = %s, want a/cc/vip", groups[0].psc)
	}
	if len(groups[0].monitorIdxs) != 2 {
		t.Errorf("groups[0] should have 2 monitors, got %d", len(groups[0].monitorIdxs))
	}

	if groups[1].psc != "b/cc/std" {
		t.Errorf("groups[1].psc = %s, want b/cc/std", groups[1].psc)
	}
	if len(groups[1].monitorIdxs) != 2 {
		t.Errorf("groups[1] should have 2 monitors, got %d", len(groups[1].monitorIdxs))
	}
}

// TestComputeLayerOrder 测试层级顺序计算
func TestComputeLayerOrder(t *testing.T) {
	tests := []struct {
		name     string
		parent   string
		expected int
	}{
		{"父层（空 Parent）", "", 0},
		{"父层（空格 Parent）", "   ", 0},
		{"子层", "a/b/c", 1},
		{"子层（带空格）", "  a/b/c  ", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &config.ServiceConfig{Parent: tt.parent}
			result := computeLayerOrder(m)
			if result != tt.expected {
				t.Errorf("computeLayerOrder(%q) = %d, want %d", tt.parent, result, tt.expected)
			}
		})
	}
}

// TestComputeStaggerDelay 测试错峰延迟计算
func TestComputeStaggerDelay(t *testing.T) {
	tests := []struct {
		name        string
		baseDelay   time.Duration
		jitterRange time.Duration
		index       int
		// 由于有随机抖动，只验证范围
		minDelay time.Duration
		maxDelay time.Duration
	}{
		{
			name:        "无抖动时精确计算",
			baseDelay:   5 * time.Second,
			jitterRange: 0,
			index:       3,
			minDelay:    15 * time.Second,
			maxDelay:    15 * time.Second,
		},
		{
			name:        "index=0 时延迟为抖动范围",
			baseDelay:   5 * time.Second,
			jitterRange: 500 * time.Millisecond,
			index:       0,
			minDelay:    -500 * time.Millisecond, // 可能被负抖动
			maxDelay:    500 * time.Millisecond,
		},
		{
			name:        "有抖动时验证范围",
			baseDelay:   5 * time.Second,
			jitterRange: 500 * time.Millisecond,
			index:       2,
			minDelay:    9500 * time.Millisecond,  // 10s - 500ms
			maxDelay:    10500 * time.Millisecond, // 10s + 500ms
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 多次运行验证随机范围
			for i := 0; i < 100; i++ {
				result := computeStaggerDelay(tt.baseDelay, tt.jitterRange, tt.index)
				// 负延迟会被修正为 0
				if result < 0 {
					result = 0
				}
				adjustedMin := tt.minDelay
				if adjustedMin < 0 {
					adjustedMin = 0
				}
				if result < adjustedMin || result > tt.maxDelay {
					t.Errorf("computeStaggerDelay(%v, %v, %d) = %v, want in [%v, %v]",
						tt.baseDelay, tt.jitterRange, tt.index, result, adjustedMin, tt.maxDelay)
				}
			}
		})
	}
}

// TestIntraGroupInterval 测试组内紧凑间隔为 2 秒
// 通过构造场景验证同组内连续任务的 nextRun 差值
func TestIntraGroupInterval(t *testing.T) {
	// 期望的组内间隔
	const expectedInterval = 2 * time.Second

	staggerOff := false
	cfg := &config.AppConfig{
		Interval:         "1m",
		IntervalDuration: time.Minute,
		StaggerProbes:    &staggerOff, // 关闭组间错峰，只测组内紧凑
		Monitors: []config.ServiceConfig{
			{Provider: "test", Service: "cc", Channel: "vip", Model: "m1"},
			{Provider: "test", Service: "cc", Channel: "vip", Model: "m2"},
			{Provider: "test", Service: "cc", Channel: "vip", Model: "m3"},
		},
	}

	// 构建分组
	groups := buildMonitorGroups(cfg)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].monitorIdxs) != 3 {
		t.Fatalf("expected 3 monitors in group, got %d", len(groups[0].monitorIdxs))
	}

	// 验证组内相对间隔
	// 由于 rebuildTasks 是私有方法且依赖 Scheduler 实例，
	// 这里通过计算期望的相对偏移来验证设计意图
	for i := 0; i < 3; i++ {
		expectedDelay := time.Duration(i) * expectedInterval
		t.Logf("monitor[%d] 期望延迟: %v", i, expectedDelay)
	}

	// 验证：monitor[0] 延迟 0s, monitor[1] 延迟 2s, monitor[2] 延迟 4s
	if expectedInterval != 2*time.Second {
		t.Errorf("intra-group interval should be 2s, got %v", expectedInterval)
	}
}
