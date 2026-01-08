package scheduler

import (
	"testing"
	"time"
)

func TestComputeStartupStaggerParams(t *testing.T) {
	makeGroup := func(models int) monitorGroup {
		if models < 0 {
			models = 0
		}
		return monitorGroup{
			psc:         "p/s/c",
			monitorIdxs: make([]int, models),
		}
	}

	tests := []struct {
		name               string
		groups             []monitorGroup
		intraInterval      time.Duration
		wantMaxWidth       time.Duration
		wantGroupBaseDelay time.Duration
		wantJitterRange    time.Duration
	}{
		{
			name:               "nil groups",
			groups:             nil,
			intraInterval:      2 * time.Second,
			wantMaxWidth:       0,
			wantGroupBaseDelay: 3 * time.Second,
			wantJitterRange:    300 * time.Millisecond,
		},
		{
			name:               "empty groups",
			groups:             []monitorGroup{},
			intraInterval:      2 * time.Second,
			wantMaxWidth:       0,
			wantGroupBaseDelay: 3 * time.Second,
			wantJitterRange:    300 * time.Millisecond,
		},
		{
			name:               "all single-model groups",
			groups:             []monitorGroup{makeGroup(1), makeGroup(1), makeGroup(1)},
			intraInterval:      2 * time.Second,
			wantMaxWidth:       0,
			wantGroupBaseDelay: 3 * time.Second,
			wantJitterRange:    300 * time.Millisecond,
		},
		{
			name:               "one 2-model group",
			groups:             []monitorGroup{makeGroup(2)},
			intraInterval:      2 * time.Second,
			wantMaxWidth:       2 * time.Second, // (2-1)*2s
			wantGroupBaseDelay: 3 * time.Second, // min 3s dominates (2s*10/8=2.5s < 3s)
			wantJitterRange:    300 * time.Millisecond,
		},
		{
			name:               "one 3-model group",
			groups:             []monitorGroup{makeGroup(3)},
			intraInterval:      2 * time.Second,
			wantMaxWidth:       4 * time.Second, // (3-1)*2s
			wantGroupBaseDelay: 5 * time.Second, // ceil(4s*10/8) = ceil(5s) = 5s
			wantJitterRange:    500 * time.Millisecond,
		},
		{
			name:               "mixed groups, max width wins",
			groups:             []monitorGroup{makeGroup(1), makeGroup(2), makeGroup(3), makeGroup(1)},
			intraInterval:      2 * time.Second,
			wantMaxWidth:       4 * time.Second,
			wantGroupBaseDelay: 5 * time.Second,
			wantJitterRange:    500 * time.Millisecond,
		},
		{
			name:               "large group avoids overlap even with jitter",
			groups:             []monitorGroup{makeGroup(6)}, // width=(6-1)*2s=10s
			intraInterval:      2 * time.Second,
			wantMaxWidth:       10 * time.Second,
			wantGroupBaseDelay: 12500 * time.Millisecond, // ceil(10s*10/8) = ceil(12.5s) = 12.5s
			wantJitterRange:    1250 * time.Millisecond,
		},
		{
			name:               "very large group",
			groups:             []monitorGroup{makeGroup(11)}, // width=(11-1)*2s=20s
			intraInterval:      2 * time.Second,
			wantMaxWidth:       20 * time.Second,
			wantGroupBaseDelay: 25 * time.Second, // ceil(20s*10/8) = 25s
			wantJitterRange:    2500 * time.Millisecond,
		},
		// 边界情况：intraGroupInterval 为 0
		{
			name:               "zero intra interval",
			groups:             []monitorGroup{makeGroup(5)},
			intraInterval:      0,
			wantMaxWidth:       0, // 0 * (5-1) = 0
			wantGroupBaseDelay: 3 * time.Second,
			wantJitterRange:    300 * time.Millisecond,
		},
		// 边界情况：非整除的 intraGroupInterval
		{
			name:               "non-divisible intra interval",
			groups:             []monitorGroup{makeGroup(3)}, // width = (3-1)*1.5s = 3s
			intraInterval:      1500 * time.Millisecond,      // 1.5s
			wantMaxWidth:       3 * time.Second,              // 3s
			wantGroupBaseDelay: 3750 * time.Millisecond,      // ceil(3s*10/8) = ceil(3.75s) = 3.75s
			wantJitterRange:    375 * time.Millisecond,       // 3.75s / 10 = 375ms
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBase, gotJitter, gotMaxWidth := computeStartupStaggerParams(tt.groups, tt.intraInterval)

			if gotMaxWidth != tt.wantMaxWidth {
				t.Errorf("maxIntraGroupWidth = %v, want %v", gotMaxWidth, tt.wantMaxWidth)
			}
			if gotBase != tt.wantGroupBaseDelay {
				t.Errorf("groupBaseDelay = %v, want %v", gotBase, tt.wantGroupBaseDelay)
			}
			if gotJitter != tt.wantJitterRange {
				t.Errorf("groupJitterRange = %v, want %v", gotJitter, tt.wantJitterRange)
			}

			// 不重叠约束（相邻组最坏情况）：baseDelay - 2*jitterRange >= maxWidth
			if gotMaxWidth > 0 && gotBase-2*gotJitter < gotMaxWidth {
				t.Errorf("no-overlap constraint violated: base=%v jitter=%v maxWidth=%v, got base-2*jitter=%v",
					gotBase, gotJitter, gotMaxWidth, gotBase-2*gotJitter)
			}

			// 启动模式下至少 3s
			if gotBase < 3*time.Second {
				t.Errorf("groupBaseDelay too small: %v, want >= 3s", gotBase)
			}

			// jitter 应为 baseDelay 的 10%
			expectedJitter := gotBase / 10
			if gotJitter != expectedJitter {
				t.Errorf("jitter ratio incorrect: got %v, want %v (10%% of %v)", gotJitter, expectedJitter, gotBase)
			}
		})
	}
}
