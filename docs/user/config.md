# 配置手册

> **Audience**: 用户 | **Last reviewed**: 2025-11-21

本文档详细说明 Relay Pulse 的配置选项、环境变量和最佳实践。

## 配置文件结构

Relay Pulse 使用 YAML 格式的配置文件，默认路径为 `config.yaml`。

### 完整配置示例

```yaml
# 全局配置
interval: "1m"           # 巡检间隔（支持 Go duration 格式）
slow_latency: "5s"       # 慢请求阈值

# 赞助商置顶配置
sponsor_pin:
  enabled: true          # 是否启用置顶功能
  max_pinned: 3          # 最多置顶数量
  min_uptime: 95.0       # 最低可用率要求
  min_level: "basic"     # 最低赞助级别

# 存储配置
storage:
  type: "sqlite"         # 存储类型: sqlite 或 postgres
  sqlite:
    path: "monitor.db"   # SQLite 数据库文件路径
  # PostgreSQL 配置（可选）
  postgres:
    host: "localhost"
    port: 5432
    user: "monitor"
    password: "password"  # 建议使用环境变量
    database: "llm_monitor"
    sslmode: "disable"    # 生产环境建议 "require"
    max_open_conns: 25
    max_idle_conns: 5
    conn_max_lifetime: "1h"

# 监控项列表
monitors:
  - provider: "88code"         # 服务商标识（必填）
    service: "cc"              # 服务类型（必填）
    category: "commercial"     # 分类（必填）: commercial 或 public
    sponsor: "团队自有"         # 赞助者（必填）
    sponsor_level: "advanced"  # 赞助等级（可选）: basic/advanced/enterprise
    channel: "vip"             # 业务通道（可选）
    price_ratio: 0.8           # 承诺倍率（可选）: 服务商声明的价格倍率
    price_variance: 0.1        # 倍率浮动（可选）: 显示为 "0.8±0.1"
    url: "https://api.88code.com/v1/chat/completions"  # 健康检查端点（必填）
    method: "POST"             # HTTP 方法（必填）
    api_key: "sk-xxx"          # API 密钥（可选，建议用环境变量）
    headers:                   # 请求头（可选）
      Authorization: "Bearer {{API_KEY}}"
      Content-Type: "application/json"
    body: |                    # 请求体（可选）
      {
        "model": "claude-3-opus",
        "messages": [{"role": "user", "content": "hi"}],
        "max_tokens": 1
      }
    success_contains: "content"  # 响应体必须包含的关键字（可选）
```

## 配置项详解

### 全局配置

#### `interval`
- **类型**: string (Go duration 格式)
- **默认值**: `"1m"`
- **说明**: 健康检查的间隔时间
- **示例**: `"30s"`, `"1m"`, `"5m"`, `"1h"`

#### `slow_latency`
- **类型**: string (Go duration 格式)
- **默认值**: `"5s"`
- **说明**: 超过此阈值的请求被标记为"慢请求"（黄色状态）
- **示例**: `"3s"`, `"5s"`, `"10s"`

#### `degraded_weight`
- **类型**: float
- **默认值**: `0.7`
- **说明**: 黄色状态在可用率统计中的权重，合法范围 0-1；填 0 视为未配置，使用默认值 0.7
- **计算公式**: `可用率 = (绿色次数 × 1.0 + 黄色次数 × degraded_weight) / 总次数 × 100`

### 赞助商置顶配置

用于在页面初始加载时置顶符合条件的赞助商监控项，用户点击任意排序按钮后置顶失效，刷新页面恢复。

```yaml
sponsor_pin:
  enabled: true           # 是否启用置顶功能（默认 true）
  max_pinned: 3           # 最多置顶数量（默认 3）
  min_uptime: 95.0        # 最低可用率要求（默认 95%）
  min_level: "basic"      # 最低赞助级别（默认 basic）
```

#### `sponsor_pin.enabled`
- **类型**: boolean
- **默认值**: `true`
- **说明**: 是否启用赞助商置顶功能

