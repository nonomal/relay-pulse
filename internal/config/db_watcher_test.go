package config

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"
)

// mockVersionStorage 用于测试的 mock 版本存储
type mockVersionStorage struct {
	mu       sync.Mutex
	versions *ConfigVersionsRecord
	err      error
}

func (m *mockVersionStorage) setVersions(v *ConfigVersionsRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.versions = v
}

func (m *mockVersionStorage) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *mockVersionStorage) GetConfigVersions() (*ConfigVersionsRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	if m.versions == nil {
		return nil, nil
	}
	c := *m.versions
	return &c, nil
}

// 满足 AdminConfigStorage 接口（其余方法不需要实现）
func (m *mockVersionStorage) ListMonitorConfigs(_ *MonitorConfigFilter) ([]*MonitorConfigRecord, int, error) {
	// 返回一个最小有效监测项
	mc := &MonitorConfigRecord{
		ID:         1,
		Provider:   "test",
		Service:    "cc",
		Enabled:    true,
		ConfigBlob: `{"url":"https://example.com","method":"GET","category":"commercial"}`,
	}
	return []*MonitorConfigRecord{mc}, 1, nil
}
func (m *mockVersionStorage) GetMonitorSecret(_ int64) (*MonitorSecretRecord, error) {
	return nil, nil
}
func (m *mockVersionStorage) ListProviderPolicies() ([]*ProviderPolicyRecord, error) {
	return nil, nil
}
func (m *mockVersionStorage) ListBadgeDefinitions() ([]*BadgeDefinitionRecord, error) {
	return nil, nil
}
func (m *mockVersionStorage) ListBadgeBindings(_ *BadgeBindingFilter) ([]*BadgeBindingRecord, error) {
	return nil, nil
}
func (m *mockVersionStorage) ListBoardConfigs() ([]*BoardConfigRecord, error) { return nil, nil }
func (m *mockVersionStorage) GetGlobalSetting(key string) (*GlobalSettingRecord, error) {
	if key == "global" {
		return &GlobalSettingRecord{
			Key:   "global",
			Value: `{"interval":"1m","slow_latency":"5s","timeout":"10s","degraded_weight":0.7}`,
		}, nil
	}
	return nil, nil
}

func TestVersionsEqual(t *testing.T) {
	tests := []struct {
		name     string
		a, b     *ConfigVersionsRecord
		expected bool
	}{
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: false,
		},
		{
			name:     "a nil",
			a:        nil,
			b:        &ConfigVersionsRecord{},
			expected: false,
		},
		{
			name:     "equal",
			a:        &ConfigVersionsRecord{Monitors: 1, Policies: 2, Badges: 3, Boards: 4, Settings: 5},
			b:        &ConfigVersionsRecord{Monitors: 1, Policies: 2, Badges: 3, Boards: 4, Settings: 5},
			expected: true,
		},
		{
			name:     "monitors differ",
			a:        &ConfigVersionsRecord{Monitors: 1, Policies: 2, Badges: 3, Boards: 4, Settings: 5},
			b:        &ConfigVersionsRecord{Monitors: 2, Policies: 2, Badges: 3, Boards: 4, Settings: 5},
			expected: false,
		},
		{
			name:     "settings differ",
			a:        &ConfigVersionsRecord{Monitors: 1, Policies: 2, Badges: 3, Boards: 4, Settings: 5},
			b:        &ConfigVersionsRecord{Monitors: 1, Policies: 2, Badges: 3, Boards: 4, Settings: 6},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := versionsEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("versionsEqual() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCloneVersions(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := cloneVersions(nil)
		if result != nil {
			t.Error("cloneVersions(nil) should return nil")
		}
	})

	t.Run("deep copy", func(t *testing.T) {
		original := &ConfigVersionsRecord{Monitors: 1, Policies: 2}
		clone := cloneVersions(original)

		if clone == original {
			t.Error("clone should be a different pointer")
		}
		if clone.Monitors != 1 || clone.Policies != 2 {
			t.Error("clone values should match original")
		}

		// 修改 clone 不影响 original
		clone.Monitors = 99
		if original.Monitors != 1 {
			t.Error("modifying clone should not affect original")
		}
	})
}

func TestVersionDiff(t *testing.T) {
	tests := []struct {
		before, after int64
		expected      string
	}{
		{1, 1, "unchanged"},
		{1, 2, "1→2"},
		{5, 10, "5→10"},
	}

	for _, tt := range tests {
		result := versionDiff(tt.before, tt.after)
		if result != tt.expected {
			t.Errorf("versionDiff(%d, %d) = %q, want %q", tt.before, tt.after, result, tt.expected)
		}
	}
}

