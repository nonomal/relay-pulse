import { useState, useCallback } from 'react';
import type { AdminFetchFn } from './useAdminAuth';
import type {
  MonitorConfig,
  MonitorListParams,
  AdminListResponse,
  AdminItemResponse,
  CreateMonitorRequest,
  UpdateMonitorRequest,
  BatchMonitorsRequest,
  MonitorConfigAudit,
  AuditListParams,
  ImportResult,
} from '../../types/admin';

interface UseAdminMonitorsOptions {
  adminFetch: AdminFetchFn;
}

interface MonitorListState {
  items: MonitorConfig[];
  total: number;
  loading: boolean;
  error: string | null;
}

/**
 * 监测项管理 API Hook
 *
 * 提供监测项的 CRUD、批量操作和审计历史查询。
 */
export function useAdminMonitors({ adminFetch }: UseAdminMonitorsOptions) {
  const [listState, setListState] = useState<MonitorListState>({
    items: [],
    total: 0,
    loading: false,
    error: null,
  });

  /** 构建查询字符串 */
  const buildQuery = (params: MonitorListParams): string => {
    const qs = new URLSearchParams();
    if (params.provider) qs.set('provider', params.provider);
    if (params.service) qs.set('service', params.service);
    if (params.channel) qs.set('channel', params.channel);
    if (params.model) qs.set('model', params.model);
    if (params.search) qs.set('search', params.search);
    if (params.enabled !== undefined) qs.set('enabled', String(params.enabled));
    if (params.include_deleted) qs.set('include_deleted', 'true');
    if (params.offset !== undefined) qs.set('offset', String(params.offset));
    if (params.limit !== undefined) qs.set('limit', String(params.limit));
    const str = qs.toString();
    return str ? `?${str}` : '';
  };

  /** 列出监测项 */
  const listMonitors = useCallback(
    async (params: MonitorListParams = {}) => {
      setListState((s) => ({ ...s, loading: true, error: null }));
      try {
        const query = buildQuery(params);
        const resp = await adminFetch<AdminListResponse<MonitorConfig>>(
          `/api/admin/monitors${query}`,
        );
        setListState({
          items: resp.data,
          total: resp.total,
          loading: false,
          error: null,
        });
        return resp;
      } catch (err) {
        const msg = err instanceof Error ? err.message : '请求失败';
        setListState((s) => ({ ...s, loading: false, error: msg }));
        throw err;
      }
    },
    [adminFetch],
  );

  /** 获取单个监测项详情 */
  const getMonitor = useCallback(
    async (id: number) => {
      const resp = await adminFetch<AdminItemResponse<MonitorConfig>>(
        `/api/admin/monitors/${id}`,
      );
      return resp.data;
    },
    [adminFetch],
  );

  /** 创建监测项 */
  const createMonitor = useCallback(
    async (req: CreateMonitorRequest) => {
      const resp = await adminFetch<AdminItemResponse<MonitorConfig>>(
        '/api/admin/monitors',
        {
          method: 'POST',
          body: JSON.stringify(req),
        },
      );
      return resp;
    },
    [adminFetch],
  );

  /** 更新监测项 */
  const updateMonitor = useCallback(
    async (id: number, req: UpdateMonitorRequest) => {
      const resp = await adminFetch<AdminItemResponse<MonitorConfig>>(
        `/api/admin/monitors/${id}`,
        {
          method: 'PUT',
          body: JSON.stringify(req),
        },
      );
      return resp;
    },
    [adminFetch],
  );

  /** 切换启用/禁用 */
  const toggleMonitorStatus = useCallback(
    async (id: number, enabled: boolean) => {
      await adminFetch(`/api/admin/monitors/${id}/status`, {
        method: 'PATCH',
        body: JSON.stringify({ enabled }),
      });
    },
    [adminFetch],
  );

  /** 软删除监测项 */
  const deleteMonitor = useCallback(
    async (id: number) => {
      await adminFetch(`/api/admin/monitors/${id}`, {
        method: 'DELETE',
      });
    },
    [adminFetch],
  );

  /** 恢复已删除的监测项 */
  const restoreMonitor = useCallback(
    async (id: number) => {
      await adminFetch(`/api/admin/monitors/${id}/restore`, {
        method: 'POST',
      });
    },
    [adminFetch],
  );

  /** 批量操作 */
  const batchMonitors = useCallback(
    async (req: BatchMonitorsRequest) => {
      await adminFetch('/api/admin/monitors/batch', {
        method: 'POST',
        body: JSON.stringify(req),
      });
    },
    [adminFetch],
  );

  /** 获取审计历史 */
  const getMonitorHistory = useCallback(
    async (id: number) => {
      const resp = await adminFetch<{ data: MonitorConfigAudit[] }>(
        `/api/admin/monitors/${id}/history`,
      );
      return resp.data;
    },
    [adminFetch],
  );

  /** 获取全局审计日志 */
  const listAudits = useCallback(
    async (params: AuditListParams = {}) => {
      const qs = new URLSearchParams();
      if (params.provider) qs.set('provider', params.provider);
      if (params.service) qs.set('service', params.service);
      if (params.action) qs.set('action', params.action);
      if (params.since !== undefined) qs.set('since', String(params.since));
      if (params.until !== undefined) qs.set('until', String(params.until));
      if (params.offset !== undefined) qs.set('offset', String(params.offset));
      if (params.limit !== undefined) qs.set('limit', String(params.limit));
      const query = qs.toString();
      const resp = await adminFetch<AdminListResponse<MonitorConfigAudit>>(
        `/api/admin/audits${query ? `?${query}` : ''}`,
      );
      return resp;
    },
    [adminFetch],
  );

  /** 导入 YAML 配置（使用 FormData 上传文件） */
  const importConfig = useCallback(
    async (file: File) => {
      const formData = new FormData();
      formData.append('file', file);
      // 不设置 Content-Type，让浏览器自动设置 multipart/form-data boundary
      const resp = await adminFetch<ImportResult>('/api/admin/import', {
        method: 'POST',
        body: formData,
        headers: {}, // 清空默认 headers，避免设置错误的 Content-Type
      });
      return resp;
    },
    [adminFetch],
  );

  return {
    ...listState,
    listMonitors,
    getMonitor,
    createMonitor,
    updateMonitor,
    toggleMonitorStatus,
    deleteMonitor,
    restoreMonitor,
    batchMonitors,
    getMonitorHistory,
    listAudits,
    importConfig,
  };
}