#### `sponsor_pin.max_pinned`
- **类型**: integer
- **默认值**: `3`
- **说明**: 最多置顶的赞助商数量

#### `sponsor_pin.min_uptime`
- **类型**: float
- **默认值**: `95.0`
- **说明**: 置顶的最低可用率要求（百分比），低于此值的赞助商不会被置顶

#### `sponsor_pin.min_level`
- **类型**: string
- **默认值**: `"basic"`
- **可选值**: `"basic"`, `"advanced"`, `"enterprise"`
- **说明**: 置顶的最低赞助级别，级别低于此值的赞助商不会被置顶
- **级别权重**: `enterprise` > `advanced` > `basic`

#### 置顶规则

1. **置顶条件**: 监控项必须同时满足以下条件才会被置顶：
   - 有 `sponsor_level` 配置
   - 无风险标记（`risks` 数组为空或未配置）
   - 可用率 ≥ `min_uptime`
   - 赞助级别 ≥ `min_level`

2. **排序规则**:
   - 置顶项按赞助级别排序（`enterprise` > `advanced` > `basic`）
   - 同级别按可用率降序排序
   - 其余项按可用率降序排序

3. **视觉效果**: 置顶项显示对应徽标颜色的淡色背景（5% 透明度）

4. **交互行为**:
   - 用户点击任意排序按钮后，置顶效果失效
   - 刷新页面后，置顶效果恢复

#### `max_concurrency`
- **类型**: integer
- **默认值**: `10`
- **说明**: 单轮巡检允许的最大并发探测数
- **特殊值**:
  - `0` 或未配置: 使用默认值 10
  - `-1`: 无限制，自动扩容到监控项数量
  - `>0`: 硬上限，超过时排队等待
- **调优建议**:
  - 小规模 (<20 项): 10-20
  - 中等规模 (20-100 项): 50-100
  - 大规模 (>100 项): `-1` 或更高值

#### `stagger_probes`
- **类型**: boolean
- **默认值**: `true`
- **说明**: 是否在单个周期内对探测进行错峰分布，避免流量突发
- **行为**:
  - `true`: 将监控项均匀分散在整个巡检周期内执行（推荐）
  - `false`: 所有监控项同时执行（仅用于调试或压测）

#### `public_base_url`
- **类型**: string
- **默认值**: `"https://relaypulse.top"`
- **说明**: 对外访问的基础 URL，用于生成 sitemap、分享链接等
- **环境变量**: `MONITOR_PUBLIC_BASE_URL`
- **格式要求**: 必须是 `http://` 或 `https://` 协议

#### `enable_concurrent_query`
- **类型**: boolean
- **默认值**: `false`
- **说明**: 启用 API 并发查询优化，显著降低 `/api/status` 接口响应时间
- **性能提升**: 10 个监控项查询时间从 ~2s 降至 ~300ms（-85%）
- **适用场景**:
  - ✅ PostgreSQL 存储（推荐）
  - ❌ SQLite 存储（无效果，会产生警告）
- **注意事项**:
  - 需要确保数据库连接池配置充足（建议 `max_open_conns >= 50`）
  - 默认关闭，向后兼容现有配置

**示例配置：**
```yaml
# 启用并发查询优化（推荐 PostgreSQL 用户启用）
enable_concurrent_query: true
```

#### `concurrent_query_limit`
- **类型**: integer
- **默认值**: `10`
- **说明**: 并发查询时的最大并发度，限制同时执行的数据库查询数量
- **仅当** `enable_concurrent_query=true` **时生效**
- **配置建议**:
  ```
  max_open_conns >= concurrent_query_limit × 并发请求数 × 1.2
  ```
  - 示例：`50 >= 10 × 3 × 1.2 = 36`（安全）
  - 如果配置不当，启动时会看到警告：
    ```
    [Config] 警告: max_open_conns(25) < concurrent_query_limit(10)，可能导致连接池等待
    ```

