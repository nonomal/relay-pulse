package api

import (
	"testing"
	"time"

	"monitor/internal/config"
	"monitor/internal/storage"
)

// TestBuildTimelineLatencyCalculation 测试延迟统计逻辑
// 验证：只有 status > 0 的记录才纳入延迟统计
func TestBuildTimelineLatencyCalculation(t *testing.T) {
	// 创建 Handler（不需要真实的 storage）
	h := &Handler{
		config: &config.AppConfig{
			DegradedWeight: 0.7,
		},
	}

	now := time.Now()

	tests := []struct {
		name            string
		records         []*storage.ProbeRecord
		expectedLatency int // 预期的平均延迟
		description     string
	}{
		{
			name: "全部可用状态",
			records: []*storage.ProbeRecord{
				{Status: 1, Latency: 100, Timestamp: now.Unix()},
				{Status: 1, Latency: 200, Timestamp: now.Unix()},
				{Status: 1, Latency: 300, Timestamp: now.Unix()},
			},
			expectedLatency: 200, // (100+200+300)/3 = 200
			description:     "3条绿色记录，平均延迟应为200ms",
		},
		{
			name: "混合可用和不可用状态",
			records: []*storage.ProbeRecord{
				{Status: 1, Latency: 100, Timestamp: now.Unix()},  // 可用，纳入统计
				{Status: 0, Latency: 5000, Timestamp: now.Unix()}, // 不可用，不纳入统计
				{Status: 1, Latency: 200, Timestamp: now.Unix()},  // 可用，纳入统计
			},
			expectedLatency: 150, // (100+200)/2 = 150，排除 status=0 的 5000ms
			description:     "2条绿色+1条红色，红色的5000ms不应纳入统计",
		},
		{
			name: "混合绿色和黄色状态",
			records: []*storage.ProbeRecord{
				{Status: 1, Latency: 100, Timestamp: now.Unix()}, // 绿色
				{Status: 2, Latency: 300, Timestamp: now.Unix()}, // 黄色（慢）
				{Status: 1, Latency: 200, Timestamp: now.Unix()}, // 绿色
			},
			expectedLatency: 200, // (100+300+200)/3 = 200，黄色也应纳入统计
			description:     "绿色和黄色都应纳入延迟统计",
		},
		{
			name: "全部不可用状态",
			records: []*storage.ProbeRecord{
				{Status: 0, Latency: 1000, Timestamp: now.Unix()},
				{Status: 0, Latency: 2000, Timestamp: now.Unix()},
			},
			expectedLatency: 0, // 没有可用记录，延迟为0
			description:     "全部不可用时，延迟应为0",
		},
		{
			name: "混合所有状态",
			records: []*storage.ProbeRecord{
				{Status: 1, Latency: 100, Timestamp: now.Unix()},  // 绿色，纳入
				{Status: 2, Latency: 200, Timestamp: now.Unix()},  // 黄色，纳入
				{Status: 0, Latency: 9999, Timestamp: now.Unix()}, // 红色，不纳入
				{Status: 1, Latency: 300, Timestamp: now.Unix()},  // 绿色，纳入
			},
			expectedLatency: 200, // (100+200+300)/3 = 200
			description:     "红色状态的超高延迟不应影响平均值",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 调用 buildTimeline（使用 now 作为 endTime，模拟动态滑动窗口）
			timeline := h.buildTimeline(tt.records, now, "24h", 0.7)

			// 找到有数据的 bucket（最后一个，因为所有记录时间戳都是 now）
			var latency int
			for _, point := range timeline {
				if point.Status != -1 { // 有数据的 bucket
					latency = point.Latency
					break
				}
			}

			if latency != tt.expectedLatency {
				t.Errorf("%s: 期望延迟 %dms，实际 %dms",
					tt.description, tt.expectedLatency, latency)
			}
		})
	}
}

