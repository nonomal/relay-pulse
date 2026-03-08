package automove

import (
	"context"
	"testing"
	"time"

	"monitor/internal/config"
	"monitor/internal/storage"
)

// mockStorage 实现 storage.Storage 接口的测试替身
type mockStorage struct {
	history map[storage.MonitorKey][]*storage.ProbeRecord
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		history: make(map[storage.MonitorKey][]*storage.ProbeRecord),
	}
}

func (m *mockStorage) Init() error                                   { return nil }
func (m *mockStorage) Close() error                                  { return nil }
func (m *mockStorage) WithContext(_ context.Context) storage.Storage { return m }
func (m *mockStorage) SaveRecord(_ *storage.ProbeRecord) error       { return nil }
func (m *mockStorage) GetLatest(_, _, _, _ string) (*storage.ProbeRecord, error) {
	return nil, nil
}
func (m *mockStorage) GetHistory(_, _, _, _ string, _ time.Time) ([]*storage.ProbeRecord, error) {
	return nil, nil
}
func (m *mockStorage) GetLatestBatch(_ []storage.MonitorKey) (map[storage.MonitorKey]*storage.ProbeRecord, error) {
	return nil, nil
}
func (m *mockStorage) GetHistoryBatch(keys []storage.MonitorKey, _ time.Time) (map[storage.MonitorKey][]*storage.ProbeRecord, error) {
	result := make(map[storage.MonitorKey][]*storage.ProbeRecord)
	for _, k := range keys {
		if records, ok := m.history[k]; ok {
			result[k] = records
		}
	}
	return result, nil
}
func (m *mockStorage) MigrateChannelData(_ []storage.ChannelMigrationMapping) error { return nil }
func (m *mockStorage) GetServiceState(_, _, _, _ string) (*storage.ServiceState, error) {
	return nil, nil
}
func (m *mockStorage) UpsertServiceState(_ *storage.ServiceState) error { return nil }
func (m *mockStorage) GetChannelState(_, _, _ string) (*storage.ChannelState, error) {
	return nil, nil
}
func (m *mockStorage) UpsertChannelState(_ *storage.ChannelState) error { return nil }
func (m *mockStorage) GetModelStatesForChannel(_, _, _ string) ([]*storage.ServiceState, error) {
	return nil, nil
}
func (m *mockStorage) SaveStatusEvent(_ *storage.StatusEvent) error { return nil }
func (m *mockStorage) GetStatusEvents(_ int64, _ int, _ *storage.EventFilters) ([]*storage.StatusEvent, error) {
	return nil, nil
}
func (m *mockStorage) GetLatestEventID() (int64, error) { return 0, nil }
func (m *mockStorage) PurgeOldRecords(_ context.Context, _ time.Time, _ int) (int64, error) {
	return 0, nil
}

// makeRecords 生成指定状态的探测记录
func makeRecords(status int, count int) []*storage.ProbeRecord {
	records := make([]*storage.ProbeRecord, count)
	for i := range records {
		records[i] = &storage.ProbeRecord{Status: status}
	}
	return records
}

func TestEvaluate_DualThreshold_DemoteHot(t *testing.T) {
	store := newMockStorage()
	key := storage.MonitorKey{Provider: "bad", Service: "cc", Channel: "vip"}

	// 100% red → availability=0% < threshold_down=50%
	store.history[key] = makeRecords(0, 20)

	cfg := &config.AppConfig{
		Boards: config.BoardsConfig{
			Enabled: true,
			AutoMove: config.BoardAutoMoveConfig{
				Enabled:               true,
				ThresholdDown:         50.0,
				ThresholdUp:           55.0,
				CheckInterval:         "30m",
				CheckIntervalDuration: 30 * time.Minute,
				MinProbes:             10,
			},
		},
		DegradedWeight:    0.7,
		BatchQueryMaxKeys: 300,
		Monitors: []config.ServiceConfig{
			{Provider: "bad", Service: "cc", Channel: "vip", Board: "hot"},
		},
	}

	svc := NewService(store, cfg)
	svc.Evaluate(context.Background())

	ov, ok := svc.GetBoardOverride(key)
	if !ok {
		t.Fatal("expected override for bad/cc/vip")
	}
	if ov.Board != "secondary" {
		t.Errorf("expected board=secondary, got %s", ov.Board)
	}
}

