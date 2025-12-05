package scheduler

import (
	"testing"
	"time"

	"monitor/internal/config"
)

// TestFindMinIntervalSkipsDisabled 测试 findMinInterval 跳过已禁用的监控项
func TestFindMinIntervalSkipsDisabled(t *testing.T) {
	s := &Scheduler{}

	tests := []struct {
		name     string
		cfg      *config.AppConfig
		expected time.Duration
	}{
		{
			name: "全部活跃",
			cfg: &config.AppConfig{
				IntervalDuration: 5 * time.Minute,
				Monitors: []config.ServiceConfig{
					{Provider: "a", IntervalDuration: 1 * time.Minute, Disabled: false},
					{Provider: "b", IntervalDuration: 2 * time.Minute, Disabled: false},
					{Provider: "c", IntervalDuration: 3 * time.Minute, Disabled: false},
				},
			},
			expected: 1 * time.Minute, // 最小的活跃 interval
		},
		{
			name: "最小 interval 的监控项被禁用",
			cfg: &config.AppConfig{
				IntervalDuration: 5 * time.Minute,
				Monitors: []config.ServiceConfig{
					{Provider: "a", IntervalDuration: 30 * time.Second, Disabled: true}, // 禁用，应跳过
					{Provider: "b", IntervalDuration: 2 * time.Minute, Disabled: false},
					{Provider: "c", IntervalDuration: 3 * time.Minute, Disabled: false},
				},
			},
			expected: 2 * time.Minute, // 跳过禁用的 30s，最小应为 2 分钟
		},
		{
			name: "全部禁用，使用全局 interval",
			cfg: &config.AppConfig{
				IntervalDuration: 5 * time.Minute,
				Monitors: []config.ServiceConfig{
					{Provider: "a", IntervalDuration: 30 * time.Second, Disabled: true},
					{Provider: "b", IntervalDuration: 1 * time.Minute, Disabled: true},
				},
			},
			expected: 5 * time.Minute, // 全部禁用，使用全局 interval
		},
		{
			name: "活跃监控项无自定义 interval",
			cfg: &config.AppConfig{
				IntervalDuration: 3 * time.Minute,
				Monitors: []config.ServiceConfig{
					{Provider: "a", IntervalDuration: 30 * time.Second, Disabled: true}, // 禁用
					{Provider: "b", IntervalDuration: 0, Disabled: false},               // 无自定义，使用全局
					{Provider: "c", IntervalDuration: 0, Disabled: false},               // 无自定义，使用全局
				},
			},
			expected: 3 * time.Minute, // 活跃项无自定义，使用全局
		},
		{
			name: "混合场景",
			cfg: &config.AppConfig{
				IntervalDuration: 5 * time.Minute,
				Monitors: []config.ServiceConfig{
					{Provider: "a", IntervalDuration: 10 * time.Second, Disabled: true},  // 禁用
					{Provider: "b", IntervalDuration: 0, Disabled: false},                // 无自定义
					{Provider: "c", IntervalDuration: 90 * time.Second, Disabled: false}, // 最小活跃
					{Provider: "d", IntervalDuration: 2 * time.Minute, Disabled: false},
				},
			},
			expected: 90 * time.Second, // 90s 是最小的活跃自定义 interval
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.findMinInterval(tt.cfg)
			if result != tt.expected {
				t.Errorf("findMinInterval() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestCountActiveMonitors 测试统计活跃监控项数量的逻辑
func TestCountActiveMonitors(t *testing.T) {
	tests := []struct {
		name           string
		monitors       []config.ServiceConfig
		expectedTotal  int
		expectedActive int
	}{
		{
			name: "全部活跃",
			monitors: []config.ServiceConfig{
				{Provider: "a", Disabled: false},
				{Provider: "b", Disabled: false},
				{Provider: "c", Disabled: false},
			},
			expectedTotal:  3,
			expectedActive: 3,
		},
		{
			name: "部分禁用",
			monitors: []config.ServiceConfig{
				{Provider: "a", Disabled: false},
				{Provider: "b", Disabled: true},
				{Provider: "c", Disabled: false},
				{Provider: "d", Disabled: true},
			},
			expectedTotal:  4,
			expectedActive: 2,
		},
		{
			name: "全部禁用",
			monitors: []config.ServiceConfig{
				{Provider: "a", Disabled: true},
				{Provider: "b", Disabled: true},
			},
			expectedTotal:  2,
			expectedActive: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total := len(tt.monitors)
			disabled := 0
			for _, m := range tt.monitors {
				if m.Disabled {
					disabled++
				}
			}
			active := total - disabled

			if total != tt.expectedTotal {
				t.Errorf("total = %d, want %d", total, tt.expectedTotal)
			}
			if active != tt.expectedActive {
				t.Errorf("active = %d, want %d", active, tt.expectedActive)
			}
		})
	}
}
