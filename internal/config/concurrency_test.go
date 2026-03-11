package config

import (
	"testing"
)

func TestMaxConcurrencyNormalize(t *testing.T) {
	tests := []struct {
		name      string
		input     int
		want      int
		wantError bool
	}{
		{
			name:      "未配置（0）应使用默认值 10",
			input:     0,
			want:      10,
			wantError: false,
		},
		{
			name:      "配置 -1 表示无限制",
			input:     -1,
			want:      -1,
			wantError: false,
		},
		{
			name:      "配置正数作为硬上限",
			input:     20,
			want:      20,
			wantError: false,
		},
		{
			name:      "非法值 -2 应报错",
			input:     -2,
			want:      0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AppConfig{
				Interval:       "1m",
				SlowLatency:    "5s",
				MaxConcurrency: tt.input,
				Monitors: []ServiceConfig{
					{
						Provider:   "test",
						Service:    "test",
						BaseURL:    "https://example.com",
						URLPattern: "{{BASE_URL}}",
						Method:     "POST",
						Category:   "public",
						Sponsor:    "test",
					},
				},
			}

			err := cfg.normalize()
			if tt.wantError {
				if err == nil {
					t.Errorf("期望报错但没有错误")
				}
				return
			}

			if err != nil {
				t.Errorf("不期望错误但得到: %v", err)
				return
			}

			if cfg.MaxConcurrency != tt.want {
				t.Errorf("MaxConcurrency = %d, want %d", cfg.MaxConcurrency, tt.want)
			}
		})
	}
}
