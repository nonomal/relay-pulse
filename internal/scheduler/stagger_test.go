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
	makeSingleModelGroups := func(count int) []monitorGroup {
		groups := make([]monitorGroup, count)
		for i := range groups {
			groups[i] = makeGroup(1)
		}
		return groups
	}

	tests := []struct {
		name               string
		groups             []monitorGroup
		intraInterval      time.Duration
		interval           time.Duration
		wantMaxWidth       time.Duration
		wantGroupBaseDelay time.Duration
		wantJitterRange    time.Duration
	}{
		{
			name:               "nil groups",
			groups:             nil,
			intraInterval:      2 * time.Second,
			interval:           5 * time.Minute,
			wantMaxWidth:       0,
			wantGroupBaseDelay: 0,
			wantJitterRange:    0,
		},
		{
			name:               "empty groups",
			groups:             []monitorGroup{},
			intraInterval:      2 * time.Second,
			interval:           5 * time.Minute,
			wantMaxWidth:       0,
			wantGroupBaseDelay: 0,
			wantJitterRange:    0,
		},
		{
			name:               "all single-model groups fill interval",
			groups:             makeSingleModelGroups(3),
			intraInterval:      2 * time.Second,
			interval:           30 * time.Second,
			wantMaxWidth:       0,
			wantGroupBaseDelay: 10 * time.Second, // 30s / 3
			wantJitterRange:    1 * time.Second,
		},
		{
			name:               "127 single-model groups + 5m interval",
			groups:             makeSingleModelGroups(127),
			intraInterval:      2 * time.Second,
			interval:           5 * time.Minute,
			wantMaxWidth:       0,
			wantGroupBaseDelay: 5 * time.Minute / 127,        // ≈ 2.362s
			wantJitterRange:    (5 * time.Minute / 127) / 10, // ≈ 236ms
		},
		{
			name:               "one 2-model group, anti-overlap dominates",
			groups:             []monitorGroup{makeGroup(2)},
			intraInterval:      2 * time.Second,
			interval:           2 * time.Second,
			wantMaxWidth:       2 * time.Second,         // (2-1)*2s
			wantGroupBaseDelay: 2500 * time.Millisecond, // ceil(2s*10/8) = 2.5s > 2s ideal
			wantJitterRange:    250 * time.Millisecond,
		},
		{
			name:               "one 3-model group",
			groups:             []monitorGroup{makeGroup(3)},
			intraInterval:      2 * time.Second,
			interval:           4 * time.Second,
			wantMaxWidth:       4 * time.Second, // (3-1)*2s
			wantGroupBaseDelay: 5 * time.Second, // ceil(4s*10/8) = 5s > 4s ideal
			wantJitterRange:    500 * time.Millisecond,
		},
		{
			name:               "mixed groups, anti-overlap wins",
			groups:             []monitorGroup{makeGroup(1), makeGroup(2), makeGroup(3), makeGroup(1)},
			intraInterval:      2 * time.Second,
			interval:           8 * time.Second,
			wantMaxWidth:       4 * time.Second,
			wantGroupBaseDelay: 5 * time.Second, // ceil(4s*10/8) = 5s > 8s/4=2s ideal
			wantJitterRange:    500 * time.Millisecond,
		},
		{
			name:               "large group avoids overlap even with jitter",
			groups:             []monitorGroup{makeGroup(6)}, // width=(6-1)*2s=10s
			intraInterval:      2 * time.Second,
			interval:           10 * time.Second,
			wantMaxWidth:       10 * time.Second,
			wantGroupBaseDelay: 12500 * time.Millisecond, // ceil(10s*10/8) = 12.5s
			wantJitterRange:    1250 * time.Millisecond,
		},
		{
			name:               "very large group",
			groups:             []monitorGroup{makeGroup(11)}, // width=(11-1)*2s=20s
			intraInterval:      2 * time.Second,
			interval:           20 * time.Second,
			wantMaxWidth:       20 * time.Second,
			wantGroupBaseDelay: 25 * time.Second, // ceil(20s*10/8) = 25s
			wantJitterRange:    2500 * time.Millisecond,
		},
		{
			name:               "zero intra interval falls back to ideal delay",
			groups:             []monitorGroup{makeGroup(5)},
			intraInterval:      0,
			interval:           30 * time.Second,
			wantMaxWidth:       0,
			wantGroupBaseDelay: 30 * time.Second, // 30s / 1 group
			wantJitterRange:    3 * time.Second,
		},
		{
			name:               "non-divisible intra interval",
			groups:             []monitorGroup{makeGroup(3)}, // width = (3-1)*1.5s = 3s
			intraInterval:      1500 * time.Millisecond,      // 1.5s
			interval:           3 * time.Second,
			wantMaxWidth:       3 * time.Second,         // 3s
			wantGroupBaseDelay: 3750 * time.Millisecond, // ceil(3s*10/8) = 3.75s
			wantJitterRange:    375 * time.Millisecond,
		},
		{
			name:               "ideal delay dominates when interval is large",
			groups:             []monitorGroup{makeGroup(3), makeGroup(2)},
			intraInterval:      2 * time.Second,
			interval:           5 * time.Minute,
			wantMaxWidth:       4 * time.Second,   // (3-1)*2s
			wantGroupBaseDelay: 150 * time.Second, // 300s / 2 = 150s >> required 5s
			wantJitterRange:    15 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBase, gotJitter, gotMaxWidth := computeStartupStaggerParams(tt.groups, tt.intraInterval, tt.interval)

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

			// 单模型场景：总展开不应超过 interval
			if gotMaxWidth == 0 && tt.interval > 0 && len(tt.groups) > 0 {
				totalSpread := gotBase * time.Duration(len(tt.groups))
				if totalSpread > tt.interval {
					t.Errorf("single-model totalSpread = %v, want <= %v", totalSpread, tt.interval)
				}
			}

			// jitter 应为 baseDelay 的 10%
			expectedJitter := gotBase / 10
			if gotJitter != expectedJitter {
				t.Errorf("jitter ratio incorrect: got %v, want %v (10%% of %v)", gotJitter, expectedJitter, gotBase)
			}
		})
	}
}

