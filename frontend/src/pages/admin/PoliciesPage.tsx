import { useState, useEffect, useCallback } from 'react';
import { Helmet } from 'react-helmet-async';
import { Plus, Trash2, RefreshCw } from 'lucide-react';
import { useAdminContext } from '../../hooks/admin/useAdminContext';
import { AdminApiError } from '../../hooks/admin/useAdminAuth';
import type {
  ProviderPolicy,
  CreateProviderPolicyRequest,
} from '../../types/admin';

export default function PoliciesPage() {
  const { adminFetch } = useAdminContext();
  const [policies, setPolicies] = useState<ProviderPolicy[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);

  const fetchPolicies = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const resp = await adminFetch<{ data: ProviderPolicy[] }>('/api/admin/policies');
      setPolicies(resp.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : '请求失败');
    } finally {
      setLoading(false);
    }
  }, [adminFetch]);

  useEffect(() => {
    fetchPolicies();
  }, [fetchPolicies]);

  const handleDelete = useCallback(
    async (id: number) => {
      if (!window.confirm('确定要删除此策略吗？')) return;
      try {
        await adminFetch(`/api/admin/policies/${id}`, { method: 'DELETE' });
        fetchPolicies();
      } catch (err) {
        if (err instanceof AdminApiError) {
          alert(err.message);
        }
      }
    },
    [adminFetch, fetchPolicies],
  );

  const handleCreate = useCallback(
    async (req: CreateProviderPolicyRequest) => {
      try {
        await adminFetch('/api/admin/policies', {
          method: 'POST',
          body: JSON.stringify(req),
        });
        setShowForm(false);
        fetchPolicies();
      } catch (err) {
        if (err instanceof AdminApiError) {
          alert(err.message);
        }
      }
    },
    [adminFetch, fetchPolicies],
  );

  return (
    <>
      <Helmet>
        <title>Provider 策略 | RP Admin</title>
      </Helmet>

      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold text-primary">Provider 策略</h1>
          <div className="flex items-center gap-2">
            <button
              onClick={fetchPolicies}
              disabled={loading}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-elevated border border-muted rounded-md text-secondary hover:text-primary transition-colors"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
              刷新
            </button>
            <button
              onClick={() => setShowForm(!showForm)}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-accent text-white rounded-md hover:bg-accent-strong transition-colors"
            >
              <Plus className="w-3.5 h-3.5" />
              新建
            </button>
          </div>
        </div>

        {/* 创建表单 */}
        {showForm && (
          <PolicyForm onSubmit={handleCreate} onCancel={() => setShowForm(false)} />
        )}

        {error && (
          <div className="p-3 bg-danger/10 border border-danger/20 rounded-md">
            <p className="text-sm text-danger">{error}</p>
          </div>
        )}

        {/* 列表 */}
        <div className="bg-surface border border-muted rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-muted bg-elevated/50">
                <th className="px-4 py-3 text-left font-medium text-secondary">ID</th>
                <th className="px-4 py-3 text-left font-medium text-secondary">类型</th>
                <th className="px-4 py-3 text-left font-medium text-secondary">Provider</th>
                <th className="px-4 py-3 text-left font-medium text-secondary">原因</th>
                <th className="px-4 py-3 text-right font-medium text-secondary">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-muted/50">
              {policies.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-4 py-12 text-center text-secondary">
                    {loading ? '加载中...' : '暂无策略'}
                  </td>
                </tr>
              ) : (
                policies.map((p) => (
                  <tr key={p.id} className="hover:bg-elevated/30 transition-colors">
                    <td className="px-4 py-3 text-muted tabular-nums">{p.id}</td>
                    <td className="px-4 py-3">
                      <PolicyTypeBadge type={p.policy_type} />
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-primary">{p.provider}</td>
                    <td className="px-4 py-3 text-secondary text-xs">{p.reason || '-'}</td>
                    <td className="px-4 py-3 text-right">
                      <button
                        onClick={() => handleDelete(p.id)}
                        className="p-1.5 text-secondary hover:text-danger transition-colors"
                        title="删除"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </>
  );
}

/** 策略类型标签 */
function PolicyTypeBadge({ type }: { type: string }) {
  const styles: Record<string, string> = {
    disabled: 'bg-danger/15 text-danger',
    hidden: 'bg-warning/15 text-warning',
    risk: 'bg-accent/15 text-accent',
  };
  return (
    <span className={`px-2 py-0.5 rounded text-xs font-medium ${styles[type] || 'bg-muted/15 text-muted'}`}>
      {type}
    </span>
  );
}

/** 创建策略表单 */
function PolicyForm({
  onSubmit,
  onCancel,
}: {
  onSubmit: (req: CreateProviderPolicyRequest) => void;
  onCancel: () => void;
}) {
  const [policyType, setPolicyType] = useState<'disabled' | 'hidden' | 'risk'>('disabled');
  const [provider, setProvider] = useState('');
  const [reason, setReason] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!provider.trim()) return;
    onSubmit({
      policy_type: policyType,
      provider: provider.trim(),
      reason: reason.trim() || undefined,
    });
  };

  return (
    <form onSubmit={handleSubmit} className="bg-surface border border-muted rounded-lg p-4 space-y-3">
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">类型</label>
          <select
            value={policyType}
            onChange={(e) => setPolicyType(e.target.value as 'disabled' | 'hidden' | 'risk')}
            className="w-full px-3 py-2 bg-elevated border border-muted rounded-md text-primary text-sm focus:outline-none focus:ring-2 focus:ring-accent/50"
          >
            <option value="disabled">disabled</option>
            <option value="hidden">hidden</option>
            <option value="risk">risk</option>
          </select>
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Provider</label>
          <input
            value={provider}
            onChange={(e) => setProvider(e.target.value)}
            placeholder="provider 名称"
            required
            className="w-full px-3 py-2 bg-elevated border border-muted rounded-md text-primary text-sm placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-accent/50"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">原因</label>
          <input
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="可选"
            className="w-full px-3 py-2 bg-elevated border border-muted rounded-md text-primary text-sm placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-accent/50"
          />
        </div>
      </div>
      <div className="flex justify-end gap-2">
        <button
          type="button"
          onClick={onCancel}
          className="px-3 py-1.5 text-sm text-secondary hover:text-primary transition-colors"
        >
          取消
        </button>
        <button
          type="submit"
          className="px-3 py-1.5 text-sm bg-accent text-white rounded-md hover:bg-accent-strong transition-colors"
        >
          创建
        </button>
      </div>
    </form>
  );
}
