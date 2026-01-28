import { useState, useEffect, useCallback } from 'react';
import { Helmet } from 'react-helmet-async';
import { Plus, Trash2, RefreshCw } from 'lucide-react';
import { useAdminContext } from '../../hooks/admin/useAdminContext';
import { AdminApiError } from '../../hooks/admin/useAdminAuth';
import type {
  BadgeDefinition,
  BadgeBinding,
  CreateBadgeDefinitionRequest,
  CreateBadgeBindingRequest,
  BadgeScope,
} from '../../types/admin';

export default function BadgesPage() {
  const { adminFetch } = useAdminContext();

  // Badge 定义
  const [definitions, setDefinitions] = useState<BadgeDefinition[]>([]);
  const [defLoading, setDefLoading] = useState(false);
  const [defError, setDefError] = useState<string | null>(null);
  const [showDefForm, setShowDefForm] = useState(false);

  // Badge 绑定
  const [bindings, setBindings] = useState<BadgeBinding[]>([]);
  const [bindLoading, setBindLoading] = useState(false);
  const [bindError, setBindError] = useState<string | null>(null);
  const [showBindForm, setShowBindForm] = useState(false);

  // 当前 Tab
  const [tab, setTab] = useState<'definitions' | 'bindings'>('definitions');

  const fetchDefinitions = useCallback(async () => {
    setDefLoading(true);
    setDefError(null);
    try {
      const resp = await adminFetch<{ data: BadgeDefinition[] }>('/api/admin/badges/definitions');
      setDefinitions(resp.data);
    } catch (err) {
      setDefError(err instanceof Error ? err.message : '请求失败');
    } finally {
      setDefLoading(false);
    }
  }, [adminFetch]);

  const fetchBindings = useCallback(async () => {
    setBindLoading(true);
    setBindError(null);
    try {
      const resp = await adminFetch<{ data: BadgeBinding[] }>('/api/admin/badges/bindings');
      setBindings(resp.data);
    } catch (err) {
      setBindError(err instanceof Error ? err.message : '请求失败');
    } finally {
      setBindLoading(false);
    }
  }, [adminFetch]);

  useEffect(() => {
    fetchDefinitions();
    fetchBindings();
  }, [fetchDefinitions, fetchBindings]);

  const handleDeleteDef = useCallback(
    async (id: string) => {
      if (!window.confirm(`确定删除 Badge "${id}" 吗？`)) return;
      try {
        await adminFetch(`/api/admin/badges/definitions/${encodeURIComponent(id)}`, {
          method: 'DELETE',
        });
        fetchDefinitions();
      } catch (err) {
        if (err instanceof AdminApiError) alert(err.message);
      }
    },
    [adminFetch, fetchDefinitions],
  );

  const handleDeleteBinding = useCallback(
    async (id: number) => {
      if (!window.confirm('确定删除此绑定吗？')) return;
      try {
        await adminFetch(`/api/admin/badges/bindings/${id}`, { method: 'DELETE' });
        fetchBindings();
      } catch (err) {
        if (err instanceof AdminApiError) alert(err.message);
      }
    },
    [adminFetch, fetchBindings],
  );

  const handleCreateDef = useCallback(
    async (req: CreateBadgeDefinitionRequest) => {
      try {
        await adminFetch('/api/admin/badges/definitions', {
          method: 'POST',
          body: JSON.stringify(req),
        });
        setShowDefForm(false);
        fetchDefinitions();
      } catch (err) {
        if (err instanceof AdminApiError) alert(err.message);
      }
    },
    [adminFetch, fetchDefinitions],
  );

  const handleCreateBinding = useCallback(
    async (req: CreateBadgeBindingRequest) => {
      try {
        await adminFetch('/api/admin/badges/bindings', {
          method: 'POST',
          body: JSON.stringify(req),
        });
        setShowBindForm(false);
        fetchBindings();
      } catch (err) {
        if (err instanceof AdminApiError) alert(err.message);
      }
    },
    [adminFetch, fetchBindings],
  );

  const isLoading = tab === 'definitions' ? defLoading : bindLoading;
  const currentError = tab === 'definitions' ? defError : bindError;
  const refresh = tab === 'definitions' ? fetchDefinitions : fetchBindings;

  return (
    <>
      <Helmet>
        <title>Badge 管理 | RP Admin</title>
      </Helmet>

      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold text-primary">Badge 管理</h1>
          <div className="flex items-center gap-2">
            <button
              onClick={refresh}
              disabled={isLoading}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-elevated border border-muted rounded-md text-secondary hover:text-primary transition-colors"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${isLoading ? 'animate-spin' : ''}`} />
              刷新
            </button>
            <button
              onClick={() =>
                tab === 'definitions'
                  ? setShowDefForm(!showDefForm)
                  : setShowBindForm(!showBindForm)
              }
              className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-accent text-white rounded-md hover:bg-accent-strong transition-colors"
            >
              <Plus className="w-3.5 h-3.5" />
              新建
            </button>
          </div>
        </div>

        {/* Tab 切换 */}
        <div className="flex gap-1 bg-elevated rounded-md p-1">
          {(['definitions', 'bindings'] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`flex-1 px-3 py-1.5 rounded text-sm font-medium transition-colors ${
                tab === t
                  ? 'bg-surface text-primary shadow-sm'
                  : 'text-secondary hover:text-primary'
              }`}
            >
              {t === 'definitions' ? 'Badge 定义' : 'Badge 绑定'}
            </button>
          ))}
        </div>

        {/* 创建表单 */}
        {tab === 'definitions' && showDefForm && (
          <DefForm onSubmit={handleCreateDef} onCancel={() => setShowDefForm(false)} />
        )}
        {tab === 'bindings' && showBindForm && (
          <BindForm
            definitions={definitions}
            onSubmit={handleCreateBinding}
            onCancel={() => setShowBindForm(false)}
          />
        )}

        {currentError && (
          <div className="p-3 bg-danger/10 border border-danger/20 rounded-md">
            <p className="text-sm text-danger">{currentError}</p>
          </div>
        )}

        {/* 定义列表 */}
        {tab === 'definitions' && (
          <div className="bg-surface border border-muted rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-muted bg-elevated/50">
                  <th className="px-4 py-3 text-left font-medium text-secondary">ID</th>
                  <th className="px-4 py-3 text-left font-medium text-secondary">Kind</th>
                  <th className="px-4 py-3 text-left font-medium text-secondary">Label</th>
                  <th className="px-4 py-3 text-center font-medium text-secondary">Weight</th>
                  <th className="px-4 py-3 text-right font-medium text-secondary">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-muted/50">
                {definitions.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="px-4 py-12 text-center text-secondary">
                      {defLoading ? '加载中...' : '暂无 Badge 定义'}
                    </td>
                  </tr>
                ) : (
                  definitions.map((d) => (
                    <tr key={d.id} className="hover:bg-elevated/30 transition-colors">
                      <td className="px-4 py-3 font-mono text-xs text-primary">{d.id}</td>
                      <td className="px-4 py-3">
                        <span className="px-2 py-0.5 rounded text-xs font-medium bg-accent/15 text-accent">
                          {d.kind}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-primary text-xs">
                        {d.label_i18n?.['zh-CN'] || d.label_i18n?.['en-US'] || JSON.stringify(d.label_i18n)}
                      </td>
                      <td className="px-4 py-3 text-center text-muted tabular-nums">{d.weight}</td>
                      <td className="px-4 py-3 text-right">
                        <button
                          onClick={() => handleDeleteDef(d.id)}
                          className="p-1.5 text-secondary hover:text-danger transition-colors"
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
        )}

        {/* 绑定列表 */}
        {tab === 'bindings' && (
          <div className="bg-surface border border-muted rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-muted bg-elevated/50">
                  <th className="px-4 py-3 text-left font-medium text-secondary">ID</th>
                  <th className="px-4 py-3 text-left font-medium text-secondary">Badge</th>
                  <th className="px-4 py-3 text-left font-medium text-secondary">Scope</th>
                  <th className="px-4 py-3 text-left font-medium text-secondary">目标</th>
                  <th className="px-4 py-3 text-right font-medium text-secondary">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-muted/50">
                {bindings.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="px-4 py-12 text-center text-secondary">
                      {bindLoading ? '加载中...' : '暂无 Badge 绑定'}
                    </td>
                  </tr>
                ) : (
                  bindings.map((b) => (
                    <tr key={b.id} className="hover:bg-elevated/30 transition-colors">
                      <td className="px-4 py-3 text-muted tabular-nums">{b.id}</td>
                      <td className="px-4 py-3 font-mono text-xs text-primary">{b.badge_id}</td>
                      <td className="px-4 py-3">
                        <span className="px-2 py-0.5 rounded text-xs font-medium bg-accent/15 text-accent">
                          {b.scope}
                        </span>
                      </td>
                      <td className="px-4 py-3 font-mono text-xs text-secondary">
                        {b.scope === 'global'
                          ? '*'
                          : [b.provider, b.service, b.channel].filter(Boolean).join(' / ')}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          onClick={() => handleDeleteBinding(b.id)}
                          className="p-1.5 text-secondary hover:text-danger transition-colors"
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
        )}
      </div>
    </>
  );
}

/** Badge 定义创建表单 */
function DefForm({
  onSubmit,
  onCancel,
}: {
  onSubmit: (req: CreateBadgeDefinitionRequest) => void;
  onCancel: () => void;
}) {
  const [id, setId] = useState('');
  const [kind, setKind] = useState<'sponsor' | 'risk' | 'feature' | 'info'>('info');
  const [weight] = useState(0);
  const [labelZh, setLabelZh] = useState('');
  const [labelEn, setLabelEn] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!id.trim() || !labelZh.trim()) return;
    const labelI18n: Record<string, string> = { 'zh-CN': labelZh.trim() };
    if (labelEn.trim()) labelI18n['en-US'] = labelEn.trim();
    onSubmit({ id: id.trim(), kind, weight, label_i18n: labelI18n });
  };

  return (
    <form onSubmit={handleSubmit} className="bg-surface border border-muted rounded-lg p-4 space-y-3">
      <div className="grid grid-cols-1 sm:grid-cols-4 gap-3">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Badge ID</label>
          <input
            value={id}
            onChange={(e) => setId(e.target.value)}
            required
            className="w-full px-3 py-2 bg-elevated border border-muted rounded-md text-primary text-sm focus:outline-none focus:ring-2 focus:ring-accent/50"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Kind</label>
          <select
            value={kind}
            onChange={(e) => setKind(e.target.value as typeof kind)}
            className="w-full px-3 py-2 bg-elevated border border-muted rounded-md text-primary text-sm focus:outline-none focus:ring-2 focus:ring-accent/50"
          >
            <option value="info">info</option>
            <option value="feature">feature</option>
            <option value="risk">risk</option>
            <option value="sponsor">sponsor</option>
          </select>
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">中文 Label</label>
          <input
            value={labelZh}
            onChange={(e) => setLabelZh(e.target.value)}
            required
            className="w-full px-3 py-2 bg-elevated border border-muted rounded-md text-primary text-sm focus:outline-none focus:ring-2 focus:ring-accent/50"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">English Label</label>
          <input
            value={labelEn}
            onChange={(e) => setLabelEn(e.target.value)}
            className="w-full px-3 py-2 bg-elevated border border-muted rounded-md text-primary text-sm focus:outline-none focus:ring-2 focus:ring-accent/50"
          />
        </div>
      </div>
      <div className="flex justify-end gap-2">
        <button type="button" onClick={onCancel} className="px-3 py-1.5 text-sm text-secondary hover:text-primary">
          取消
        </button>
        <button type="submit" className="px-3 py-1.5 text-sm bg-accent text-white rounded-md hover:bg-accent-strong">
          创建
        </button>
      </div>
    </form>
  );
}

/** Badge 绑定创建表单 */
function BindForm({
  definitions,
  onSubmit,
  onCancel,
}: {
  definitions: BadgeDefinition[];
  onSubmit: (req: CreateBadgeBindingRequest) => void;
  onCancel: () => void;
}) {
  const [badgeId, setBadgeId] = useState('');
  const [scope, setScope] = useState<BadgeScope>('global');
  const [provider, setProvider] = useState('');
  const [service, setService] = useState('');
  const [channel, setChannel] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!badgeId) return;
    onSubmit({
      badge_id: badgeId,
      scope,
      provider: scope !== 'global' ? provider.trim() : undefined,
      service: scope === 'service' || scope === 'channel' ? service.trim() : undefined,
      channel: scope === 'channel' ? channel.trim() : undefined,
    });
  };

  return (
    <form onSubmit={handleSubmit} className="bg-surface border border-muted rounded-lg p-4 space-y-3">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Badge</label>
          <select
            value={badgeId}
            onChange={(e) => setBadgeId(e.target.value)}
            required
            className="w-full px-3 py-2 bg-elevated border border-muted rounded-md text-primary text-sm focus:outline-none focus:ring-2 focus:ring-accent/50"
          >
            <option value="">选择 Badge</option>
            {definitions.map((d) => (
              <option key={d.id} value={d.id}>
                {d.id} ({d.kind})
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Scope</label>
          <select
            value={scope}
            onChange={(e) => setScope(e.target.value as BadgeScope)}
            className="w-full px-3 py-2 bg-elevated border border-muted rounded-md text-primary text-sm focus:outline-none focus:ring-2 focus:ring-accent/50"
          >
            <option value="global">global</option>
            <option value="provider">provider</option>
            <option value="service">service</option>
            <option value="channel">channel</option>
          </select>
        </div>
      </div>
      {scope !== 'global' && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <input
            value={provider}
            onChange={(e) => setProvider(e.target.value)}
            placeholder="Provider"
            required
            className="px-3 py-2 bg-elevated border border-muted rounded-md text-primary text-sm focus:outline-none focus:ring-2 focus:ring-accent/50"
          />
          {(scope === 'service' || scope === 'channel') && (
            <input
              value={service}
              onChange={(e) => setService(e.target.value)}
              placeholder="Service"
              required
              className="px-3 py-2 bg-elevated border border-muted rounded-md text-primary text-sm focus:outline-none focus:ring-2 focus:ring-accent/50"
            />
          )}
          {scope === 'channel' && (
            <input
              value={channel}
              onChange={(e) => setChannel(e.target.value)}
              placeholder="Channel"
              required
              className="px-3 py-2 bg-elevated border border-muted rounded-md text-primary text-sm focus:outline-none focus:ring-2 focus:ring-accent/50"
            />
          )}
        </div>
      )}
      <div className="flex justify-end gap-2">
        <button type="button" onClick={onCancel} className="px-3 py-1.5 text-sm text-secondary hover:text-primary">
          取消
        </button>
        <button type="submit" className="px-3 py-1.5 text-sm bg-accent text-white rounded-md hover:bg-accent-strong">
          创建
        </button>
      </div>
    </form>
  );
}
