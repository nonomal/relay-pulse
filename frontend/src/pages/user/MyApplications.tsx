import { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { ProtectedRoute } from '../../components/auth';
import type { Application } from '../../types/application';
import {
  fetchMyApplications,
  deleteApplication,
  canEditApplication,
} from '../../api/application';

// 状态徽章
function StatusBadge({ status }: { status: Application['status'] }) {
  const { t } = useTranslation();

  const statusLabels: Record<Application['status'], string> = {
    pending_test: t('application.status.pendingTest', 'Pending Test'),
    test_passed: t('application.status.testPassed', 'Test Passed'),
    test_failed: t('application.status.testFailed', 'Test Failed'),
    pending_review: t('application.status.pendingReview', 'Pending Review'),
    approved: t('application.status.approved', 'Approved'),
    rejected: t('application.status.rejected', 'Rejected'),
  };

  const colorClasses: Record<Application['status'], string> = {
    pending_test: 'bg-muted/20 text-muted',
    test_passed: 'bg-success/20 text-success',
    test_failed: 'bg-danger/20 text-danger',
    pending_review: 'bg-warning/20 text-warning',
    approved: 'bg-success/20 text-success',
    rejected: 'bg-danger/20 text-danger',
  };

  return (
    <span className={`px-2 py-1 text-xs font-medium rounded ${colorClasses[status]}`}>
      {statusLabels[status]}
    </span>
  );
}

// 申请卡片
function ApplicationCard({
  application,
  onDelete,
}: {
  application: Application;
  onDelete: (id: number) => void;
}) {
  const { t } = useTranslation();
  const [isDeleting, setIsDeleting] = useState(false);

  const handleDelete = async () => {
    if (!confirm(t('application.confirmDelete', 'Are you sure you want to delete this application?'))) {
      return;
    }

    setIsDeleting(true);
    try {
      await deleteApplication(application.id);
      onDelete(application.id);
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to delete');
    } finally {
      setIsDeleting(false);
    }
  };

  const canEdit = canEditApplication(application);

  return (
    <div className="bg-surface rounded-lg p-4 border border-muted/20">
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <h3 className="font-medium text-primary">{application.provider_name}</h3>
            {application.channel_name && (
              <span className="text-sm text-muted">/ {application.channel_name}</span>
            )}
          </div>
          <p className="text-sm text-secondary mt-1">
            {application.service_id.toUpperCase()}
          </p>
        </div>
        <StatusBadge status={application.status} />
      </div>

      <div className="mt-3 text-sm text-muted">
        <p className="truncate">{application.request_url}</p>
      </div>

      {application.reject_reason && (
        <div className="mt-3 p-2 bg-danger/10 rounded text-sm text-danger">
          {t('application.rejectReason', 'Reason')}: {application.reject_reason}
        </div>
      )}

      <div className="mt-4 flex items-center justify-between">
        <span className="text-xs text-muted">
          {t('application.createdAt', 'Created')}: {new Date(application.created_at * 1000).toLocaleDateString()}
        </span>

        <div className="flex items-center gap-2">
          {canEdit && (
            <>
              <a
                href={`/user/applications/${application.id}/edit`}
                className="px-3 py-1 text-sm text-accent hover:text-accent-strong transition-colors"
              >
                {t('common.edit', 'Edit')}
              </a>
              <button
                onClick={handleDelete}
                disabled={isDeleting}
                className="px-3 py-1 text-sm text-danger hover:text-danger/80 transition-colors disabled:opacity-50"
              >
                {isDeleting ? t('common.deleting', 'Deleting...') : t('common.delete', 'Delete')}
              </button>
            </>
          )}
          {application.status === 'test_passed' && (
            <a
              href={`/user/applications/${application.id}/submit`}
              className="px-3 py-1 text-sm bg-accent hover:bg-accent-strong text-white rounded transition-colors"
            >
              {t('application.submitForReview', 'Submit')}
            </a>
          )}
        </div>
      </div>
    </div>
  );
}

// 空状态
function EmptyState() {
  const { t } = useTranslation();

  return (
    <div className="text-center py-12">
      <div className="w-16 h-16 mx-auto mb-4 rounded-full bg-muted/20 flex items-center justify-center">
        <svg className="w-8 h-8 text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
          />
        </svg>
      </div>
      <h3 className="text-lg font-medium text-primary">
        {t('application.noApplications', 'No applications yet')}
      </h3>
      <p className="mt-1 text-secondary">
        {t('application.noApplicationsDesc', 'Submit your first monitoring application to get started.')}
      </p>
      <a
        href="/user/applications/new"
        className="inline-block mt-4 px-6 py-2 bg-accent hover:bg-accent-strong text-white rounded-lg transition-colors"
      >
        {t('application.createNew', 'Create Application')}
      </a>
    </div>
  );
}

// 主内容组件
function MyApplicationsContent() {
  const { t } = useTranslation();
  const [applications, setApplications] = useState<Application[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // 加载申请列表
  useEffect(() => {
    fetchMyApplications()
      .then((res) => setApplications(res.data))
      .catch((err) => setError(err.message))
      .finally(() => setIsLoading(false));
  }, []);

  // 删除申请后更新列表
  const handleDelete = useCallback((id: number) => {
    setApplications((prev) => prev.filter((app) => app.id !== id));
  }, []);

  return (
    <div className="min-h-screen bg-page">
      <div className="max-w-4xl mx-auto px-4 py-8">
        {/* 标题栏 */}
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold text-primary">
              {t('application.myApplications', 'My Applications')}
            </h1>
            <p className="mt-1 text-secondary">
              {t('application.myApplicationsDesc', 'Manage your monitoring applications')}
            </p>
          </div>
          <a
            href="/user/applications/new"
            className="px-4 py-2 bg-accent hover:bg-accent-strong text-white rounded-lg transition-colors"
          >
            {t('application.createNew', 'New Application')}
          </a>
        </div>

        {/* 错误提示 */}
        {error && (
          <div className="mb-6 p-4 bg-danger/10 border border-danger/30 rounded-lg text-danger">
            {error}
          </div>
        )}

        {/* 内容 */}
        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <div className="animate-spin rounded-full h-8 w-8 border-2 border-accent border-t-transparent" />
          </div>
        ) : applications.length === 0 ? (
          <EmptyState />
        ) : (
          <div className="space-y-4">
            {applications.map((app) => (
              <ApplicationCard key={app.id} application={app} onDelete={handleDelete} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// 导出带保护的页面组件
export default function MyApplications() {
  return (
    <ProtectedRoute>
      <MyApplicationsContent />
    </ProtectedRoute>
  );
}