func TestEvaluate_DualThreshold_PromoteSecondary(t *testing.T) {
	store := newMockStorage()
	key := storage.MonitorKey{Provider: "good", Service: "cc", Channel: "vip"}

	// 100% green → availability=100% >= threshold_up=55%
	store.history[key] = makeRecords(1, 20)

	cfg := &config.AppConfig{
		Boards: config.BoardsConfig{
			Enabled: true,
			AutoMove: config.BoardAutoMoveConfig{
				Enabled:               true,
				ThresholdDown:         50.0,
				ThresholdUp:           55.0,
				CheckInterval:         "30m",
				CheckIntervalDuration: 30 * time.Minute,
				MinProbes:             10,
			},
		},
		DegradedWeight:    0.7,
		BatchQueryMaxKeys: 300,
		Monitors: []config.ServiceConfig{
			{Provider: "good", Service: "cc", Channel: "vip", Board: "secondary"},
		},
	}

	svc := NewService(store, cfg)
	svc.Evaluate(context.Background())

	ov, ok := svc.GetBoardOverride(key)
	if !ok {
		t.Fatal("expected override for good/cc/vip")
	}
	if ov.Board != "hot" {
		t.Errorf("expected board=hot, got %s", ov.Board)
	}
}

func TestEvaluate_DualThreshold_HysteresisBuffer(t *testing.T) {
	store := newMockStorage()
	key := storage.MonitorKey{Provider: "mid", Service: "cc", Channel: "vip"}

	// 52% availability: between threshold_down(50%) and threshold_up(55%)
	// 52 green + 48 red out of 100 → 52%
	records := make([]*storage.ProbeRecord, 100)
	for i := 0; i < 52; i++ {
		records[i] = &storage.ProbeRecord{Status: 1}
	}
	for i := 52; i < 100; i++ {
		records[i] = &storage.ProbeRecord{Status: 0}
	}
	store.history[key] = records

	// As secondary with 52% (< threshold_up 55%): should NOT promote
	cfg := &config.AppConfig{
		Boards: config.BoardsConfig{
			Enabled: true,
			AutoMove: config.BoardAutoMoveConfig{
				Enabled:               true,
				ThresholdDown:         50.0,
				ThresholdUp:           55.0,
				CheckInterval:         "30m",
				CheckIntervalDuration: 30 * time.Minute,
				MinProbes:             10,
			},
		},
		DegradedWeight:    0.7,
		BatchQueryMaxKeys: 300,
		Monitors: []config.ServiceConfig{
			{Provider: "mid", Service: "cc", Channel: "vip", Board: "secondary"},
		},
	}

	svc := NewService(store, cfg)
	svc.Evaluate(context.Background())

	_, ok := svc.GetBoardOverride(key)
	if ok {
		t.Error("expected no override for secondary monitor at 52% (between thresholds)")
	}

	// As hot with 52% (> threshold_down 50%): should NOT demote
	cfg.Monitors[0].Board = "hot"
	svc2 := NewService(store, cfg)
	svc2.Evaluate(context.Background())

	_, ok = svc2.GetBoardOverride(key)
	if ok {
		t.Error("expected no override for hot monitor at 52% (between thresholds)")
	}
}

