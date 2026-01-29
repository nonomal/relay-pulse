import { useState, useEffect, useCallback } from 'react';
import { Plus, Edit2, Trash2, RefreshCw, ChevronDown, ChevronRight } from 'lucide-react';

// 类型定义
interface TemplateModel {
  id: number;
  template_id: number;
  model_key: string;
  display_name: string;
  enabled: boolean;
  sort_order: number;
}

interface Template {
  id: number;
  service_id: string;
  name: string;
  slug: string;
  description?: string;
  is_default: boolean;
  request_method: string;
  timeout_ms: number;
  slow_latency_ms: number;
  models?: TemplateModel[];
  created_at: number;
  updated_at: number;
}

interface Service {
  id: string;
  name: string;
}

// API 函数
const API_BASE = '/api/v1/admin';

function getToken(): string | null {
  return localStorage.getItem('relay-pulse-token');
}

function authHeaders(): HeadersInit {
  const token = getToken();
  return {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
}

async function fetchTemplates(serviceId?: string): Promise<{ data: Template[]; total: number }> {
  const params = new URLSearchParams();
  if (serviceId) params.set('service_id', serviceId);
  params.set('with_models', 'true');
  const res = await fetch(`${API_BASE}/templates?${params}`, { headers: authHeaders() });
  if (!res.ok) throw new Error('Failed to fetch templates');
  return res.json();
}

async function fetchServices(): Promise<{ data: Service[] }> {
  const res = await fetch('/api/v1/public/services');
  if (!res.ok) throw new Error('Failed to fetch services');
  return res.json();
}

async function createTemplate(data: Partial<Template>): Promise<{ data: Template }> {
  const res = await fetch(`${API_BASE}/templates`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error || 'Failed to create template');
  }
  return res.json();
}

async function updateTemplate(id: number, data: Partial<Template>): Promise<{ data: Template }> {
  const res = await fetch(`${API_BASE}/templates/${id}`, {
    method: 'PATCH',
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error || 'Failed to update template');
  }
  return res.json();
}

async function deleteTemplate(id: number): Promise<void> {
  const res = await fetch(`${API_BASE}/templates/${id}`, {
    method: 'DELETE',
    headers: authHeaders(),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error || 'Failed to delete template');
  }
}

async function createTemplateModel(templateId: number, data: Partial<TemplateModel>): Promise<{ data: TemplateModel }> {
  const res = await fetch(`${API_BASE}/templates/${templateId}/models`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error || 'Failed to create model');
  }
  return res.json();
}

async function updateTemplateModel(templateId: number, modelId: number, data: Partial<TemplateModel>): Promise<{ data: TemplateModel }> {
  const res = await fetch(`${API_BASE}/templates/${templateId}/models/${modelId}`, {
    method: 'PATCH',
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error || 'Failed to update model');
  }
  return res.json();
}

async function deleteTemplateModel(templateId: number, modelId: number): Promise<void> {
  const res = await fetch(`${API_BASE}/templates/${templateId}/models/${modelId}`, {
    method: 'DELETE',
    headers: authHeaders(),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error || 'Failed to delete model');
  }
}

