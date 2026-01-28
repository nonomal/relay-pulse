import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Helmet } from 'react-helmet-async';
import { Save, ArrowLeft, Plus, Trash2, Eye, EyeOff } from 'lucide-react';
import { useAdminContext } from '../../hooks/admin/useAdminContext';
import { useAdminMonitors } from '../../hooks/admin/useAdminMonitors';
import { AdminApiError } from '../../hooks/admin/useAdminAuth';
import type { ConfigPayload, MonitorConfig } from '../../types/admin';

/** 动态 Headers 键值对 */
interface HeaderEntry {
  key: string;
  value: string;
}

/** 表单状态 */
interface FormState {
  // 四元组（创建后只读）
  provider: string;
  service: string;
  channel: string;
  model: string;
  // 基本信息
  name: string;
  enabled: boolean;
  parent_key: string;
  api_key: string;
  // 请求配置
  url: string;
  method: string;
  headers: HeaderEntry[];
  body: string;
  success_contains: string;
  // 时间配置
  interval: string;
  slow_latency: string;
  timeout: string;
  retry: string;
  // 高级
  proxy: string;
  // 元数据
  category: string;
  sponsor: string;
  sponsor_url: string;
  sponsor_level: string;
  board: string;
  cold_reason: string;
}

const EMPTY_FORM: FormState = {
  provider: '',
  service: '',
  channel: '',
  model: '',
  name: '',
  enabled: true,
  parent_key: '',
  api_key: '',
  url: '',
  method: 'POST',
  headers: [{ key: 'Content-Type', value: 'application/json' }],
  body: '',
  success_contains: '',
  interval: '',
  slow_latency: '',
  timeout: '',
  retry: '',
  proxy: '',
  category: 'commercial',
  sponsor: '',
  sponsor_url: '',
  sponsor_level: '',
  board: '',
  cold_reason: '',
};

/** 将 MonitorConfig 转换为表单状态 */
function configToForm(m: MonitorConfig): FormState {
  const cfg = (m.config || {}) as ConfigPayload;
  const headers: HeaderEntry[] = cfg.headers
    ? Object.entries(cfg.headers).map(([key, value]) => ({ key, value }))
    : [];

  return {
    provider: m.provider,
    service: m.service,
    channel: m.channel,
    model: m.model,
    name: m.name || '',
    enabled: m.enabled,
    parent_key: m.parent_key || '',
    api_key: '',
    url: cfg.url || '',
    method: cfg.method || 'POST',
    headers,
    body: cfg.body || '',
    success_contains: cfg.success_contains || '',
    interval: cfg.interval || '',
    slow_latency: cfg.slow_latency || '',
    timeout: cfg.timeout || '',
    retry: cfg.retry != null ? String(cfg.retry) : '',
    proxy: cfg.proxy || '',
    category: cfg.category || 'commercial',
    sponsor: cfg.sponsor || '',
    sponsor_url: cfg.sponsor_url || '',
    sponsor_level: cfg.sponsor_level || '',
    board: cfg.board || '',
    cold_reason: cfg.cold_reason || '',
  };
}

/** 将表单状态转换为 ConfigPayload */
function formToConfig(form: FormState): ConfigPayload {
  const headers: Record<string, string> = {};
  for (const h of form.headers) {
    const k = h.key.trim();
    if (k) headers[k] = h.value;
  }

  const config: ConfigPayload = {
    url: form.url,
    method: form.method,
    category: form.category as ConfigPayload['category'],
  };

  if (Object.keys(headers).length > 0) config.headers = headers;
  if (form.body.trim()) config.body = form.body;
  if (form.success_contains.trim()) config.success_contains = form.success_contains;
  if (form.interval.trim()) config.interval = form.interval;
  if (form.slow_latency.trim()) config.slow_latency = form.slow_latency;
  if (form.timeout.trim()) config.timeout = form.timeout;
  if (form.retry.trim()) config.retry = parseInt(form.retry, 10) || undefined;
  if (form.proxy.trim()) config.proxy = form.proxy;
  if (form.sponsor.trim()) config.sponsor = form.sponsor;
  if (form.sponsor_url.trim()) config.sponsor_url = form.sponsor_url;
  if (form.sponsor_level.trim()) config.sponsor_level = form.sponsor_level;
  if (form.board.trim()) config.board = form.board;
  if (form.cold_reason.trim()) config.cold_reason = form.cold_reason;

  return config;
}