func TestEvaluate_DualThreshold_PreviousOverridePreserved(t *testing.T) {
	store := newMockStorage()
	key := storage.MonitorKey{Provider: "mid", Service: "cc", Channel: "vip"}

	// 52% availability: between threshold_down(50%) and threshold_up(55%)
	records := make([]*storage.ProbeRecord, 100)
	for i := 0; i < 52; i++ {
		records[i] = &storage.ProbeRecord{Status: 1}
	}
	for i := 52; i < 100; i++ {
		records[i] = &storage.ProbeRecord{Status: 0}
	}
	store.history[key] = records

	cfg := &config.AppConfig{
		Boards: config.BoardsConfig{
			Enabled: true,
			AutoMove: config.BoardAutoMoveConfig{
				Enabled:               true,
				ThresholdDown:         50.0,
				ThresholdUp:           55.0,
				CheckInterval:         "30m",
				CheckIntervalDuration: 30 * time.Minute,
				MinProbes:             10,
			},
		},
		DegradedWeight:    0.7,
		BatchQueryMaxKeys: 300,
		Monitors: []config.ServiceConfig{
			{Provider: "mid", Service: "cc", Channel: "vip", Board: "hot"},
		},
	}

	svc := NewService(store, cfg)

	// First: demote with 0% availability
	store.history[key] = makeRecords(0, 100)
	svc.Evaluate(context.Background())
	ov, ok := svc.GetBoardOverride(key)
	if !ok || ov.Board != "secondary" {
		t.Fatal("expected demote to secondary")
	}

	// Second: availability recovers to 52% (in buffer zone)
	// Override should be preserved — still secondary
	store.history[key] = records
	svc.Evaluate(context.Background())
	ov, ok = svc.GetBoardOverride(key)
	if !ok || ov.Board != "secondary" {
		t.Errorf("expected override preserved as secondary in buffer zone, got ok=%v board=%s", ok, ov.Board)
	}
}

func TestEvaluate_MinProbes_Skip(t *testing.T) {
	store := newMockStorage()
	key := storage.MonitorKey{Provider: "new", Service: "cc", Channel: "vip"}

	// Only 5 records < min_probes=10: should skip
	store.history[key] = makeRecords(0, 5)

	cfg := &config.AppConfig{
		Boards: config.BoardsConfig{
			Enabled: true,
			AutoMove: config.BoardAutoMoveConfig{
				Enabled:               true,
				ThresholdDown:         50.0,
				ThresholdUp:           55.0,
				CheckInterval:         "30m",
				CheckIntervalDuration: 30 * time.Minute,
				MinProbes:             10,
			},
		},
		DegradedWeight:    0.7,
		BatchQueryMaxKeys: 300,
		Monitors: []config.ServiceConfig{
			{Provider: "new", Service: "cc", Channel: "vip", Board: "hot"},
		},
	}

	svc := NewService(store, cfg)
	svc.Evaluate(context.Background())

	_, ok := svc.GetBoardOverride(key)
	if ok {
		t.Error("expected no override when probes < min_probes")
	}
}

func TestEvaluate_ColdExcluded(t *testing.T) {
	store := newMockStorage()
	key := storage.MonitorKey{Provider: "cold", Service: "cc", Channel: "vip"}

	store.history[key] = makeRecords(0, 20)

	cfg := &config.AppConfig{
		Boards: config.BoardsConfig{
			Enabled: true,
			AutoMove: config.BoardAutoMoveConfig{
				Enabled:               true,
				ThresholdDown:         50.0,
				ThresholdUp:           55.0,
				CheckInterval:         "30m",
				CheckIntervalDuration: 30 * time.Minute,
				MinProbes:             10,
			},
		},
		DegradedWeight:    0.7,
		BatchQueryMaxKeys: 300,
		Monitors: []config.ServiceConfig{
			{Provider: "cold", Service: "cc", Channel: "vip", Board: "cold"},
		},
	}

	svc := NewService(store, cfg)
	svc.Evaluate(context.Background())

	_, ok := svc.GetBoardOverride(key)
	if ok {
		t.Error("expected no override for cold board monitors")
	}
}

func TestEvaluate_DisabledClears(t *testing.T) {
	store := newMockStorage()
	key := storage.MonitorKey{Provider: "bad", Service: "cc", Channel: "vip"}
	store.history[key] = makeRecords(0, 20)

	cfg := &config.AppConfig{
		Boards: config.BoardsConfig{
			Enabled: true,
			AutoMove: config.BoardAutoMoveConfig{
				Enabled:               true,
				ThresholdDown:         50.0,
				ThresholdUp:           55.0,
				CheckInterval:         "30m",
				CheckIntervalDuration: 30 * time.Minute,
				MinProbes:             10,
			},
		},
		DegradedWeight:    0.7,
		BatchQueryMaxKeys: 300,
		Monitors: []config.ServiceConfig{
			{Provider: "bad", Service: "cc", Channel: "vip", Board: "hot"},
		},
	}

	svc := NewService(store, cfg)
	svc.Evaluate(context.Background())

	// Verify override exists
	_, ok := svc.GetBoardOverride(key)
	if !ok {
		t.Fatal("expected override after evaluate")
	}

	// Disable auto_move → UpdateConfig should clear overrides
	cfg2 := *cfg
	cfg2.Boards.AutoMove.Enabled = false
	svc.UpdateConfig(&cfg2)

	_, ok = svc.GetBoardOverride(key)
	if ok {
		t.Error("expected overrides cleared after disabling auto_move")
	}
}

