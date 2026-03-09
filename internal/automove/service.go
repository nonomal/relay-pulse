package automove

import (
	"context"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"monitor/internal/config"
	"monitor/internal/logger"
	"monitor/internal/storage"
)

// MonitorOverride 运行时覆盖字段（不修改配置，仅在 API 层应用）。
type MonitorOverride struct {
	Board        string              // 板块覆盖（"hot"/"secondary"）
	SponsorLevel config.SponsorLevel // 赞助等级覆盖（空值表示不覆盖）
}

// Service 自动移板服务。
// 定期基于 7 天可用率和赞助到期状态评估 hot ↔ secondary 归属，维护运行时 override map。
// 调度器和配置文件不受影响，仅在 API 层覆盖 board/sponsor_level 字段。
type Service struct {
	storage storage.Storage

	cfgMu sync.RWMutex
	cfg   *config.AppConfig

	// 原子指针替换：evaluate 生成新 map → Store；Handler 读取 → Load。
	// nil 表示无 override（auto_move 未启用或无需移板）。
	overrides atomic.Pointer[map[storage.MonitorKey]MonitorOverride]

	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewService 创建自动移板服务（不启动 goroutine）。
func NewService(store storage.Storage, cfg *config.AppConfig) *Service {
	svc := &Service{
		storage: store,
		stopCh:  make(chan struct{}),
	}
	svc.cfg = cfg
	return svc
}

// Start 在独立 goroutine 中启动定时评估循环。
// 启动时立即执行一次 Evaluate。
func (s *Service) Start(ctx context.Context) {
	go s.loop(ctx)
}

// Stop 优雅关闭评估循环。
func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

// UpdateConfig 热更新配置。若 auto_move 被禁用，立即清空 override。
// 若仍启用，清理新配置中已不再参与自动移板的监测项的旧 override
// （board 变为 cold、被 disabled/hidden、或变为子通道的情况）。
func (s *Service) UpdateConfig(cfg *config.AppConfig) {
	s.cfgMu.Lock()
	s.cfg = cfg
	s.cfgMu.Unlock()

	// 若禁用，立即清空 override
	if !cfg.Boards.Enabled || !cfg.Boards.AutoMove.Enabled {
		s.overrides.Store(nil)
		return
	}

	// auto_move 仍启用：清理已不再参与自动移板的监测项的旧 override
	s.purgeStaleOverrides(cfg)
}

// purgeStaleOverrides 从当前 override map 中移除不再符合自动移板条件的 key。
// 保留条件与 evaluate() 一致：非 disabled、非 hidden、无 parent、board != cold。
func (s *Service) purgeStaleOverrides(cfg *config.AppConfig) {
	ptr := s.overrides.Load()
	if ptr == nil {
		return
	}

	// 构建仍可参与自动移板的 key 集合（与 evaluate 中的过滤逻辑一致）
	eligible := make(map[storage.MonitorKey]struct{})
	for _, m := range cfg.Monitors {
		if m.Disabled || m.Hidden {
			continue
		}
		if strings.TrimSpace(m.Parent) != "" {
			continue
		}
		board := strings.ToLower(strings.TrimSpace(m.Board))
		if board == "" {
			board = "hot"
		}
		if board == "cold" {
			continue
		}
		key := storage.MonitorKey{
			Provider: m.Provider,
			Service:  m.Service,
			Channel:  m.Channel,
			Model:    m.Model,
		}
		eligible[key] = struct{}{}
	}

	// 构建新 map，仅保留仍符合条件的 override（不原地修改，保证并发安全）
	filtered := make(map[storage.MonitorKey]MonitorOverride)
	for key, ov := range *ptr {
		if _, ok := eligible[key]; ok {
			filtered[key] = ov
		}
	}

	if len(filtered) == 0 {
		s.overrides.Store(nil)
	} else {
		s.overrides.Store(&filtered)
	}
}

// GetBoardOverride 查询指定监测项的 override。
// 返回 (MonitorOverride{}, false) 表示无 override，应使用配置原始值。
func (s *Service) GetBoardOverride(key storage.MonitorKey) (MonitorOverride, bool) {
	ptr := s.overrides.Load()
	if ptr == nil {
		return MonitorOverride{}, false
	}
	ov, ok := (*ptr)[key]
	return ov, ok
}

// Overrides 返回当前 override map 快照（只读）。
// 调用方应在单次请求内缓存返回值以保证一致性。
func (s *Service) Overrides() map[storage.MonitorKey]MonitorOverride {
	ptr := s.overrides.Load()
	if ptr == nil {
		return nil
	}
	return *ptr
}

// SetOverrides 替换当前 override map（用于测试注入）。
func (s *Service) SetOverrides(overrides map[storage.MonitorKey]MonitorOverride) {
	if len(overrides) == 0 {
		s.overrides.Store(nil)
	} else {
		s.overrides.Store(&overrides)
	}
}

// Evaluate 执行一次完整的可用率评估和移板判断。
// 可导出，供测试和启动时首次调用。
func (s *Service) Evaluate(ctx context.Context) {
	snap := s.snapshot()
	if snap == nil {
		s.overrides.Store(nil)
		return
	}

	if !snap.boardsEnabled || !snap.autoMove.Enabled {
		s.overrides.Store(nil)
		return
	}

	overrides, stats := s.evaluate(ctx, snap)

	// 原子替换
	if len(overrides) == 0 {
		s.overrides.Store(nil)
	} else {
		s.overrides.Store(&overrides)
	}

	logger.Info("AutoMover", "评估完成",
		"checked", stats.checked,
		"demoted", stats.demoted,
		"promoted", stats.promoted,
		"expired", stats.expired,
		"skipped_min_probes", stats.skippedMinProbes)
}

// --- 内部实现 ---

type evalSnapshot struct {
	boardsEnabled     bool
	autoMove          config.BoardAutoMoveConfig
	degradedWeight    float64
	batchQueryMaxKeys int
	storageType       string
	monitors          []config.ServiceConfig
}

type evalStats struct {
	checked          int
	demoted          int
	promoted         int
	expired          int
	skippedMinProbes int
}

func (s *Service) snapshot() *evalSnapshot {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()

	cfg := s.cfg
	if cfg == nil {
		return nil
	}

	storageType := strings.ToLower(strings.TrimSpace(cfg.Storage.Type))
	if storageType == "" {
		storageType = "sqlite"
	}

	return &evalSnapshot{
		boardsEnabled:     cfg.Boards.Enabled,
		autoMove:          cfg.Boards.AutoMove,
		degradedWeight:    cfg.DegradedWeight,
		batchQueryMaxKeys: cfg.BatchQueryMaxKeys,
		storageType:       storageType,
		monitors:          cfg.Monitors,
	}
}

func (s *Service) loop(ctx context.Context) {
	// 首次立即执行
	s.Evaluate(ctx)

	for {
		interval := s.checkInterval()
		// ±10% jitter 避免所有实例同步
		jitter := time.Duration(float64(interval) * (rand.Float64()*0.2 - 0.1))
		timer := time.NewTimer(interval + jitter)

		select {
		case <-timer.C:
			s.Evaluate(ctx)
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.stopCh:
			timer.Stop()
			return
		}
	}
}

func (s *Service) checkInterval() time.Duration {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()

	if s.cfg == nil {
		return 30 * time.Minute
	}
	d := s.cfg.Boards.AutoMove.CheckIntervalDuration
	if d <= 0 {
		return 30 * time.Minute
	}
	return d
}

// currentOverrides 返回当前 override map 的快照（可能为 nil）
func (s *Service) currentOverrides() map[storage.MonitorKey]MonitorOverride {
	ptr := s.overrides.Load()
	if ptr == nil {
		return nil
	}
	return *ptr
}

func (s *Service) evaluate(ctx context.Context, snap *evalSnapshot) (map[storage.MonitorKey]MonitorOverride, evalStats) {
	var stats evalStats
	overrides := make(map[storage.MonitorKey]MonitorOverride)
	today := time.Now().UTC().Truncate(24 * time.Hour)

	// 收集 hot/secondary 的根监测项（排除 parent/disabled/hidden/cold）
	type candidate struct {
		key         storage.MonitorKey
		configBoard string
	}
	var candidates []candidate

	for _, m := range snap.monitors {
		if m.Disabled || m.Hidden {
			continue
		}
		// 仅处理根监测项（无 parent）
		if strings.TrimSpace(m.Parent) != "" {
			continue
		}
		board := strings.ToLower(strings.TrimSpace(m.Board))
		if board == "" {
			board = "hot"
		}
		if board == "cold" {
			continue
		}

		key := storage.MonitorKey{
			Provider: m.Provider,
			Service:  m.Service,
			Channel:  m.Channel,
			Model:    m.Model,
		}

		// 到期检查：到期日当天仍有效，次日起自动降级并移入备板
		if expiresAt := strings.TrimSpace(m.ExpiresAt); expiresAt != "" {
			if expiresDate, err := time.Parse("2006-01-02", expiresAt); err == nil && today.After(expiresDate) {
				ov := MonitorOverride{Board: "secondary"}
				// 仅当赞助等级高于 pulse 时降级为 pulse（避免低等级被"升级"）
				if m.SponsorLevel.Weight() > config.SponsorLevelPulse.Weight() {
					ov.SponsorLevel = config.SponsorLevelPulse
				}
				overrides[key] = ov
				stats.expired++
				logger.Info("AutoMover", "赞助到期，自动降级",
					"monitor", key.Provider+"/"+key.Service+"/"+key.Channel,
					"expires_at", expiresAt)
				continue // 跳过可用率评估
			}
		}

		candidates = append(candidates, candidate{
			key:         key,
			configBoard: board,
		})
	}

	if len(candidates) == 0 {
		if len(overrides) == 0 {
			return nil, stats
		}
		return overrides, stats
	}

	// 构建批量查询 keys
	keys := make([]storage.MonitorKey, len(candidates))
	for i, c := range candidates {
		keys[i] = c.key
	}

	// 分批查询历史记录（考虑 SQLite 参数上限）
	batchSize := snap.batchQueryMaxKeys
	if batchSize <= 0 {
		batchSize = 300
	}
	if snap.storageType == "sqlite" {
		const sqliteMaxParams = 999
		const keyParams = 4
		maxKeys := sqliteMaxParams / keyParams
		if batchSize > maxKeys {
			batchSize = maxKeys
		}
	}

	store := s.storage.WithContext(ctx)
	since := time.Now().UTC().AddDate(0, 0, -7)

	// 合并所有批次结果
	allHistory := make(map[storage.MonitorKey][]*storage.ProbeRecord)
	for start := 0; start < len(keys); start += batchSize {
		end := start + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		batch := keys[start:end]
		historyMap, err := store.GetHistoryBatch(batch, since)
		if err != nil {
			if ctx.Err() == nil {
				logger.Warn("AutoMover", "批量查询历史记录失败", "error", err)
			}
			return overrides, stats
		}
		for k, v := range historyMap {
			allHistory[k] = v
		}
	}

	// 读取当前 override 状态，用于计算"有效板块"
	currentOverrides := s.currentOverrides()

	// 双阈值评估：以"有效板块"（config + 当前 override）为基准
	for _, c := range candidates {
		stats.checked++
		records := allHistory[c.key]
		availability, total := CalculateAvailability(records, snap.degradedWeight)
		if total < snap.autoMove.MinProbes {
			stats.skippedMinProbes++
			continue
		}

		// 有效板块 = 当前 override 优先，否则取配置值
		effectiveBoard := c.configBoard
		if ov, ok := currentOverrides[c.key]; ok && ov.Board != "" {
			effectiveBoard = ov.Board
		}

		switch effectiveBoard {
		case "hot":
			if availability < snap.autoMove.ThresholdDown {
				overrides[c.key] = MonitorOverride{Board: "secondary"}
				stats.demoted++
				logger.Info("AutoMover", "自动移板: hot→secondary",
					"monitor", c.key.Provider+"/"+c.key.Service+"/"+c.key.Channel,
					"availability", availability,
					"threshold_down", snap.autoMove.ThresholdDown)
			}
		case "secondary":
			if availability >= snap.autoMove.ThresholdUp {
				// 仅当配置原始板也需要保持 override 时设置
				if c.configBoard != "hot" {
					overrides[c.key] = MonitorOverride{Board: "hot"}
				}
				// 若 configBoard 本就是 hot，不需要 override（清除即可）
				stats.promoted++
				logger.Info("AutoMover", "自动移板: secondary→hot",
					"monitor", c.key.Provider+"/"+c.key.Service+"/"+c.key.Channel,
					"availability", availability,
					"threshold_up", snap.autoMove.ThresholdUp)
			} else if c.configBoard == "hot" {
				// 配置是 hot 但之前被降级，可用率还没达到 threshold_up → 保持降级
				overrides[c.key] = MonitorOverride{Board: "secondary"}
			}
		}
	}

	return overrides, stats
}
