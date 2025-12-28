package storage

import (
	"context"
	"time"
)

// SubStatus 细分状态码（字符串形式，便于扩展和前后端统一）
type SubStatus string

const (
	SubStatusNone            SubStatus = ""                 // 默认值（绿色或灰色无需细分）
	SubStatusSlowLatency     SubStatus = "slow_latency"     // 响应慢
	SubStatusRateLimit       SubStatus = "rate_limit"       // 限流（429）
	SubStatusServerError     SubStatus = "server_error"     // 服务器错误（5xx）
	SubStatusClientError     SubStatus = "client_error"     // 客户端错误（4xx）
	SubStatusAuthError       SubStatus = "auth_error"       // 认证/权限失败（401/403）
	SubStatusInvalidRequest  SubStatus = "invalid_request"  // 请求参数错误（400）
	SubStatusNetworkError    SubStatus = "network_error"    // 网络错误（连接失败）
	SubStatusContentMismatch SubStatus = "content_mismatch" // 内容校验失败
)

// ProbeRecord 探测记录
type ProbeRecord struct {
	ID        int64
	Provider  string
	Service   string
	Channel   string    // 业务通道标识
	Status    int       // 1=绿, 0=红, 2=黄
	SubStatus SubStatus // 细分状态（黄色/红色原因）
	HttpCode  int       // HTTP 状态码（0 表示非 HTTP 错误，如网络错误）
	Latency   int       // ms
	Timestamp int64     // Unix时间戳
}

// TimePoint 时间轴数据点（用于前端展示）
type TimePoint struct {
	Time         string       `json:"time"`          // 格式化时间标签（如 "15:04" 或 "2006-01-02"）
	Timestamp    int64        `json:"timestamp"`     // Unix 时间戳（秒），用于前端精确时间计算
	Status       int          `json:"status"`        // 状态码：1=绿，0=红，2=黄，-1=缺失（bucket内最后一条记录）
	Latency      int          `json:"latency"`       // 平均延迟（毫秒）
	Availability float64      `json:"availability"`  // 可用率百分比（0-100），缺失时为 -1
	StatusCounts StatusCounts `json:"status_counts"` // 各状态计数
}

// StatusCounts 记录一个时间块内各状态出现次数
type StatusCounts struct {
	Available   int `json:"available"`   // 绿色（可用）次数
	Degraded    int `json:"degraded"`    // 黄色（波动/降级）次数
	Unavailable int `json:"unavailable"` // 红色（不可用）次数
	Missing     int `json:"missing"`     // 灰色（无数据/未配置）次数

	// 细分统计（黄色波动细分）
	SlowLatency int `json:"slow_latency"` // 黄色-响应慢次数
	RateLimit   int `json:"rate_limit"`   // 限流次数（HTTP 429，当前视为红色不可用）

	// 细分统计（红色不可用细分）
	ServerError     int `json:"server_error"`     // 红色-服务器错误次数（5xx）
	ClientError     int `json:"client_error"`     // 红色-客户端错误次数（4xx）
	AuthError       int `json:"auth_error"`       // 红色-认证失败次数（401/403）
	InvalidRequest  int `json:"invalid_request"`  // 红色-请求参数错误次数（400）
	NetworkError    int `json:"network_error"`    // 红色-连接失败次数
	ContentMismatch int `json:"content_mismatch"` // 红色-内容校验失败次数

	// HTTP 错误码细分统计
	// key: SubStatus 类型（如 "server_error", "client_error"）
	// value: 错误码 -> 出现次数 的映射
	HttpCodeBreakdown map[string]map[int]int `json:"http_code_breakdown,omitempty"`
}

// ChannelMigrationMapping 表示 provider/service 对应的目标 channel
type ChannelMigrationMapping struct {
	Provider string
	Service  string
	Channel  string
}

// MonitorKey 监测项唯一键（provider/service/channel）
// 用于批量查询时作为 map 的 key，避免字符串拼接的歧义和冲突
type MonitorKey struct {
	Provider string
	Service  string
	Channel  string
}

// ===== 状态订阅通知（事件）相关类型 =====

// EventType 事件类型
type EventType string

const (
	EventTypeDown EventType = "DOWN" // 可用 → 不可用
	EventTypeUp   EventType = "UP"   // 不可用 → 可用
)

// ServiceState 服务状态机持久化状态
// 用于追踪每个监测项的"稳定状态"和抖动计数器
type ServiceState struct {
	Provider string
	Service  string
	Channel  string

	// StableAvailable 稳定态可用性：-1=未初始化, 0=不可用, 1=可用
	StableAvailable int

	// StreakCount 当前连续次数（累计相同可用性方向的次数）
	StreakCount int

	// StreakStatus 连续状态方向：0=不可用, 1=可用
	StreakStatus int

	// LastRecordID 最后处理的探测记录 ID
	LastRecordID int64

	// LastTimestamp 最后更新时间戳（Unix 秒）
	LastTimestamp int64
}

// StatusEvent 状态变更事件
type StatusEvent struct {
	ID       int64
	Provider string
	Service  string
	Channel  string

	// EventType 事件类型（DOWN/UP）
	EventType EventType

	// FromStatus 变更前状态码（0/1/2）
	FromStatus int

	// ToStatus 变更后状态码（0/1/2）
	ToStatus int

	// TriggerRecordID 触发该事件的探测记录 ID
	TriggerRecordID int64

	// ObservedAt 探测时间（来自 ProbeRecord.Timestamp）
	ObservedAt int64

	// CreatedAt 事件创建时间（Unix 秒）
	CreatedAt int64

	// Meta 元数据（JSON 格式，包含 http_code, latency, sub_status 等）
	Meta map[string]any
}

