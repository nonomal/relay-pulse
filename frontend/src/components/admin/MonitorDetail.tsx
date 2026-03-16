import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { MonitorConfig, MonitorFile } from '../../types/monitor';
import type { ProbeResult } from '../../hooks/useMonitorAdmin';

interface MonitorDetailProps {
  fetchTemplates: () => Promise<string[]>;
  monitorFile: MonitorFile;
  monitorKey: string;
  onBack: () => void;
  onSave: (file: MonitorFile, revision: number) => Promise<void>;
  onDelete: () => void;
  onToggle: (field: 'disabled' | 'hidden', value: boolean) => void;
  onProbe: () => Promise<ProbeResult | null>;
  isProbing?: boolean;
  probeResult?: ProbeResult | null;
  probeError?: string | null;
}

type EditableFields = Pick<MonitorConfig,
  'provider_name' | 'channel_name' | 'provider_url' | 'template' | 'base_url' | 'api_key' | 'proxy' |
  'category' | 'sponsor_level' | 'board' | 'interval' | 'listed_since' | 'expires_at' |
  'key_type' | 'auto_cold_exempt' | 'auto_move_exempt'
>;

interface ChildEdit {
  _original?: MonitorConfig;
  model: string;
  template: string;
  base_url: string;
  api_key: string;
}

interface SelectOption {
  value: string;
  label: string;
}

