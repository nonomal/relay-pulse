package events

import (
	"reflect"
	"testing"

	"monitor/internal/config"
)

func TestService_UpdateActiveModels(t *testing.T) {
	monitors := []config.ServiceConfig{
		{Provider: "p1", Service: "s1", Channel: "c1", Model: "model-a"},
		{Provider: "p1", Service: "s1", Channel: "c1", Model: "model-a"},                        // 重复，应去重
		{Provider: "p1", Service: "s1", Channel: "c1", Model: "model-b"},                        // 同通道第二个模型
		{Provider: "p1", Service: "s1", Channel: "c1", Model: "model-disabled", Disabled: true}, // 应排除
		{Provider: "p1", Service: "s1", Channel: "c1", Model: "model-cold", Board: "cold"},      // boards 开启时排除
		{Provider: "p1", Service: "s1", Channel: "c1", Model: ""},                               // 空 model 应排除
		{Provider: "p1", Service: "s1", Channel: "c2", Model: "model-x"},                        // 另一个通道
		{Provider: "p2", Service: "s2", Channel: "c3", Model: "model-z"},                        // 另一个 provider
	}

	tests := []struct {
		name          string
		boardsEnabled bool
		want          map[string][]string
	}{
		{
			name:          "boards_disabled_keeps_cold",
			boardsEnabled: false,
			want: map[string][]string{
				"p1/s1/c1": {"model-a", "model-b", "model-cold"},
				"p1/s1/c2": {"model-x"},
				"p2/s2/c3": {"model-z"},
			},
		},
		{
			name:          "boards_enabled_excludes_cold",
			boardsEnabled: true,
			want: map[string][]string{
				"p1/s1/c1": {"model-a", "model-b"},
				"p1/s1/c2": {"model-x"},
				"p2/s2/c3": {"model-z"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &Service{
				activeModels: make(map[string][]string),
			}
			svc.UpdateActiveModels(monitors, tt.boardsEnabled)

			svc.activeModelsMu.RLock()
			got := cloneMap(svc.activeModels)
			svc.activeModelsMu.RUnlock()

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("activeModels =\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

func TestService_UpdateActiveModels_EmptyInput(t *testing.T) {
	svc := &Service{
		activeModels: map[string][]string{
			"old/key/here": {"model-old"},
		},
	}

	svc.UpdateActiveModels(nil, false)

	svc.activeModelsMu.RLock()
	count := len(svc.activeModels)
	svc.activeModelsMu.RUnlock()

	if count != 0 {
		t.Fatalf("empty monitors should clear active models, got %d entries", count)
	}
}

func TestService_enrichChannelEventMeta(t *testing.T) {
	modelStates := []*ServiceState{
		{Model: "model-down", StableAvailable: 0, StreakStatus: 0},
		{Model: "model-up", StableAvailable: 1, StreakStatus: 1},
		{Model: "model-unknown", StableAvailable: -1, StreakStatus: 0},
	}

	t.Run("down_event_models_alias", func(t *testing.T) {
		svc := &Service{}
		event := &StatusEvent{
			EventType: EventTypeDown,
			Meta:      map[string]any{"scope": "channel"},
		}

		svc.enrichChannelEventMeta(event, modelStates)

		// models 应指向 down_models
		gotModels, ok := event.Meta["models"].([]string)
		if !ok {
			t.Fatalf("models type = %T, want []string", event.Meta["models"])
		}
		if !reflect.DeepEqual(gotModels, []string{"model-down"}) {
			t.Fatalf("models = %v, want [model-down]", gotModels)
		}

		// down_models / up_models
		gotDown := event.Meta["down_models"].([]string)
		gotUp := event.Meta["up_models"].([]string)
		if !reflect.DeepEqual(gotDown, []string{"model-down"}) {
			t.Fatalf("down_models = %v", gotDown)
		}
		if !reflect.DeepEqual(gotUp, []string{"model-up"}) {
			t.Fatalf("up_models = %v", gotUp)
		}

		// model_states 应包含 3 个条目
		ms, ok := event.Meta["model_states"].(map[string]any)
		if !ok || len(ms) != 3 {
			t.Fatalf("model_states len = %d, want 3", len(ms))
		}

		// scope 不应被覆盖
		if event.Meta["scope"] != "channel" {
			t.Fatalf("scope was overwritten")
		}
	})

	t.Run("up_event_models_alias", func(t *testing.T) {
		svc := &Service{}
		event := &StatusEvent{
			EventType: EventTypeUp,
			Meta:      map[string]any{},
		}

		svc.enrichChannelEventMeta(event, modelStates)

		gotModels := event.Meta["models"].([]string)
		if !reflect.DeepEqual(gotModels, []string{"model-up"}) {
			t.Fatalf("models = %v, want [model-up]", gotModels)
		}
	})

	t.Run("nil_meta_initialized", func(t *testing.T) {
		svc := &Service{}
		event := &StatusEvent{
			EventType: EventTypeDown,
			Meta:      nil,
		}

		svc.enrichChannelEventMeta(event, modelStates)

		if event.Meta == nil {
			t.Fatal("Meta should be initialized")
		}
	})
}

func TestService_getActiveModels_ReturnsCopy(t *testing.T) {
	svc := &Service{
		activeModels: map[string][]string{
			"p/s/c": {"model-a", "model-b"},
		},
	}

	got := svc.getActiveModels("p", "s", "c")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	// 修改返回值不应影响内部状态
	got[0] = "MUTATED"
	internal := svc.getActiveModels("p", "s", "c")
	if internal[0] == "MUTATED" {
		t.Fatal("getActiveModels should return a copy, internal state was modified")
	}
}

func TestService_getActiveModels_MissingKey(t *testing.T) {
	svc := &Service{
		activeModels: make(map[string][]string),
	}

	got := svc.getActiveModels("nonexistent", "x", "y")
	if len(got) != 0 {
		t.Fatalf("missing key should return empty slice, got %v", got)
	}
}

func cloneMap(src map[string][]string) map[string][]string {
	dst := make(map[string][]string, len(src))
	for k, v := range src {
		dst[k] = append([]string(nil), v...)
	}
	return dst
}
