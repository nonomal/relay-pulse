package events

import (
	"testing"

	"monitor/internal/storage"
)

func TestNewChannelDetector_NormalizesThreshold(t *testing.T) {
	tests := []struct {
		name string
		cfg  ChannelDetectorConfig
		want int
	}{
		{"valid_threshold", ChannelDetectorConfig{DownThreshold: 3}, 3},
		{"zero_threshold", ChannelDetectorConfig{DownThreshold: 0}, 1},
		{"negative_threshold", ChannelDetectorConfig{DownThreshold: -5}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewChannelDetector(tt.cfg)
			if detector.cfg.DownThreshold != tt.want {
				t.Fatalf("DownThreshold = %d, want %d", detector.cfg.DownThreshold, tt.want)
			}
		})
	}
}

func TestChannelDetector_updateCounts(t *testing.T) {
	detector := NewChannelDetector(ChannelDetectorConfig{DownThreshold: 1})

	tests := []struct {
		name            string
		channel         *ChannelState
		prevModelStable int
		newModelStable  int
		wantKnown       int
		wantDown        int
	}{
		{
			name:            "first_init_up",
			channel:         &ChannelState{KnownCount: 0, DownCount: 0},
			prevModelStable: -1, newModelStable: 1,
			wantKnown: 1, wantDown: 0,
		},
		{
			name:            "first_init_down",
			channel:         &ChannelState{KnownCount: 0, DownCount: 0},
			prevModelStable: -1, newModelStable: 0,
			wantKnown: 1, wantDown: 1,
		},
		{
			name:            "up_to_down",
			channel:         &ChannelState{KnownCount: 2, DownCount: 0},
			prevModelStable: 1, newModelStable: 0,
			wantKnown: 2, wantDown: 1,
		},
		{
			name:            "down_to_up",
			channel:         &ChannelState{KnownCount: 2, DownCount: 2},
			prevModelStable: 0, newModelStable: 1,
			wantKnown: 2, wantDown: 1,
		},
		{
			name:            "no_change_up",
			channel:         &ChannelState{KnownCount: 2, DownCount: 0},
			prevModelStable: 1, newModelStable: 1,
			wantKnown: 2, wantDown: 0,
		},
		{
			name:            "no_change_down",
			channel:         &ChannelState{KnownCount: 2, DownCount: 1},
			prevModelStable: 0, newModelStable: 0,
			wantKnown: 2, wantDown: 1,
		},
		{
			name:            "prevent_negative_down_count",
			channel:         &ChannelState{KnownCount: 1, DownCount: 0},
			prevModelStable: 0, newModelStable: 1,
			wantKnown: 1, wantDown: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector.updateCounts(tt.channel, tt.prevModelStable, tt.newModelStable)

			if tt.channel.KnownCount != tt.wantKnown {
				t.Fatalf("KnownCount = %d, want %d", tt.channel.KnownCount, tt.wantKnown)
			}
			if tt.channel.DownCount != tt.wantDown {
				t.Fatalf("DownCount = %d, want %d", tt.channel.DownCount, tt.wantDown)
			}
		})
	}
}

