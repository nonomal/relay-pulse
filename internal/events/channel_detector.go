package events

import (
	"time"

	"monitor/internal/storage"
)

// ChannelState 通道级状态（复用 storage 定义）
type ChannelState = storage.ChannelState

// ChannelDetectorConfig 通道级检测器配置
type ChannelDetectorConfig struct {
	// DownThreshold N 个模型 DOWN 触发通道 DOWN（默认 1）
	DownThreshold int
}

// DefaultChannelConfig 返回默认通道检测器配置
func DefaultChannelConfig() ChannelDetectorConfig {
	return ChannelDetectorConfig{
		DownThreshold: 1,
	}
}

// ChannelDetector 通道级状态机检测器
// 基于通道内模型的 DOWN 数量检测通道级状态变更
type ChannelDetector struct {
	cfg ChannelDetectorConfig
}

// NewChannelDetector 创建通道级检测器
func NewChannelDetector(cfg ChannelDetectorConfig) *ChannelDetector {
	if cfg.DownThreshold < 1 {
		cfg.DownThreshold = 1
	}
	return &ChannelDetector{cfg: cfg}
}

// DetectChannelResult 通道检测结果
type DetectChannelResult struct {
	// NewChannelState 更新后的通道状态
	NewChannelState *ChannelState

	// Event 产生的通道级事件（nil 表示无事件）
	Event *StatusEvent
}

// DetectChannel 检测通道级状态变更（增量模式）
//
// 输入：
//   - prevChannel: 上一次的通道状态（nil 表示首次）
//   - prevModelStable: 模型的上一次稳定态（-1=未初始化, 0=不可用, 1=可用）
//   - newModelStable: 模型的新稳定态（0 或 1）
//   - totalModels: 该通道的活跃模型总数
//   - record: 触发的探测记录
//
// 输出：
//   - result: 检测结果，包含新通道状态和可能的事件
//
// 状态机逻辑：
//   - 通道 DOWN：down_count >= channel_down_threshold
//   - 通道 UP：down_count == 0 && known_count == total_models（所有模型都可用）
func (d *ChannelDetector) DetectChannel(
	prevChannel *ChannelState,
	prevModelStable int,
	newModelStable int,
	totalModels int,
	record *storage.ProbeRecord,
) *DetectChannelResult {
	now := time.Now().Unix()

	// 初始化通道状态
	var newChannel *ChannelState
	if prevChannel == nil {
		newChannel = &ChannelState{
			Provider:        record.Provider,
			Service:         record.Service,
			Channel:         record.Channel,
			StableAvailable: -1, // 未初始化
			DownCount:       0,
			KnownCount:      0,
			LastRecordID:    record.ID,
			LastTimestamp:   record.Timestamp,
		}
	} else {
		// 复制状态
		newChannel = &ChannelState{
			Provider:        prevChannel.Provider,
			Service:         prevChannel.Service,
			Channel:         prevChannel.Channel,
			StableAvailable: prevChannel.StableAvailable,
			DownCount:       prevChannel.DownCount,
			KnownCount:      prevChannel.KnownCount,
			LastRecordID:    record.ID,
			LastTimestamp:   record.Timestamp,
		}
	}

	// 增量更新计数
	d.updateCounts(newChannel, prevModelStable, newModelStable)

	// 检测通道级状态变更
	prevStable := -1
	if prevChannel != nil {
		prevStable = prevChannel.StableAvailable
	}

	// 增量模式下，首次初始化时需要额外判断是否触发 DOWN 事件
	event := d.detectStateChangeIncremental(prevStable, newChannel, totalModels, record, now, newModelStable)

	return &DetectChannelResult{
		NewChannelState: newChannel,
		Event:           event,
	}
}

