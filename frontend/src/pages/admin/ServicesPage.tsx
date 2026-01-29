import { useState, useEffect, useCallback } from 'react';
import { Plus, Edit2, Trash2, RefreshCw } from 'lucide-react';

// 类型定义
interface Service {
  id: string;
  name: string;
  icon_svg?: string | null;
  default_template_id?: number | null;
  status: string;
  sort_order: number;
  created_at: number;
  updated_at: number;
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

async function fetchServices(): Promise<{ data: Service[] }> {
  const res = await fetch(`${API_BASE}/services`, { headers: authHeaders() });
  if (!res.ok) throw new Error('Failed to fetch services');
  return res.json();
}

async function createService(data: Partial<Service>): Promise<{ data: Service }> {
  const res = await fetch(`${API_BASE}/services`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error || 'Failed to create service');
  }
  return res.json();
}

async function updateService(id: string, data: Partial<Service>): Promise<{ data: Service }> {
  const res = await fetch(`${API_BASE}/services/${id}`, {
    method: 'PATCH',
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error || 'Failed to update service');
  }
  return res.json();
}

async function deleteService(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/services/${id}`, {
    method: 'DELETE',
    headers: authHeaders(),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error || 'Failed to delete service');
  }
}

// 服务表单对话框
function ServiceFormDialog({
  service,
  onClose,
  onSave,
}: {
  service: Service | null;
  onClose: () => void;
  onSave: () => void;
}) {
  const isEdit = !!service;
  const [formData, setFormData] = useState({
    id: service?.id || '',
    name: service?.name || '',
    status: service?.status || 'active',
    sort_order: service?.sort_order ?? 0,
    default_template_id: service?.default_template_id != null ? String(service.default_template_id) : '',
    icon_svg: service?.icon_svg || '',
  });
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setError(null);

    try {
      const defaultTemplateId = formData.default_template_id
        ? parseInt(formData.default_template_id, 10)
        : null;
      const payload: Partial<Service> = {
        name: formData.name,
        status: formData.status,
        sort_order: formData.sort_order,
        default_template_id: defaultTemplateId,
        icon_svg: formData.icon_svg || null,
      };

      if (isEdit) {
        await updateService(service.id, payload);
      } else {
        payload.id = formData.id;
        await createService(payload);
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
          {isEdit ? '编辑服务' : '创建服务'}
        </h3>

        {error && (
          <div className="mb-4 p-3 bg-danger/10 border border-danger/30 rounded text-danger text-sm">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-secondary mb-1">服务 ID *</label>
            <input
              type="text"
              value={formData.id}
              onChange={(e) => setFormData({ ...formData, id: e.target.value })}
              disabled={isEdit}
              required
              placeholder="e.g., cc / cx / gm"
              className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent disabled:opacity-50"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-secondary mb-1">服务名称 *</label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              required
              className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">排序</label>
              <input
                type="number"
                value={formData.sort_order}
                onChange={(e) => setFormData({ ...formData, sort_order: parseInt(e.target.value, 10) || 0 })}
                className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
              />
            </div>
            <div>
              <label className="flex items-center gap-2 mt-6">
                <input
                  type="checkbox"
                  checked={formData.status === 'active'}
                  onChange={(e) => setFormData({ ...formData, status: e.target.checked ? 'active' : 'disabled' })}
                  className="rounded"
                />
                <span className="text-sm text-secondary">启用</span>
              </label>
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-secondary mb-1">默认模板 ID</label>
            <input
              type="number"
              value={formData.default_template_id}
              onChange={(e) => setFormData({ ...formData, default_template_id: e.target.value })}
              placeholder="可选"
              className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-secondary mb-1">SVG 图标源码</label>
            <textarea
              value={formData.icon_svg}
              onChange={(e) => setFormData({ ...formData, icon_svg: e.target.value })}
              rows={6}
              placeholder="<svg>...</svg>"
              className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent resize-none font-mono text-xs"
            />
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

// 服务卡片
function ServiceCard({
  service,
  onEdit,
  onDelete,
}: {
  service: Service;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const isActive = service.status === 'active';

  return (
    <div className="bg-surface rounded-lg border border-muted/20 overflow-hidden">
      <div className="p-4">
        <div className="flex items-start justify-between">
          <div className="flex-1">
            <div className="flex items-center gap-2">
              <h3 className="font-medium text-primary">{service.name}</h3>
              <span className="px-2 py-0.5 text-xs bg-muted/20 text-muted rounded">{service.id}</span>
              <span
                className={`px-2 py-0.5 text-xs rounded ${
                  isActive ? 'bg-success/20 text-success' : 'bg-muted/20 text-muted'
                }`}
              >
                {isActive ? '启用' : '停用'}
              </span>
            </div>
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

        <div className="flex items-center gap-4 mt-2 text-xs text-muted">
          <span>排序: {service.sort_order}</span>
          <span>默认模板: {service.default_template_id ? `#${service.default_template_id}` : '未设置'}</span>
          <span>{service.icon_svg ? '已配置图标' : '未配置图标'}</span>
        </div>
      </div>
    </div>
  );
}

// 主页面
export default function ServicesPage() {
  const [services, setServices] = useState<Service[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [editingService, setEditingService] = useState<Service | null>(null);

  const loadData = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await fetchServices();
      setServices(res.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Load failed');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const handleDelete = async (service: Service) => {
    if (!confirm(`确定要删除服务「${service.name}」吗？`)) return;
    try {
      await deleteService(service.id);
      loadData();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Delete failed');
    }
  };

  return (
    <div className="space-y-6">
      {/* 工具栏 */}
      <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center justify-between">
        <button
          onClick={loadData}
          disabled={isLoading}
          className="flex items-center gap-2 px-4 py-2 bg-surface hover:bg-elevated text-secondary hover:text-primary rounded-lg transition-colors"
        >
          <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
          刷新
        </button>

        <button
          onClick={() => setShowForm(true)}
          className="flex items-center gap-2 px-4 py-2 bg-accent hover:bg-accent-strong text-white rounded-lg transition-colors"
        >
          <Plus className="w-4 h-4" />
          创建服务
        </button>
      </div>

      {/* 错误提示 */}
      {error && (
        <div className="p-4 bg-danger/10 border border-danger/30 rounded-lg text-danger">
          {error}
        </div>
      )}

      {/* 服务列表 */}
      {isLoading ? (
        <div className="flex items-center justify-center py-12">
          <RefreshCw className="w-6 h-6 animate-spin text-muted" />
        </div>
      ) : services.length === 0 ? (
        <div className="text-center py-12 text-muted">
          暂无服务，点击"创建服务"添加
        </div>
      ) : (
        <div className="space-y-4">
          {services.map((service) => (
            <ServiceCard
              key={service.id}
              service={service}
              onEdit={() => setEditingService(service)}
              onDelete={() => handleDelete(service)}
            />
          ))}
        </div>
      )}

      {/* 服务表单对话框 */}
      {(showForm || editingService) && (
        <ServiceFormDialog
          service={editingService}
          onClose={() => {
            setShowForm(false);
            setEditingService(null);
          }}
          onSave={loadData}
        />
      )}
    </div>
  );
}
