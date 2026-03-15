import { useTranslation } from 'react-i18next';
import type { MonitorSummary } from '../../types/monitor';

interface MonitorListProps {
  monitors: MonitorSummary[];
  total: number;
  isLoading: boolean;
  boardFilter: string;
  setBoardFilter: (v: string) => void;
  statusFilter: string;
  setStatusFilter: (v: string) => void;
  searchQuery: string;
  setSearchQuery: (v: string) => void;
  onSelect: (key: string) => void;
  onRefresh: () => void;
}

export function MonitorList({
  monitors, total, isLoading,
  boardFilter, setBoardFilter,
  statusFilter, setStatusFilter,
  searchQuery, setSearchQuery,
  onSelect, onRefresh,
}: MonitorListProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-4">
      {/* 过滤栏 */}
      <div className="flex flex-wrap gap-3 items-center">
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder={t('admin.monitors.searchPlaceholder')}
          className="flex-1 min-w-[200px] px-3 py-2 rounded-lg bg-elevated border border-default text-primary text-sm focus:outline-none focus:border-accent"
        />

        <select
          value={boardFilter}
          onChange={(e) => setBoardFilter(e.target.value)}
          className="px-3 py-2 rounded-lg bg-elevated border border-default text-primary text-sm"
        >
          <option value="">{t('admin.monitors.allBoards')}</option>
          <option value="hot">Hot</option>
          <option value="secondary">Secondary</option>
          <option value="cold">Cold</option>
        </select>

        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="px-3 py-2 rounded-lg bg-elevated border border-default text-primary text-sm"
        >
          <option value="">{t('admin.monitors.allStatus')}</option>
          <option value="active">{t('admin.monitors.statusActive')}</option>
          <option value="disabled">{t('admin.monitors.statusDisabled')}</option>
          <option value="hidden">{t('admin.monitors.statusHidden')}</option>
        </select>

        <button
          onClick={onRefresh}
          className="px-3 py-2 rounded-lg border border-default text-secondary hover:text-primary text-sm transition"
        >
          {t('admin.monitors.refresh')}
        </button>
      </div>

      {/* 统计 */}
      <div className="text-sm text-muted">
        {t('admin.monitors.total', { count: total })}
      </div>

      {/* 表格 */}
      {isLoading ? (
        <div className="text-center py-8 text-muted">{t('admin.table.loading')}</div>
      ) : monitors.length === 0 ? (
        <div className="text-center py-8 text-muted">{t('admin.table.empty')}</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-default text-left text-muted">
                <th className="py-2 px-3">{t('admin.monitors.colProvider')}</th>
                <th className="py-2 px-3">{t('admin.monitors.colService')}</th>
                <th className="py-2 px-3">{t('admin.monitors.colChannel')}</th>
                <th className="py-2 px-3">{t('admin.monitors.colModels')}</th>
                <th className="py-2 px-3">{t('admin.monitors.colBoard')}</th>
                <th className="py-2 px-3">{t('admin.monitors.colStatus')}</th>
                <th className="py-2 px-3">{t('admin.monitors.colSource')}</th>
              </tr>
            </thead>
            <tbody>
              {monitors.map((m) => (
                <tr
                  key={m.key}
                  onClick={() => onSelect(m.key)}
                  className="border-b border-default/50 hover:bg-elevated/50 cursor-pointer transition"
                >
                  <td className="py-2.5 px-3 text-primary font-medium">{m.provider}</td>
                  <td className="py-2.5 px-3 text-secondary">{m.service}</td>
                  <td className="py-2.5 px-3 text-secondary">{m.channel}</td>
                  <td className="py-2.5 px-3 text-muted">{m.model_count}</td>
                  <td className="py-2.5 px-3">
                    <BoardBadge board={m.board} />
                  </td>
                  <td className="py-2.5 px-3">
                    <StatusBadge disabled={m.disabled} hidden={m.hidden} />
                  </td>
                  <td className="py-2.5 px-3 text-muted text-xs">{m.source}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function BoardBadge({ board }: { board: string }) {
  const { t } = useTranslation();
  const colors: Record<string, string> = {
    hot: 'bg-success/20 text-success',
    secondary: 'bg-warning/20 text-warning',
    cold: 'bg-muted/20 text-muted',
  };
  const labels: Record<string, string> = {
    hot: t('admin.monitors.boardHot'),
    secondary: t('admin.monitors.boardSecondary'),
    cold: t('admin.monitors.boardCold'),
  };
  return (
    <span className={`px-2 py-0.5 rounded text-xs font-medium ${colors[board] || 'bg-muted/10 text-muted'}`}>
      {labels[board] || labels.hot}
    </span>
  );
}

function StatusBadge({ disabled, hidden }: { disabled: boolean; hidden: boolean }) {
  const { t } = useTranslation();
  if (disabled) {
    return <span className="px-2 py-0.5 rounded text-xs font-medium bg-danger/20 text-danger">{t('admin.monitors.statusDisabled')}</span>;
  }
  if (hidden) {
    return <span className="px-2 py-0.5 rounded text-xs font-medium bg-warning/20 text-warning">{t('admin.monitors.statusHidden')}</span>;
  }
  return <span className="px-2 py-0.5 rounded text-xs font-medium bg-success/20 text-success">{t('admin.monitors.statusActive')}</span>;
}
