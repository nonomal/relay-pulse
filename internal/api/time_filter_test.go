package api

import (
	"testing"
	"time"

	"monitor/internal/storage"
)

// TestParseTimeFilter 测试时段参数解析
func TestParseTimeFilter(t *testing.T) {
	tests := []struct {
		name          string
		param         string
		wantErr       bool
		errContains   string
		crossMidnight bool
	}{
		// 正常情况
		{name: "空参数", param: "", wantErr: false},
		{name: "正常范围 09:00-17:00", param: "09:00-17:00", wantErr: false, crossMidnight: false},
		{name: "正常范围 00:00-24:00", param: "00:00-24:00", wantErr: false, crossMidnight: false},
		{name: "半小时粒度", param: "09:30-17:30", wantErr: false, crossMidnight: false},

		// 跨午夜情况
		{name: "跨午夜 22:00-04:00", param: "22:00-04:00", wantErr: false, crossMidnight: true},
		{name: "跨午夜 23:30-00:30", param: "23:30-00:30", wantErr: false, crossMidnight: true},
		{name: "跨午夜 18:00-06:00", param: "18:00-06:00", wantErr: false, crossMidnight: true},

		// 错误情况
		{name: "格式错误-无冒号", param: "0900-1700", wantErr: true, errContains: "无效的时段格式"},
		{name: "格式错误-无连字符", param: "09:00 17:00", wantErr: true, errContains: "无效的时段格式"},
		{name: "分钟非00或30", param: "09:15-17:00", wantErr: true, errContains: "分钟必须为 00 或 30"},
		{name: "开始小时超范围", param: "24:00-17:00", wantErr: true, errContains: "开始小时必须在 0-23 范围内"},
		{name: "结束小时超范围", param: "09:00-25:00", wantErr: true, errContains: "结束小时必须在 0-24 范围内"},
		{name: "24:30 无效", param: "09:00-24:30", wantErr: true, errContains: "24 点只允许 24:00"},
		{name: "开始等于结束", param: "09:00-09:00", wantErr: true, errContains: "开始时间不能等于结束时间"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseTimeFilter(tt.param)

			if tt.wantErr {
				if err == nil {
					t.Errorf("期望错误但没有返回错误")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("错误信息不匹配: got %q, want contains %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("不期望错误但返回了: %v", err)
				return
			}

			if tt.param == "" {
				if filter != nil {
					t.Errorf("空参数应返回 nil filter")
				}
				return
			}

			if filter.CrossMidnight != tt.crossMidnight {
				t.Errorf("CrossMidnight = %v, want %v", filter.CrossMidnight, tt.crossMidnight)
			}
		})
	}
}

// TestTimeFilter_Contains 测试时段包含判断
func TestTimeFilter_Contains(t *testing.T) {
	// 辅助函数：创建指定 UTC 时间
	makeTime := func(hour, minute int) time.Time {
		return time.Date(2024, 1, 15, hour, minute, 0, 0, time.UTC)
	}

	tests := []struct {
		name     string
		filter   TimeFilter
		time     time.Time
		expected bool
	}{
		// 正常范围 09:00-17:00
		{"09:00-17:00 at 08:59", TimeFilter{9, 0, 17, 0, false}, makeTime(8, 59), false},
		{"09:00-17:00 at 09:00", TimeFilter{9, 0, 17, 0, false}, makeTime(9, 0), true},
		{"09:00-17:00 at 12:00", TimeFilter{9, 0, 17, 0, false}, makeTime(12, 0), true},
		{"09:00-17:00 at 16:59", TimeFilter{9, 0, 17, 0, false}, makeTime(16, 59), true},
		{"09:00-17:00 at 17:00", TimeFilter{9, 0, 17, 0, false}, makeTime(17, 0), false}, // 右开区间

		// 跨午夜 22:00-04:00
		{"22:00-04:00 at 21:59", TimeFilter{22, 0, 4, 0, true}, makeTime(21, 59), false},
		{"22:00-04:00 at 22:00", TimeFilter{22, 0, 4, 0, true}, makeTime(22, 0), true},
		{"22:00-04:00 at 23:30", TimeFilter{22, 0, 4, 0, true}, makeTime(23, 30), true},
		{"22:00-04:00 at 00:00", TimeFilter{22, 0, 4, 0, true}, makeTime(0, 0), true},
		{"22:00-04:00 at 03:59", TimeFilter{22, 0, 4, 0, true}, makeTime(3, 59), true},
		{"22:00-04:00 at 04:00", TimeFilter{22, 0, 4, 0, true}, makeTime(4, 0), false}, // 右开区间
		{"22:00-04:00 at 10:00", TimeFilter{22, 0, 4, 0, true}, makeTime(10, 0), false},
		{"22:00-04:00 at 15:00", TimeFilter{22, 0, 4, 0, true}, makeTime(15, 0), false},

		// 边界情况：00:00-24:00（全天）
		{"00:00-24:00 at 00:00", TimeFilter{0, 0, 24, 0, false}, makeTime(0, 0), true},
		{"00:00-24:00 at 12:00", TimeFilter{0, 0, 24, 0, false}, makeTime(12, 0), true},
		{"00:00-24:00 at 23:59", TimeFilter{0, 0, 24, 0, false}, makeTime(23, 59), true},

		// 半小时粒度
		{"09:30-10:30 at 09:29", TimeFilter{9, 30, 10, 30, false}, makeTime(9, 29), false},
		{"09:30-10:30 at 09:30", TimeFilter{9, 30, 10, 30, false}, makeTime(9, 30), true},
		{"09:30-10:30 at 10:29", TimeFilter{9, 30, 10, 30, false}, makeTime(10, 29), true},
		{"09:30-10:30 at 10:30", TimeFilter{9, 30, 10, 30, false}, makeTime(10, 30), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filter.Contains(tt.time)
			if result != tt.expected {
				t.Errorf("Contains(%v) = %v, want %v", tt.time.Format("15:04"), result, tt.expected)
			}
		})
	}
}

