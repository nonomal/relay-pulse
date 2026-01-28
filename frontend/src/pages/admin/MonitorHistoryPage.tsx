import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Helmet } from 'react-helmet-async';
import { ArrowLeft, RefreshCw, History, ChevronDown, ChevronRight } from 'lucide-react';
import { useAdminContext } from '../../hooks/admin/useAdminContext';
import { useAdminMonitors } from '../../hooks/admin/useAdminMonitors';
import type { MonitorConfigAudit, MonitorConfig } from '../../types/admin';

/** 操作类型标签样式 */
const ACTION_STYLES: Record<string, { label: string; className: string }> = {
  create: { label: '创建', className: 'bg-success/15 text-success' },
  update: { label: '更新', className: 'bg-accent/15 text-accent' },
  delete: { label: '删除', className: 'bg-danger/15 text-danger' },
  restore: { label: '恢复', className: 'bg-warning/15 text-warning' },
  rotate_secret: { label: '轮换密钥', className: 'bg-accent/15 text-accent' },
};

export default function MonitorHistoryPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { adminFetch } = useAdminContext();
  const { getMonitor, getMonitorHistory } = useAdminMonitors({ adminFetch });

  const monitorId = parseInt(id || '0', 10);

  const [monitor, setMonitor] = useState<MonitorConfig | null>(null);
  const [audits, setAudits] = useState<MonitorConfigAudit[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    if (!monitorId) return;
    setLoading(true);
    setError(null);
    try {
      const [m, history] = await Promise.all([
        getMonitor(monitorId),
        getMonitorHistory(monitorId),
      ]);
      setMonitor(m);
      setAudits(history);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, [monitorId, getMonitor, getMonitorHistory]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const monitorLabel = monitor
    ? [monitor.provider, monitor.service, monitor.channel, monitor.model]
        .filter(Boolean)
        .join(' / ')
    : `#${monitorId}`;

  return (
    <>
      <Helmet>
        <title>变更历史 - {monitorLabel} | RP Admin</title>
      </Helmet>

      <div className="space-y-4 max-w-4xl">
        {/* 页头 */}
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => navigate(`/admin/monitors/${monitorId}`)}
            className="p-1.5 text-secondary hover:text-primary transition-colors"
          >
            <ArrowLeft className="w-5 h-5" />
          </button>
          <div className="flex-1">
            <h1 className="text-xl font-bold text-primary">变更历史</h1>
            <p className="text-sm text-secondary font-mono">{monitorLabel}</p>
          </div>
          <button
            onClick={fetchData}
            disabled={loading}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-elevated border border-muted rounded-md text-secondary hover:text-primary transition-colors"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            刷新
          </button>
        </div>

        {error && (
          <div className="p-3 bg-danger/10 border border-danger/20 rounded-md">
            <p className="text-sm text-danger">{error}</p>
          </div>
        )}

        {loading && audits.length === 0 ? (
          <div className="py-12 text-center text-secondary text-sm">加载中...</div>
        ) : audits.length === 0 ? (
          <div className="py-12 text-center">
            <History className="w-10 h-10 text-muted mx-auto mb-3" />
            <p className="text-sm text-secondary">暂无变更记录</p>
          </div>
        ) : (
          <div className="space-y-2">
            {audits.map((audit) => (
              <AuditEntry key={audit.id} audit={audit} />
            ))}
          </div>
        )}
      </div>
    </>
  );
}

/** 单条审计记录 */
function AuditEntry({ audit }: { audit: MonitorConfigAudit }) {
  const [expanded, setExpanded] = useState(false);
  const actionStyle = ACTION_STYLES[audit.action] || {
    label: audit.action,
    className: 'bg-muted/15 text-muted',
  };

  const time = new Date(audit.created_at * 1000).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });

  const hasDiff = audit.before_blob || audit.after_blob;

  return (
    <div className="bg-surface border border-muted rounded-lg overflow-hidden">
      <button
        type="button"
        onClick={() => hasDiff && setExpanded(!expanded)}
        className={`w-full flex items-center gap-3 px-4 py-3 text-left ${
          hasDiff ? 'cursor-pointer hover:bg-elevated/30' : 'cursor-default'
        } transition-colors`}
      >
        {/* 展开图标 */}
        <span className="text-muted shrink-0 w-4">
          {hasDiff &&
            (expanded ? (
              <ChevronDown className="w-4 h-4" />
            ) : (
              <ChevronRight className="w-4 h-4" />
            ))}
        </span>

        {/* 操作类型 */}
        <span
          className={`px-2 py-0.5 rounded text-xs font-medium shrink-0 ${actionStyle.className}`}
        >
          {actionStyle.label}
        </span>

        {/* 版本信息 */}
        <span className="text-xs text-muted tabular-nums shrink-0">
          {audit.before_version != null && audit.after_version != null
            ? `v${audit.before_version} → v${audit.after_version}`
            : audit.after_version != null
              ? `v${audit.after_version}`
              : ''}
        </span>

        {/* Secret 变更标记 */}
        {audit.secret_changed && (
          <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-warning/15 text-warning shrink-0">
            Key 已变更
          </span>
        )}

        {/* 时间 */}
        <span className="text-xs text-secondary tabular-nums ml-auto shrink-0">{time}</span>

        {/* 操作者 */}
        {audit.actor_ip && (
          <span className="text-xs text-muted shrink-0" title={audit.user_agent || ''}>
            {audit.actor_ip}
          </span>
        )}
      </button>

      {/* 展开内容：配置 diff */}
      {expanded && hasDiff && (
        <div className="border-t border-muted px-4 py-3 space-y-3">
          {audit.reason && (
            <p className="text-xs text-secondary">
              <span className="text-muted">原因：</span>
              {audit.reason}
            </p>
          )}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {audit.before_blob && (
              <div>
                <p className="text-xs text-muted mb-1">变更前</p>
                <pre className="text-xs text-secondary bg-elevated rounded-md p-3 overflow-x-auto max-h-64 overflow-y-auto font-mono">
                  {formatJSON(audit.before_blob)}
                </pre>
              </div>
            )}
            {audit.after_blob && (
              <div>
                <p className="text-xs text-muted mb-1">变更后</p>
                <pre className="text-xs text-primary bg-elevated rounded-md p-3 overflow-x-auto max-h-64 overflow-y-auto font-mono">
                  {formatJSON(audit.after_blob)}
                </pre>
              </div>
            )}
          </div>
          {audit.request_id && (
            <p className="text-[10px] text-muted">
              Request ID: <span className="font-mono">{audit.request_id}</span>
              {audit.batch_id && (
                <>
                  {' '}
                  | Batch: <span className="font-mono">{audit.batch_id}</span>
                </>
              )}
            </p>
          )}
        </div>
      )}
    </div>
  );
}

/** 格式化 JSON 字符串 */
function formatJSON(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}