func TestChannelDetector_DetectChannel_Incremental(t *testing.T) {
	t.Run("first_init_all_up_sets_available_no_event", func(t *testing.T) {
		detector := NewChannelDetector(ChannelDetectorConfig{DownThreshold: 1})
		record := testChannelRecord(1, "model-a")

		result := detector.DetectChannel(nil, -1, 1, 1, record)

		if result.Event != nil {
			t.Fatalf("first init all up should not emit event, got %+v", result.Event)
		}
		if result.NewChannelState.StableAvailable != 1 {
			t.Fatalf("StableAvailable = %d, want 1", result.NewChannelState.StableAvailable)
		}
		if result.NewChannelState.KnownCount != 1 {
			t.Fatalf("KnownCount = %d, want 1", result.NewChannelState.KnownCount)
		}
	})

	t.Run("first_init_partial_known_stays_uninitialized", func(t *testing.T) {
		detector := NewChannelDetector(ChannelDetectorConfig{DownThreshold: 1})
		record := testChannelRecord(2, "model-a")

		// totalModels=2, 只有 1 个 model 已知且 up → StableAvailable 留 -1
		result := detector.DetectChannel(nil, -1, 1, 2, record)

		if result.Event != nil {
			t.Fatalf("partial init should not emit event, got %+v", result.Event)
		}
		if result.NewChannelState.StableAvailable != -1 {
			t.Fatalf("StableAvailable = %d, want -1", result.NewChannelState.StableAvailable)
		}
	})

	t.Run("first_init_down_emits_down_event", func(t *testing.T) {
		detector := NewChannelDetector(ChannelDetectorConfig{DownThreshold: 1})
		record := testChannelRecord(3, "model-a")
		record.HttpCode = 503
		record.SubStatus = storage.SubStatusServerError

		result := detector.DetectChannel(nil, -1, 0, 2, record)

		if result.Event == nil {
			t.Fatal("first init down should emit DOWN event")
		}
		if result.Event.EventType != EventTypeDown {
			t.Fatalf("EventType = %s, want DOWN", result.Event.EventType)
		}
		if result.NewChannelState.StableAvailable != 0 {
			t.Fatalf("StableAvailable = %d, want 0", result.NewChannelState.StableAvailable)
		}
		// 验证 Meta 包含 scope=channel
		if scope, ok := result.Event.Meta["scope"]; !ok || scope != "channel" {
			t.Fatalf("Meta[scope] = %v, want 'channel'", scope)
		}
	})

	t.Run("down_threshold_boundary", func(t *testing.T) {
		// threshold=2, 第一个 model down 不触发, 第二个触发
		detector := NewChannelDetector(ChannelDetectorConfig{DownThreshold: 2})
		prev := &ChannelState{
			Provider: "p", Service: "s", Channel: "c",
			StableAvailable: 1, KnownCount: 2, DownCount: 0,
			LastRecordID: 10, LastTimestamp: 100,
		}

		// 第一个 model down → DownCount=1 < threshold=2, 不触发
		r1 := detector.DetectChannel(prev, 1, 0, 2, testChannelRecord(11, "model-a"))
		if r1.Event != nil {
			t.Fatalf("below threshold should not emit, got %+v", r1.Event)
		}
		if r1.NewChannelState.DownCount != 1 {
			t.Fatalf("DownCount = %d, want 1", r1.NewChannelState.DownCount)
		}

		// 第二个 model down → DownCount=2 >= threshold=2, 触发 DOWN
		r2 := detector.DetectChannel(r1.NewChannelState, 1, 0, 2, testChannelRecord(12, "model-b"))
		if r2.Event == nil {
			t.Fatal("at threshold should emit DOWN event")
		}
		if r2.Event.EventType != EventTypeDown {
			t.Fatalf("EventType = %s, want DOWN", r2.Event.EventType)
		}
		if r2.NewChannelState.StableAvailable != 0 {
			t.Fatalf("StableAvailable = %d, want 0", r2.NewChannelState.StableAvailable)
		}
	})

	t.Run("up_requires_all_models_known_and_zero_down", func(t *testing.T) {
		detector := NewChannelDetector(ChannelDetectorConfig{DownThreshold: 1})

		// 通道 DOWN 状态，1/2 model known, model-a 从 down→up
		prev := &ChannelState{
			Provider: "p", Service: "s", Channel: "c",
			StableAvailable: 0, KnownCount: 1, DownCount: 1,
			LastRecordID: 20, LastTimestamp: 200,
		}
		r1 := detector.DetectChannel(prev, 0, 1, 2, testChannelRecord(21, "model-a"))
		if r1.Event != nil {
			t.Fatalf("UP should not emit before all models known, got %+v", r1.Event)
		}

		// 现在 2/2 known, down_count=0 → 应触发 UP
		prev2 := &ChannelState{
			Provider: "p", Service: "s", Channel: "c",
			StableAvailable: 0, KnownCount: 2, DownCount: 1,
			LastRecordID: 22, LastTimestamp: 220,
		}
		r2 := detector.DetectChannel(prev2, 0, 1, 2, testChannelRecord(23, "model-b"))
		if r2.Event == nil {
			t.Fatal("UP should emit when all models known and down_count=0")
		}
		if r2.Event.EventType != EventTypeUp {
			t.Fatalf("EventType = %s, want UP", r2.Event.EventType)
		}
		if r2.NewChannelState.StableAvailable != 1 {
			t.Fatalf("StableAvailable = %d, want 1", r2.NewChannelState.StableAvailable)
		}
	})

	t.Run("up_not_emitted_for_zero_total_models", func(t *testing.T) {
		detector := NewChannelDetector(ChannelDetectorConfig{DownThreshold: 1})
		prev := &ChannelState{
			Provider: "p", Service: "s", Channel: "c",
			StableAvailable: 0, KnownCount: 0, DownCount: 0,
		}
		// totalModels=0 → 不应触发 UP
		r := detector.DetectChannel(prev, -1, 1, 0, testChannelRecord(30, "model-x"))
		if r.Event != nil {
			t.Fatalf("should not emit UP for zero total models, got %+v", r.Event)
		}
	})
}

