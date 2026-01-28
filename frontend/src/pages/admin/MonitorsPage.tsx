import { useState, useEffect, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Helmet } from 'react-helmet-async';
import {
  Search,
  Plus,
  RefreshCw,
  Trash2,
  RotateCcw,
  ChevronLeft,
  ChevronRight,
  ToggleLeft,
  ToggleRight,
  Pencil,
  History,
} from 'lucide-react';
import { useAdminContext } from '../../hooks/admin/useAdminContext';
import { useAdminMonitors } from '../../hooks/admin/useAdminMonitors';
import type { MonitorConfig, MonitorListParams } from '../../types/admin';

const PAGE_SIZE = 20;

export default function MonitorsPage() {
  const navigate = useNavigate();
  const { adminFetch } = useAdminContext();
  const {
    items,
    total,
    loading,
    error,
    listMonitors,
    toggleMonitorStatus,
    deleteMonitor,
    restoreMonitor,
  } = useAdminMonitors({ adminFetch });

  // 查询状态
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(0);
  const [showDeleted, setShowDeleted] = useState(false);

  // 操作中的项目 ID（用于禁用按钮防止重复操作）
  const [actioningId, setActioningId] = useState<number | null>(null);

  // 查询参数
  const params = useMemo<MonitorListParams>(
    () => ({
      search: search || undefined,
      include_deleted: showDeleted || undefined,
      offset: page * PAGE_SIZE,
      limit: PAGE_SIZE,
    }),
    [search, showDeleted, page],
  );

  // 加载数据
  const fetchData = useCallback(() => {
    listMonitors(params).catch(() => {
      // error 已在 hook 内处理
    });
  }, [listMonitors, params]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // 搜索防抖
  const [searchInput, setSearchInput] = useState('');
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearch(searchInput);
      setPage(0);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  // 总页数
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  // 操作处理
  const handleToggle = useCallback(
    async (item: MonitorConfig) => {
      setActioningId(item.id);
      try {
        await toggleMonitorStatus(item.id, !item.enabled);
        fetchData();
      } catch {
        // AdminApiError 已包含错误信息
      } finally {
        setActioningId(null);
      }
    },
    [toggleMonitorStatus, fetchData],
  );

  const handleDelete = useCallback(
    async (id: number) => {
      if (!window.confirm('确定要删除此监测项吗？（可恢复）')) return;
      setActioningId(id);
      try {
        await deleteMonitor(id);
        fetchData();
      } catch {
        // error handled
      } finally {
        setActioningId(null);
      }
    },
    [deleteMonitor, fetchData],
  );

  const handleRestore = useCallback(
    async (id: number) => {
      setActioningId(id);
      try {
        await restoreMonitor(id);
        fetchData();
      } catch {
        // error handled
      } finally {
        setActioningId(null);
      }
    },
    [restoreMonitor, fetchData],
  );

  /** 格式化 Unix 时间戳 */
  const formatTime = (ts: number): string => {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  /** 构建四元组标识 */
  const monitorKey = (m: MonitorConfig): string => {
    const parts = [m.provider, m.service];
    if (m.channel) parts.push(m.channel);
    if (m.model) parts.push(m.model);
    return parts.join(' / ');
  };

  return (
    <>
      <Helmet>
        <title>监测项管理 | RP Admin</title>
      </Helmet>

      <div className="space-y-4">
        {/* 页头 */}
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
          <h1 className="text-xl font-bold text-primary">监测项管理</h1>
          <div className="flex items-center gap-2">
            <button
              onClick={fetchData}
              disabled={loading}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-elevated border border-muted rounded-md text-secondary hover:text-primary transition-colors disabled:opacity-50"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
              刷新
            </button>
            <button
              onClick={() => navigate('/admin/monitors/new')}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-accent text-white rounded-md hover:bg-accent-strong transition-colors"
            >
              <Plus className="w-3.5 h-3.5" />
              新建
            </button>
          </div>
        </div>

        {/* 搜索和过滤 */}
        <div className="flex flex-col sm:flex-row gap-3">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted" />
            <input
              type="text"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="搜索 provider / service / channel..."
              className="w-full pl-9 pr-3 py-2 bg-elevated border border-muted rounded-md text-primary text-sm placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-accent/50"
            />
          </div>
          <label className="inline-flex items-center gap-2 px-3 py-2 bg-elevated border border-muted rounded-md text-sm text-secondary cursor-pointer select-none">
            <input
              type="checkbox"
              checked={showDeleted}
              onChange={(e) => {
                setShowDeleted(e.target.checked);
                setPage(0);
              }}
              className="accent-accent"
            />
            显示已删除
          </label>
        </div>

        {/* 错误提示 */}
        {error && (
          <div className="p-3 bg-danger/10 border border-danger/20 rounded-md">
            <p className="text-sm text-danger">{error}</p>
          </div>
        )}

        {/* 表格 */}
        <div className="bg-surface border border-muted rounded-lg overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-muted bg-elevated/50">
                  <th className="px-4 py-3 text-left font-medium text-secondary">ID</th>
                  <th className="px-4 py-3 text-left font-medium text-secondary">标识</th>
                  <th className="px-4 py-3 text-left font-medium text-secondary">名称</th>
                  <th className="px-4 py-3 text-center font-medium text-secondary">状态</th>
                  <th className="px-4 py-3 text-center font-medium text-secondary">Key</th>
                  <th className="px-4 py-3 text-left font-medium text-secondary">更新时间</th>
                  <th className="px-4 py-3 text-right font-medium text-secondary">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-muted/50">
                {loading && items.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="px-4 py-12 text-center text-secondary">
                      加载中...
                    </td>
                  </tr>
                ) : items.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="px-4 py-12 text-center text-secondary">
                      {search ? '没有匹配的监测项' : '暂无监测项'}
                    </td>
                  </tr>
                ) : (
                  items.map((item) => {
                    const isDeleted = !!item.deleted_at;
                    const isActioning = actioningId === item.id;

                    return (
                      <tr
                        key={item.id}
                        className={`hover:bg-elevated/30 transition-colors ${
                          isDeleted ? 'opacity-50' : ''
                        }`}
                      >
                        {/* ID */}
                        <td className="px-4 py-3 text-muted tabular-nums">{item.id}</td>

                        {/* 四元组标识 */}
                        <td className="px-4 py-3">
                          <span className="font-mono text-xs text-primary">
                            {monitorKey(item)}
                          </span>
                        </td>

                        {/* 名称 */}
                        <td className="px-4 py-3 text-primary">
                          {item.name || <span className="text-muted">-</span>}
                        </td>

                        {/* 启用状态 */}
                        <td className="px-4 py-3 text-center">
                          {isDeleted ? (
                            <span className="text-xs text-danger">已删除</span>
                          ) : (
                            <button
                              onClick={() => handleToggle(item)}
                              disabled={isActioning}
                              className="inline-flex items-center gap-1 disabled:opacity-50"
                              title={item.enabled ? '点击禁用' : '点击启用'}
                            >
                              {item.enabled ? (
                                <ToggleRight className="w-5 h-5 text-success" />
                              ) : (
                                <ToggleLeft className="w-5 h-5 text-muted" />
                              )}
                            </button>
                          )}
                        </td>

                        {/* API Key */}
                        <td className="px-4 py-3 text-center">
                          {item.has_api_key === true ? (
                            <span className="text-xs text-success" title={item.api_key_masked || '已设置'}>
                              ●
                            </span>
                          ) : item.has_api_key === false ? (
                            <span className="text-xs text-muted" title="未设置">○</span>
                          ) : (
                            <span className="text-xs text-muted">-</span>
                          )}
                        </td>

                        {/* 更新时间 */}
                        <td className="px-4 py-3 text-secondary text-xs tabular-nums">
                          {formatTime(item.updated_at)}
                        </td>

                        {/* 操作 */}
                        <td className="px-4 py-3 text-right">
                          <div className="flex items-center justify-end gap-1">
                            {isDeleted ? (
                              <button
                                onClick={() => handleRestore(item.id)}
                                disabled={isActioning}
                                className="p-1.5 text-secondary hover:text-success transition-colors disabled:opacity-50"
                                title="恢复"
                              >
                                <RotateCcw className="w-3.5 h-3.5" />
                              </button>
                            ) : (
                              <>
                                <button
                                  onClick={() => navigate(`/admin/monitors/${item.id}`)}
                                  className="p-1.5 text-secondary hover:text-accent transition-colors"
                                  title="编辑"
                                >
                                  <Pencil className="w-3.5 h-3.5" />
                                </button>
                                <button
                                  onClick={() => navigate(`/admin/monitors/${item.id}/history`)}
                                  className="p-1.5 text-secondary hover:text-accent transition-colors"
                                  title="变更历史"
                                >
                                  <History className="w-3.5 h-3.5" />
                                </button>
                                <button
                                  onClick={() => handleDelete(item.id)}
                                  disabled={isActioning}
                                  className="p-1.5 text-secondary hover:text-danger transition-colors disabled:opacity-50"
                                  title="删除"
                                >
                                  <Trash2 className="w-3.5 h-3.5" />
                                </button>
                              </>
                            )}
                          </div>
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>

          {/* 分页 */}
          {total > PAGE_SIZE && (
            <div className="flex items-center justify-between px-4 py-3 border-t border-muted">
              <span className="text-xs text-secondary">
                共 {total} 条，第 {page + 1}/{totalPages} 页
              </span>
              <div className="flex items-center gap-1">
                <button
                  onClick={() => setPage((p) => Math.max(0, p - 1))}
                  disabled={page === 0}
                  className="p-1.5 text-secondary hover:text-primary disabled:opacity-30 disabled:cursor-not-allowed"
                >
                  <ChevronLeft className="w-4 h-4" />
                </button>
                <button
                  onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
                  disabled={page >= totalPages - 1}
                  className="p-1.5 text-secondary hover:text-primary disabled:opacity-30 disabled:cursor-not-allowed"
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