func TestEvaluate_ExpiredChannel_DemotedAndDowngraded(t *testing.T) {
	store := newMockStorage()
	key := storage.MonitorKey{Provider: "expired", Service: "cc", Channel: "vip"}

	// 即使可用率 100%，到期后也应降级
	store.history[key] = makeRecords(1, 20)

	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	cfg := &config.AppConfig{
		Boards: config.BoardsConfig{
			Enabled: true,
			AutoMove: config.BoardAutoMoveConfig{
				Enabled:               true,
				ThresholdDown:         50.0,
				ThresholdUp:           55.0,
				CheckInterval:         "30m",
				CheckIntervalDuration: 30 * time.Minute,
				MinProbes:             10,
			},
		},
		DegradedWeight:    0.7,
		BatchQueryMaxKeys: 300,
		Monitors: []config.ServiceConfig{
			{
				Provider:     "expired",
				Service:      "cc",
				Channel:      "vip",
				Board:        "hot",
				SponsorLevel: config.SponsorLevelBackbone,
				ExpiresAt:    yesterday,
			},
		},
	}

	svc := NewService(store, cfg)
	svc.Evaluate(context.Background())

	ov, ok := svc.GetBoardOverride(key)
	if !ok {
		t.Fatal("expected override for expired channel")
	}
	if ov.Board != "secondary" {
		t.Errorf("expected board=secondary, got %s", ov.Board)
	}
	if ov.SponsorLevel != config.SponsorLevelPulse {
		t.Errorf("expected sponsor_level=pulse, got %s", ov.SponsorLevel)
	}
}

func TestEvaluate_NotYetExpired_NoExpiryOverride(t *testing.T) {
	store := newMockStorage()
	key := storage.MonitorKey{Provider: "active", Service: "cc", Channel: "vip"}

	// 100% green → should not be demoted by availability or expiry
	store.history[key] = makeRecords(1, 20)

	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	cfg := &config.AppConfig{
		Boards: config.BoardsConfig{
			Enabled: true,
			AutoMove: config.BoardAutoMoveConfig{
				Enabled:               true,
				ThresholdDown:         50.0,
				ThresholdUp:           55.0,
				CheckInterval:         "30m",
				CheckIntervalDuration: 30 * time.Minute,
				MinProbes:             10,
			},
		},
		DegradedWeight:    0.7,
		BatchQueryMaxKeys: 300,
		Monitors: []config.ServiceConfig{
			{
				Provider:     "active",
				Service:      "cc",
				Channel:      "vip",
				Board:        "hot",
				SponsorLevel: config.SponsorLevelBackbone,
				ExpiresAt:    tomorrow,
			},
		},
	}

	svc := NewService(store, cfg)
	svc.Evaluate(context.Background())

	_, ok := svc.GetBoardOverride(key)
	if ok {
		t.Error("expected no override for not-yet-expired channel")
	}
}

