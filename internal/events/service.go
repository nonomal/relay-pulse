package events

import (
	"fmt"
	"sync"

	"monitor/internal/config"
	"monitor/internal/logger"
	"monitor/internal/storage"
)

// Service 事件服务
// 协调检测器和存储层，处理探测结果并生成事件
type Service struct {
	detector         *Detector
	channelDetector  *ChannelDetector
	storage          storage.Storage
	enabled          bool
	mode             string // "model" 或 "channel"
	channelCountMode string // "incremental" 或 "recompute"

	// 锁管理
	locks sync.Map // key -> *sync.Mutex，防止同一监测项并发处理导致状态机错乱

	// 活跃模型索引（用于 channel 模式）
	activeModels   map[string][]string // "provider/service/channel" -> []model
	activeModelsMu sync.RWMutex
}

// ServiceConfig 事件服务配置
type ServiceConfig struct {
	DetectorConfig        DetectorConfig
	ChannelDetectorConfig ChannelDetectorConfig
	Mode                  string // "model" 或 "channel"
	ChannelCountMode      string // "incremental" 或 "recompute"
	Enabled               bool
}

// NewService 创建事件服务
func NewService(cfg ServiceConfig, store storage.Storage) (*Service, error) {
	if !cfg.Enabled {
		return &Service{
			enabled: false,
		}, nil
	}

	detector, err := NewDetector(cfg.DetectorConfig)
	if err != nil {
		return nil, err
	}

	mode := cfg.Mode
	if mode == "" {
		mode = "model"
	}

	channelCountMode := cfg.ChannelCountMode
	if channelCountMode == "" {
		channelCountMode = "recompute" // 默认使用重算模式
	}

	svc := &Service{
		detector:         detector,
		channelDetector:  NewChannelDetector(cfg.ChannelDetectorConfig),
		storage:          store,
		enabled:          true,
		mode:             mode,
		channelCountMode: channelCountMode,
		activeModels:     make(map[string][]string),
	}

	return svc, nil
}

// IsEnabled 返回事件服务是否启用
func (s *Service) IsEnabled() bool {
	return s.enabled
}

// GetMode 返回当前事件模式
func (s *Service) GetMode() string {
	return s.mode
}

// UpdateActiveModels 从配置更新活跃模型索引
func (s *Service) UpdateActiveModels(monitors []config.ServiceConfig) {
	index := make(map[string][]string)
	seen := make(map[string]map[string]struct{})

	for _, m := range monitors {
		if m.Disabled {
			continue
		}
		model := m.Model
		if model == "" {
			continue
		}
		psc := m.Provider + "/" + m.Service + "/" + m.Channel
		if _, ok := seen[psc]; !ok {
			seen[psc] = make(map[string]struct{})
		}
		if _, ok := seen[psc][model]; ok {
			continue // 去重
		}
		seen[psc][model] = struct{}{}
		index[psc] = append(index[psc], model)
	}

	s.activeModelsMu.Lock()
	s.activeModels = index
	s.activeModelsMu.Unlock()

	logger.Debug("events", "活跃模型索引已更新", "channels", len(index))
}