// TestTimeFilter_String 测试时段字符串表示
func TestTimeFilter_String(t *testing.T) {
	tests := []struct {
		filter   TimeFilter
		expected string
	}{
		{TimeFilter{9, 0, 17, 0, false}, "09:00-17:00"},
		{TimeFilter{22, 0, 4, 0, true}, "22:00-04:00"},
		{TimeFilter{0, 0, 24, 0, false}, "00:00-24:00"},
		{TimeFilter{9, 30, 17, 30, false}, "09:30-17:30"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.filter.String()
			if result != tt.expected {
				t.Errorf("String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestBuildTimelineWithTimeFilter 测试带时段过滤的时间轴构建
func TestBuildTimelineWithTimeFilter(t *testing.T) {
	h := &Handler{}

	// 创建测试记录（模拟一天内不同时间的探测）
	baseTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC) // 明天 00:00

	// 记录分布在不同时间段
	records := []*storage.ProbeRecord{
		{Status: 1, Latency: 100, Timestamp: baseTime.Add(8 * time.Hour).Unix()},  // 08:00 - 不在工作时间
		{Status: 1, Latency: 200, Timestamp: baseTime.Add(10 * time.Hour).Unix()}, // 10:00 - 在工作时间
		{Status: 1, Latency: 300, Timestamp: baseTime.Add(14 * time.Hour).Unix()}, // 14:00 - 在工作时间
		{Status: 1, Latency: 400, Timestamp: baseTime.Add(18 * time.Hour).Unix()}, // 18:00 - 不在工作时间
		{Status: 1, Latency: 500, Timestamp: baseTime.Add(23 * time.Hour).Unix()}, // 23:00 - 在晚间跨午夜时段
	}

	t.Run("工作时间过滤 09:00-17:00", func(t *testing.T) {
		filter := &TimeFilter{StartHour: 9, StartMinute: 0, EndHour: 17, EndMinute: 0, CrossMidnight: false}
		timeline := h.buildTimeline(records, endTime, "24h", 0.7, filter)

		// 统计有数据的 bucket 数量
		var dataCount int
		for _, point := range timeline {
			if point.Status != -1 {
				dataCount++
			}
		}

		// 应该只有 10:00 和 14:00 两条记录被包含
		if dataCount != 2 {
			t.Errorf("工作时间过滤后应有 2 个有数据的 bucket，实际 %d 个", dataCount)
		}
	})

	t.Run("跨午夜过滤 22:00-04:00", func(t *testing.T) {
		filter := &TimeFilter{StartHour: 22, StartMinute: 0, EndHour: 4, EndMinute: 0, CrossMidnight: true}
		timeline := h.buildTimeline(records, endTime, "24h", 0.7, filter)

		// 统计有数据的 bucket 数量
		var dataCount int
		for _, point := range timeline {
			if point.Status != -1 {
				dataCount++
			}
		}

		// 只有 23:00 的记录应该被包含
		if dataCount != 1 {
			t.Errorf("跨午夜过滤后应有 1 个有数据的 bucket，实际 %d 个", dataCount)
		}
	})

	t.Run("无过滤（全天）", func(t *testing.T) {
		timeline := h.buildTimeline(records, endTime, "24h", 0.7, nil)

		// 统计有数据的 bucket 数量
		var dataCount int
		for _, point := range timeline {
			if point.Status != -1 {
				dataCount++
			}
		}

		// 所有 5 条记录应该被包含（但可能在不同 bucket）
		if dataCount < 4 { // 至少 4 个（08, 10, 14, 18 在不同小时，23 也是）
			t.Errorf("无过滤时应有至少 4 个有数据的 bucket，实际 %d 个", dataCount)
		}
	})
}

// 辅助函数：检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