// TestBuildTimelineLatencyRounding 测试延迟四舍五入
func TestBuildTimelineLatencyRounding(t *testing.T) {
	h := &Handler{
		config: &config.AppConfig{
			DegradedWeight: 0.7,
		},
	}

	now := time.Now()

	// 测试四舍五入：(100+101)/2 = 100.5 应该四舍五入为 101
	records := []*storage.ProbeRecord{
		{Status: 1, Latency: 100, Timestamp: now.Unix()},
		{Status: 1, Latency: 101, Timestamp: now.Unix()},
	}

	timeline := h.buildTimeline(records, now, "24h", 0.7)

	var latency int
	for _, point := range timeline {
		if point.Status != -1 {
			latency = point.Latency
			break
		}
	}

	// 100.5 四舍五入应为 101（不是 100）
	if latency != 101 {
		t.Errorf("四舍五入测试失败: 期望 101ms，实际 %dms", latency)
	}
}

// TestAlignTimestamp 测试时间对齐逻辑
func TestAlignTimestamp(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name     string
		input    time.Time
		align    string
		expected time.Time
	}{
		{
			name:     "hour 对齐 - 中间时间",
			input:    time.Date(2024, 1, 15, 17, 48, 30, 0, time.UTC),
			align:    "hour",
			expected: time.Date(2024, 1, 15, 18, 0, 0, 0, time.UTC), // 向上取整到 18:00
		},
		{
			name:     "hour 对齐 - 整点时间",
			input:    time.Date(2024, 1, 15, 17, 0, 0, 0, time.UTC),
			align:    "hour",
			expected: time.Date(2024, 1, 15, 17, 0, 0, 0, time.UTC), // 已是整点，保持不变
		},
		{
			name:     "day 对齐 - 中间时间",
			input:    time.Date(2024, 1, 15, 12, 30, 0, 0, time.UTC),
			align:    "day",
			expected: time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC), // 向上取整到明天 00:00
		},
		{
			name:     "day 对齐 - 午夜时间",
			input:    time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			align:    "day",
			expected: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), // 已是午夜，保持不变
		},
		{
			name:     "无对齐 - 保持原值",
			input:    time.Date(2024, 1, 15, 12, 30, 45, 123, time.UTC),
			align:    "",
			expected: time.Date(2024, 1, 15, 12, 30, 45, 123, time.UTC), // 保持不变
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.alignTimestamp(tt.input, tt.align)
			if !result.Equal(tt.expected) {
				t.Errorf("alignTimestamp(%v, %q) = %v, want %v",
					tt.input, tt.align, result, tt.expected)
			}
		})
	}
}

// TestParseTimeRange7d30dDayAlign 测试 7d/30d 自动按天对齐
func TestParseTimeRange7d30dDayAlign(t *testing.T) {
	h := &Handler{}

	// 测试 7d 和 30d 的 endTime 是否按天对齐（向上取整到明天 00:00 UTC）
	// 注意：由于使用 time.Now()，我们只验证 endTime 是否为 00:00:00 UTC

	t.Run("7d 自动按天对齐", func(t *testing.T) {
		_, endTime := h.parseTimeRange("7d", "")
		// endTime 应该是某天的 00:00:00 UTC
		if endTime.Hour() != 0 || endTime.Minute() != 0 || endTime.Second() != 0 {
			t.Errorf("7d endTime 应为整天 00:00:00 UTC，实际为 %v", endTime)
		}
	})

	t.Run("30d 自动按天对齐", func(t *testing.T) {
		_, endTime := h.parseTimeRange("30d", "")
		// endTime 应该是某天的 00:00:00 UTC
		if endTime.Hour() != 0 || endTime.Minute() != 0 || endTime.Second() != 0 {
			t.Errorf("30d endTime 应为整天 00:00:00 UTC，实际为 %v", endTime)
		}
	})

	t.Run("7d/30d 忽略 align 参数", func(t *testing.T) {
		// 即使传入 align="hour"，7d 也应该使用 day 对齐
		_, endTime7d := h.parseTimeRange("7d", "hour")
		_, endTime30d := h.parseTimeRange("30d", "hour")

		if endTime7d.Hour() != 0 || endTime7d.Minute() != 0 {
			t.Errorf("7d 应忽略 align=hour 参数，使用 day 对齐，实际 endTime=%v", endTime7d)
		}
		if endTime30d.Hour() != 0 || endTime30d.Minute() != 0 {
			t.Errorf("30d 应忽略 align=hour 参数，使用 day 对齐，实际 endTime=%v", endTime30d)
		}
	})
}