**示例配置：**
```yaml
enable_concurrent_query: true
concurrent_query_limit: 10  # 根据数据库连接池大小调整
```

### 存储配置

#### SQLite（默认）

```yaml
storage:
  type: "sqlite"
  sqlite:
    path: "monitor.db"  # 数据库文件路径（相对或绝对路径）
```

**适用场景**:
- 单机部署
- 开发环境
- 小规模监控（< 100 个监控项）

**限制**:
- 不支持多副本（水平扩展）
- K8s 环境需要 PersistentVolume

#### PostgreSQL

```yaml
storage:
  type: "postgres"
  postgres:
    host: "postgres-service"    # 数据库主机
    port: 5432                  # 端口
    user: "monitor"             # 用户名
    password: "secret"          # 密码（建议用环境变量）
    database: "llm_monitor"     # 数据库名
    sslmode: "require"          # SSL 模式: disable, require, verify-full
    max_open_conns: 50          # 最大打开连接数（自动调整）
    max_idle_conns: 10          # 最大空闲连接数（自动调整）
    conn_max_lifetime: "1h"     # 连接最大生命周期
```

**连接池自动调整**：
- 如果未配置 `max_open_conns` 和 `max_idle_conns`，系统会根据 `enable_concurrent_query` 自动设置：
  - **并发模式**（`enable_concurrent_query=true`）：`50` / `10`
  - **串行模式**（`enable_concurrent_query=false`）：`25` / `5`
- 如果已配置，则使用配置值（不会自动调整）

**连接池大小建议**：

| 配置项 | 计算公式 | 示例 |
|--------|---------|------|
| `max_open_conns` | `max_concurrency + concurrent_query_limit × 并发请求数 + 缓冲` | `15 + 10 × 2 + 5 = 40` |
| `max_idle_conns` | `max_open_conns / 3 ~ 5` | `40 / 4 = 10` |

- **`max_concurrency`**：探测并发数（配置中的 `max_concurrency`，默认 15）
- **`concurrent_query_limit`**：API 查询并发数（配置中的 `concurrent_query_limit`，默认 10）
- **并发请求数**：预期同时访问 `/api/status` 的用户数

**示例配置**（42 个监控项，生产环境）：

```yaml
# 探测配置
max_concurrency: 15        # 探测并发数
stagger_probes: true       # 错峰调度

# API 查询优化
enable_concurrent_query: true
concurrent_query_limit: 10

# PostgreSQL 连接池
storage:
  type: "postgres"
  postgres:
    max_open_conns: 40     # 15 + 10 × 2 + 5 = 40
    max_idle_conns: 10
    conn_max_lifetime: "1h"
```

**适用场景**:
- Kubernetes 多副本部署
- 高可用需求
- 大规模监控（> 100 个监控项）

**初始化数据库**:

```sql
CREATE DATABASE llm_monitor;
CREATE USER monitor WITH PASSWORD 'your_password';
GRANT ALL PRIVILEGES ON DATABASE llm_monitor TO monitor;
```

### 数据保留策略

