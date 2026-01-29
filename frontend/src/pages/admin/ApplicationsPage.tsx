import { useState, useEffect, useCallback } from 'react';
import { Check, X, Eye, RefreshCw } from 'lucide-react';
import type { AdminApplication, ApplicationStatus } from '../../types/admin';
import {
  fetchApplications,
  approveApplication,
  rejectApplication,
  getApplicationStatusLabel,
  getApplicationStatusColor,
} from '../../api/admin';

// 状态筛选选项
const STATUS_OPTIONS: { value: ApplicationStatus | ''; label: string }[] = [
  { value: '', label: '全部状态' },
  { value: 'pending_review', label: '待审核' },
  { value: 'pending_test', label: '待测试' },
  { value: 'test_passed', label: '测试通过' },
  { value: 'test_failed', label: '测试失败' },
  { value: 'approved', label: '已通过' },
  { value: 'rejected', label: '已拒绝' },
];

// 状态徽章
function StatusBadge({ status }: { status: ApplicationStatus }) {
  const colorClass = getApplicationStatusColor(status);
  const label = getApplicationStatusLabel(status);
  return (
    <span className={`px-2 py-1 text-xs font-medium rounded ${colorClass}`}>
      {label}
    </span>
  );
}

// 拒绝对话框
function RejectDialog({
  isOpen,
  onClose,
  onConfirm,
  isLoading,
}: {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: (reason: string) => void;
  isLoading: boolean;
}) {
  const [reason, setReason] = useState('');

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-elevated rounded-lg p-6 w-full max-w-md mx-4">
        <h3 className="text-lg font-semibold text-primary mb-4">拒绝申请</h3>
        <div className="mb-4">
          <label className="block text-sm font-medium text-secondary mb-1">
            拒绝原因
          </label>
          <textarea
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="请输入拒绝原因..."
            rows={3}
            className="w-full px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary placeholder-muted focus:outline-none focus:border-accent resize-none"
          />
        </div>
        <div className="flex justify-end gap-3">
          <button
            onClick={onClose}
            disabled={isLoading}
            className="px-4 py-2 text-secondary hover:text-primary transition-colors"
          >
            取消
          </button>
          <button
            onClick={() => onConfirm(reason)}
            disabled={isLoading || !reason.trim()}
            className="px-4 py-2 bg-danger hover:bg-danger/80 text-white rounded-lg disabled:opacity-50 transition-colors"
          >
            {isLoading ? '处理中...' : '确认拒绝'}
          </button>
        </div>
      </div>
    </div>
  );
}