// DetectChannelWithCounts 使用预计算的 counts 检测通道级状态变更（重算模式）
// 用于 recompute 模式，counts 由 service.go 基于活跃模型集合计算
//
// 输入：
//   - prevChannel: 上一次的通道状态（nil 表示首次）
//   - downCount: 当前 DOWN 的模型数量（由调用方计算）
//   - knownCount: 当前已知状态的模型数量（由调用方计算）
//   - totalModels: 该通道的活跃模型总数
//   - record: 触发的探测记录
//
// 输出：
//   - result: 检测结果，包含新通道状态和可能的事件
func (d *ChannelDetector) DetectChannelWithCounts(
	prevChannel *ChannelState,
	downCount int,
	knownCount int,
	totalModels int,
	record *storage.ProbeRecord,
) *DetectChannelResult {
	now := time.Now().Unix()

	// 初始化通道状态
	var newChannel *ChannelState
	if prevChannel == nil {
		newChannel = &ChannelState{
			Provider:        record.Provider,
			Service:         record.Service,
			Channel:         record.Channel,
			StableAvailable: -1, // 未初始化
			DownCount:       downCount,
			KnownCount:      knownCount,
			LastRecordID:    record.ID,
			LastTimestamp:   record.Timestamp,
		}
	} else {
		newChannel = &ChannelState{
			Provider:        prevChannel.Provider,
			Service:         prevChannel.Service,
			Channel:         prevChannel.Channel,
			StableAvailable: prevChannel.StableAvailable,
			DownCount:       downCount,
			KnownCount:      knownCount,
			LastRecordID:    record.ID,
			LastTimestamp:   record.Timestamp,
		}
	}

	// 检测通道级状态变更
	prevStable := -1
	if prevChannel != nil {
		prevStable = prevChannel.StableAvailable
	}

	event := d.detectStateChange(prevStable, newChannel, totalModels, record, now)

	return &DetectChannelResult{
		NewChannelState: newChannel,
		Event:           event,
	}
}

// detectStateChangeIncremental 增量模式下检测状态变更（需要额外处理首次初始化）
func (d *ChannelDetector) detectStateChangeIncremental(
	prevStable int,
	newChannel *ChannelState,
	totalModels int,
	record *storage.ProbeRecord,
	now int64,
	newModelStable int,
) *StatusEvent {
	var event *StatusEvent

	// 检测 DOWN 事件
	// 条件：之前可用或未初始化，现在 down_count >= threshold
	if (prevStable == 1 || prevStable == -1) && newChannel.DownCount >= d.cfg.DownThreshold {
		// 只有当之前是可用状态时才触发 DOWN 事件
		// 首次初始化（prevStable == -1）且直接 DOWN 的情况也触发事件
		if prevStable == 1 || (prevStable == -1 && newChannel.KnownCount >= 1) {
			// 首次初始化时，如果第一个模型就是 DOWN，需要触发通道 DOWN
			if prevStable == 1 || newModelStable == 0 {
				event = &StatusEvent{
					Provider:        record.Provider,
					Service:         record.Service,
					Channel:         record.Channel,
					Model:           "", // 空字符串表示通道级事件
					EventType:       EventTypeDown,
					FromStatus:      1,
					ToStatus:        0,
					TriggerRecordID: record.ID,
					ObservedAt:      record.Timestamp,
					CreatedAt:       now,
					Meta: map[string]any{
						"scope":                  "channel",
						"trigger_model":          record.Model,
						"down_count":             newChannel.DownCount,
						"known_count":            newChannel.KnownCount,
						"total_models":           totalModels,
						"channel_down_threshold": d.cfg.DownThreshold,
						"http_code":              record.HttpCode,
						"latency_ms":             record.Latency,
						"sub_status":             string(record.SubStatus),
					},
				}
				newChannel.StableAvailable = 0
			}
		}
	}

	// 检测 UP 事件
	// 条件：之前不可用，现在 down_count == 0 且所有模型都已知
	if prevStable == 0 && newChannel.DownCount == 0 && newChannel.KnownCount == totalModels && totalModels > 0 {
		event = &StatusEvent{
			Provider:        record.Provider,
			Service:         record.Service,
			Channel:         record.Channel,
			Model:           "", // 空字符串表示通道级事件
			EventType:       EventTypeUp,
			FromStatus:      0,
			ToStatus:        1,
			TriggerRecordID: record.ID,
			ObservedAt:      record.Timestamp,
			CreatedAt:       now,
			Meta: map[string]any{
				"scope":         "channel",
				"trigger_model": record.Model,
				"down_count":    newChannel.DownCount,
				"known_count":   newChannel.KnownCount,
				"total_models":  totalModels,
				"http_code":     record.HttpCode,
				"latency_ms":    record.Latency,
			},
		}
		newChannel.StableAvailable = 1
	}

	// 首次初始化且所有模型都可用时，设置为可用状态（不触发事件）
	if prevStable == -1 && newChannel.DownCount == 0 && newChannel.KnownCount == totalModels && totalModels > 0 {
		newChannel.StableAvailable = 1
	}

	return event
}