func TestChannelDetector_DetectChannelWithCounts(t *testing.T) {
	tests := []struct {
		name          string
		threshold     int
		prev          *ChannelState
		downCount     int
		knownCount    int
		totalModels   int
		wantStable    int
		wantEvent     bool
		wantEventType EventType
	}{
		{
			name: "first_init_down_emits_event", threshold: 1,
			prev: nil, downCount: 1, knownCount: 1, totalModels: 2,
			wantStable: 0, wantEvent: true, wantEventType: EventTypeDown,
		},
		{
			name: "first_init_all_up_no_event", threshold: 1,
			prev: nil, downCount: 0, knownCount: 2, totalModels: 2,
			wantStable: 1, wantEvent: false,
		},
		{
			name: "below_threshold_no_event", threshold: 2,
			prev: &ChannelState{
				Provider: "p", Service: "s", Channel: "c",
				StableAvailable: 1, KnownCount: 3, DownCount: 0,
			},
			downCount: 1, knownCount: 3, totalModels: 3,
			wantStable: 1, wantEvent: false,
		},
		{
			name: "at_threshold_emits_down", threshold: 2,
			prev: &ChannelState{
				Provider: "p", Service: "s", Channel: "c",
				StableAvailable: 1, KnownCount: 3, DownCount: 1,
			},
			downCount: 2, knownCount: 3, totalModels: 3,
			wantStable: 0, wantEvent: true, wantEventType: EventTypeDown,
		},
		{
			name: "up_emits_when_all_known_zero_down", threshold: 1,
			prev: &ChannelState{
				Provider: "p", Service: "s", Channel: "c",
				StableAvailable: 0, KnownCount: 2, DownCount: 1,
			},
			downCount: 0, knownCount: 2, totalModels: 2,
			wantStable: 1, wantEvent: true, wantEventType: EventTypeUp,
		},
		{
			name: "up_blocked_not_all_known", threshold: 1,
			prev: &ChannelState{
				Provider: "p", Service: "s", Channel: "c",
				StableAvailable: 0, KnownCount: 1, DownCount: 0,
			},
			downCount: 0, knownCount: 1, totalModels: 2,
			wantStable: 0, wantEvent: false,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewChannelDetector(ChannelDetectorConfig{DownThreshold: tt.threshold})
			record := testChannelRecord(int64(100+i), "model-x")

			result := detector.DetectChannelWithCounts(tt.prev, tt.downCount, tt.knownCount, tt.totalModels, record)

			if result.NewChannelState.StableAvailable != tt.wantStable {
				t.Fatalf("StableAvailable = %d, want %d", result.NewChannelState.StableAvailable, tt.wantStable)
			}
			if result.NewChannelState.DownCount != tt.downCount {
				t.Fatalf("DownCount = %d, want %d", result.NewChannelState.DownCount, tt.downCount)
			}
			if result.NewChannelState.KnownCount != tt.knownCount {
				t.Fatalf("KnownCount = %d, want %d", result.NewChannelState.KnownCount, tt.knownCount)
			}
			if (result.Event != nil) != tt.wantEvent {
				t.Fatalf("event presence = %v, want %v", result.Event != nil, tt.wantEvent)
			}
			if tt.wantEvent && result.Event.EventType != tt.wantEventType {
				t.Fatalf("EventType = %s, want %s", result.Event.EventType, tt.wantEventType)
			}
		})
	}
}

func testChannelRecord(id int64, model string) *storage.ProbeRecord {
	return &storage.ProbeRecord{
		ID:        id,
		Provider:  "provider-a",
		Service:   "service-a",
		Channel:   "channel-a",
		Model:     model,
		Status:    0,
		HttpCode:  500,
		Latency:   321,
		Timestamp: 1700000000 + id,
	}
}