// 申请详情对话框
function DetailDialog({
  application,
  onClose,
}: {
  application: AdminApplication | null;
  onClose: () => void;
}) {
  if (!application) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-elevated rounded-lg p-6 w-full max-w-2xl mx-4 max-h-[80vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold text-primary">申请详情</h3>
          <button onClick={onClose} className="text-muted hover:text-primary">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="text-sm text-muted">服务商名称</label>
              <p className="text-primary font-medium">{application.provider_name}</p>
            </div>
            <div>
              <label className="text-sm text-muted">通道名称</label>
              <p className="text-primary">{application.channel_name || '-'}</p>
            </div>
            <div>
              <label className="text-sm text-muted">服务类型</label>
              <p className="text-primary">{application.service_id.toUpperCase()}</p>
            </div>
            <div>
              <label className="text-sm text-muted">供应商类型</label>
              <p className="text-primary">{application.vendor_type === 'merchant' ? '商家' : '个人'}</p>
            </div>
            <div>
              <label className="text-sm text-muted">状态</label>
              <div className="mt-1">
                <StatusBadge status={application.status} />
              </div>
            </div>
            <div>
              <label className="text-sm text-muted">申请人</label>
              <p className="text-primary">{application.applicant?.username || application.applicant_user_id}</p>
            </div>
          </div>

          <div>
            <label className="text-sm text-muted">API 端点</label>
            <p className="text-primary font-mono text-sm break-all">{application.request_url}</p>
          </div>

          {application.website_url && (
            <div>
              <label className="text-sm text-muted">官网地址</label>
              <p className="text-primary">
                <a href={application.website_url} target="_blank" rel="noopener noreferrer" className="text-accent hover:underline">
                  {application.website_url}
                </a>
              </p>
            </div>
          )}

          {application.reject_reason && (
            <div className="p-3 bg-danger/10 rounded-lg">
              <label className="text-sm text-danger font-medium">拒绝原因</label>
              <p className="text-danger mt-1">{application.reject_reason}</p>
            </div>
          )}

          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <label className="text-muted">创建时间</label>
              <p className="text-secondary">{new Date(application.created_at * 1000).toLocaleString()}</p>
            </div>
            {application.reviewed_at && (
              <div>
                <label className="text-muted">审核时间</label>
                <p className="text-secondary">{new Date(application.reviewed_at * 1000).toLocaleString()}</p>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

// 主页面组件
export default function ApplicationsPage() {
  const [applications, setApplications] = useState<AdminApplication[]>([]);
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // 筛选状态
  const [statusFilter, setStatusFilter] = useState<ApplicationStatus | ''>('');
  const [page, setPage] = useState(0);
  const pageSize = 20;

  // 对话框状态
  const [selectedApp, setSelectedApp] = useState<AdminApplication | null>(null);
  const [rejectingApp, setRejectingApp] = useState<AdminApplication | null>(null);
  const [isProcessing, setIsProcessing] = useState(false);

  // 加载数据
  const loadData = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await fetchApplications({
        status: statusFilter || undefined,
        offset: page * pageSize,
        limit: pageSize,
      });
      setApplications(res.data);
      setTotal(res.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setIsLoading(false);
    }
  }, [statusFilter, page]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // 审核通过
  const handleApprove = async (app: AdminApplication) => {
    if (!confirm(`确定要通过「${app.provider_name}」的申请吗？`)) return;

    setIsProcessing(true);
    try {
      await approveApplication(app.id);
      loadData();
    } catch (err) {
      alert(err instanceof Error ? err.message : '操作失败');
    } finally {
      setIsProcessing(false);
    }
  };

  // 审核拒绝
  const handleReject = async (reason: string) => {
    if (!rejectingApp) return;

    setIsProcessing(true);
    try {
      await rejectApplication(rejectingApp.id, { reject_reason: reason });
      setRejectingApp(null);
      loadData();
    } catch (err) {
      alert(err instanceof Error ? err.message : '操作失败');
    } finally {
      setIsProcessing(false);
    }
  };

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className="space-y-6">
      {/* 工具栏 */}
      <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center justify-between">
        <div className="flex items-center gap-4">
          {/* 状态筛选 */}
          <select
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value as ApplicationStatus | '');
              setPage(0);
            }}
            className="px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
          >
            {STATUS_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>

        <button
          onClick={loadData}
          disabled={isLoading}
          className="flex items-center gap-2 px-4 py-2 bg-surface hover:bg-elevated text-secondary hover:text-primary rounded-lg transition-colors"
        >
          <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
          刷新
        </button>
      </div>

      {/* 错误提示 */}
      {error && (
        <div className="p-4 bg-danger/10 border border-danger/30 rounded-lg text-danger">
          {error}
        </div>
      )}

      {/* 表格 */}
      <div className="bg-surface rounded-lg border border-muted/20 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-muted/10">
              <tr>
                <th className="px-4 py-3 text-left text-sm font-medium text-secondary">服务商</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-secondary">服务</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-secondary">申请人</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-secondary">状态</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-secondary">申请时间</th>
                <th className="px-4 py-3 text-right text-sm font-medium text-secondary">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-muted/10">
              {isLoading ? (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-muted">
                    <RefreshCw className="w-6 h-6 animate-spin mx-auto mb-2" />
                    加载中...
                  </td>
                </tr>
              ) : applications.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-muted">
                    暂无申请记录
                  </td>
                </tr>
              ) : (
                applications.map((app) => (
                  <tr key={app.id} className="hover:bg-muted/5">
                    <td className="px-4 py-3">
                      <div>
                        <p className="font-medium text-primary">{app.provider_name}</p>
                        {app.channel_name && (
                          <p className="text-sm text-muted">{app.channel_name}</p>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-secondary">{app.service_id.toUpperCase()}</td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        {app.applicant?.avatar_url && (
                          <img
                            src={app.applicant.avatar_url}
                            alt=""
                            className="w-6 h-6 rounded-full"
                          />
                        )}
                        <span className="text-secondary">{app.applicant?.username || '-'}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge status={app.status} />
                    </td>
                    <td className="px-4 py-3 text-sm text-muted">
                      {new Date(app.created_at * 1000).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-2">
                        <button
                          onClick={() => setSelectedApp(app)}
                          className="p-1.5 text-muted hover:text-primary hover:bg-muted/20 rounded transition-colors"
                          title="查看详情"
                        >
                          <Eye className="w-4 h-4" />
                        </button>
                        {app.status === 'pending_review' && (
                          <>
                            <button
                              onClick={() => handleApprove(app)}
                              disabled={isProcessing}
                              className="p-1.5 text-success hover:bg-success/20 rounded transition-colors"
                              title="通过"
                            >
                              <Check className="w-4 h-4" />
                            </button>
                            <button
                              onClick={() => setRejectingApp(app)}
                              disabled={isProcessing}
                              className="p-1.5 text-danger hover:bg-danger/20 rounded transition-colors"
                              title="拒绝"
                            >
                              <X className="w-4 h-4" />
                            </button>
                          </>
                        )}
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* 分页 */}
        {totalPages > 1 && (
          <div className="px-4 py-3 border-t border-muted/20 flex items-center justify-between">
            <p className="text-sm text-muted">
              共 {total} 条记录
            </p>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setPage((p) => Math.max(0, p - 1))}
                disabled={page === 0}
                className="px-3 py-1 text-sm bg-muted/20 hover:bg-muted/30 rounded disabled:opacity-50 transition-colors"
              >
                上一页
              </button>
              <span className="text-sm text-secondary">
                {page + 1} / {totalPages}
              </span>
              <button
                onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
                disabled={page >= totalPages - 1}
                className="px-3 py-1 text-sm bg-muted/20 hover:bg-muted/30 rounded disabled:opacity-50 transition-colors"
              >
                下一页
              </button>
            </div>
          </div>
        )}
      </div>

      {/* 详情对话框 */}
      <DetailDialog application={selectedApp} onClose={() => setSelectedApp(null)} />

      {/* 拒绝对话框 */}
      <RejectDialog
        isOpen={!!rejectingApp}
        onClose={() => setRejectingApp(null)}
        onConfirm={handleReject}
        isLoading={isProcessing}
      />
    </div>
  );
}
