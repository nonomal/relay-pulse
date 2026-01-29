-- RelayPulse v1.0.0 Phase 1 数据层 DDL（PostgreSQL）
-- 说明：该文件为 v1.0 新增表结构，实际建表逻辑内嵌到 PostgresAdminConfigStorage.Init()
-- 执行顺序：先创建基础表，再创建依赖表，最后添加外键约束和修改现有表

-- 需要 gen_random_uuid() 和加密函数
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- =====================================================
-- 1. 用户体系
-- =====================================================

-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    github_id BIGINT UNIQUE NOT NULL,
    username TEXT NOT NULL,
    avatar_url TEXT,
    email TEXT,
    role TEXT NOT NULL DEFAULT 'user',  -- admin/user
    status TEXT NOT NULL DEFAULT 'active',  -- active/disabled
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

-- 用户会话表
CREATE TABLE IF NOT EXISTS user_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    expires_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    last_seen_at BIGINT,              -- 最后活跃时间
    revoked_at BIGINT,                -- 撤销时间
    ip TEXT NOT NULL DEFAULT '',      -- 登录 IP
    user_agent TEXT NOT NULL DEFAULT '' -- User-Agent
);
CREATE INDEX IF NOT EXISTS idx_user_sessions_token ON user_sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires ON user_sessions(expires_at);

-- =====================================================
-- 2. 服务类型（Claude Code, Codex, Gemini 等）
-- =====================================================

