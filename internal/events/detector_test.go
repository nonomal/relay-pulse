package events

import (
	"testing"

	"monitor/internal/storage"
)

func TestNewDetector(t *testing.T) {
	tests := []struct {
		name    string
		cfg     DetectorConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     DetectorConfig{DownThreshold: 2, UpThreshold: 1},
			wantErr: false,
		},
		{
			name:    "down_threshold zero",
			cfg:     DetectorConfig{DownThreshold: 0, UpThreshold: 1},
			wantErr: true,
		},
		{
			name:    "up_threshold zero",
			cfg:     DetectorConfig{DownThreshold: 2, UpThreshold: 0},
			wantErr: true,
		},
		{
			name:    "both negative",
			cfg:     DetectorConfig{DownThreshold: -1, UpThreshold: -1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDetector(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDetector() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDetector_Detect_FirstProbe(t *testing.T) {
	detector, _ := NewDetector(DetectorConfig{DownThreshold: 2, UpThreshold: 1})

	// 首次探测绿色
	record := &storage.ProbeRecord{
		ID:        1,
		Provider:  "test-provider",
		Service:   "test-service",
		Channel:   "",
		Status:    1, // 绿色
		Timestamp: 1000,
	}

	newState, event, err := detector.Detect(nil, record)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	// 首次探测不应产生事件
	if event != nil {
		t.Errorf("首次探测不应产生事件, got event: %+v", event)
	}

	// 状态应该初始化为可用
	if newState.StableAvailable != 1 {
		t.Errorf("StableAvailable = %d, want 1", newState.StableAvailable)
	}
	if newState.StreakCount != 1 {
		t.Errorf("StreakCount = %d, want 1", newState.StreakCount)
	}
}

func TestDetector_Detect_FirstProbeDown(t *testing.T) {
	detector, _ := NewDetector(DetectorConfig{DownThreshold: 2, UpThreshold: 1})

	// 首次探测红色
	record := &storage.ProbeRecord{
		ID:        1,
		Provider:  "test-provider",
		Service:   "test-service",
		Channel:   "",
		Status:    0, // 红色
		Timestamp: 1000,
	}

	newState, event, err := detector.Detect(nil, record)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	// 首次探测不应产生事件
	if event != nil {
		t.Errorf("首次探测不应产生事件, got event: %+v", event)
	}

	// 状态应该初始化为不可用
	if newState.StableAvailable != 0 {
		t.Errorf("StableAvailable = %d, want 0", newState.StableAvailable)
	}
}

func TestDetector_Detect_DownEvent(t *testing.T) {
	detector, _ := NewDetector(DetectorConfig{DownThreshold: 2, UpThreshold: 1})

	// 初始状态：可用
	prevState := &ServiceState{
		Provider:        "test-provider",
		Service:         "test-service",
		Channel:         "",
		StableAvailable: 1,
		StreakCount:     0,
		StreakStatus:    1,
		LastRecordID:    1,
		LastTimestamp:   1000,
	}

	// 第一次红色：不触发事件
	record1 := &storage.ProbeRecord{
		ID:        2,
		Provider:  "test-provider",
		Service:   "test-service",
		Channel:   "",
		Status:    0,
		Timestamp: 2000,
	}

	state1, event1, err := detector.Detect(prevState, record1)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if event1 != nil {
		t.Errorf("第一次红色不应触发事件, got: %+v", event1)
	}
	if state1.StreakCount != 1 {
		t.Errorf("StreakCount = %d, want 1", state1.StreakCount)
	}

	// 第二次红色：触发 DOWN 事件
	record2 := &storage.ProbeRecord{
		ID:        3,
		Provider:  "test-provider",
		Service:   "test-service",
		Channel:   "",
		Status:    0,
		HttpCode:  503,
		SubStatus: storage.SubStatusServerError,
		Timestamp: 3000,
	}

	state2, event2, err := detector.Detect(state1, record2)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if event2 == nil {
		t.Fatal("第二次红色应触发 DOWN 事件")
	}
	if event2.EventType != EventTypeDown {
		t.Errorf("EventType = %s, want DOWN", event2.EventType)
	}
	if event2.FromStatus != 1 || event2.ToStatus != 0 {
		t.Errorf("FromStatus = %d, ToStatus = %d, want 1 → 0", event2.FromStatus, event2.ToStatus)
	}

	// 稳定态应更新为不可用
	if state2.StableAvailable != 0 {
		t.Errorf("StableAvailable = %d, want 0", state2.StableAvailable)
	}
}

func TestDetector_Detect_UpEvent(t *testing.T) {
	detector, _ := NewDetector(DetectorConfig{DownThreshold: 2, UpThreshold: 1})

	// 初始状态：不可用
	prevState := &ServiceState{
		Provider:        "test-provider",
		Service:         "test-service",
		Channel:         "",
		StableAvailable: 0,
		StreakCount:     0,
		StreakStatus:    0,
		LastRecordID:    1,
		LastTimestamp:   1000,
	}

	// 第一次绿色：触发 UP 事件（阈值为 1）
	record := &storage.ProbeRecord{
		ID:        2,
		Provider:  "test-provider",
		Service:   "test-service",
		Channel:   "",
		Status:    1,
		HttpCode:  200,
		Timestamp: 2000,
	}

	newState, event, err := detector.Detect(prevState, record)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if event == nil {
		t.Fatal("应触发 UP 事件")
	}
	if event.EventType != EventTypeUp {
		t.Errorf("EventType = %s, want UP", event.EventType)
	}
	if event.FromStatus != 0 || event.ToStatus != 1 {
		t.Errorf("FromStatus = %d, ToStatus = %d, want 0 → 1", event.FromStatus, event.ToStatus)
	}

	// 稳定态应更新为可用
	if newState.StableAvailable != 1 {
		t.Errorf("StableAvailable = %d, want 1", newState.StableAvailable)
	}
}

func TestDetector_Detect_YellowAsAvailable(t *testing.T) {
	detector, _ := NewDetector(DetectorConfig{DownThreshold: 2, UpThreshold: 1})

	// 初始状态：不可用
	prevState := &ServiceState{
		Provider:        "test-provider",
		Service:         "test-service",
		Channel:         "",
		StableAvailable: 0,
		StreakCount:     0,
		StreakStatus:    0,
		LastRecordID:    1,
		LastTimestamp:   1000,
	}

	// 黄色状态应视为可用，触发 UP 事件
	record := &storage.ProbeRecord{
		ID:        2,
		Provider:  "test-provider",
		Service:   "test-service",
		Channel:   "",
		Status:    2, // 黄色
		HttpCode:  200,
		SubStatus: storage.SubStatusSlowLatency,
		Timestamp: 2000,
	}

	newState, event, err := detector.Detect(prevState, record)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if event == nil {
		t.Fatal("黄色状态应触发 UP 事件")
	}
	if event.EventType != EventTypeUp {
		t.Errorf("EventType = %s, want UP", event.EventType)
	}

	// 稳定态应更新为可用
	if newState.StableAvailable != 1 {
		t.Errorf("StableAvailable = %d, want 1", newState.StableAvailable)
	}
}

func TestDetector_Detect_Flapping(t *testing.T) {
	detector, _ := NewDetector(DetectorConfig{DownThreshold: 3, UpThreshold: 2})

	// 初始状态：可用
	state := &ServiceState{
		Provider:        "test-provider",
		Service:         "test-service",
		Channel:         "",
		StableAvailable: 1,
		StreakCount:     0,
		StreakStatus:    1,
		LastRecordID:    1,
		LastTimestamp:   1000,
	}

	// 模拟抖动：红-绿-红-绿，不应触发事件
	statuses := []int{0, 1, 0, 1}
	for i, status := range statuses {
		record := &storage.ProbeRecord{
			ID:        int64(i + 2),
			Provider:  "test-provider",
			Service:   "test-service",
			Channel:   "",
			Status:    status,
			Timestamp: int64(2000 + i*1000),
		}

		var event *StatusEvent
		var err error
		state, event, err = detector.Detect(state, record)
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if event != nil {
			t.Errorf("抖动场景不应触发事件, record %d got event: %+v", i, event)
		}
	}

	// 稳定态应保持可用
	if state.StableAvailable != 1 {
		t.Errorf("抖动后 StableAvailable = %d, want 1", state.StableAvailable)
	}
}

func TestDetector_Detect_NilRecord(t *testing.T) {
	detector, _ := NewDetector(DetectorConfig{DownThreshold: 2, UpThreshold: 1})

	_, _, err := detector.Detect(nil, nil)
	if err == nil {
		t.Error("应返回错误当 record 为 nil")
	}
}
