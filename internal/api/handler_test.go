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