func TestDBWatcherValidation(t *testing.T) {
	t.Run("nil loader", func(t *testing.T) {
		w := &DBWatcher{}
		err := w.validate()
		if err == nil || err.Error() != "配置加载器为空" {
			t.Errorf("expected '配置加载器为空', got %v", err)
		}
	})

	t.Run("nil dbProvider", func(t *testing.T) {
		w := &DBWatcher{loader: &Loader{}}
		err := w.validate()
		if err == nil || err.Error() != "数据库配置提供者为空" {
			t.Errorf("expected '数据库配置提供者为空', got %v", err)
		}
	})

	t.Run("nil currentConfig", func(t *testing.T) {
		w := &DBWatcher{
			loader: &Loader{
				dbProvider: &DBConfigProvider{},
			},
		}
		err := w.validate()
		if err == nil || err.Error() != "当前配置为空，无法进行热更新" {
			t.Errorf("expected '当前配置为空', got %v", err)
		}
	})

	t.Run("valid", func(t *testing.T) {
		w := &DBWatcher{
			loader: &Loader{
				dbProvider:    &DBConfigProvider{},
				currentConfig: &AppConfig{},
			},
		}
		err := w.validate()
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})
}

func TestDBWatcherNextInterval(t *testing.T) {
	t.Run("no jitter", func(t *testing.T) {
		w := &DBWatcher{
			interval:    10 * time.Second,
			jitterRatio: 0,
		}
		interval := w.nextInterval()
		if interval != 10*time.Second {
			t.Errorf("expected 10s, got %v", interval)
		}
	})

	t.Run("with jitter", func(t *testing.T) {
		w := &DBWatcher{
			interval:    10 * time.Second,
			jitterRatio: 0.2,
			rng:         rand.New(rand.NewSource(42)),
		}

		// 运行多次，确保抖动在合理范围内
		for i := 0; i < 100; i++ {
			interval := w.nextInterval()
			minExpected := 8 * time.Second  // 10s * (1 - 0.2) = 8s
			maxExpected := 12 * time.Second // 10s * (1 + 0.2) = 12s
			if interval < minExpected || interval > maxExpected {
				t.Errorf("interval %v out of range [%v, %v]", interval, minExpected, maxExpected)
			}
		}
	})
}

func TestDBWatcherEstablishBaseline(t *testing.T) {
	mockStorage := &mockVersionStorage{
		versions: &ConfigVersionsRecord{
			Monitors: 1, Policies: 1, Badges: 1, Boards: 1, Settings: 1,
		},
	}

	w := &DBWatcher{
		loader: &Loader{
			dbProvider: &DBConfigProvider{storage: mockStorage},
		},
	}

	w.establishBaseline()

	if w.lastVersions == nil {
		t.Fatal("lastVersions should be set after establishing baseline")
	}
	if w.lastVersions.Monitors != 1 {
		t.Errorf("expected monitors version 1, got %d", w.lastVersions.Monitors)
	}
}

func TestDBWatcherPollOnceNoChange(t *testing.T) {
	mockStorage := &mockVersionStorage{
		versions: &ConfigVersionsRecord{
			Monitors: 1, Policies: 1, Badges: 1, Boards: 1, Settings: 1,
		},
	}

	reloadCalled := false
	w := &DBWatcher{
		loader: &Loader{
			dbProvider: &DBConfigProvider{storage: mockStorage},
		},
		onReload: func(cfg *AppConfig) {
			reloadCalled = true
		},
		lastVersions: &ConfigVersionsRecord{
			Monitors: 1, Policies: 1, Badges: 1, Boards: 1, Settings: 1,
		},
	}

	w.pollOnce(context.Background())

	if reloadCalled {
		t.Error("reload should not be called when versions haven't changed")
	}
}

func TestDBWatcherPollOnceWithChange(t *testing.T) {
	mockStorage := &mockVersionStorage{
		versions: &ConfigVersionsRecord{
			Monitors: 2, Policies: 1, Badges: 1, Boards: 1, Settings: 1,
		},
	}

	reloadCalled := false
	w := &DBWatcher{
		loader: &Loader{
			dbProvider:    &DBConfigProvider{storage: mockStorage},
			currentConfig: &AppConfig{Interval: "1m"},
		},
		onReload: func(cfg *AppConfig) {
			reloadCalled = true
		},
		lastVersions: &ConfigVersionsRecord{
			Monitors: 1, Policies: 1, Badges: 1, Boards: 1, Settings: 1,
		},
	}

	w.pollOnce(context.Background())

	if !reloadCalled {
		t.Error("reload should be called when versions change")
	}
	if w.lastVersions.Monitors != 2 {
		t.Errorf("lastVersions.Monitors should be updated to 2, got %d", w.lastVersions.Monitors)
	}
}

