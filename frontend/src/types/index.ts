// API 响应类型定义
export interface TimePoint {
  time: string;         // 格式化时间标签（如 "15:04" 或 "2006-01-02"）
  timestamp: number;    // Unix 时间戳（秒）
  status: number;       // 1=可用, 0=不可用, 2=波动, -1=缺失（bucket内最后一条）
  latency: number;      // 平均延迟(ms)
  availability: number; // 可用率百分比(0-100)，缺失时为 -1
  status_counts?: StatusCounts; // 各状态计数（可选，向后兼容）
}

export interface StatusCounts {
  available: number;   // 绿色（可用）次数
  degraded: number;    // 黄色（波动/降级）次数
  unavailable: number; // 红色（不可用）次数
  missing: number;     // 灰色（无数据/未配置）次数

  // 黄色波动细分
  slow_latency: number; // 响应慢次数
  rate_limit: number;   // 限流次数

  // 红色不可用细分
  server_error: number;     // 服务器错误次数（5xx）
  client_error: number;     // 客户端错误次数（4xx）
  auth_error: number;       // 认证失败次数（401/403）
  invalid_request: number;  // 请求参数错误次数（400）
  network_error: number;    // 连接失败次数
  content_mismatch: number; // 内容校验失败次数
}

export interface CurrentStatus {
  status: number;
  latency: number;
  timestamp: number;
}

// 赞助商等级类型
export type SponsorLevel = 'basic' | 'advanced' | 'enterprise';

// 赞助商置顶配置（来自 API meta）
export interface SponsorPinConfig {
  enabled: boolean;
  max_pinned: number;
  min_uptime: number;
  min_level: SponsorLevel;
}

// 风险徽标（单级）
export interface RiskBadge {
  label: string;           // 简短标签，如"跑路风险"
  discussionUrl?: string;  // 讨论页面链接（可选）
}

export interface MonitorResult {
  provider: string;
  provider_slug: string;               // URL slug（用于生成专属页面链接）
  provider_url?: string;               // 服务商官网链接
  service: string;
  category: 'commercial' | 'public';  // 分类：commercial（商业站）或 public（公益站）
  sponsor: string;                     // 赞助者
  sponsor_url?: string;                // 赞助者链接
  sponsor_level?: SponsorLevel;        // 赞助商等级：basic/advanced/enterprise
  risks?: RiskBadge[];                 // 风险徽标数组
  price_min?: number;                  // 参考倍率下限
  price_max?: number;                  // 参考倍率
  listed_days?: number;                // 收录天数
  channel: string;                     // 业务通道标识
  current_status: CurrentStatus | null;
  timeline: TimePoint[];
}

export interface ApiResponse {
  meta: {
    period: string;
    count: number;
    timeline_mode?: 'raw' | 'aggregated';  // 时间线模式：raw=原始记录，aggregated=聚合数据
    slow_latency_ms?: number;  // 慢延迟阈值（毫秒），用于延迟颜色渐变
    sponsor_pin?: SponsorPinConfig;  // 赞助商置顶配置
  };
  data: MonitorResult[];
}

// 前端状态枚举
export type StatusKey = 'AVAILABLE' | 'DEGRADED' | 'UNAVAILABLE' | 'MISSING';

export interface StatusConfig {
  color: string;
  text: string;
  glow: string;
  label: string;
  weight: number;
}

export const STATUS_MAP: Record<number, StatusKey> = {
  1: 'AVAILABLE',
  2: 'DEGRADED',
  0: 'UNAVAILABLE',
  '-1': 'MISSING',  // 缺失数据
};

// 处理后的数据类型
export interface ProcessedMonitorData {
  id: string;
  providerId: string;
  providerSlug: string;                // URL slug（用于生成专属页面链接）
  providerName: string;
  providerUrl?: string | null;         // 服务商官网链接
  serviceType: string;
  category: 'commercial' | 'public';  // 分类
  sponsor: string;                     // 赞助者
  sponsorUrl?: string | null;          // 赞助者链接
  sponsorLevel?: SponsorLevel;         // 赞助商等级
  risks?: RiskBadge[];                 // 风险徽标数组
  priceMin?: number | null;            // 参考倍率下限
  priceMax?: number | null;            // 参考倍率
  listedDays?: number | null;          // 收录天数
  channel?: string;                    // 业务通道标识
  pinned?: boolean;                    // 是否为置顶项（由排序逻辑标记）
  history: Array<{
    index: number;
    status: StatusKey;
    timestamp: string;
    timestampNum: number;     // Unix 时间戳（秒）
    latency: number;
    availability: number;     // 可用率百分比(0-100)，缺失时为 -1
    statusCounts: StatusCounts; // 各状态计数
  }>;
  currentStatus: StatusKey;
  uptime: number;             // 可用率百分比
  lastCheckTimestamp?: number; // 最后检测时间（Unix 时间戳，秒）
  lastCheckLatency?: number;   // 最后检测延迟（毫秒）
}

// 时间范围配置
export interface TimeRange {
  id: string;
  label: string;
  points: number;
  unit: 'minute' | 'hour' | 'day';
}

// 服务商配置
export interface Provider {
  id: string;
  name: string;
  services: string[];
}

// 排序配置
export interface SortConfig {
  key: string;
  direction: 'asc' | 'desc';
}

// Tooltip 状态
export interface TooltipState {
  show: boolean;
  x: number;
  y: number;
  data: {
    index: number;
    status: StatusKey;
    timestamp: string;
    timestampNum: number;  // Unix 时间戳（秒）
    latency: number;
    availability: number;  // 可用率百分比(0-100)，缺失时为 -1
    statusCounts: StatusCounts; // 各状态计数
  } | null;
}

// 视图模式
export type ViewMode = 'table' | 'grid';

// 服务商选项（用于筛选器）
export interface ProviderOption {
  value: string;  // 规范化的键（小写），用于筛选
  label: string;  // 显示标签（保留原始大小写）
}

// 时段筛选预设
export interface TimeFilterPreset {
  id: string;           // 预设 ID（如 'all', 'work', 'morning'）
  labelKey: string;     // i18n 翻译 key
  value: string | null; // 时段值：null=全天, "09:00-17:00"=自定义
}