- 服务会自动保留最近 30 天的 `probe_history` 数据，后台定时器每 24 小时调用 `CleanOldRecords(30)` 删除更早的样本。
- 该策略对 SQLite 与 PostgreSQL 均生效，无需额外配置即可防止数据库无限增长。
- 保留窗口目前固定为 30 天，如需调整需修改源码或在 Issue 中提出新特性需求。
- 运维层面的验证与手动清理命令请参考 [运维手册 - 数据保留策略（历史文档，仅供参考）](../../archive/docs/user/operations.md#数据保留策略)。

### 监控项配置

#### 必填字段

##### `provider`
- **类型**: string
- **说明**: 服务商标识（用于分组和显示）
- **示例**: `"openai"`, `"anthropic"`, `"88code"`

##### `service`
- **类型**: string
- **说明**: 服务类型标识
- **示例**: `"gpt-4"`, `"claude"`, `"cc"`, `"cx"`

##### `category`
- **类型**: string
- **说明**: 分类标识
- **可选值**: `"commercial"`（推广站）, `"public"`（公益站）

##### `sponsor`
- **类型**: string
- **说明**: 提供 API Key 的赞助者名称
- **示例**: `"团队自有"`, `"用户捐赠"`, `"John Doe"`

##### `url`
- **类型**: string
- **说明**: 健康检查的 HTTP 端点
- **示例**: `"https://api.openai.com/v1/chat/completions"`

##### `method`
- **类型**: string
- **说明**: HTTP 请求方法
- **可选值**: `"GET"`, `"POST"`, `"PUT"`, `"DELETE"`, `"PATCH"`

#### 可选字段

##### `provider_slug`
- **类型**: string
- **说明**: 服务商的 URL 短标识，用于生成 `/p/<slug>` 专属页面链接
- **默认值**: 未配置时自动使用 `provider` 的小写形式
- **格式要求**: 仅允许小写字母 (a-z)、数字 (0-9)、连字符 (-)，不能以连字符开头或结尾，不能有连续连字符
- **示例**: `"88code"`, `"openai"`, `"my-provider"`

##### `provider_url`
- **类型**: string
- **说明**: 服务商官网链接（可选），前端展示为外部跳转
- **格式要求**: 必须是 `http://` 或 `https://` 协议
- **示例**: `"https://88code.com"`, `"https://openai.com"`

##### `sponsor_url`
- **类型**: string
- **说明**: 赞助者展示用链接（可选），例如个人主页或组织网站
- **格式要求**: 必须是 `http://` 或 `https://` 协议
- **示例**: `"https://example.com/sponsor"`

##### `sponsor_level`
- **类型**: string
- **说明**: 赞助商等级徽章（可选），在前端显示对应图标
- **有效值**:
  | 值 | 名称 | 图标 | 说明 |
  |---|---|---|---|
  | `basic` | 节点支持 | 🔻 | 已赞助高频监测资源 |
  | `advanced` | 核心服务商 | ⬢ | 多线路深度监测 |
  | `enterprise` | 全球伙伴 | 💠 | RelayPulse 顶级赞助商 |
- **示例**: `"advanced"`

##### `channel`
- **类型**: string
- **说明**: 业务通道标识（用于区分同一服务的不同渠道）
- **示例**: `"vip"`, `"free"`, `"premium"`

##### `price_ratio`
- **类型**: number（可选）
- **说明**: 服务商上架时声明的承诺倍率，相对于官方价格的比例
- **约束**: 不能为负数
- **排序**: 支持在表格中按倍率排序，未配置的排最后
- **示例**: `0.8`（表示官方价格的 0.8 倍）

##### `price_variance`
- **类型**: number（可选）
- **说明**: 倍率的浮动范围，前端显示为 `price_ratio±price_variance`
- **约束**:
  - 不能为负数
  - 必须同时配置 `price_ratio`
  - `price_ratio - price_variance` 不能为负数（即下界不能为负）
- **示例**: `0.1`（配合 `price_ratio: 0.8` 显示为 "0.8±0.1"）

##### `api_key`
- **类型**: string
- **说明**: API 密钥（强烈建议使用环境变量代替）
- **示例**: `"sk-xxx"`

##### `headers`
- **类型**: map[string]string
- **说明**: 自定义请求头
- **占位符**: `{{API_KEY}}` 会被替换为实际的 API Key
- **示例**:
  ```yaml
  headers:
    Authorization: "Bearer {{API_KEY}}"
    Content-Type: "application/json"
    X-Custom-Header: "value"
  ```

##### `body`
- **类型**: string 或 `!include` 引用
- **说明**: 请求体内容
- **占位符**: `{{API_KEY}}` 会被替换
- **示例**:
  ```yaml
  # 内联方式
  body: |
    {
      "model": "gpt-4",
      "messages": [{"role": "user", "content": "test"}],
      "max_tokens": 1
    }

  # 引用外部文件
  body: "!include data/gpt4_request.json"
  ```

##### `success_contains`
- **类型**: string
- **说明**: 响应体必须包含的关键字（用于语义验证）
- **示例**: `"content"`, `"choices"`, `"success"`, `"pong"`
- **行为**:
  - 仅在 HTTP 返回 **2xx 状态码**、且非 429 限流场景下生效；
  - 当响应内容（包含常见流式 SSE 响应聚合后的文本）**不包含**此关键字时，
    会将该次探测标记为 **红色不可用**（`content_mismatch`），即使 HTTP 状态码是 2xx；
  - 支持常见的流式响应格式（如 Anthropic 的 `content_block_delta`、
    OpenAI 的 `choices[].delta.content`），会自动拼接增量文本再进行关键字匹配。

##### `interval`
- **类型**: string (Go duration 格式)
- **说明**: 该监控项的自定义巡检间隔（可选），覆盖全局 `interval`
- **示例**: `"30s"`, `"1m"`, `"5m"`
- **使用场景**:
  - **高频监控**：付费服务商需要更短的检测间隔（如 `"1m"`）
  - **低频监控**：成本敏感或稳定服务使用更长间隔（如 `"15m"`）
- **配置示例**:
  ```yaml
  interval: "5m"  # 全局默认 5 分钟
  monitors:
    - provider: "高优先级服务商"
      interval: "1m"   # 覆盖：每 1 分钟检测一次
      # ...
    - provider: "普通服务商"
      # 不配置 interval，使用全局 5 分钟
      # ...
  ```

##### `hidden`
- **类型**: boolean
- **默认值**: `false`
- **说明**: 临时下架该监控项（隐藏但继续监控）
- **行为**:
  - 调度器继续探测，存储结果（用于整改证据）
  - API `/api/status` 默认不返回（可加 `?include_hidden=true` 调试）
  - 前端不展示
  - sitemap 不包含
- **示例**:
  ```yaml
  - provider: "问题商家"
    service: "cc"
    hidden: true
    hidden_reason: "服务质量不达标，待整改"
  ```

##### `hidden_reason`
- **类型**: string
- **说明**: 下架原因（可选，用于运维审计）
- **示例**: `"服务质量不达标，待整改"`, `"该通道临时维护"`

##### `disabled`
- **类型**: boolean
- **默认值**: `false`
- **说明**: 彻底停用该监控项（不探测、不存储、不展示）
- **行为**:
  - 调度器不创建任务，不探测
  - 不写入数据库
  - API `/api/status` 不返回（即使加 `?include_hidden=true` 也不返回）
  - 前端不展示
  - sitemap 不包含
- **适用场景**: 商家已彻底关闭、不再需要监控
- **示例**:
  ```yaml
  - provider: "已关站商家"
    service: "cc"
    disabled: true
    disabled_reason: "商家已跑路"
  ```

##### `disabled_reason`
- **类型**: string
- **说明**: 停用原因（可选，用于运维审计）
- **示例**: `"商家已跑路"`, `"服务永久关闭"`

### 临时下架配置

用于临时下架服务商（如商家不配合整改），支持两种级别：

#### Provider 级别下架

批量下架整个服务商的所有监控项：

```yaml
hidden_providers:
  - provider: "问题商家A"
    reason: "服务质量不达标，待整改"
  - provider: "问题商家B"
    reason: "API 频繁超时，沟通整改中"

monitors:
  - provider: "问题商家A"  # 自动继承 hidden=true
    service: "cc"
    # ...
```

#### Monitor 级别下架

下架单个监控项：

```yaml
monitors:
  - provider: "正常商家"
    service: "cc"
    hidden: true                    # 临时下架
    hidden_reason: "该通道临时维护"  # 下架原因
    # ...
```

#### 优先级规则

| Provider Hidden | Monitor Hidden | 最终状态 | 原因来源 |
|-----------------|----------------|----------|----------|
| ✅ | ❌ | **隐藏** | provider.reason |
| ❌ | ✅ | **隐藏** | monitor.hidden_reason |
| ✅ | ✅ | **隐藏** | monitor.hidden_reason（优先） |
| ❌ | ❌ | **显示** | - |

#### 调试接口

```bash
# 查看包含隐藏项的完整列表（内部调试用）
curl "http://localhost:8080/api/status?include_hidden=true"
```

### 彻底停用配置

用于彻底停用服务商（如商家已跑路、永久关闭），与"临时下架"的区别是**不会继续探测和存储数据**。

#### Provider 级别停用

批量停用整个服务商的所有监控项：

```yaml
disabled_providers:
  - provider: "已跑路商家A"
    reason: "商家已跑路，不再监控"
  - provider: "已关站商家B"
    reason: "服务永久关闭"

monitors:
  - provider: "已跑路商家A"  # 自动继承 disabled=true
    service: "cc"
    # ...
```

#### Monitor 级别停用

停用单个监控项：

```yaml
monitors:
  - provider: "正常商家"
    service: "legacy-channel"
    disabled: true                    # 彻底停用
    disabled_reason: "该通道已废弃"   # 停用原因
    # ...
```

#### 优先级规则

| Provider Disabled | Monitor Disabled | 最终状态 | 原因来源 |
|-------------------|------------------|----------|----------|
| ✅ | ❌ | **停用** | provider.reason |
| ❌ | ✅ | **停用** | monitor.disabled_reason |
| ✅ | ✅ | **停用** | monitor.disabled_reason（优先） |
| ❌ | ❌ | 继续检查 hidden | - |

#### Disabled vs Hidden 对比

| 特性 | `disabled=true` | `hidden=true` |
|------|-----------------|---------------|
| 调度器探测 | ❌ 不探测 | ✅ 继续探测 |
| 数据存储 | ❌ 不存储 | ✅ 继续存储 |
| API 返回 | ❌ 永不返回 | ❌ 默认不返回，可用 `include_hidden=true` 查看 |
| 适用场景 | 商家跑路、服务永久关闭 | 临时整改、待观察 |

## 环境变量覆盖

为了安全性，强烈建议使用环境变量来管理 API Key，而不是写在配置文件中。

### API Key 环境变量

**命名规则**:

```
MONITOR_<PROVIDER>_<SERVICE>_API_KEY
```

- `<PROVIDER>`: 配置中的 `provider` 字段（大写，`-` 替换为 `_`）
- `<SERVICE>`: 配置中的 `service` 字段（大写，`-` 替换为 `_`）

**示例**:

| 配置 | 环境变量名 |
|------|-----------|
| `provider: "88code"`, `service: "cc"` | `MONITOR_88CODE_CC_API_KEY` |
| `provider: "openai"`, `service: "gpt-4"` | `MONITOR_OPENAI_GPT4_API_KEY` |
| `provider: "anthropic"`, `service: "claude-3"` | `MONITOR_ANTHROPIC_CLAUDE3_API_KEY` |

**使用方式**:

```bash
# 方式1：直接导出
export MONITOR_88CODE_CC_API_KEY="sk-your-real-key"
./monitor

# 方式2：使用 .env 文件（推荐）
echo "MONITOR_88CODE_CC_API_KEY=sk-your-real-key" > .env
docker compose --env-file .env up -d
```

### 存储配置环境变量

#### SQLite

```bash
MONITOR_STORAGE_TYPE=sqlite
MONITOR_SQLITE_PATH=/data/monitor.db
```

#### PostgreSQL

```bash
MONITOR_STORAGE_TYPE=postgres
MONITOR_POSTGRES_HOST=postgres-service
MONITOR_POSTGRES_PORT=5432
MONITOR_POSTGRES_USER=monitor
MONITOR_POSTGRES_PASSWORD=your_secure_password
MONITOR_POSTGRES_DATABASE=llm_monitor
MONITOR_POSTGRES_SSLMODE=require
```

### CORS 配置

```bash
# 允许额外的跨域来源（逗号分隔）
MONITOR_CORS_ORIGINS=http://localhost:5173,http://localhost:3000
```

### 前端环境变量

前端支持以下环境变量（需在构建时设置）：

#### API 配置

```bash
# API 基础 URL（可选，默认为相对路径）
VITE_API_BASE_URL=http://localhost:8080

# 是否使用 Mock 数据（开发调试用）
VITE_USE_MOCK_DATA=false
```

#### Google Analytics（可选）

```bash
# GA4 Measurement ID（格式: G-XXXXXXXXXX）
VITE_GA_MEASUREMENT_ID=G-XXXXXXXXXX
```

**获取 GA4 Measurement ID**：
1. 访问 [Google Analytics](https://analytics.google.com/)
2. 创建或选择属性
3. 在"管理" > "数据流" > "网站"中查看 Measurement ID

**使用方式**：

```bash
# 开发环境：在 frontend/.env.development 中设置
VITE_GA_MEASUREMENT_ID=

# 生产环境：在 frontend/.env.production 中设置
VITE_GA_MEASUREMENT_ID=G-XXXXXXXXXX

# 或在构建时通过环境变量传入
export VITE_GA_MEASUREMENT_ID=G-XXXXXXXXXX
cd frontend && npm run build
```

**追踪事件**：

GA4 会自动追踪以下事件：
- **页面浏览**（自动） - 用户访问仪表板
- **用户筛选**：
  - `change_time_range` - 切换时间范围（24h/7d/30d）
  - `filter_service` - 筛选服务提供商或服务类型
  - `filter_channel` - 筛选业务通道
  - `filter_category` - 筛选分类（commercial/public）
- **用户交互**：
  - `change_view_mode` - 切换视图模式（table/grid）
  - `manual_refresh` - 点击刷新按钮
  - `click_external_link` - 点击外部链接（查看提供商/赞助商）
- **性能监控**：
  - `api_request` - API 请求性能（包含延迟、成功/失败状态）
  - `api_error` - API 错误（包含错误类型：HTTP_XXX、NETWORK_ERROR）

**注意**：
- 开发环境建议留空 `VITE_GA_MEASUREMENT_ID`，避免污染生产数据
- 如果未设置 Measurement ID，GA4 脚本不会加载

## 配置验证

服务启动时会自动验证配置：

### 验证规则

1. **必填字段检查**: `provider`, `service`, `category`, `sponsor`, `url`, `method`
2. **HTTP 方法校验**: 必须是 `GET`, `POST`, `PUT`, `DELETE`, `PATCH` 之一
3. **唯一性检查**: `provider + service + channel` 组合必须唯一
4. **`category` 枚举**: 必须是 `commercial` 或 `public`
5. **存储类型校验**: 必须是 `sqlite` 或 `postgres`

### 验证失败示例

```bash
# 缺少必填字段
❌ 无法加载配置文件: monitor[0]: 缺少必填字段 'category'

# 重复的 provider + service + channel
❌ 无法加载配置文件: 重复的监控项: provider=88code, service=cc, channel=

# 无效的 HTTP 方法
❌ 无法加载配置文件: monitor[0]: 无效的 method 'INVALID'
```

## 配置热更新

Relay Pulse 支持配置文件的热更新，修改配置后无需重启服务。

### 工作原理

1. 使用 `fsnotify` 监听配置文件变更
2. 检测到变更后，先验证新配置
3. 如果验证通过，原子性地更新运行时配置
4. 如果验证失败，保持旧配置并输出错误日志

### 使用示例

```bash
# 启动服务
docker compose up -d

# 修改配置（添加新的监控项）
vi config.yaml

# 观察日志
docker compose logs -f monitor

# 应该看到:
# [Config] 检测到配置文件变更，正在重载...
# [Config] 热更新成功！已加载 3 个监控任务
# [Scheduler] 配置已更新，下次巡检将使用新配置
# [Scheduler] 立即触发巡检
```

### 注意事项

- **存储配置不支持热更新**: 修改 `storage` 配置需要重启服务
- **环境变量不热更新**: 环境变量覆盖的 API Key 不会热更新
- **语法错误**: 如果新配置有语法错误，服务会保持旧配置并输出错误

## 配置最佳实践

### 1. API Key 管理

❌ **不推荐**（不安全）:

```yaml
monitors:
  - provider: "openai"
    api_key: "sk-proj-real-key-here"  # 不要写在配置文件中！
```

✅ **推荐**（安全）:

```yaml
monitors:
  - provider: "openai"
    # api_key 留空，使用环境变量
```

```bash
# .env 文件（添加到 .gitignore）
MONITOR_OPENAI_GPT4_API_KEY=sk-proj-real-key-here
```

### 2. 大型请求体

❌ **不推荐**（配置文件过长）:

```yaml
body: |
  {
    "model": "gpt-4",
    "messages": [/* 很长的消息列表 */],
    "max_tokens": 1000,
    "temperature": 0.7,
    /* 更多配置... */
  }
```

✅ **推荐**（使用 `!include`）:

```yaml
body: "!include data/gpt4_request.json"
```

```json
// data/gpt4_request.json
{
  "model": "gpt-4",
  "messages": [/* 很长的消息列表 */],
  "max_tokens": 1000,
  "temperature": 0.7
}
```

### 3. 环境隔离

```bash
# 开发环境
config.yaml                # 本地开发配置
.env.local                 # 本地 API Keys（添加到 .gitignore）

# 生产环境
config.production.yaml     # 生产配置（不含敏感信息）
deploy/relaypulse.env      # 生产 API Keys（添加到 .gitignore）
```

### 4. 安全加固

1. **所有敏感信息使用环境变量**
2. **生产环境启用 PostgreSQL SSL**: `sslmode: "require"`
3. **限制 CORS**: 只允许信任的域名
4. **定期轮换 API Key**
5. **使用最小权限原则**: 数据库用户只授予必要权限

## 配置示例库

### 示例1：OpenAI GPT-4

```yaml
monitors:
  - provider: "openai"
    service: "gpt-4"
    category: "commercial"
    sponsor: "团队"
    url: "https://api.openai.com/v1/chat/completions"
    method: "POST"
    headers:
      Authorization: "Bearer {{API_KEY}}"
      Content-Type: "application/json"
    body: |
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "hi"}],
        "max_tokens": 1
      }
    success_contains: "choices"
```

### 示例2：Anthropic Claude

```yaml
monitors:
  - provider: "anthropic"
    service: "claude-3"
    category: "public"
    sponsor: "社区"
    url: "https://api.anthropic.com/v1/messages"
    method: "POST"
    headers:
      x-api-key: "{{API_KEY}}"
      anthropic-version: "2023-06-01"
      Content-Type: "application/json"
    body: |
      {
        "model": "claude-3-opus-20240229",
        "messages": [{"role": "user", "content": "hi"}],
        "max_tokens": 1
      }
    success_contains: "content"
```

### 示例3：自定义 REST API

```yaml
monitors:
  - provider: "custom-api"
    service: "health"
    category: "public"
    sponsor: "自有"
    url: "https://api.example.com/health"
    method: "GET"
    success_contains: "ok"
```

## 故障排查

### 配置不生效

1. 检查配置文件路径是否正确
2. 查看日志中的验证错误
3. 确认环境变量格式正确

### 热更新失败

1. 检查配置文件语法（YAML 格式）
2. 验证必填字段是否完整
3. 查看日志中的具体错误信息

### 数据库连接失败

1. PostgreSQL: 检查 `host`, `port`, `user`, `password` 是否正确
2. SQLite: 检查文件路径和权限
3. 查看数据库日志

## 下一步

- [运维手册（历史文档，仅供参考）](../../archive/docs/user/operations.md) - 日常运维与故障排查
- [API 端点示例](../../README.md#-api-端点) - 当前权威参考（正式 API 规范整理中）