func TestComputeRequiredBaseDelay(t *testing.T) {
	tests := []struct {
		name      string
		maxWidth  time.Duration
		ratioNum  int64
		ratioDen  int64
		wantDelay time.Duration
	}{
		{
			name:     "zero width",
			maxWidth: 0,
			ratioNum: 1, ratioDen: 10,
			wantDelay: 0,
		},
		{
			name:     "startup ±10%: 4s width",
			maxWidth: 4 * time.Second,
			ratioNum: 1, ratioDen: 10, // denom = 10 - 2 = 8
			wantDelay: 5 * time.Second, // ceil(4s*10/8) = 5s
		},
		{
			name:     "hot-update ±5%: 4s width",
			maxWidth: 4 * time.Second,
			ratioNum: 1, ratioDen: 20, // denom = 20 - 2 = 18
			wantDelay: 4444444445, // ceil(4s*20/18) ≈ 4.44s
		},
		{
			name:     "hot-update ±5%: 10s width",
			maxWidth: 10 * time.Second,
			ratioNum: 1, ratioDen: 20, // denom = 18
			wantDelay: 11111111112, // ceil(10s*20/18) ≈ 11.11s
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeRequiredBaseDelay(tt.maxWidth, tt.ratioNum, tt.ratioDen)
			if got != tt.wantDelay {
				t.Errorf("computeRequiredBaseDelay(%v, %d, %d) = %v, want %v",
					tt.maxWidth, tt.ratioNum, tt.ratioDen, got, tt.wantDelay)
			}
		})
	}
}

func TestHotUpdateStaggerLogic(t *testing.T) {
	// 模拟热更新分支的逻辑：验证 127 单模型组 + 5m interval 不溢出
	makeGroup := func(models int) monitorGroup {
		return monitorGroup{psc: "p/s/c", monitorIdxs: make([]int, models)}
	}

	const jitterRatioNum int64 = 1
	const jitterRatioDen int64 = 20 // 热更新 ±5%
	const intraGroupInterval = 2 * time.Second

	t.Run("127 single-model groups + 5m", func(t *testing.T) {
		groups := make([]monitorGroup, 127)
		for i := range groups {
			groups[i] = makeGroup(1)
		}
		interval := 5 * time.Minute

		idealBaseDelay := interval / time.Duration(len(groups))
		maxWidth := computeMaxIntraGroupWidth(groups, intraGroupInterval)
		required := computeRequiredBaseDelay(maxWidth, jitterRatioNum, jitterRatioDen)

		baseDelay := idealBaseDelay
		if required > baseDelay {
			baseDelay = required
		}

		totalSpread := baseDelay * time.Duration(len(groups))
		if totalSpread > interval {
			t.Errorf("hot-update 127 groups overflow: totalSpread=%v > interval=%v", totalSpread, interval)
		}
		if maxWidth != 0 {
			t.Errorf("expected maxWidth=0 for single-model groups, got %v", maxWidth)
		}
	})

	t.Run("4 groups with 3-model max + 5m", func(t *testing.T) {
		groups := []monitorGroup{makeGroup(1), makeGroup(2), makeGroup(3), makeGroup(1)}
		interval := 5 * time.Minute

		idealBaseDelay := interval / time.Duration(len(groups))
		maxWidth := computeMaxIntraGroupWidth(groups, intraGroupInterval)
		required := computeRequiredBaseDelay(maxWidth, jitterRatioNum, jitterRatioDen)

		baseDelay := idealBaseDelay
		if required > baseDelay {
			baseDelay = required
		}

		// idealBaseDelay = 75s >> required ≈ 4.44s, so ideal wins
		if baseDelay != idealBaseDelay {
			t.Errorf("expected ideal delay %v to dominate, got %v", idealBaseDelay, baseDelay)
		}
		totalSpread := baseDelay * time.Duration(len(groups))
		if totalSpread > interval {
			t.Errorf("hot-update 4 groups overflow: totalSpread=%v > interval=%v", totalSpread, interval)
		}
	})
}