func TestEvaluate_ExpiresToday_StillValid(t *testing.T) {
	store := newMockStorage()
	key := storage.MonitorKey{Provider: "today", Service: "cc", Channel: "vip"}

	store.history[key] = makeRecords(1, 20)

	today := time.Now().UTC().Format("2006-01-02")
	cfg := &config.AppConfig{
		Boards: config.BoardsConfig{
			Enabled: true,
			AutoMove: config.BoardAutoMoveConfig{
				Enabled:               true,
				ThresholdDown:         50.0,
				ThresholdUp:           55.0,
				CheckInterval:         "30m",
				CheckIntervalDuration: 30 * time.Minute,
				MinProbes:             10,
			},
		},
		DegradedWeight:    0.7,
		BatchQueryMaxKeys: 300,
		Monitors: []config.ServiceConfig{
			{
				Provider:     "today",
				Service:      "cc",
				Channel:      "vip",
				Board:        "hot",
				SponsorLevel: config.SponsorLevelCore,
				ExpiresAt:    today,
			},
		},
	}

	svc := NewService(store, cfg)
	svc.Evaluate(context.Background())

	_, ok := svc.GetBoardOverride(key)
	if ok {
		t.Error("expected no override for channel expiring today (still valid)")
	}
}

func TestEvaluate_ExpiredAndAvailability_Coexist(t *testing.T) {
	store := newMockStorage()
	expiredKey := storage.MonitorKey{Provider: "expired", Service: "cc", Channel: "vip"}
	badKey := storage.MonitorKey{Provider: "bad", Service: "cc", Channel: "vip"}

	store.history[expiredKey] = makeRecords(1, 20) // good availability but expired
	store.history[badKey] = makeRecords(0, 20)     // bad availability

	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	cfg := &config.AppConfig{
		Boards: config.BoardsConfig{
			Enabled: true,
			AutoMove: config.BoardAutoMoveConfig{
				Enabled:               true,
				ThresholdDown:         50.0,
				ThresholdUp:           55.0,
				CheckInterval:         "30m",
				CheckIntervalDuration: 30 * time.Minute,
				MinProbes:             10,
			},
		},
		DegradedWeight:    0.7,
		BatchQueryMaxKeys: 300,
		Monitors: []config.ServiceConfig{
			{Provider: "expired", Service: "cc", Channel: "vip", Board: "hot", SponsorLevel: config.SponsorLevelBeacon, ExpiresAt: yesterday},
			{Provider: "bad", Service: "cc", Channel: "vip", Board: "hot"},
		},
	}

	svc := NewService(store, cfg)
	svc.Evaluate(context.Background())

	// Expired channel: board=secondary, sponsor_level=pulse
	ov, ok := svc.GetBoardOverride(expiredKey)
	if !ok {
		t.Fatal("expected override for expired channel")
	}
	if ov.Board != "secondary" || ov.SponsorLevel != config.SponsorLevelPulse {
		t.Errorf("expired: expected board=secondary+level=pulse, got board=%s level=%s", ov.Board, ov.SponsorLevel)
	}

	// Bad availability channel: board=secondary, sponsor_level unchanged (empty)
	ov2, ok := svc.GetBoardOverride(badKey)
	if !ok {
		t.Fatal("expected override for bad availability channel")
	}
	if ov2.Board != "secondary" {
		t.Errorf("bad availability: expected board=secondary, got %s", ov2.Board)
	}
	if ov2.SponsorLevel != "" {
		t.Errorf("bad availability: expected empty sponsor_level (no downgrade), got %s", ov2.SponsorLevel)
	}
}

func TestEvaluate_ChildMonitorsExcluded(t *testing.T) {
	store := newMockStorage()
	childKey := storage.MonitorKey{Provider: "p", Service: "s", Channel: "c", Model: "child-model"}
	store.history[childKey] = makeRecords(0, 20)

	cfg := &config.AppConfig{
		Boards: config.BoardsConfig{
			Enabled: true,
			AutoMove: config.BoardAutoMoveConfig{
				Enabled:               true,
				ThresholdDown:         50.0,
				ThresholdUp:           55.0,
				CheckInterval:         "30m",
				CheckIntervalDuration: 30 * time.Minute,
				MinProbes:             10,
			},
		},
		DegradedWeight:    0.7,
		BatchQueryMaxKeys: 300,
		Monitors: []config.ServiceConfig{
			{Provider: "p", Service: "s", Channel: "c", Model: "child-model", Parent: "p/s/c", Board: "hot"},
		},
	}

	svc := NewService(store, cfg)
	svc.Evaluate(context.Background())

	_, ok := svc.GetBoardOverride(childKey)
	if ok {
		t.Error("expected no override for child monitors (have parent)")
	}
}