CREATE TABLE IF NOT EXISTS services (
    id TEXT PRIMARY KEY,  -- cc, cx, gm
    name TEXT NOT NULL,   -- Claude Code, Codex, Gemini
    icon_svg TEXT,        -- SVG 图标源码
    default_template_id INT,  -- 默认模板（用户提交时自动使用），延迟添加外键
    status TEXT NOT NULL DEFAULT 'active',
    sort_order INT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

-- =====================================================
-- 3. 监测模板系统
-- =====================================================

-- 监测模板（基础配置）
CREATE TABLE IF NOT EXISTS monitor_templates (
    id SERIAL PRIMARY KEY,
    service_id TEXT NOT NULL REFERENCES services(id),
    name TEXT NOT NULL,           -- 模板名称
    slug TEXT NOT NULL,           -- 模板标识
    description TEXT,
    is_default BOOLEAN NOT NULL DEFAULT false,

    -- 基础请求配置（可被模型级覆盖）
    -- 注意：request_url 由用户提交，不在模板中定义
    request_method TEXT NOT NULL DEFAULT 'POST',
    base_request_headers JSONB,   -- 通用请求头（含占位符如 {{API_KEY}}）
    base_request_body JSONB,      -- 通用请求体（含占位符）
    base_response_checks JSONB,   -- 通用响应检查

    timeout_ms INT NOT NULL DEFAULT 10000,
    slow_latency_ms INT NOT NULL DEFAULT 5000,
    retry_policy JSONB,           -- 重试策略

    created_by UUID REFERENCES users(id),
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    UNIQUE(service_id, slug)
);

-- 模板多模型配置
CREATE TABLE IF NOT EXISTS monitor_template_models (
    id SERIAL PRIMARY KEY,
    template_id INT NOT NULL REFERENCES monitor_templates(id) ON DELETE CASCADE,
    model_key TEXT NOT NULL,          -- 模型标识，如 "gpt-4.1", "claude-3.5"
    display_name TEXT NOT NULL,       -- 显示名称

    -- 模型级覆盖配置
    request_body_overrides JSONB,     -- 覆盖 base_request_body 的字段
    response_checks_overrides JSONB,  -- 覆盖 base_response_checks 的字段

    enabled BOOLEAN NOT NULL DEFAULT true,
    sort_order INT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    UNIQUE(template_id, model_key)
);
CREATE INDEX IF NOT EXISTS idx_template_models_template ON monitor_template_models(template_id, enabled);

-- =====================================================
-- 4. 监测项申请系统
-- =====================================================

-- 监测项申请表
CREATE TABLE IF NOT EXISTS monitor_applications (
    id SERIAL PRIMARY KEY,
    applicant_user_id UUID NOT NULL REFERENCES users(id),
    service_id TEXT NOT NULL REFERENCES services(id),
    template_id INT NOT NULL REFERENCES monitor_templates(id),

    -- 模板快照（防止模板修改影响已提交申请）
    template_snapshot JSONB NOT NULL,  -- 申请时的模板配置快照

    -- 申请信息
    provider_name TEXT NOT NULL,      -- 服务商名称
    channel_name TEXT,                -- 通道名称（可选）
    vendor_type TEXT NOT NULL,        -- merchant/individual
    website_url TEXT,                 -- 官网地址（可带 AFF）
    request_url TEXT NOT NULL,        -- API 端点 URL（用户提交）

    -- API Key 安全存储（使用 AES-GCM 加密）
    api_key_encrypted BYTEA,          -- 加密的 API Key
    api_key_nonce BYTEA,              -- 加密随机数

    -- 状态流转: pending_test → test_failed/test_passed → pending_review → approved/rejected
    status TEXT NOT NULL DEFAULT 'pending_test',
    reject_reason TEXT,

    -- 审核信息
    reviewer_user_id UUID REFERENCES users(id),
    reviewed_at BIGINT,

    -- 关联最后一次成功的测试会话（延迟添加外键）
    last_test_session_id INT,

    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_applications_status ON monitor_applications(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_applications_user ON monitor_applications(applicant_user_id, created_at DESC);
-- 防止重复申请同一服务商+通道（排除已拒绝的）
CREATE UNIQUE INDEX IF NOT EXISTS uq_applications_provider_channel
    ON monitor_applications(applicant_user_id, provider_name, COALESCE(channel_name, ''))
    WHERE status NOT IN ('rejected');

-- 申请测试会话（一次完整的多模型测试）
CREATE TABLE IF NOT EXISTS application_test_sessions (
    id SERIAL PRIMARY KEY,
    application_id INT NOT NULL REFERENCES monitor_applications(id) ON DELETE CASCADE,

    -- 模板快照（测试时锁定，确保结果与审核依据一致）
    template_snapshot JSONB NOT NULL,

    status TEXT NOT NULL DEFAULT 'pending',  -- pending/running/done
    summary JSONB,                    -- 聚合统计 {"total": 3, "passed": 2, "failed": 1, "avg_latency_ms": 820}

    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_test_sessions_app ON application_test_sessions(application_id, created_at DESC);

-- 申请测试结果（每个模型的测试结果）
CREATE TABLE IF NOT EXISTS application_test_results (
    id SERIAL PRIMARY KEY,
    session_id INT NOT NULL REFERENCES application_test_sessions(id) ON DELETE CASCADE,
    template_model_id INT NOT NULL,   -- 快照中的模型 ID（不用外键，因为模板可能变更）
    model_key TEXT NOT NULL,

    status TEXT NOT NULL,             -- pass/fail
    latency_ms INT,
    http_code INT,
    error_message TEXT,

    -- 请求/响应快照（脱敏后存储，用于调试和审核）
    request_snapshot JSONB,           -- 脱敏：移除 Authorization、API Key
    response_snapshot JSONB,          -- 脱敏：截断过长内容

    checked_at BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_test_results_session ON application_test_results(session_id, status);

-- =====================================================
-- 5. 监测项表（新建，替代 monitor_configs）
-- =====================================================

CREATE TABLE IF NOT EXISTS monitors (
    id SERIAL PRIMARY KEY,

    -- 四元组标识（保留 provider）
    provider TEXT NOT NULL,                      -- 服务商标识，如 "88code"
    provider_name TEXT NOT NULL DEFAULT '',      -- 服务商显示名称
    service TEXT NOT NULL,                       -- 服务标识，如 "cc"
    service_name TEXT NOT NULL DEFAULT '',       -- 服务显示名称
    channel TEXT NOT NULL DEFAULT '',            -- 通道标识，如 "vip1"
    channel_name TEXT NOT NULL DEFAULT '',       -- 通道显示名称
    model TEXT NOT NULL DEFAULT '',              -- 模型标识（可多个，逗号分隔）

    -- 模板关联（自动跟随模板更新）
    template_id INT REFERENCES monitor_templates(id),

    -- 配置（可覆盖模板配置）
    url TEXT NOT NULL DEFAULT '',                -- 请求 URL
    method TEXT NOT NULL DEFAULT 'POST',
    headers JSONB,                               -- 请求头
    body JSONB,                                  -- 请求体
    success_contains TEXT NOT NULL DEFAULT '',   -- 响应检查
    interval TEXT NOT NULL DEFAULT '1m',
    timeout TEXT NOT NULL DEFAULT '10s',
    slow_latency TEXT NOT NULL DEFAULT '5s',

    -- 状态
    enabled BOOLEAN NOT NULL DEFAULT true,
    board_id INT,                                -- 板块

    -- 用户提交相关
    owner_user_id UUID REFERENCES users(id),     -- 用户提交的监测项
    vendor_type TEXT NOT NULL DEFAULT '',        -- merchant/individual
    website_url TEXT NOT NULL DEFAULT '',        -- 官网地址（可带 AFF）
    application_id INT,                          -- 关联申请（延迟添加外键）

    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);
-- 唯一性约束：(provider, service, channel)
CREATE UNIQUE INDEX IF NOT EXISTS uq_monitors_key
    ON monitors(provider, service, channel);
CREATE INDEX IF NOT EXISTS idx_monitors_service ON monitors(service, enabled);
CREATE INDEX IF NOT EXISTS idx_monitors_board ON monitors(board_id, enabled);
CREATE INDEX IF NOT EXISTS idx_monitors_owner ON monitors(owner_user_id);

-- =====================================================
-- 6. 迁移映射表（用于关联历史数据）
-- =====================================================

CREATE TABLE IF NOT EXISTS monitor_id_mapping (
    old_id INT NOT NULL DEFAULT 0,              -- monitor_configs.id（可为 0 表示无旧 ID）
    new_id INT NOT NULL REFERENCES monitors(id),
    legacy_provider TEXT NOT NULL DEFAULT '',   -- 旧 provider
    legacy_service TEXT NOT NULL DEFAULT '',    -- 旧 service
    legacy_channel TEXT NOT NULL DEFAULT '',    -- 旧 channel
    migrated_at BIGINT NOT NULL,
    PRIMARY KEY (new_id)                        -- 以 new_id 为主键，确保一对一映射
);
-- 支持通过旧 ID 查询（当 old_id > 0 时）
CREATE UNIQUE INDEX IF NOT EXISTS uq_mapping_old_id
    ON monitor_id_mapping(old_id) WHERE old_id > 0;
-- 支持通过旧三元组查询
CREATE UNIQUE INDEX IF NOT EXISTS uq_mapping_legacy_key
    ON monitor_id_mapping(legacy_provider, legacy_service, legacy_channel)
    WHERE legacy_provider != '' OR legacy_service != '' OR legacy_channel != '';

-- =====================================================
-- 7. 管理员操作审计日志
-- =====================================================

CREATE TABLE IF NOT EXISTS admin_audit_logs (
    id SERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    action TEXT NOT NULL,           -- create/update/delete/approve/reject
    resource_type TEXT NOT NULL,    -- template/service/monitor/badge/user/application
    resource_id TEXT NOT NULL,
    changes JSONB,                  -- 变更内容（旧值 → 新值）
    ip_address TEXT,
    user_agent TEXT,
    created_at BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user ON admin_audit_logs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON admin_audit_logs(resource_type, resource_id, created_at DESC);

-- =====================================================
-- 8. 现有表修改
-- =====================================================

-- 徽标表新增字段
ALTER TABLE badge_definitions ADD COLUMN IF NOT EXISTS category TEXT;  -- sponsor_level/metric/negative/vendor_type
ALTER TABLE badge_definitions ADD COLUMN IF NOT EXISTS svg_source TEXT;  -- SVG 源码

-- probe_history 增加 monitor_id 字段
ALTER TABLE probe_history ADD COLUMN IF NOT EXISTS monitor_id INT REFERENCES monitors(id);
CREATE INDEX IF NOT EXISTS idx_probe_history_monitor ON probe_history(monitor_id, created_at DESC);

-- status_events 增加 monitor_id 字段
ALTER TABLE status_events ADD COLUMN IF NOT EXISTS monitor_id INT REFERENCES monitors(id);
CREATE INDEX IF NOT EXISTS idx_status_events_monitor ON status_events(monitor_id, created_at DESC);

-- =====================================================
-- 9. 延迟添加的外键约束（避免循环依赖）
-- =====================================================

-- 服务默认模板外键
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'fk_services_default_template'
    ) THEN
        ALTER TABLE services ADD CONSTRAINT fk_services_default_template
            FOREIGN KEY (default_template_id) REFERENCES monitor_templates(id);
    END IF;
END $$;

-- 申请关联测试会话外键
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'fk_applications_test_session'
    ) THEN
        ALTER TABLE monitor_applications ADD CONSTRAINT fk_applications_test_session
            FOREIGN KEY (last_test_session_id) REFERENCES application_test_sessions(id);
    END IF;
END $$;

-- 监测项关联申请外键
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'fk_monitors_application'
    ) THEN
        ALTER TABLE monitors ADD CONSTRAINT fk_monitors_application
            FOREIGN KEY (application_id) REFERENCES monitor_applications(id);
    END IF;
END $$;