// lockForModel 获取指定模型的锁（model 模式使用）
func (s *Service) lockForModel(provider, service, channel, model string) *sync.Mutex {
	key := "model:" + provider + "\n" + service + "\n" + channel + "\n" + model
	v, _ := s.locks.LoadOrStore(key, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// lockForChannel 获取指定通道的锁（channel 模式使用）
func (s *Service) lockForChannel(provider, service, channel string) *sync.Mutex {
	key := "channel:" + provider + "\n" + service + "\n" + channel
	v, _ := s.locks.LoadOrStore(key, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// ProcessRecord 处理探测记录
// 检测状态变更并保存事件
//
// 返回产生的事件（如有），用于日志记录等
func (s *Service) ProcessRecord(record *storage.ProbeRecord) (*StatusEvent, error) {
	if !s.enabled {
		return nil, nil
	}

	if record == nil {
		return nil, fmt.Errorf("record 不能为空")
	}

	if s.mode == "channel" {
		return s.processRecordChannelMode(record)
	}
	return s.processRecordModelMode(record)
}

// processRecordModelMode 模型级事件处理（原有逻辑）
func (s *Service) processRecordModelMode(record *storage.ProbeRecord) (*StatusEvent, error) {
	// 同一监测项串行化：否则 Scheduler 允许同任务重叠时，会出现：
	// - 两个 goroutine 读到同一个 prev，重复触发事件
	// - last_record_id 倒退/覆盖，导致 streak 计算错误
	mu := s.lockForModel(record.Provider, record.Service, record.Channel, record.Model)
	mu.Lock()
	defer mu.Unlock()

	// 获取当前状态
	prev, err := s.storage.GetServiceState(record.Provider, record.Service, record.Channel, record.Model)
	if err != nil {
		logger.Error("events", "获取服务状态失败",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel, "model", record.Model,
			"error", err)
		return nil, err
	}

	// 防御：忽略乱序/重复记录（Scheduler 允许同监测项重叠探测时更常见）
	if prev != nil && prev.LastRecordID > 0 && record.ID <= prev.LastRecordID {
		return nil, nil
	}

	// 检测状态变更
	newState, event, err := s.detector.Detect(prev, record)
	if err != nil {
		logger.Error("events", "检测状态变更失败",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel,
			"error", err)
		return nil, err
	}

	// 保存新状态
	if err := s.storage.UpsertServiceState(newState); err != nil {
		logger.Error("events", "保存服务状态失败",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel,
			"error", err)
		return nil, err
	}

	// 保存事件（如有）
	if event != nil {
		if err := s.storage.SaveStatusEvent(event); err != nil {
			logger.Error("events", "保存状态事件失败",
				"provider", record.Provider, "service", record.Service, "channel", record.Channel,
				"event_type", event.EventType,
				"error", err)
			return nil, err
		}

		logger.Info("events", "状态变更事件",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel,
			"event_type", event.EventType,
			"from_status", event.FromStatus, "to_status", event.ToStatus)
	}

	return event, nil
}

// processRecordChannelMode 通道级事件处理
func (s *Service) processRecordChannelMode(record *storage.ProbeRecord) (*StatusEvent, error) {
	// 按通道加锁，确保同一通道内的所有模型串行处理
	mu := s.lockForChannel(record.Provider, record.Service, record.Channel)
	mu.Lock()
	defer mu.Unlock()

	// 1. 获取模型的当前状态
	prevModel, err := s.storage.GetServiceState(record.Provider, record.Service, record.Channel, record.Model)
	if err != nil {
		logger.Error("events", "获取服务状态失败",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel, "model", record.Model,
			"error", err)
		return nil, err
	}

	// 防御：忽略乱序/重复记录
	if prevModel != nil && prevModel.LastRecordID > 0 && record.ID <= prevModel.LastRecordID {
		return nil, nil
	}

	// 2. 使用模型检测器更新模型状态（但不落库模型级事件）
	newModelState, _, err := s.detector.Detect(prevModel, record)
	if err != nil {
		logger.Error("events", "检测模型状态变更失败",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel, "model", record.Model,
			"error", err)
		return nil, err
	}

	// 3. 保存模型状态
	if err := s.storage.UpsertServiceState(newModelState); err != nil {
		logger.Error("events", "保存模型状态失败",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel, "model", record.Model,
			"error", err)
		return nil, err
	}

	// 4. 获取通道状态
	prevChannel, err := s.storage.GetChannelState(record.Provider, record.Service, record.Channel)
	if err != nil {
		logger.Error("events", "获取通道状态失败",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel,
			"error", err)
		return nil, err
	}

	// 5. 检测通道级状态变更（根据 channelCountMode 选择逻辑）
	var result *DetectChannelResult
	// 一次性获取活跃模型快照，确保 totalModels 与后续逻辑一致
	activeModelList := s.getActiveModels(record.Provider, record.Service, record.Channel)
	totalModels := len(activeModelList)

	// 边界检查：活跃模型列表为空时跳过通道级事件检测
	if totalModels == 0 {
		logger.Debug("events", "活跃模型列表为空，跳过通道级事件检测",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel, "model", record.Model)
		return nil, nil
	}

	// 边界检查：非活跃模型的探测记录不参与通道级状态更新（防止热更新后旧任务污染计数）
	isActiveModel := false
	for _, m := range activeModelList {
		if m == record.Model {
			isActiveModel = true
			break
		}
	}
	if !isActiveModel {
		logger.Debug("events", "收到非活跃模型探测记录，跳过通道级事件检测",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel, "model", record.Model)
		return nil, nil
	}

	// 确定实际使用的计数模式
	effectiveCountMode := s.channelCountMode
	useRecompute := s.channelCountMode == "recompute"

	prevModelStable := -1
	if prevModel != nil {
		prevModelStable = prevModel.StableAvailable
	}

	// incremental 模式自愈：检测到异常状态时自动回退到 recompute 做一次校准
	if !useRecompute && prevChannel != nil {
		needsRecalibration := false
		// 计数超出合理范围
		if prevChannel.KnownCount > totalModels ||
			prevChannel.DownCount > totalModels ||
			prevChannel.DownCount > prevChannel.KnownCount ||
			prevChannel.KnownCount < 0 ||
			prevChannel.DownCount < 0 {
			needsRecalibration = true
			logger.Warn("events", "incremental 模式检测到计数异常，自动回退 recompute 校准",
				"provider", record.Provider, "service", record.Service, "channel", record.Channel,
				"known_count", prevChannel.KnownCount, "down_count", prevChannel.DownCount, "total_models", totalModels)
		}
		// channel_states 已初始化但 known_count=0 而模型已有状态（迁移场景）
		if prevChannel.KnownCount == 0 && prevModelStable != -1 {
			needsRecalibration = true
			logger.Warn("events", "incremental 模式检测到迁移场景，自动回退 recompute 校准",
				"provider", record.Provider, "service", record.Service, "channel", record.Channel)
		}
		if needsRecalibration {
			useRecompute = true
			effectiveCountMode = "recompute(auto)"
		}
	}

	// channel_states 为空时，incremental 也需要回退到 recompute 初始化
	if !useRecompute && prevChannel == nil {
		useRecompute = true
		effectiveCountMode = "recompute(init)"
		logger.Debug("events", "channel_states 为空，使用 recompute 初始化",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel)
	}

	var activeModelStates []*ServiceState // 用于复用查询结果

	if useRecompute {
		// 重算模式：基于活跃模型集合重新计算 counts
		result, activeModelStates, err = s.detectChannelRecompute(prevChannel, activeModelList, record)
		if err != nil {
			return nil, err
		}
	} else {
		// 增量模式：使用原有的增量计数逻辑
		result = s.channelDetector.DetectChannel(
			prevChannel,
			prevModelStable,
			newModelState.StableAvailable,
			totalModels,
			record,
		)
	}

	// 6. 保存通道状态
	if err := s.storage.UpsertChannelState(result.NewChannelState); err != nil {
		logger.Error("events", "保存通道状态失败",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel,
			"error", err)
		return nil, err
	}

	// 7. 如果有通道级事件，构建完整的 Meta 并保存
	if result.Event != nil {
		// 复用 recompute 模式的查询结果，或重新查询
		if activeModelStates != nil {
			s.enrichChannelEventMeta(result.Event, activeModelStates)
		} else {
			modelStates, err := s.getActiveModelStatesWithList(record.Provider, record.Service, record.Channel, activeModelList)
			if err != nil {
				logger.Warn("events", "获取通道模型状态失败，事件 Meta 可能不完整",
					"provider", record.Provider, "service", record.Service, "channel", record.Channel,
					"error", err)
				// Meta 兜底：确保 models 字段存在，使用 trigger_model 作为最小信息
				if result.Event.Meta == nil {
					result.Event.Meta = make(map[string]any)
				}
				result.Event.Meta["models"] = []string{record.Model}
				result.Event.Meta["down_models"] = []string{}
				result.Event.Meta["up_models"] = []string{}
			} else {
				s.enrichChannelEventMeta(result.Event, modelStates)
			}
		}

		// 保存事件
		if err := s.storage.SaveStatusEvent(result.Event); err != nil {
			logger.Error("events", "保存通道状态事件失败",
				"provider", record.Provider, "service", record.Service, "channel", record.Channel,
				"event_type", result.Event.EventType,
				"error", err)
			return nil, err
		}

		logger.Info("events", "通道状态变更事件",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel,
			"event_type", result.Event.EventType,
			"trigger_model", record.Model,
			"down_count", result.NewChannelState.DownCount,
			"known_count", result.NewChannelState.KnownCount,
			"total_models", totalModels,
			"count_mode", effectiveCountMode)
	}

	return result.Event, nil
}

// detectChannelRecompute 重算模式下检测通道级状态变更
// 基于活跃模型集合重新计算 down_count 和 known_count
// 返回值包含活跃模型状态列表，供事件 Meta enrich 复用
func (s *Service) detectChannelRecompute(
	prevChannel *ChannelState,
	activeModelList []string,
	record *storage.ProbeRecord,
) (*DetectChannelResult, []*ServiceState, error) {
	totalModels := len(activeModelList)

	// 获取通道下所有模型的状态（一次查询）
	modelStates, err := s.storage.GetModelStatesForChannel(record.Provider, record.Service, record.Channel)
	if err != nil {
		logger.Error("events", "获取通道模型状态失败",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel,
			"error", err)
		return nil, nil, err
	}

	// 构建模型状态映射
	stateMap := make(map[string]*ServiceState, len(modelStates))
	for _, ms := range modelStates {
		stateMap[ms.Model] = ms
	}

	// 基于活跃模型集合计算 counts
	downCount := 0
	knownCount := 0
	for _, model := range activeModelList {
		if ms, ok := stateMap[model]; ok {
			// 模型有状态记录
			if ms.StableAvailable != -1 {
				knownCount++
				if ms.StableAvailable == 0 {
					downCount++
				}
			}
		}
		// 模型无状态记录时不计入 known_count
	}

	// 使用预计算的 counts 检测状态变更
	result := s.channelDetector.DetectChannelWithCounts(
		prevChannel,
		downCount,
		knownCount,
		totalModels,
		record,
	)

	// 复用查询结果：按活跃模型列表构建状态切片，供事件 Meta enrich 使用
	activeStates := make([]*ServiceState, 0, len(activeModelList))
	for _, model := range activeModelList {
		if ms, ok := stateMap[model]; ok {
			activeStates = append(activeStates, ms)
		}
	}

	return result, activeStates, nil
}

// getActiveModels 获取通道的活跃模型列表（返回副本，避免外部修改）
func (s *Service) getActiveModels(provider, service, channel string) []string {
	psc := provider + "/" + service + "/" + channel
	s.activeModelsMu.RLock()
	models := s.activeModels[psc]
	s.activeModelsMu.RUnlock()
	// 返回副本，避免外部修改影响内部状态
	return append([]string(nil), models...)
}

// getActiveModelStatesWithList 获取通道下活跃模型的状态（使用传入的活跃模型列表）
func (s *Service) getActiveModelStatesWithList(provider, service, channel string, activeModelList []string) ([]*ServiceState, error) {
	activeSet := make(map[string]bool)
	for _, m := range activeModelList {
		activeSet[m] = true
	}

	// 获取所有模型状态
	allStates, err := s.storage.GetModelStatesForChannel(provider, service, channel)
	if err != nil {
		return nil, err
	}

	// 过滤只保留活跃模型
	result := make([]*ServiceState, 0, len(activeModelList))
	for _, ms := range allStates {
		if activeSet[ms.Model] {
			result = append(result, ms)
		}
	}

	return result, nil
}

// enrichChannelEventMeta 为通道级事件补充模型状态详情
func (s *Service) enrichChannelEventMeta(event *StatusEvent, modelStates []*ServiceState) {
	if event.Meta == nil {
		event.Meta = make(map[string]any)
	}

	var downModels []string
	var upModels []string
	modelStatesMap := make(map[string]any)

	for _, ms := range modelStates {
		stateInfo := map[string]any{
			"stable":      ms.StableAvailable,
			"last_status": ms.StreakStatus,
		}
		modelStatesMap[ms.Model] = stateInfo

		if ms.StableAvailable == 0 {
			downModels = append(downModels, ms.Model)
		} else if ms.StableAvailable == 1 {
			upModels = append(upModels, ms.Model)
		}
	}

	event.Meta["down_models"] = downModels
	event.Meta["up_models"] = upModels
	event.Meta["model_states"] = modelStatesMap

	// 兼容 notifier 现有逻辑：填充 models 字段（DOWN 事件时为 down_models）
	if event.EventType == EventTypeDown {
		event.Meta["models"] = downModels
	} else {
		event.Meta["models"] = upModels
	}
}
