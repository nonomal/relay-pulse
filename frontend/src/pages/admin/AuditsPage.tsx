import { useState, useEffect, useCallback } from 'react';
import { Helmet } from 'react-helmet-async';
import { RefreshCw, ChevronLeft, ChevronRight, Filter, X } from 'lucide-react';
import { useAdminContext } from '../../hooks/admin/useAdminContext';
import { useAdminMonitors } from '../../hooks/admin/useAdminMonitors';
import type { MonitorConfigAudit, AuditListParams } from '../../types/admin';

const PAGE_SIZE = 20;

const ACTION_LABELS: Record<string, { label: string; className: string }> = {
  create: { label: '创建', className: 'bg-success/20 text-success' },
  update: { label: '更新', className: 'bg-accent/20 text-accent' },
  delete: { label: '删除', className: 'bg-danger/20 text-danger' },
  restore: { label: '恢复', className: 'bg-warning/20 text-warning' },
  rotate_secret: { label: '密钥轮换', className: 'bg-purple-500/20 text-purple-400' },
};

function formatTimestamp(ts: number): string {
  const date = new Date(ts * 1000);
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

export default function AuditsPage() {
  const { adminFetch } = useAdminContext();
  const { listAudits } = useAdminMonitors({ adminFetch });

  const [audits, setAudits] = useState<MonitorConfigAudit[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // 过滤参数
  const [filterProvider, setFilterProvider] = useState('');
  const [filterService, setFilterService] = useState('');
  const [filterAction, setFilterAction] = useState<AuditListParams['action'] | ''>('');
  const [showFilters, setShowFilters] = useState(false);

  // 分页
  const [page, setPage] = useState(0);

  const fetchAudits = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const params: AuditListParams = {
        offset: page * PAGE_SIZE,
        limit: PAGE_SIZE,
      };
      if (filterProvider.trim()) params.provider = filterProvider.trim();
      if (filterService.trim()) params.service = filterService.trim();
      if (filterAction) params.action = filterAction;

      const resp = await listAudits(params);
      setAudits(resp.data);
      setTotal(resp.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : '请求失败');
    } finally {
      setLoading(false);
    }
  }, [listAudits, page, filterProvider, filterService, filterAction]);

  useEffect(() => {
    fetchAudits();
  }, [fetchAudits]);

  const handleApplyFilters = () => {
    setPage(0);
    setShowFilters(false);
  };

  const handleClearFilters = () => {
    setFilterProvider('');
    setFilterService('');
    setFilterAction('');
    setPage(0);
    setShowFilters(false);
  };

  const hasFilters = filterProvider || filterService || filterAction;
  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <>
      <Helmet>
        <title>审计日志 | RP Admin</title>
      </Helmet>

      <div className="space-y-4">
        {/* 标题栏 */}
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold text-primary">审计日志</h1>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setShowFilters(!showFilters)}
              className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md transition-colors ${
                hasFilters
                  ? 'bg-accent text-white'
                  : 'bg-elevated border border-muted text-secondary hover:text-primary'
              }`}
            >
              <Filter className="w-3.5 h-3.5" />
              筛选
              {hasFilters && <span className="ml-1 text-xs">(已启用)</span>}
            </button>
            <button
              onClick={fetchAudits}
              disabled={loading}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-elevated border border-muted rounded-md text-secondary hover:text-primary transition-colors"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            </button>
          </div>
        </div>

        {/* 筛选面板 */}
        {showFilters && (
          <div className="bg-surface border border-muted rounded-lg p-4 space-y-3">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <label className="block text-xs text-secondary mb-1">Provider</label>
                <input
                  type="text"
                  value={filterProvider}
                  onChange={(e) => setFilterProvider(e.target.value)}
                  placeholder="按 provider 筛选"
                  className="w-full px-3 py-1.5 bg-elevated border border-muted rounded-md text-sm text-primary placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-accent/50"
                />
              </div>
              <div>
                <label className="block text-xs text-secondary mb-1">Service</label>
                <input
                  type="text"
                  value={filterService}
                  onChange={(e) => setFilterService(e.target.value)}
                  placeholder="按 service 筛选"
                  className="w-full px-3 py-1.5 bg-elevated border border-muted rounded-md text-sm text-primary placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-accent/50"
                />
              </div>
              <div>
                <label className="block text-xs text-secondary mb-1">操作类型</label>
                <select
                  value={filterAction}
                  onChange={(e) => setFilterAction(e.target.value as AuditListParams['action'] | '')}
                  className="w-full px-3 py-1.5 bg-elevated border border-muted rounded-md text-sm text-primary focus:outline-none focus:ring-2 focus:ring-accent/50"
                >
                  <option value="">全部</option>
                  <option value="create">创建</option>
                  <option value="update">更新</option>
                  <option value="delete">删除</option>
                  <option value="restore">恢复</option>
                  <option value="rotate_secret">密钥轮换</option>
                </select>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={handleApplyFilters}
                className="px-4 py-1.5 text-sm bg-accent text-white rounded-md hover:bg-accent-strong transition-colors"
              >
                应用筛选
              </button>
              {hasFilters && (
                <button
                  onClick={handleClearFilters}
                  className="inline-flex items-center gap-1 px-3 py-1.5 text-sm text-secondary hover:text-primary transition-colors"
                >
                  <X className="w-3.5 h-3.5" />
                  清除
                </button>
              )}
            </div>
          </div>
        )}

        {/* 错误提示 */}
        {error && (
          <div className="bg-danger/10 border border-danger/30 rounded-lg p-3 text-sm text-danger">
            {error}
          </div>
        )}

        {/* 审计日志列表 */}
        <div className="bg-surface border border-muted rounded-lg overflow-hidden">
          {loading && audits.length === 0 ? (
            <div className="p-8 text-center text-secondary">加载中...</div>
          ) : audits.length === 0 ? (
            <div className="p-8 text-center text-secondary">暂无审计记录</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="bg-elevated border-b border-muted">
                  <tr>
                    <th className="px-4 py-2.5 text-left font-medium text-secondary">时间</th>
                    <th className="px-4 py-2.5 text-left font-medium text-secondary">操作</th>
                    <th className="px-4 py-2.5 text-left font-medium text-secondary">监测项</th>
                    <th className="px-4 py-2.5 text-left font-medium text-secondary">操作者</th>
                    <th className="px-4 py-2.5 text-left font-medium text-secondary">详情</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-muted">
                  {audits.map((audit) => {
                    const actionInfo = ACTION_LABELS[audit.action] || {
                      label: audit.action,
                      className: 'bg-muted text-secondary',
                    };
                    const monitorKey = [audit.provider, audit.service, audit.channel, audit.model]
                      .filter(Boolean)
                      .join('/');

                    return (
                      <tr key={audit.id} className="hover:bg-elevated/50">
                        <td className="px-4 py-2.5 text-secondary whitespace-nowrap">
                          {formatTimestamp(audit.created_at)}
                        </td>
                        <td className="px-4 py-2.5">
                          <span
                            className={`inline-block px-2 py-0.5 text-xs font-medium rounded ${actionInfo.className}`}
                          >
                            {actionInfo.label}
                          </span>
                        </td>
                        <td className="px-4 py-2.5">
                          <code className="text-xs bg-elevated px-1.5 py-0.5 rounded text-accent">
                            {monitorKey}
                          </code>
                        </td>
                        <td className="px-4 py-2.5 text-secondary">
                          {audit.actor || '-'}
                          {audit.actor_ip && (
                            <span className="ml-1 text-xs text-muted">({audit.actor_ip})</span>
                          )}
                        </td>
                        <td className="px-4 py-2.5 text-secondary">
                          <div className="flex items-center gap-2 text-xs">
                            {audit.secret_changed && (
                              <span className="px-1.5 py-0.5 bg-warning/20 text-warning rounded">
                                密钥变更
                              </span>
                            )}
                            {audit.before_version !== undefined &&
                              audit.after_version !== undefined && (
                                <span className="text-muted">
                                  v{audit.before_version} → v{audit.after_version}
                                </span>
                              )}
                            {audit.request_id && (
                              <span className="text-muted font-mono">{audit.request_id}</span>
                            )}
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}

          {/* 分页 */}
          {total > PAGE_SIZE && (
            <div className="flex items-center justify-between px-4 py-3 border-t border-muted bg-elevated/50">
              <div className="text-sm text-secondary">
                共 {total} 条记录，第 {page + 1} / {totalPages} 页
              </div>
              <div className="flex items-center gap-1">
                <button
                  onClick={() => setPage((p) => Math.max(0, p - 1))}
                  disabled={page === 0}
                  className="p-1.5 rounded text-secondary hover:text-primary hover:bg-elevated disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <ChevronLeft className="w-4 h-4" />
                </button>
                <button
                  onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
                  disabled={page >= totalPages - 1}
                  className="p-1.5 rounded text-secondary hover:text-primary hover:bg-elevated disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <ChevronRight className="w-4 h-4" />
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </>
  );
}
