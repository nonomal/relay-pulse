import { useState, useCallback, useEffect, useMemo } from 'react';
import { apiGet, apiPost, apiDelete, ApiError } from '../utils/apiClient';
import type { AdminChangeRequest, ChangeRequestStatus } from '../types/change';

export function useChangeAdmin(token: string) {
  const [changes, setChanges] = useState<AdminChangeRequest[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<ChangeRequestStatus | 'all'>('all');
  const [selectedChange, setSelectedChange] = useState<AdminChangeRequest | null>(null);

  const headers = useMemo(
    (): Record<string, string> => (token ? { Authorization: `Bearer ${token}` } : {}),
    [token],
  );

  const fetchList = useCallback(async () => {
    if (!token) return;
    setIsLoading(true);
    setError(null);
    try {
      const params = statusFilter !== 'all' ? `?status=${statusFilter}` : '';
      const resp = await apiGet<{ changes: AdminChangeRequest[]; total: number }>(`/api/admin/changes${params}`, { headers });
      setChanges(resp.changes || []);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to load change requests');
    } finally {
      setIsLoading(false);
    }
  }, [token, statusFilter, headers]);

  useEffect(() => {
    fetchList();
  }, [fetchList]);

  const fetchDetail = useCallback(async (id: string) => {
    if (!token) return;
    setError(null);
    try {
      const resp = await apiGet<{ change: AdminChangeRequest; new_key?: string }>(`/api/admin/changes/${id}`, { headers });
      setSelectedChange(resp.change);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to load change request detail');
    }
  }, [token, headers]);

  const approveChange = useCallback(async (id: string, note?: string) => {
    if (!token) return;
    setError(null);
    try {
      await apiPost(`/api/admin/changes/${id}/approve`, { note }, { headers });
      await fetchList();
      setSelectedChange(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to approve');
    }
  }, [token, headers, fetchList]);

  const rejectChange = useCallback(async (id: string, note: string) => {
    if (!token) return;
    setError(null);
    try {
      await apiPost(`/api/admin/changes/${id}/reject`, { note }, { headers });
      await fetchList();
      setSelectedChange(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to reject');
    }
  }, [token, headers, fetchList]);

  const applyChange = useCallback(async (id: string) => {
    if (!token) return;
    setError(null);
    try {
      await apiPost(`/api/admin/changes/${id}/apply`, {}, { headers });
      await fetchList();
      setSelectedChange(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to apply');
    }
  }, [token, headers, fetchList]);

  const deleteChange = useCallback(async (id: string) => {
    if (!token) return;
    setError(null);
    try {
      await apiDelete(`/api/admin/changes/${id}`, { headers });
      await fetchList();
      setSelectedChange(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to delete');
    }
  }, [token, headers, fetchList]);

  return {
    changes,
    isLoading,
    error,
    statusFilter,
    setStatusFilter,
    selectedChange,
    setSelectedChange,
    fetchList,
    fetchDetail,
    approveChange,
    rejectChange,
    applyChange,
    deleteChange,
  };
}