func TestDBWatcherStartStop(t *testing.T) {
	mockStorage := &mockVersionStorage{
		versions: &ConfigVersionsRecord{
			Monitors: 1, Policies: 1, Badges: 1, Boards: 1, Settings: 1,
		},
	}

	w := NewDBWatcher(
		&Loader{
			dbProvider:    &DBConfigProvider{storage: mockStorage},
			currentConfig: &AppConfig{Interval: "1m"},
		},
		func(cfg *AppConfig) {},
		WithDBWatchInterval(50*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())

	err := w.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 等待一小段时间确保基线建立
	time.Sleep(100 * time.Millisecond)

	if w.lastVersions == nil {
		t.Error("lastVersions should be set after start")
	}

	cancel()

	// 等待 goroutine 退出
	time.Sleep(100 * time.Millisecond)
}

func TestDBWatcherStopIdempotent(t *testing.T) {
	w := &DBWatcher{stopCh: make(chan struct{})}

	// 调用多次 Stop 不应 panic
	w.Stop()
	w.Stop()
	w.Stop()
}

func TestDBWatcherBaselineFailureRecovery(t *testing.T) {
	mockStorage := &mockVersionStorage{
		err: errors.New("connection failed"),
	}

	w := &DBWatcher{
		loader: &Loader{
			dbProvider: &DBConfigProvider{storage: mockStorage},
		},
	}

	// 建立基线失败
	w.establishBaseline()

	if w.lastVersions != nil {
		t.Error("lastVersions should be nil after baseline failure")
	}
	if !w.forceReloadNext {
		t.Error("forceReloadNext should be true after baseline failure")
	}
	if w.baselineOK {
		t.Error("baselineOK should be false after baseline failure")
	}
}

func TestDBWatcherPollOnceWithNilVersions(t *testing.T) {
	mockStorage := &mockVersionStorage{
		versions: nil, // 返回 nil
	}

	reloadCalled := false
	w := &DBWatcher{
		loader: &Loader{
			dbProvider: &DBConfigProvider{storage: mockStorage},
		},
		onReload: func(cfg *AppConfig) {
			reloadCalled = true
		},
		lastVersions: &ConfigVersionsRecord{
			Monitors: 1, Policies: 1, Badges: 1, Boards: 1, Settings: 1,
		},
	}

	// 不应 panic
	w.pollOnce(context.Background())

	if reloadCalled {
		t.Error("reload should not be called when versions is nil")
	}
}

func TestDBWatcherForceReloadAfterBaselineRecovery(t *testing.T) {
	mockStorage := &mockVersionStorage{
		versions: &ConfigVersionsRecord{
			Monitors: 1, Policies: 1, Badges: 1, Boards: 1, Settings: 1,
		},
	}

	reloadCalled := false
	w := &DBWatcher{
		loader: &Loader{
			dbProvider:    &DBConfigProvider{storage: mockStorage},
			currentConfig: &AppConfig{Interval: "1m"},
		},
		onReload: func(cfg *AppConfig) {
			reloadCalled = true
		},
		forceReloadNext: true, // 模拟基线失败后的状态
		baselineOK:      false,
	}

	w.pollOnce(context.Background())

	if !reloadCalled {
		t.Error("reload should be called when forceReloadNext is true")
	}
	if w.forceReloadNext {
		t.Error("forceReloadNext should be reset to false")
	}
	if !w.baselineOK {
		t.Error("baselineOK should be true after recovery")
	}
}

func TestDBWatcherEstablishBaselineWithNilVersions(t *testing.T) {
	mockStorage := &mockVersionStorage{
		versions: nil, // 返回 nil
	}

	w := &DBWatcher{
		loader: &Loader{
			dbProvider: &DBConfigProvider{storage: mockStorage},
		},
	}

	w.establishBaseline()

	if w.lastVersions != nil {
		t.Error("lastVersions should remain nil when versions is nil")
	}
	if !w.forceReloadNext {
		t.Error("forceReloadNext should be true when versions is nil")
	}
	if w.baselineOK {
		t.Error("baselineOK should be false when versions is nil")
	}
}