// EventFilters 事件查询过滤器
type EventFilters struct {
	Provider string      // 按 provider 过滤（可选）
	Service  string      // 按 service 过滤（可选）
	Channel  string      // 按 channel 过滤（可选）
	Types    []EventType // 按事件类型过滤（可选，如 ["DOWN", "UP"]）
}

// Storage 存储接口
//
// 索引依赖说明：
// - GetLatest 和 GetHistory 的性能依赖于 idx_provider_service_channel_timestamp 索引
// - 两个方法都必须包含完整的 (provider, service, channel) 等值条件
// - ⚠️ 如果新增不带 channel 参数的查询方法，需要重新评估索引策略
type Storage interface {
	// Init 初始化存储
	Init() error

	// Close 关闭存储
	Close() error

	// WithContext 返回绑定指定 context 的存储实例
	// 用于支持请求级别的超时和取消，不修改原实例，便于并发请求安全复用
	WithContext(ctx context.Context) Storage

	// SaveRecord 保存探测记录
	SaveRecord(record *ProbeRecord) error

	// GetLatest 获取最新记录
	// 要求：必须传入 provider, service, channel 三个参数（索引覆盖）
	GetLatest(provider, service, channel string) (*ProbeRecord, error)

	// GetHistory 获取历史记录（时间范围）
	// 要求：必须传入 provider, service, channel 三个参数（索引覆盖）
	GetHistory(provider, service, channel string, since time.Time) ([]*ProbeRecord, error)

	// GetLatestBatch 批量获取每个监测项的最新记录
	// 返回 map 中缺失的 key 表示该监测项没有任何记录
	// 用于 7d/30d 场景优化，将 N 个监测项的 GetLatest 从 N 次往返降为 1 次
	GetLatestBatch(keys []MonitorKey) (map[MonitorKey]*ProbeRecord, error)

	// GetHistoryBatch 批量获取多个监测项的历史记录（时间范围）
	// 返回 map 中缺失的 key 表示该监测项没有任何记录
	// 用于 7d/30d 场景优化，将 N 个监测项的 GetHistory 从 N 次往返降为 1 次
	GetHistoryBatch(keys []MonitorKey, since time.Time) (map[MonitorKey][]*ProbeRecord, error)

	// MigrateChannelData 将 channel 为空的历史记录迁移到最新配置
	// 注意：一次性操作，无需索引优化
	MigrateChannelData(mappings []ChannelMigrationMapping) error

	// ===== 状态订阅通知（事件）相关方法 =====

	// GetServiceState 获取服务状态机持久化状态
	// 返回 nil, nil 表示该监测项尚未初始化状态
	GetServiceState(provider, service, channel string) (*ServiceState, error)

	// UpsertServiceState 写入或更新服务状态机持久化状态
	UpsertServiceState(state *ServiceState) error

	// SaveStatusEvent 保存状态变更事件
	// 使用唯一约束确保幂等（相同 provider/service/channel/event_type/trigger_record_id 不重复插入）
	SaveStatusEvent(event *StatusEvent) error

	// GetStatusEvents 查询状态变更事件列表
	// sinceID: 从该 ID 之后开始（游标分页，不包含该 ID）
	// limit: 最多返回条数
	// filters: 可选过滤条件
	GetStatusEvents(sinceID int64, limit int, filters *EventFilters) ([]*StatusEvent, error)

	// GetLatestEventID 获取最新事件 ID（用于客户端初始化游标）
	// 返回 0 表示没有任何事件
	GetLatestEventID() (int64, error)
}

// ===== DB 侧时间轴聚合相关类型 =====

// DailyTimeFilter 每日时段过滤器（UTC 时区）
//
// 说明：
// - StartMinutes/EndMinutes 的取值范围为 [0, 1440]（1440 表示 24:00）
// - CrossMidnight 为 true 表示跨午夜：如 22:00-04:00
// - 语义为左闭右开区间：[start, end)
type DailyTimeFilter struct {
	StartMinutes  int
	EndMinutes    int
	CrossMidnight bool
}

// AggBucketRow 表示单个监测项在某个 bucket 内的聚合结果（由数据库返回）
//
// 注意：
//   - BucketIndex 使用与 api.buildTimeline 完全一致的"从前往后"索引：
//     0 表示最旧 bucket，bucketCount-1 表示最新 bucket
//   - Total 为 bucket 内记录总数（已应用 timeFilter 过滤）
//   - LastStatus 为 bucket 内最新一条记录的状态（用于 TimePoint.Status）
//   - LatencySum/LatencyCount 为 status > 0 的延迟聚合（与 buildTimeline 一致）
//   - AllLatencySum/AllLatencyCount 为 latency > 0 的延迟聚合（用于"全不可用时"参考）
type AggBucketRow struct {
	BucketIndex     int
	Total           int
	LastStatus      int
	LatencySum      int64
	LatencyCount    int
	AllLatencySum   int64
	AllLatencyCount int
	StatusCounts    StatusCounts
}

// TimelineAggStorage 为"时间轴聚合下推到数据库"提供的可选能力接口
//
// 仅 PostgreSQL 实现；SQLite 不实现该接口，API 层会自动回退到原有逻辑。
type TimelineAggStorage interface {
	// GetTimelineAggBatch 批量获取多个监测项的时间轴 bucket 聚合结果（时间范围）
	//
	// since/endTime 由 API 的 parseTimeRange 计算得到：
	// - 仅聚合 (since, endTime] 的数据，严格排除 timestamp==since 的边界数据
	// - 聚合窗口由 bucketCount + bucketWindow 决定（与 api.determineBucketStrategy 一致）
	GetTimelineAggBatch(keys []MonitorKey, since, endTime time.Time, bucketCount int, bucketWindow time.Duration, timeFilter *DailyTimeFilter) (map[MonitorKey][]AggBucketRow, error)
}
