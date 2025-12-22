// Package events 提供状态订阅通知功能
// 用于检测服务状态变更并生成事件
package events

import "monitor/internal/storage"

// EventType 事件类型（复用 storage 定义，保持一致性）
type EventType = storage.EventType

const (
	EventTypeDown = storage.EventTypeDown // 可用 → 不可用
	EventTypeUp   = storage.EventTypeUp   // 不可用 → 可用
)

// ServiceState 服务状态（复用 storage 定义）
type ServiceState = storage.ServiceState

// StatusEvent 状态事件（复用 storage 定义）
type StatusEvent = storage.StatusEvent

// EventFilters 事件过滤器（复用 storage 定义）
type EventFilters = storage.EventFilters

// DetectorConfig 检测器配置
type DetectorConfig struct {
	// DownThreshold 连续 N 次不可用触发 DOWN 事件（默认 2）
	DownThreshold int

	// UpThreshold 连续 N 次可用触发 UP 事件（默认 1）
	UpThreshold int
}

// DefaultConfig 返回默认配置
func DefaultConfig() DetectorConfig {
	return DetectorConfig{
		DownThreshold: 2,
		UpThreshold:   1,
	}
}
