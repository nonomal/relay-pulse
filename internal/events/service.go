package events

import (
	"fmt"
	"sync"

	"monitor/internal/logger"
	"monitor/internal/storage"
)

// Service 事件服务
// 协调检测器和存储层，处理探测结果并生成事件
type Service struct {
	detector *Detector
	storage  storage.Storage
	enabled  bool
	locks    sync.Map // key(provider/service/channel) -> *sync.Mutex，防止同一监测项并发处理导致状态机错乱
}

// NewService 创建事件服务
func NewService(cfg DetectorConfig, store storage.Storage, enabled bool) (*Service, error) {
	if !enabled {
		return &Service{
			enabled: false,
		}, nil
	}

	detector, err := NewDetector(cfg)
	if err != nil {
		return nil, err
	}

	return &Service{
		detector: detector,
		storage:  store,
		enabled:  true,
	}, nil
}

// IsEnabled 返回事件服务是否启用
func (s *Service) IsEnabled() bool {
	return s.enabled
}

// lockFor 获取指定监测项的锁
func (s *Service) lockFor(provider, service, channel string) *sync.Mutex {
	key := provider + "\n" + service + "\n" + channel
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

	// 同一监测项串行化：否则 Scheduler 允许同任务重叠时，会出现：
	// - 两个 goroutine 读到同一个 prev，重复触发事件
	// - last_record_id 倒退/覆盖，导致 streak 计算错误
	mu := s.lockFor(record.Provider, record.Service, record.Channel)
	mu.Lock()
	defer mu.Unlock()

	// 获取当前状态
	prev, err := s.storage.GetServiceState(record.Provider, record.Service, record.Channel)
	if err != nil {
		logger.Error("events", "获取服务状态失败",
			"provider", record.Provider, "service", record.Service, "channel", record.Channel,
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
