import { useState, useCallback, useEffect } from 'react';
import { apiGet, apiPost, apiPut, apiDelete, ApiError } from '../utils/apiClient';
import type {
  MonitorSummary,
  MonitorFile,
  AdminMonitorListResponse,
  AdminMonitorDetailResponse,
} from '../types/monitor';

export function useMonitorAdmin(token: string) {
  const [monitors, setMonitors] = useState<MonitorSummary[]>([]);
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Filters
  const [boardFilter, setBoardFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [searchQuery, setSearchQuery] = useState('');

  // Detail
  const [selectedMonitor, setSelectedMonitor] = useState<MonitorFile | null>(null);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);

  const authHeaders = useCallback((): HeadersInit => ({
    Authorization: `Bearer ${token}`,
  }), [token]);

  // Fetch list
  const fetchList = useCallback(async () => {
    if (!token) return;
    setIsLoading(true);
    setError(null);

    try {
      const params = new URLSearchParams();
      if (boardFilter) params.set('board', boardFilter);
      if (statusFilter) params.set('status', statusFilter);
      if (searchQuery) params.set('q', searchQuery);

      const qs = params.toString();
      const resp = await apiGet<AdminMonitorListResponse>(
        `/api/admin/monitors${qs ? '?' + qs : ''}`,
        { headers: authHeaders() },
      );
      setMonitors(resp.monitors || []);
      setTotal(resp.total);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : '加载失败');
    } finally {
      setIsLoading(false);
    }
  }, [token, boardFilter, statusFilter, searchQuery, authHeaders]);

  useEffect(() => {
    if (token) fetchList();
  }, [token, fetchList]);

  // Fetch detail
  const fetchDetail = useCallback(async (key: string) => {
    if (!token) return;
    setError(null);

    try {
      const resp = await apiGet<AdminMonitorDetailResponse>(
        `/api/admin/monitors/${key}`,
        { headers: authHeaders() },
      );
      setSelectedMonitor(resp.monitor);
      setSelectedKey(key);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : '加载详情失败');
    }
  }, [token, authHeaders]);

  // Create
  const createMonitor = useCallback(async (file: MonitorFile) => {
    if (!token) return;
    setError(null);

    try {
      await apiPost<AdminMonitorDetailResponse>(
        '/api/admin/monitors',
        file,
        { headers: authHeaders() },
      );
      fetchList();
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : '创建失败';
      setError(msg);
      throw e;
    }
  }, [token, authHeaders, fetchList]);

  // Update
  const updateMonitor = useCallback(async (key: string, file: MonitorFile, revision: number) => {
    if (!token) return;
    setError(null);

    try {
      const resp = await apiPut<{ monitor: MonitorFile }>(
        `/api/admin/monitors/${key}`,
        { revision, monitor: file },
        { headers: authHeaders() },
      );
      setSelectedMonitor(resp.monitor);
      fetchList();
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : '更新失败';
      setError(msg);
      throw e;
    }
  }, [token, authHeaders, fetchList]);

  // Delete
  const deleteMonitor = useCallback(async (key: string) => {
    if (!token) return;
    setError(null);

    try {
      await apiDelete(`/api/admin/monitors/${key}`, { headers: authHeaders() });
      setSelectedMonitor(null);
      setSelectedKey(null);
      fetchList();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : '删除失败');
    }
  }, [token, authHeaders, fetchList]);

  // Toggle
  const toggleMonitor = useCallback(async (key: string, field: 'disabled' | 'hidden', value: boolean) => {
    if (!token) return;
    setError(null);

    try {
      const resp = await apiPost<{ monitor: MonitorFile }>(
        `/api/admin/monitors/${key}/toggle`,
        { field, value },
        { headers: authHeaders() },
      );
      setSelectedMonitor(resp.monitor);
      fetchList();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : '切换失败');
    }
  }, [token, authHeaders, fetchList]);

  // Probe
  const probeMonitor = useCallback(async (key: string): Promise<{ jobId: string } | null> => {
    if (!token) return null;
    setError(null);

    try {
      const resp = await apiPost<{ job_id: string; status: string }>(
        `/api/admin/monitors/${key}/probe`,
        {},
        { headers: authHeaders() },
      );
      return { jobId: resp.job_id };
    } catch (e) {
      setError(e instanceof ApiError ? e.message : '测试创建失败');
      return null;
    }
  }, [token, authHeaders]);

  return {
    monitors,
    total,
    isLoading,
    error,

    boardFilter,
    setBoardFilter,
    statusFilter,
    setStatusFilter,
    searchQuery,
    setSearchQuery,
    fetchList,

    selectedMonitor,
    selectedKey,
    setSelectedMonitor,
    setSelectedKey,
    fetchDetail,
    createMonitor,
    updateMonitor,
    deleteMonitor,
    toggleMonitor,
    probeMonitor,
  };
}