export function MonitorDetail({
  fetchTemplates, monitorFile, monitorKey, onBack,
  onSave, onDelete, onToggle, onProbe,
  isProbing, probeResult, probeError,
}: MonitorDetailProps) {
  const { t } = useTranslation();
  const [isEditing, setIsEditing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [templates, setTemplates] = useState<string[]>([]);

  const root = monitorFile.monitors.find(m => !m.parent) || monitorFile.monitors[0];
  const children = monitorFile.monitors.filter(m => m.parent);

  const [editFields, setEditFields] = useState<EditableFields>({
    provider_name: root?.provider_name || '',
    channel_name: root?.channel_name || '',
    provider_url: root?.provider_url || '',
    template: root?.template || '',
    base_url: root?.base_url || '',
    api_key: root?.api_key || '',
    proxy: root?.proxy || '',
    category: root?.category || '',
    sponsor_level: root?.sponsor_level || '',
    board: root?.board || 'hot',
    interval: root?.interval || '',
    listed_since: root?.listed_since || '',
    expires_at: root?.expires_at || '',
    key_type: root?.key_type || '',
    auto_cold_exempt: root?.auto_cold_exempt ?? false,
    auto_move_exempt: root?.auto_move_exempt ?? false,
  });

  const [editChildren, setEditChildren] = useState<ChildEdit[]>([]);

  useEffect(() => {
    let active = true;
    fetchTemplates()
      .then(items => { if (active) setTemplates(items); })
      .catch(() => { if (active) setTemplates([]); });
    return () => { active = false; };
  }, [fetchTemplates]);

  const templateOptions = withCurrentOption(
    [
      { value: '', label: t('admin.monitors.templateNone') },
      ...Array.from(new Set(templates)).sort().map(name => ({ value: name, label: name })),
    ],
    isEditing ? editFields.template : root?.template,
  );

  const categoryOptions = withCurrentOption([
    { value: 'commercial', label: t('admin.monitors.categoryCommercial') },
    { value: 'public', label: t('admin.monitors.categoryPublic') },
  ], isEditing ? editFields.category : root?.category);

  const sponsorLevelOptions = withCurrentOption([
    { value: '', label: t('admin.monitors.sponsorLevels.none') },
    { value: 'public', label: t('admin.monitors.sponsorLevels.public') },
    { value: 'signal', label: t('admin.monitors.sponsorLevels.signal') },
    { value: 'pulse', label: t('admin.monitors.sponsorLevels.pulse') },
    { value: 'beacon', label: t('admin.monitors.sponsorLevels.beacon') },
    { value: 'backbone', label: t('admin.monitors.sponsorLevels.backbone') },
    { value: 'core', label: t('admin.monitors.sponsorLevels.core') },
  ], isEditing ? editFields.sponsor_level : root?.sponsor_level);

  const boardOptions = withCurrentOption([
    { value: 'hot', label: t('admin.monitors.boardHot') },
    { value: 'secondary', label: t('admin.monitors.boardSecondary') },
    { value: 'cold', label: t('admin.monitors.boardCold') },
  ], isEditing ? editFields.board : (root?.board || 'hot'));

  const toChildEdits = (items: MonitorConfig[]): ChildEdit[] =>
    items.map(c => ({
      _original: c,
      model: c.model || '',
      template: c.template || '',
      base_url: c.base_url || '',
      api_key: c.api_key || '',
    }));

  const startEditing = () => {
    setEditFields({
      provider_name: root?.provider_name || '',
      channel_name: root?.channel_name || '',
      provider_url: root?.provider_url || '',
      template: root?.template || '',
      base_url: root?.base_url || '',
      api_key: root?.api_key || '',
      proxy: root?.proxy || '',
      category: root?.category || '',
      sponsor_level: root?.sponsor_level || '',
      board: root?.board || 'hot',
      interval: root?.interval || '',
      listed_since: root?.listed_since || '',
      expires_at: root?.expires_at || '',
      key_type: root?.key_type || '',
      auto_cold_exempt: root?.auto_cold_exempt ?? false,
      auto_move_exempt: root?.auto_move_exempt ?? false,
    });
    setEditChildren(toChildEdits(children));
    setSaveError(null);
    setIsEditing(true);
  };

  const cancelEditing = () => {
    setIsEditing(false);
    setSaveError(null);
  };

  const addChild = () => {
    setEditChildren(prev => [...prev, { model: '', template: '', base_url: '', api_key: '' }]);
  };

  const removeChild = (index: number) => {
    setEditChildren(prev => prev.filter((_, i) => i !== index));
  };

  const updateChild = (index: number, field: keyof ChildEdit, value: string) => {
    setEditChildren(prev => prev.map((c, i) => i === index ? { ...c, [field]: value } : c));
  };

  const handleSave = async () => {
    setIsSaving(true);
    setSaveError(null);
    try {
      const parentPath = `${root.provider}/${root.service}/${root.channel}`;
      const updatedRoot = { ...root, ...editFields };
      const updatedChildren: MonitorConfig[] = editChildren.map(c => ({
        ...c._original,
        provider: c._original?.provider || '',
        service: c._original?.service || '',
        channel: c._original?.channel || '',
        parent: parentPath,
        model: c.model.trim() || undefined,
        template: c.template || undefined,
        base_url: c.base_url || undefined,
        api_key: c.api_key || undefined,
      }));
      const updatedFile: MonitorFile = {
        ...monitorFile,
        monitors: [updatedRoot, ...updatedChildren],
      };
      await onSave(updatedFile, monitorFile.metadata.revision);
      setIsEditing(false);
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : String(e));
    } finally {
      setIsSaving(false);
    }
  };

  const handleProbe = async () => {
    await onProbe();
  };

  const handleDelete = async () => {
    setIsDeleting(true);
    try {
      onDelete();
    } finally {
      setIsDeleting(false);
      setShowDeleteConfirm(false);
    }
  };

  const updateField = <K extends keyof EditableFields>(key: K, value: EditableFields[K]) => {
    setEditFields(prev => ({ ...prev, [key]: value }));
  };

  return (
    <div className="space-y-6">
      {/* 顶栏 */}
      <div className="flex items-center justify-between">
        <button
          onClick={onBack}
          className="text-accent hover:text-accent-strong text-sm transition"
        >
          {t('admin.monitors.backToList')}
        </button>
        <div className="text-xs text-muted">
          {monitorKey} | rev:{monitorFile.metadata.revision} | {monitorFile.metadata.source}
        </div>
      </div>

      {/* 标题 */}
      <h2 className="text-xl font-bold text-primary">
        {root?.provider}/{root?.service}/{root?.channel}
      </h2>

      {/* 保存错误 */}
      {saveError && (
        <div className="p-3 bg-danger/10 border border-danger/20 rounded-lg text-danger text-sm">
          {saveError}
        </div>
      )}

      {/* 父通道详情 */}
      <div className="bg-surface rounded-lg border border-default p-4 space-y-3">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-semibold text-primary">{t('admin.monitors.parentChannel')}</h3>
          {!isEditing ? (
            <button
              onClick={startEditing}
              className="px-3 py-1 text-xs rounded-lg border border-accent/30 text-accent hover:bg-accent/10 transition"
            >
              {t('admin.monitors.edit')}
            </button>
          ) : (
            <div className="flex gap-2">
              <button
                onClick={handleSave}
                disabled={isSaving}
                className="px-3 py-1 text-xs rounded-lg bg-accent/10 text-accent font-medium hover:bg-accent/20 transition disabled:opacity-50"
              >
                {isSaving ? t('admin.detail.saving') : t('admin.detail.save')}
              </button>
              <button
                onClick={cancelEditing}
                className="px-3 py-1 text-xs rounded-lg border border-default text-secondary hover:text-primary transition"
              >
                {t('admin.detail.cancel')}
              </button>
            </div>
          )}
        </div>

        <div className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
          <Field label={t('admin.monitors.field.provider')} value={root?.provider} />
          <EditableField
            label={t('admin.monitors.field.providerName')}
            value={isEditing ? editFields.provider_name : root?.provider_name}
            editing={isEditing}
            onChange={v => updateField('provider_name', v)}
          />
          <Field label={t('admin.monitors.field.service')} value={root?.service} />
          <Field label={t('admin.monitors.field.channel')} value={root?.channel} />
          <EditableField
            label={t('admin.monitors.field.channelName')}
            value={isEditing ? editFields.channel_name : root?.channel_name}
            editing={isEditing}
            onChange={v => updateField('channel_name', v)}
          />
          <EditableSelectField
            label={t('admin.monitors.field.template')}
            value={isEditing ? editFields.template : root?.template}
            editing={isEditing}
            onChange={v => updateField('template', v)}
            options={templateOptions}
          />
          <EditableField
            label={t('admin.monitors.field.baseUrl')}
            value={isEditing ? editFields.base_url : root?.base_url}
            editing={isEditing}
            onChange={v => updateField('base_url', v)}
          />
          <EditableSelectField
            label={t('admin.monitors.field.category')}
            value={isEditing ? editFields.category : root?.category}
            editing={isEditing}
            onChange={v => updateField('category', v)}
            options={categoryOptions}
          />
          <EditableSelectField
            label={t('admin.monitors.field.sponsorLevel')}
            value={isEditing ? editFields.sponsor_level : root?.sponsor_level}
            editing={isEditing}
            onChange={v => updateField('sponsor_level', v)}
            options={sponsorLevelOptions}
          />
          <EditableSelectField
            label={t('admin.monitors.field.board')}
            value={isEditing ? editFields.board : (root?.board || 'hot')}
            editing={isEditing}
            onChange={v => updateField('board', v)}
            options={boardOptions}
          />
          <EditableField
            label={t('admin.monitors.field.interval')}
            value={isEditing ? editFields.interval : root?.interval}
            editing={isEditing}
            onChange={v => updateField('interval', v)}
          />
          <EditableField
            label={t('admin.monitors.field.apiKey')}
            value={isEditing ? editFields.api_key : (root?.api_key ? `***${root.api_key.slice(-4)}` : '')}
            editing={isEditing}
            onChange={v => updateField('api_key', v)}
            type="password"
          />
          <EditableField
            label={t('admin.monitors.field.proxy')}
            value={isEditing ? editFields.proxy : root?.proxy}
            editing={isEditing}
            onChange={v => updateField('proxy', v)}
          />
          <EditableField
            label={t('admin.monitors.field.listedSince')}
            value={isEditing ? editFields.listed_since : root?.listed_since}
            editing={isEditing}
            onChange={v => updateField('listed_since', v)}
          />
          <EditableField
            label={t('admin.monitors.field.expiresAt')}
            value={isEditing ? editFields.expires_at : root?.expires_at}
            editing={isEditing}
            onChange={v => updateField('expires_at', v)}
            type="date"
          />
          <EditableField
            label={t('admin.monitors.field.providerUrl')}
            value={isEditing ? editFields.provider_url : root?.provider_url}
            editing={isEditing}
            onChange={v => updateField('provider_url', v)}
            type="url"
          />
          <EditableSelectField
            label={t('admin.monitors.field.keyType')}
            value={isEditing ? (editFields.key_type || '') : (root?.key_type || '')}
            editing={isEditing}
            onChange={v => updateField('key_type', v)}
            options={[
              { value: '', label: t('admin.monitors.keyTypeDefault') },
              { value: 'official', label: t('admin.monitors.keyTypeOfficial') },
              { value: 'user', label: t('admin.monitors.keyTypeUser') },
            ]}
          />
          <EditableBoolField
            label={t('admin.monitors.field.autoColdExempt')}
            hint={t('admin.monitors.field.autoColdExemptHint')}
            value={isEditing ? (editFields.auto_cold_exempt ?? false) : (root?.auto_cold_exempt ?? false)}
            editing={isEditing}
            onChange={v => updateField('auto_cold_exempt', v)}
          />
          <EditableBoolField
            label={t('admin.monitors.field.autoMoveExempt')}
            hint={t('admin.monitors.field.autoMoveExemptHint')}
            value={isEditing ? (editFields.auto_move_exempt ?? false) : (root?.auto_move_exempt ?? false)}
            editing={isEditing}
            onChange={v => updateField('auto_move_exempt', v)}
          />
        </div>

        {/* 状态切换 */}
        <div className="flex gap-3 pt-2">
          <button
            onClick={() => onToggle('disabled', !root?.disabled)}
            className={`px-3 py-1.5 text-xs rounded-lg border transition ${
              root?.disabled
                ? 'border-success/30 text-success hover:bg-success/10'
                : 'border-danger/30 text-danger hover:bg-danger/10'
            }`}
          >
            {root?.disabled ? t('admin.monitors.enable') : t('admin.monitors.disable')}
          </button>
          <button
            onClick={() => onToggle('hidden', !root?.hidden)}
            className={`px-3 py-1.5 text-xs rounded-lg border transition ${
              root?.hidden
                ? 'border-success/30 text-success hover:bg-success/10'
                : 'border-warning/30 text-warning hover:bg-warning/10'
            }`}
          >
            {root?.hidden ? t('admin.monitors.show') : t('admin.monitors.hide')}
          </button>
        </div>
      </div>

      {/* 子通道 */}
      <div className="bg-surface rounded-lg border border-default p-4 space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold text-primary">
            {t('admin.monitors.childChannels')} ({isEditing ? editChildren.length : children.length})
          </h3>
          {isEditing && (
            <button
              onClick={addChild}
              className="px-3 py-1 text-xs rounded-lg border border-dashed border-accent/40 text-accent hover:bg-accent/5 transition"
            >
              + {t('admin.monitors.addChild')}
            </button>
          )}
        </div>

        {isEditing ? (
          /* 编辑态：每个子通道展开为可编辑行 */
          editChildren.length === 0 ? (
            <p className="text-xs text-muted">{t('admin.monitors.noChildren')}</p>
          ) : (
            <div className="space-y-3">
              {editChildren.map((child, i) => (
                <div key={i} className="grid grid-cols-[1fr_1fr_1fr_1fr_auto] gap-2 items-end border-b border-default/30 pb-3 last:border-0">
                  <div>
                    <label className="block text-xs text-muted mb-0.5">{t('admin.monitors.field.model')}</label>
                    <input
                      value={child.model}
                      onChange={e => updateChild(i, 'model', e.target.value)}
                      placeholder={t('admin.monitors.form.modelPlaceholder')}
                      className="w-full px-2 py-1 rounded bg-elevated border border-default text-primary text-sm placeholder:text-muted/50 focus:outline-none focus:border-accent"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-muted mb-0.5">{t('admin.monitors.field.template')}</label>
                    <select
                      value={child.template}
                      onChange={e => updateChild(i, 'template', e.target.value)}
                      className="w-full px-2 py-1 rounded bg-elevated border border-default text-primary text-sm focus:outline-none focus:border-accent"
                    >
                      {withCurrentOption(templateOptions, child.template).map(opt => (
                        <option key={opt.value} value={opt.value}>{opt.label}</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="block text-xs text-muted mb-0.5">{t('admin.monitors.field.baseUrl')}</label>
                    <input
                      value={child.base_url}
                      onChange={e => updateChild(i, 'base_url', e.target.value)}
                      className="w-full px-2 py-1 rounded bg-elevated border border-default text-primary text-sm focus:outline-none focus:border-accent"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-muted mb-0.5">{t('admin.monitors.field.apiKey')}</label>
                    <input
                      type="password"
                      value={child.api_key}
                      onChange={e => updateChild(i, 'api_key', e.target.value)}
                      className="w-full px-2 py-1 rounded bg-elevated border border-default text-primary text-sm focus:outline-none focus:border-accent"
                    />
                  </div>
                  <button
                    onClick={() => removeChild(i)}
                    className="px-2 py-1 text-danger hover:text-danger/80 text-sm transition"
                    title={t('admin.monitors.removeChild')}
                  >
                    &times;
                  </button>
                </div>
              ))}
            </div>
          )
        ) : (
          /* 查看态 */
          children.length === 0 ? (
            <p className="text-xs text-muted">{t('admin.monitors.noChildren')}</p>
          ) : (
            <div className="space-y-2">
              {children.map((child, i) => (
                <div key={i} className="flex items-center gap-4 text-sm py-1.5 border-b border-default/30 last:border-0">
                  <span className="text-primary font-medium">{child.model || t('admin.monitors.noModel')}</span>
                  <span className="text-muted">{child.template}</span>
                  {child.base_url && <span className="text-muted text-xs truncate max-w-[200px]">{child.base_url}</span>}
                </div>
              ))}
            </div>
          )
        )}
      </div>

      {/* 操作栏 */}
      <div className="flex gap-3">
        <button
          onClick={handleProbe}
          disabled={isProbing}
          className="px-4 py-2 rounded-lg bg-accent/10 text-accent text-sm font-medium hover:bg-accent/20 transition disabled:opacity-50"
        >
          {isProbing ? t('admin.monitors.probing') : t('admin.monitors.probe')}
        </button>

        {probeResult && (
          <div className="flex items-center gap-3 self-center text-xs">
            <span className={`inline-block w-2 h-2 rounded-full ${
              probeResult.probeStatus === 1 ? 'bg-success' :
              probeResult.probeStatus === 2 ? 'bg-warning' : 'bg-danger'
            }`} />
            <span className="text-primary">{probeResult.latency}ms</span>
            <span className="text-muted">HTTP {probeResult.httpCode}</span>
            {probeResult.errorMessage && (
              <span className="text-danger truncate max-w-[200px]" title={probeResult.errorMessage}>
                {probeResult.errorMessage}
              </span>
            )}
          </div>
        )}

        {probeError && (
          <span className="self-center text-xs text-danger">{probeError}</span>
        )}

        <div className="flex-1" />

        {!showDeleteConfirm ? (
          <button
            onClick={() => setShowDeleteConfirm(true)}
            className="px-4 py-2 rounded-lg border border-danger/30 text-danger text-sm hover:bg-danger/10 transition"
          >
            {t('admin.monitors.archive')}
          </button>
        ) : (
          <div className="flex gap-2 items-center">
            <span className="text-xs text-danger">{t('admin.monitors.confirmArchive')}</span>
            <button
              onClick={handleDelete}
              disabled={isDeleting}
              className="px-3 py-1.5 rounded-lg bg-danger text-white text-xs font-medium hover:bg-danger/80 transition disabled:opacity-50"
            >
              {isDeleting ? '...' : t('admin.monitors.confirmYes')}
            </button>
            <button
              onClick={() => setShowDeleteConfirm(false)}
              className="px-3 py-1.5 rounded-lg border border-default text-secondary text-xs hover:text-primary transition"
            >
              {t('admin.detail.cancel')}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

function Field({ label, value }: { label: string; value?: string | number | null }) {
  return (
    <div>
      <span className="text-muted">{label}: </span>
      <span className="text-primary">{value || '-'}</span>
    </div>
  );
}

function EditableSelectField({
  label, value, editing, onChange, options,
}: {
  label: string;
  value?: string | null;
  editing: boolean;
  onChange: (v: string) => void;
  options: SelectOption[];
}) {
  const currentValue = String(value || '');
  const displayLabel = options.find(o => o.value === currentValue)?.label ?? currentValue;

  if (!editing) {
    return <Field label={label} value={displayLabel || '-'} />;
  }

  return (
    <div>
      <label className="block text-xs text-muted mb-0.5">{label}</label>
      <select
        value={currentValue}
        onChange={e => onChange(e.target.value)}
        className="w-full px-2 py-1 rounded bg-elevated border border-default text-primary text-sm focus:outline-none focus:border-accent"
      >
        {options.map(opt => (
          <option key={opt.value} value={opt.value}>{opt.label}</option>
        ))}
      </select>
    </div>
  );
}

function EditableField({
  label, value, editing, onChange, type = 'text',
}: {
  label: string;
  value?: string | number | null;
  editing: boolean;
  onChange: (v: string) => void;
  type?: string;
}) {
  if (!editing) {
    return <Field label={label} value={value} />;
  }
  return (
    <div>
      <label className="block text-xs text-muted mb-0.5">{label}</label>
      <input
        type={type}
        value={String(value || '')}
        onChange={e => onChange(e.target.value)}
        className="w-full px-2 py-1 rounded bg-elevated border border-default text-primary text-sm focus:outline-none focus:border-accent"
      />
    </div>
  );
}

function EditableBoolField({
  label, hint, value, editing, onChange,
}: {
  label: string;
  hint?: string;
  value: boolean;
  editing: boolean;
  onChange: (v: boolean) => void;
}) {
  if (!editing) {
    return (
      <div>
        <span className="text-muted">{label}: </span>
        <span className="text-primary">{value ? '✓' : '-'}</span>
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-0.5">
      <label className="flex items-center gap-2 cursor-pointer">
        <input
          type="checkbox"
          checked={value}
          onChange={e => onChange(e.target.checked)}
          className="accent-accent"
        />
        <span className="text-xs text-primary">{label}</span>
      </label>
      {hint && <span className="text-xs text-muted ml-5">{hint}</span>}
    </div>
  );
}

function withCurrentOption(options: SelectOption[], current?: string | null): SelectOption[] {
  if (!current || options.some(o => o.value === current)) return options;
  return [...options, { value: current, label: current }];
}