export default function MonitorEditPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { adminFetch } = useAdminContext();
  const { getMonitor, createMonitor, updateMonitor } = useAdminMonitors({ adminFetch });

  const isEdit = id != null;

  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [version, setVersion] = useState(0);
  const [hasApiKey, setHasApiKey] = useState(false);
  const [apiKeyMasked, setApiKeyMasked] = useState('');
  const [showApiKey, setShowApiKey] = useState(false);

  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [warning, setWarning] = useState<string | null>(null);

  // 加载编辑数据
  useEffect(() => {
    if (!isEdit) return;
    const monitorId = parseInt(id, 10);
    if (isNaN(monitorId)) return;

    setLoading(true);
    setError(null);
    getMonitor(monitorId)
      .then((m) => {
        setForm(configToForm(m));
        setVersion(m.version);
        setHasApiKey(m.has_api_key === true);
        setApiKeyMasked(m.api_key_masked || '');
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : '加载失败');
      })
      .finally(() => setLoading(false));
  }, [isEdit, id, getMonitor]);

  /** 更新表单字段 */
  const setField = useCallback(<K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  }, []);

  /** Headers 操作 */
  const addHeader = useCallback(() => {
    setForm((prev) => ({
      ...prev,
      headers: [...prev.headers, { key: '', value: '' }],
    }));
  }, []);

  const updateHeader = useCallback((index: number, field: 'key' | 'value', value: string) => {
    setForm((prev) => {
      const headers = [...prev.headers];
      headers[index] = { ...headers[index], [field]: value };
      return { ...prev, headers };
    });
  }, []);

  const removeHeader = useCallback((index: number) => {
    setForm((prev) => ({
      ...prev,
      headers: prev.headers.filter((_, i) => i !== index),
    }));
  }, []);

  /** 提交表单 */
  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      setError(null);
      setWarning(null);

      // 基础校验
      if (!form.provider.trim()) {
        setError('Provider 不能为空');
        return;
      }
      if (!form.service.trim()) {
        setError('Service 不能为空');
        return;
      }
      if (!form.url.trim()) {
        setError('URL 不能为空');
        return;
      }

      setSaving(true);
      try {
        const config = formToConfig(form);

        if (isEdit) {
          const req = {
            name: form.name || undefined,
            enabled: form.enabled,
            parent_key: form.parent_key || undefined,
            config,
            version,
            api_key: form.api_key || undefined,
          };
          const resp = await updateMonitor(parseInt(id, 10), req);
          if (resp.warning) setWarning(resp.warning);
          navigate('/admin/monitors');
        } else {
          const req = {
            provider: form.provider.trim(),
            service: form.service.trim(),
            channel: form.channel.trim() || undefined,
            model: form.model.trim() || undefined,
            name: form.name || undefined,
            enabled: form.enabled,
            parent_key: form.parent_key || undefined,
            config,
            api_key: form.api_key || undefined,
          };
          const resp = await createMonitor(req);
          if (resp.warning) setWarning(resp.warning);
          navigate('/admin/monitors');
        }
      } catch (err) {
        if (err instanceof AdminApiError) {
          if (err.status === 409) {
            setError('版本冲突，数据已被其他人修改，请刷新后重试');
          } else {
            setError(err.message);
          }
        } else {
          setError(err instanceof Error ? err.message : '保存失败');
        }
      } finally {
        setSaving(false);
      }
    },
    [form, isEdit, id, version, createMonitor, updateMonitor, navigate],
  );

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <p className="text-sm text-secondary">加载中...</p>
      </div>
    );
  }

  const title = isEdit ? '编辑监测项' : '新建监测项';

  return (
    <>
      <Helmet>
        <title>{title} | RP Admin</title>
      </Helmet>

      <form onSubmit={handleSubmit} className="space-y-6 max-w-4xl">
        {/* 页头 */}
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => navigate('/admin/monitors')}
            className="p-1.5 text-secondary hover:text-primary transition-colors"
          >
            <ArrowLeft className="w-5 h-5" />
          </button>
          <h1 className="text-xl font-bold text-primary flex-1">{title}</h1>
          <button
            type="submit"
            disabled={saving}
            className="inline-flex items-center gap-1.5 px-4 py-2 text-sm bg-accent text-white rounded-md hover:bg-accent-strong transition-colors disabled:opacity-50"
          >
            <Save className="w-4 h-4" />
            {saving ? '保存中...' : '保存'}
          </button>
        </div>

        {error && (
          <div className="p-3 bg-danger/10 border border-danger/20 rounded-md">
            <p className="text-sm text-danger">{error}</p>
          </div>
        )}
        {warning && (
          <div className="p-3 bg-warning/10 border border-warning/20 rounded-md">
            <p className="text-sm text-warning">{warning}</p>
          </div>
        )}

        {/* 四元组标识 */}
        <Section title="监测标识">
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <Field label="Provider" required>
              <input
                value={form.provider}
                onChange={(e) => setField('provider', e.target.value)}
                disabled={isEdit}
                required
                placeholder="例: 88code"
                className={inputClass(isEdit)}
              />
            </Field>
            <Field label="Service" required>
              <input
                value={form.service}
                onChange={(e) => setField('service', e.target.value)}
                disabled={isEdit}
                required
                placeholder="例: cc"
                className={inputClass(isEdit)}
              />
            </Field>
            <Field label="Channel">
              <input
                value={form.channel}
                onChange={(e) => setField('channel', e.target.value)}
                disabled={isEdit}
                placeholder="可选"
                className={inputClass(isEdit)}
              />
            </Field>
            <Field label="Model">
              <input
                value={form.model}
                onChange={(e) => setField('model', e.target.value)}
                disabled={isEdit}
                placeholder="可选"
                className={inputClass(isEdit)}
              />
            </Field>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-3">
            <Field label="显示名称">
              <input
                value={form.name}
                onChange={(e) => setField('name', e.target.value)}
                placeholder="可选，UI 显示用"
                className={inputClass()}
              />
            </Field>
            <Field label="状态">
              <label className="inline-flex items-center gap-2 h-[38px] cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={form.enabled}
                  onChange={(e) => setField('enabled', e.target.checked)}
                  className="accent-accent w-4 h-4"
                />
                <span className="text-sm text-primary">
                  {form.enabled ? '已启用' : '已禁用'}
                </span>
              </label>
            </Field>
          </div>
        </Section>

        {/* 请求配置 */}
        <Section title="请求配置">
          <div className="grid grid-cols-1 sm:grid-cols-4 gap-3">
            <Field label="Method" className="sm:col-span-1">
              <select
                value={form.method}
                onChange={(e) => setField('method', e.target.value)}
                className={inputClass()}
              >
                <option value="GET">GET</option>
                <option value="POST">POST</option>
                <option value="PUT">PUT</option>
                <option value="PATCH">PATCH</option>
                <option value="HEAD">HEAD</option>
              </select>
            </Field>
            <Field label="URL" required className="sm:col-span-3">
              <input
                value={form.url}
                onChange={(e) => setField('url', e.target.value)}
                required
                placeholder="https://api.example.com/v1/chat/completions"
                className={inputClass()}
              />
            </Field>
          </div>

          {/* Headers */}
          <div className="mt-3">
            <div className="flex items-center justify-between mb-1">
              <label className="text-xs font-medium text-secondary">Headers</label>
              <button
                type="button"
                onClick={addHeader}
                className="inline-flex items-center gap-1 text-xs text-accent hover:text-accent-strong transition-colors"
              >
                <Plus className="w-3 h-3" />
                添加
              </button>
            </div>
            {form.headers.length === 0 ? (
              <p className="text-xs text-muted py-2">无自定义 Headers</p>
            ) : (
              <div className="space-y-1.5">
                {form.headers.map((h, i) => (
                  <div key={i} className="flex gap-2">
                    <input
                      value={h.key}
                      onChange={(e) => updateHeader(i, 'key', e.target.value)}
                      placeholder="Header 名"
                      className={`flex-1 ${inputClass()}`}
                    />
                    <input
                      value={h.value}
                      onChange={(e) => updateHeader(i, 'value', e.target.value)}
                      placeholder="Header 值（支持 {{API_KEY}}）"
                      className={`flex-[2] ${inputClass()}`}
                    />
                    <button
                      type="button"
                      onClick={() => removeHeader(i)}
                      className="p-2 text-secondary hover:text-danger transition-colors shrink-0"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Body */}
          <Field label="Body" className="mt-3">
            <textarea
              value={form.body}
              onChange={(e) => setField('body', e.target.value)}
              rows={6}
              spellCheck={false}
              placeholder='{"model": "claude-3-opus", "messages": [...]}'
              className={`${inputClass()} font-mono resize-y`}
            />
          </Field>

          <Field label="Success Contains" className="mt-3">
            <input
              value={form.success_contains}
              onChange={(e) => setField('success_contains', e.target.value)}
              placeholder="可选，2xx 响应体须包含的关键词"
              className={inputClass()}
            />
          </Field>
        </Section>

        {/* 认证配置 */}
        <Section title="认证配置">
          {isEdit && hasApiKey && (
            <p className="text-xs text-secondary mb-2">
              当前 API Key: <span className="font-mono text-accent">{apiKeyMasked || '已设置'}</span>
              <span className="text-muted ml-2">（留空则不修改，输入新值则覆盖）</span>
            </p>
          )}
          <Field label="API Key">
            <div className="relative">
              <input
                type={showApiKey ? 'text' : 'password'}
                value={form.api_key}
                onChange={(e) => setField('api_key', e.target.value)}
                placeholder={isEdit && hasApiKey ? '留空不修改，输入新值覆盖' : 'sk-...'}
                autoComplete="off"
                className={`${inputClass()} pr-10`}
              />
              <button
                type="button"
                onClick={() => setShowApiKey(!showApiKey)}
                className="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-muted hover:text-secondary transition-colors"
              >
                {showApiKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
          </Field>
        </Section>

        {/* 时间配置 */}
        <Section title="时间配置">
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <Field label="Interval">
              <input
                value={form.interval}
                onChange={(e) => setField('interval', e.target.value)}
                placeholder="例: 1m"
                className={inputClass()}
              />
            </Field>
            <Field label="Slow Latency">
              <input
                value={form.slow_latency}
                onChange={(e) => setField('slow_latency', e.target.value)}
                placeholder="例: 5s"
                className={inputClass()}
              />
            </Field>
            <Field label="Timeout">
              <input
                value={form.timeout}
                onChange={(e) => setField('timeout', e.target.value)}
                placeholder="例: 10s"
                className={inputClass()}
              />
            </Field>
            <Field label="Retry">
              <input
                type="number"
                value={form.retry}
                onChange={(e) => setField('retry', e.target.value)}
                min={0}
                max={5}
                placeholder="0"
                className={inputClass()}
              />
            </Field>
          </div>
        </Section>

        {/* 高级配置 */}
        <Section title="高级配置">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <Field label="代理">
              <input
                value={form.proxy}
                onChange={(e) => setField('proxy', e.target.value)}
                placeholder="HTTP/SOCKS5 代理地址"
                className={inputClass()}
              />
            </Field>
            <Field label="父配置 (Parent Key)">
              <input
                value={form.parent_key}
                onChange={(e) => setField('parent_key', e.target.value)}
                placeholder="provider/service/channel"
                className={inputClass()}
              />
            </Field>
          </div>
        </Section>

        {/* 元数据 */}
        <Section title="元数据">
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
            <Field label="Category">
              <select
                value={form.category}
                onChange={(e) => setField('category', e.target.value)}
                className={inputClass()}
              >
                <option value="commercial">commercial</option>
                <option value="public">public</option>
              </select>
            </Field>
            <Field label="Board">
              <select
                value={form.board}
                onChange={(e) => setField('board', e.target.value)}
                className={inputClass()}
              >
                <option value="">不指定</option>
                <option value="hot">hot (热门)</option>
                <option value="secondary">secondary (常规)</option>
                <option value="cold">cold (冷门)</option>
              </select>
            </Field>
            <Field label="Sponsor">
              <input
                value={form.sponsor}
                onChange={(e) => setField('sponsor', e.target.value)}
                placeholder="赞助者名称"
                className={inputClass()}
              />
            </Field>
            <Field label="Sponsor URL">
              <input
                value={form.sponsor_url}
                onChange={(e) => setField('sponsor_url', e.target.value)}
                placeholder="赞助者链接"
                className={inputClass()}
              />
            </Field>
            <Field label="Sponsor Level">
              <input
                value={form.sponsor_level}
                onChange={(e) => setField('sponsor_level', e.target.value)}
                placeholder="例: gold / silver"
                className={inputClass()}
              />
            </Field>
            <Field label="Cold Reason">
              <input
                value={form.cold_reason}
                onChange={(e) => setField('cold_reason', e.target.value)}
                placeholder="冷板原因说明"
                className={inputClass()}
              />
            </Field>
          </div>
        </Section>
      </form>
    </>
  );
}

// ===== 辅助组件 =====

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="bg-surface border border-muted rounded-lg p-4 space-y-1">
      <h2 className="text-sm font-semibold text-primary mb-3">{title}</h2>
      {children}
    </div>
  );
}

function Field({
  label,
  required,
  className,
  children,
}: {
  label: string;
  required?: boolean;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div className={className}>
      <label className="block text-xs font-medium text-secondary mb-1">
        {label}
        {required && <span className="text-danger ml-0.5">*</span>}
      </label>
      {children}
    </div>
  );
}

function inputClass(disabled?: boolean): string {
  const base =
    'w-full px-3 py-2 bg-elevated border border-muted rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-accent/50';
  return disabled
    ? `${base} text-muted cursor-not-allowed`
    : `${base} text-primary placeholder:text-muted`;
}