// detectStateChange 重算模式下检测状态变更
func (d *ChannelDetector) detectStateChange(
	prevStable int,
	newChannel *ChannelState,
	totalModels int,
	record *storage.ProbeRecord,
	now int64,
) *StatusEvent {
	var event *StatusEvent

	// 检测 DOWN 事件
	// 条件：之前可用或未初始化，现在 down_count >= threshold
	if (prevStable == 1 || prevStable == -1) && newChannel.DownCount >= d.cfg.DownThreshold {
		// 只有当之前是可用状态，或首次初始化且有已知模型时才触发 DOWN 事件
		if prevStable == 1 || (prevStable == -1 && newChannel.KnownCount >= 1) {
			event = &StatusEvent{
				Provider:        record.Provider,
				Service:         record.Service,
				Channel:         record.Channel,
				Model:           "", // 空字符串表示通道级事件
				EventType:       EventTypeDown,
				FromStatus:      1,
				ToStatus:        0,
				TriggerRecordID: record.ID,
				ObservedAt:      record.Timestamp,
				CreatedAt:       now,
				Meta: map[string]any{
					"scope":                  "channel",
					"trigger_model":          record.Model,
					"down_count":             newChannel.DownCount,
					"known_count":            newChannel.KnownCount,
					"total_models":           totalModels,
					"channel_down_threshold": d.cfg.DownThreshold,
					"http_code":              record.HttpCode,
					"latency_ms":             record.Latency,
					"sub_status":             string(record.SubStatus),
				},
			}
			newChannel.StableAvailable = 0
		}
	}

	// 检测 UP 事件
	// 条件：之前不可用，现在 down_count == 0 且所有模型都已知
	if prevStable == 0 && newChannel.DownCount == 0 && newChannel.KnownCount == totalModels && totalModels > 0 {
		event = &StatusEvent{
			Provider:        record.Provider,
			Service:         record.Service,
			Channel:         record.Channel,
			Model:           "", // 空字符串表示通道级事件
			EventType:       EventTypeUp,
			FromStatus:      0,
			ToStatus:        1,
			TriggerRecordID: record.ID,
			ObservedAt:      record.Timestamp,
			CreatedAt:       now,
			Meta: map[string]any{
				"scope":         "channel",
				"trigger_model": record.Model,
				"down_count":    newChannel.DownCount,
				"known_count":   newChannel.KnownCount,
				"total_models":  totalModels,
				"http_code":     record.HttpCode,
				"latency_ms":    record.Latency,
			},
		}
		newChannel.StableAvailable = 1
	}

	// 首次初始化且所有模型都可用时，设置为可用状态（不触发事件）
	if prevStable == -1 && newChannel.DownCount == 0 && newChannel.KnownCount == totalModels && totalModels > 0 {
		newChannel.StableAvailable = 1
	}

	return event
}

// updateCounts 根据模型状态变化增量更新计数
//
// 状态转换规则：
//   - prevModelStable == -1（首次）：known_count++，若 newModelStable == 0 则 down_count++
//   - 1 -> 0：down_count++
//   - 0 -> 1：down_count--
//   - 其他（无变化）：不变
func (d *ChannelDetector) updateCounts(channel *ChannelState, prevModelStable, newModelStable int) {
	if prevModelStable == -1 {
		// 首次初始化该模型
		channel.KnownCount++
		if newModelStable == 0 {
			channel.DownCount++
		}
	} else if prevModelStable == 1 && newModelStable == 0 {
		// 可用 -> 不可用
		channel.DownCount++
	} else if prevModelStable == 0 && newModelStable == 1 {
		// 不可用 -> 可用
		channel.DownCount--
		if channel.DownCount < 0 {
			channel.DownCount = 0 // 防止负数
		}
	}
	// 其他情况（状态无变化）不更新计数
}
