package events

import (
	"fmt"
	"time"

	"monitor/internal/storage"
)

// Detector 状态机检测器
// 基于连续阈值检测服务可用性变更，生成 DOWN/UP 事件
type Detector struct {
	cfg DetectorConfig
}

// NewDetector 创建状态机检测器
func NewDetector(cfg DetectorConfig) (*Detector, error) {
	if cfg.DownThreshold < 1 {
		return nil, fmt.Errorf("down_threshold 必须 >= 1，当前值: %d", cfg.DownThreshold)
	}
	if cfg.UpThreshold < 1 {
		return nil, fmt.Errorf("up_threshold 必须 >= 1，当前值: %d", cfg.UpThreshold)
	}
	return &Detector{cfg: cfg}, nil
}

// Detect 检测状态变更
//
// 输入：
//   - prev: 上一次的服务状态（nil 表示首次探测）
//   - record: 最新的探测记录
//
// 输出：
//   - newState: 更新后的服务状态
//   - event: 产生的事件（nil 表示无事件）
//   - error: 错误信息
//
// 状态机逻辑：
//   - 将 probe status (0/1/2) 映射为二值可用性：1或2 → 可用(1)，0 → 不可用(0)
//   - 首次探测只初始化状态，不产生事件
//   - 稳定态为"可用"时，连续 N 次不可用触发 DOWN 事件
//   - 稳定态为"不可用"时，连续 M 次可用触发 UP 事件
func (d *Detector) Detect(prev *ServiceState, record *storage.ProbeRecord) (*ServiceState, *StatusEvent, error) {
	if record == nil {
		return nil, nil, fmt.Errorf("record 不能为空")
	}

	// 将 probe status 映射为二值可用性
	// 绿色(1)和黄色(2)都视为可用，红色(0)视为不可用
	currentAvailable := 0
	if record.Status == 1 || record.Status == 2 {
		currentAvailable = 1
	}

	now := time.Now().Unix()

	// 首次探测：初始化状态，不产生事件
	if prev == nil {
		newState := &ServiceState{
			Provider:        record.Provider,
			Service:         record.Service,
			Channel:         record.Channel,
			StableAvailable: currentAvailable,
			StreakCount:     1,
			StreakStatus:    currentAvailable,
			LastRecordID:    record.ID,
			LastTimestamp:   record.Timestamp,
		}
		return newState, nil, nil
	}

	// 复制状态
	newState := &ServiceState{
		Provider:        record.Provider,
		Service:         record.Service,
		Channel:         record.Channel,
		StableAvailable: prev.StableAvailable,
		StreakCount:     prev.StreakCount,
		StreakStatus:    prev.StreakStatus,
		LastRecordID:    record.ID,
		LastTimestamp:   record.Timestamp,
	}

	// 更新 streak 计数
	if currentAvailable == prev.StreakStatus {
		// 与上次方向一致，累加
		newState.StreakCount = prev.StreakCount + 1
	} else {
		// 方向改变，重置
		newState.StreakCount = 1
		newState.StreakStatus = currentAvailable
	}

	var event *StatusEvent

	// 检测状态变更
	if prev.StableAvailable == 1 && currentAvailable == 0 {
		// 当前稳定态是"可用"，检测是否触发 DOWN
		if newState.StreakCount >= d.cfg.DownThreshold {
			event = &StatusEvent{
				Provider:        record.Provider,
				Service:         record.Service,
				Channel:         record.Channel,
				EventType:       EventTypeDown,
				FromStatus:      1, // 从可用
				ToStatus:        record.Status,
				TriggerRecordID: record.ID,
				ObservedAt:      record.Timestamp,
				CreatedAt:       now,
				Meta: map[string]any{
					"http_code":  record.HttpCode,
					"latency_ms": record.Latency,
					"sub_status": string(record.SubStatus),
				},
			}
			// 更新稳定态
			newState.StableAvailable = 0
			newState.StreakCount = 0
			newState.StreakStatus = 0
		}
	} else if prev.StableAvailable == 0 && currentAvailable == 1 {
		// 当前稳定态是"不可用"，检测是否触发 UP
		if newState.StreakCount >= d.cfg.UpThreshold {
			event = &StatusEvent{
				Provider:        record.Provider,
				Service:         record.Service,
				Channel:         record.Channel,
				EventType:       EventTypeUp,
				FromStatus:      0, // 从不可用
				ToStatus:        record.Status,
				TriggerRecordID: record.ID,
				ObservedAt:      record.Timestamp,
				CreatedAt:       now,
				Meta: map[string]any{
					"http_code":  record.HttpCode,
					"latency_ms": record.Latency,
				},
			}
			// 更新稳定态
			newState.StableAvailable = 1
			newState.StreakCount = 0
			newState.StreakStatus = 1
		}
	}

	return newState, event, nil
}
