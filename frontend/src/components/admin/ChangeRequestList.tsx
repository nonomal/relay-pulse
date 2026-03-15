import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Loader2, AlertCircle, ChevronDown, ChevronUp, Check, X, Play, Trash2 } from 'lucide-react';
import type { AdminChangeRequest, ChangeRequestStatus } from '../../types/change';

interface ChangeRequestListProps {
  changes: AdminChangeRequest[];
  isLoading: boolean;
  statusFilter: ChangeRequestStatus | 'all';
  setStatusFilter: (f: ChangeRequestStatus | 'all') => void;
  onSelect: (id: string) => void;
  onApprove: (id: string) => void;
  onReject: (id: string, note: string) => void;
  onApply: (id: string) => void;
  onDelete: (id: string) => void;
  error: string | null;
}

const STATUS_FILTERS: (ChangeRequestStatus | 'all')[] = ['all', 'pending', 'approved', 'rejected', 'applied'];

export function ChangeRequestList({
  changes,
  isLoading,
  statusFilter,
  setStatusFilter,
  onApprove,
  onReject,
  onApply,
  onDelete,
  error,
}: ChangeRequestListProps) {
  const { t } = useTranslation();
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [rejectNote, setRejectNote] = useState('');
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);

  const statusLabel = (status: string) => {
    const map: Record<string, string> = {
      pending: t('admin.changes.statusPending'),
      approved: t('admin.changes.statusApproved'),
      rejected: t('admin.changes.statusRejected'),
      applied: t('admin.changes.statusApplied'),
    };
    return map[status] || status;
  };

  const statusColor = (status: string) => {
    switch (status) {
      case 'pending': return 'text-warning';
      case 'approved': return 'text-accent';
      case 'rejected': return 'text-danger';
      case 'applied': return 'text-success';
      default: return 'text-muted';
    }
  };

  return (
    <div className="space-y-4">
      {/* Status filter */}
      <div className="flex gap-1 flex-wrap">
        {STATUS_FILTERS.map(f => (
          <button
            key={f}
            onClick={() => setStatusFilter(f)}
            className={`px-3 py-1.5 text-xs rounded-lg transition ${
              statusFilter === f
                ? 'bg-accent/20 text-accent font-medium'
                : 'bg-elevated text-muted hover:text-secondary'
            }`}
          >
            {f === 'all' ? t('admin.filter.all') : statusLabel(f)}
          </button>
        ))}
      </div>

      {error && (
        <div className="flex items-center gap-2 p-3 rounded-lg bg-danger/10 text-danger text-sm">
          <AlertCircle size={16} />
          <span>{error}</span>
        </div>
      )}

      {isLoading ? (
        <div className="flex items-center justify-center py-8 text-muted">
          <Loader2 size={20} className="animate-spin mr-2" />
          {t('admin.table.loading')}
        </div>
      ) : changes.length === 0 ? (
        <div className="text-center py-8 text-muted">{t('admin.changes.empty')}</div>
      ) : (
        <div className="space-y-2">
          {changes.map(cr => {
            const isExpanded = expandedId === cr.public_id;
            let proposedChanges: Record<string, string> = {};
            try { proposedChanges = JSON.parse(cr.proposed_changes); } catch { /* ignore */ }

            return (
              <div key={cr.public_id} className="rounded-xl border border-default bg-surface overflow-hidden">
                {/* Row header */}
                <button
                  onClick={() => setExpandedId(isExpanded ? null : cr.public_id)}
                  className="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-elevated/50 transition"
                >
                  <code className="text-xs text-muted font-mono">{cr.public_id.slice(0, 8)}</code>
                  <span className="text-sm text-primary font-medium flex-1 truncate">{cr.target_key}</span>
                  <span className={`text-xs px-2 py-0.5 rounded-md bg-muted/20 ${statusColor(cr.status)}`}>
                    {statusLabel(cr.status)}
                  </span>
                  <span className="text-xs text-muted">
                    {cr.apply_mode === 'auto' ? t('admin.changes.modeAuto') : t('admin.changes.modeManual')}
                  </span>
                  <span className="text-xs text-muted">
                    {new Date(cr.created_at * 1000).toLocaleDateString()}
                  </span>
                  {isExpanded ? <ChevronUp size={14} className="text-muted" /> : <ChevronDown size={14} className="text-muted" />}
                </button>

                {/* Expanded detail */}
                {isExpanded && (
                  <div className="px-4 pb-4 border-t border-default/50 space-y-3">
                    {/* Proposed changes */}
                    <div className="mt-3">
                      <div className="text-xs font-medium text-muted mb-1">{t('admin.changes.proposedChanges')}</div>
                      <div className="space-y-1">
                        {Object.entries(proposedChanges).map(([k, v]) => (
                          <div key={k} className="flex gap-2 text-sm">
                            <span className="text-muted min-w-[100px]">{k}:</span>
                            <span className="text-primary font-medium">{v}</span>
                          </div>
                        ))}
                      </div>
                    </div>

                    {/* New API Key indicator */}
                    {cr.new_key_last4 && (
                      <div className="text-sm">
                        <span className="text-muted">{t('admin.changes.newApiKey')}:</span>{' '}
                        <span className="text-primary">...{cr.new_key_last4}</span>
                      </div>
                    )}

                    {/* Test info */}
                    {cr.requires_test && cr.test_passed_at && (
                      <div className="text-xs text-muted">
                        {t('admin.changes.testInfo')}: {cr.test_latency_ms}ms / HTTP {cr.test_http_code}
                      </div>
                    )}

                    {/* Admin note */}
                    {cr.admin_note && (
                      <div className="text-xs text-secondary italic">{cr.admin_note}</div>
                    )}

                    {/* Actions */}
                    <div className="flex gap-2 pt-2">
                      {cr.status === 'pending' && (
                        <>
                          <button
                            onClick={() => onApprove(cr.public_id)}
                            className="flex items-center gap-1 px-3 py-1.5 text-xs rounded-lg bg-accent/10 text-accent hover:bg-accent/20 transition"
                          >
                            <Check size={12} />{t('admin.changes.approve')}
                          </button>
                          <div className="flex items-center gap-1">
                            <input
                              type="text"
                              value={rejectNote}
                              onChange={e => setRejectNote(e.target.value)}
                              placeholder={t('admin.changes.rejectNotePlaceholder')}
                              className="px-2 py-1 text-xs rounded-lg bg-elevated border border-default text-primary w-48"
                            />
                            <button
                              onClick={() => { onReject(cr.public_id, rejectNote); setRejectNote(''); }}
                              className="flex items-center gap-1 px-3 py-1.5 text-xs rounded-lg bg-danger/10 text-danger hover:bg-danger/20 transition"
                            >
                              <X size={12} />{t('admin.changes.reject')}
                            </button>
                          </div>
                        </>
                      )}
                      {cr.status === 'approved' && cr.apply_mode === 'auto' && (
                        <button
                          onClick={() => onApply(cr.public_id)}
                          className="flex items-center gap-1 px-3 py-1.5 text-xs rounded-lg bg-success/10 text-success hover:bg-success/20 transition"
                        >
                          <Play size={12} />{t('admin.changes.apply')}
                        </button>
                      )}
                      {confirmDeleteId === cr.public_id ? (
                        <div className="flex items-center gap-1">
                          <span className="text-xs text-danger">{t('admin.changes.confirmDelete')}</span>
                          <button
                            onClick={() => { onDelete(cr.public_id); setConfirmDeleteId(null); }}
                            className="px-2 py-1 text-xs rounded bg-danger text-white"
                          >
                            {t('admin.changes.delete')}
                          </button>
                          <button
                            onClick={() => setConfirmDeleteId(null)}
                            className="px-2 py-1 text-xs rounded border border-default text-muted"
                          >
                            {t('common.cancel')}
                          </button>
                        </div>
                      ) : (
                        <button
                          onClick={() => setConfirmDeleteId(cr.public_id)}
                          className="flex items-center gap-1 px-3 py-1.5 text-xs rounded-lg text-muted hover:text-danger transition ml-auto"
                        >
                          <Trash2 size={12} />{t('admin.changes.delete')}
                        </button>
                      )}
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