// 模板表单对话框
function TemplateFormDialog({
  template,
  services,
  onClose,
  onSave,
}: {
  template: Template | null;
  services: Service[];
  onClose: () => void;
  onSave: () => void;
}) {
  const isEdit = !!template;
  const [formData, setFormData] = useState({
    service_id: template?.service_id || '',
    name: template?.name || '',
    slug: template?.slug || '',
    description: template?.description || '',
    is_default: template?.is_default || false,
    request_method: template?.request_method || 'POST',
    timeout_ms: template?.timeout_ms || 10000,
    slow_latency_ms: template?.slow_latency_ms || 5000,
  });
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setError(null);

    try {
      if (isEdit) {
        await updateTemplate(template.id, formData);
      } else {
        await createTemplate(formData);
      }
      onSave();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Operation failed');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-elevated rounded-lg p-6 w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto">
        <h3 className="text-lg font-semibold text-primary mb-4">
          {isEdit ? '编辑模板' : '创建模板'}
        </h3>

        {error && (
          <div className="mb-4 p-3 bg-danger/10 border border-danger/30 rounded text-danger text-sm">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-secondary mb-1">服务 *</label>
            <select
              value={formData.service_id}
              onChange={(e) => setFormData({ ...formData, service_id: e.target.value })}
              disabled={isEdit}
              required
              className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent disabled:opacity-50"
            >
              <option value="">选择服务</option>
              {services.map((s) => (
                <option key={s.id} value={s.id}>{s.name} ({s.id})</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-secondary mb-1">模板名称 *</label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              required
              className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-secondary mb-1">Slug *</label>
            <input
              type="text"
              value={formData.slug}
              onChange={(e) => setFormData({ ...formData, slug: e.target.value })}
              required
              placeholder="e.g., chat-completions"
              className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-secondary mb-1">描述</label>
            <textarea
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              rows={2}
              className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent resize-none"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">请求方法</label>
              <select
                value={formData.request_method}
                onChange={(e) => setFormData({ ...formData, request_method: e.target.value })}
                className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
              >
                <option value="POST">POST</option>
                <option value="GET">GET</option>
              </select>
            </div>
            <div>
              <label className="flex items-center gap-2 mt-6">
                <input
                  type="checkbox"
                  checked={formData.is_default}
                  onChange={(e) => setFormData({ ...formData, is_default: e.target.checked })}
                  className="rounded"
                />
                <span className="text-sm text-secondary">设为默认模板</span>
              </label>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">超时 (ms)</label>
              <input
                type="number"
                value={formData.timeout_ms}
                onChange={(e) => setFormData({ ...formData, timeout_ms: parseInt(e.target.value) || 10000 })}
                className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">慢响应阈值 (ms)</label>
              <input
                type="number"
                value={formData.slow_latency_ms}
                onChange={(e) => setFormData({ ...formData, slow_latency_ms: parseInt(e.target.value) || 5000 })}
                className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
              />
            </div>
          </div>

          <div className="flex justify-end gap-3 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-secondary hover:text-primary transition-colors"
            >
              取消
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="px-4 py-2 bg-accent hover:bg-accent-strong text-white rounded-lg disabled:opacity-50 transition-colors"
            >
              {isSubmitting ? '保存中...' : '保存'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// 模型表单对话框
function ModelFormDialog({
  templateId,
  model,
  onClose,
  onSave,
}: {
  templateId: number;
  model: TemplateModel | null;
  onClose: () => void;
  onSave: () => void;
}) {
  const isEdit = !!model;
  const [formData, setFormData] = useState({
    model_key: model?.model_key || '',
    display_name: model?.display_name || '',
    enabled: model?.enabled ?? true,
    sort_order: model?.sort_order || 0,
  });
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setError(null);

    try {
      if (isEdit) {
        await updateTemplateModel(templateId, model.id, formData);
      } else {
        await createTemplateModel(templateId, formData);
      }
      onSave();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Operation failed');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-elevated rounded-lg p-6 w-full max-w-md mx-4">
        <h3 className="text-lg font-semibold text-primary mb-4">
          {isEdit ? '编辑模型' : '添加模型'}
        </h3>

        {error && (
          <div className="mb-4 p-3 bg-danger/10 border border-danger/30 rounded text-danger text-sm">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-secondary mb-1">模型标识 *</label>
            <input
              type="text"
              value={formData.model_key}
              onChange={(e) => setFormData({ ...formData, model_key: e.target.value })}
              required
              placeholder="e.g., gpt-4, claude-3-opus"
              className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-secondary mb-1">显示名称 *</label>
            <input
              type="text"
              value={formData.display_name}
              onChange={(e) => setFormData({ ...formData, display_name: e.target.value })}
              required
              placeholder="e.g., GPT-4, Claude 3 Opus"
              className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">排序</label>
              <input
                type="number"
                value={formData.sort_order}
                onChange={(e) => setFormData({ ...formData, sort_order: parseInt(e.target.value) || 0 })}
                className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
              />
            </div>
            <div>
              <label className="flex items-center gap-2 mt-6">
                <input
                  type="checkbox"
                  checked={formData.enabled}
                  onChange={(e) => setFormData({ ...formData, enabled: e.target.checked })}
                  className="rounded"
                />
                <span className="text-sm text-secondary">启用</span>
              </label>
            </div>
          </div>

          <div className="flex justify-end gap-3 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-secondary hover:text-primary transition-colors"
            >
              取消
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="px-4 py-2 bg-accent hover:bg-accent-strong text-white rounded-lg disabled:opacity-50 transition-colors"
            >
              {isSubmitting ? '保存中...' : '保存'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// 模板卡片
function TemplateCard({
  template,
  onEdit,
  onDelete,
  onRefresh,
}: {
  template: Template;
  onEdit: () => void;
  onDelete: () => void;
  onRefresh: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const [editingModel, setEditingModel] = useState<TemplateModel | null>(null);
  const [showModelForm, setShowModelForm] = useState(false);

  const handleDeleteModel = async (model: TemplateModel) => {
    if (!confirm(`确定要删除模型「${model.display_name}」吗？`)) return;
    try {
      await deleteTemplateModel(template.id, model.id);
      onRefresh();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Delete failed');
    }
  };

  return (
    <div className="bg-surface rounded-lg border border-muted/20 overflow-hidden">
      {/* 模板头部 */}
      <div className="p-4">
        <div className="flex items-start justify-between">
          <div className="flex-1">
            <div className="flex items-center gap-2">
              <button
                onClick={() => setExpanded(!expanded)}
                className="text-muted hover:text-primary"
              >
                {expanded ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
              </button>
              <h3 className="font-medium text-primary">{template.name}</h3>
              {template.is_default && (
                <span className="px-2 py-0.5 text-xs bg-accent/20 text-accent rounded">默认</span>
              )}
            </div>
            <p className="text-sm text-muted mt-1 ml-6">
              {template.service_id.toUpperCase()} / {template.slug}
            </p>
            {template.description && (
              <p className="text-sm text-secondary mt-1 ml-6">{template.description}</p>
            )}
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={onEdit}
              className="p-1.5 text-muted hover:text-primary hover:bg-muted/20 rounded transition-colors"
              title="编辑"
            >
              <Edit2 className="w-4 h-4" />
            </button>
            <button
              onClick={onDelete}
              className="p-1.5 text-muted hover:text-danger hover:bg-danger/20 rounded transition-colors"
              title="删除"
            >
              <Trash2 className="w-4 h-4" />
            </button>
          </div>
        </div>

        <div className="flex items-center gap-4 mt-2 ml-6 text-xs text-muted">
          <span>{template.request_method}</span>
          <span>超时: {template.timeout_ms}ms</span>
          <span>慢响应: {template.slow_latency_ms}ms</span>
          <span>{template.models?.length || 0} 个模型</span>
        </div>
      </div>

      {/* 模型列表 */}
      {expanded && (
        <div className="border-t border-muted/20 p-4 bg-muted/5">
          <div className="flex items-center justify-between mb-3">
            <h4 className="text-sm font-medium text-secondary">模型配置</h4>
            <button
              onClick={() => setShowModelForm(true)}
              className="flex items-center gap-1 px-2 py-1 text-xs text-accent hover:bg-accent/20 rounded transition-colors"
            >
              <Plus className="w-3 h-3" />
              添加模型
            </button>
          </div>

          {template.models && template.models.length > 0 ? (
            <div className="space-y-2">
              {template.models.map((model) => (
                <div
                  key={model.id}
                  className="flex items-center justify-between p-2 bg-surface rounded border border-muted/20"
                >
                  <div className="flex items-center gap-3">
                    <span className={`w-2 h-2 rounded-full ${model.enabled ? 'bg-success' : 'bg-muted'}`} />
                    <div>
                      <span className="text-sm text-primary">{model.display_name}</span>
                      <span className="text-xs text-muted ml-2">({model.model_key})</span>
                    </div>
                  </div>
                  <div className="flex items-center gap-1">
                    <button
                      onClick={() => setEditingModel(model)}
                      className="p-1 text-muted hover:text-primary hover:bg-muted/20 rounded"
                    >
                      <Edit2 className="w-3 h-3" />
                    </button>
                    <button
                      onClick={() => handleDeleteModel(model)}
                      className="p-1 text-muted hover:text-danger hover:bg-danger/20 rounded"
                    >
                      <Trash2 className="w-3 h-3" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted text-center py-4">暂无模型配置</p>
          )}
        </div>
      )}

      {/* 模型表单对话框 */}
      {(showModelForm || editingModel) && (
        <ModelFormDialog
          templateId={template.id}
          model={editingModel}
          onClose={() => {
            setShowModelForm(false);
            setEditingModel(null);
          }}
          onSave={onRefresh}
        />
      )}
    </div>
  );
}

// 主页面
export default function TemplatesPage() {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [services, setServices] = useState<Service[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [serviceFilter, setServiceFilter] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<Template | null>(null);

  const loadData = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const [templatesRes, servicesRes] = await Promise.all([
        fetchTemplates(serviceFilter || undefined),
        fetchServices(),
      ]);
      setTemplates(templatesRes.data);
      setServices(servicesRes.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Load failed');
    } finally {
      setIsLoading(false);
    }
  }, [serviceFilter]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const handleDelete = async (template: Template) => {
    if (!confirm(`确定要删除模板「${template.name}」吗？这将同时删除所有关联的模型配置。`)) return;
    try {
      await deleteTemplate(template.id);
      loadData();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Delete failed');
    }
  };

  return (
    <div className="space-y-6">
      {/* 工具栏 */}
      <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center justify-between">
        <div className="flex items-center gap-4">
          <select
            value={serviceFilter}
            onChange={(e) => setServiceFilter(e.target.value)}
            className="px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
          >
            <option value="">全部服务</option>
            {services.map((s) => (
              <option key={s.id} value={s.id}>{s.name}</option>
            ))}
          </select>

          <button
            onClick={loadData}
            disabled={isLoading}
            className="flex items-center gap-2 px-4 py-2 bg-surface hover:bg-elevated text-secondary hover:text-primary rounded-lg transition-colors"
          >
            <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
            刷新
          </button>
        </div>

        <button
          onClick={() => setShowForm(true)}
          className="flex items-center gap-2 px-4 py-2 bg-accent hover:bg-accent-strong text-white rounded-lg transition-colors"
        >
          <Plus className="w-4 h-4" />
          创建模板
        </button>
      </div>

      {/* 错误提示 */}
      {error && (
        <div className="p-4 bg-danger/10 border border-danger/30 rounded-lg text-danger">
          {error}
        </div>
      )}

      {/* 模板列表 */}
      {isLoading ? (
        <div className="flex items-center justify-center py-12">
          <RefreshCw className="w-6 h-6 animate-spin text-muted" />
        </div>
      ) : templates.length === 0 ? (
        <div className="text-center py-12 text-muted">
          暂无模板，点击"创建模板"添加
        </div>
      ) : (
        <div className="space-y-4">
          {templates.map((template) => (
            <TemplateCard
              key={template.id}
              template={template}
              onEdit={() => setEditingTemplate(template)}
              onDelete={() => handleDelete(template)}
              onRefresh={loadData}
            />
          ))}
        </div>
      )}

      {/* 模板表单对话框 */}
      {(showForm || editingTemplate) && (
        <TemplateFormDialog
          template={editingTemplate}
          services={services}
          onClose={() => {
            setShowForm(false);
            setEditingTemplate(null);
          }}
          onSave={loadData}
        />
      )}
    </div>
  );
}
