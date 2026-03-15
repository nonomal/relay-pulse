import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { MonitorConfig, MonitorFile } from '../../types/monitor';

interface MonitorFormProps {
  fetchTemplates: () => Promise<string[]>;
  onSave: (file: MonitorFile) => Promise<void>;
  onCancel: () => void;
}

interface ChildDraft {
  model: string;
  template: string;
  base_url: string;
  api_key: string;
}

const EMPTY_CONFIG: MonitorConfig = {
  provider: '',
  service: '',
  channel: '',
  channel_name: '',
  provider_name: '',
  template: '',
  base_url: '',
  api_key: '',
  category: 'commercial',
  sponsor_level: '',
  board: 'hot',
  interval: '',
  listed_since: '',
};

const EMPTY_CHILD: ChildDraft = { model: '', template: '', base_url: '', api_key: '' };

export function MonitorForm({ fetchTemplates, onSave, onCancel }: MonitorFormProps) {
  const { t } = useTranslation();
  const [config, setConfig] = useState<MonitorConfig>({ ...EMPTY_CONFIG });
  const [children, setChildren] = useState<ChildDraft[]>([]);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [templates, setTemplates] = useState<string[]>([]);

  useEffect(() => {
    let active = true;
    fetchTemplates()
      .then(items => { if (active) setTemplates(items); })
      .catch(() => { if (active) setTemplates([]); });
    return () => { active = false; };
  }, [fetchTemplates]);

  const templateOptions = mergeTemplateNames(templates, config.template);

  const sponsorLevelOptions = [
    { value: '', label: t('admin.monitors.sponsorLevels.none') },
    { value: 'public', label: t('admin.monitors.sponsorLevels.public') },
    { value: 'signal', label: t('admin.monitors.sponsorLevels.signal') },
    { value: 'pulse', label: t('admin.monitors.sponsorLevels.pulse') },
    { value: 'beacon', label: t('admin.monitors.sponsorLevels.beacon') },
    { value: 'backbone', label: t('admin.monitors.sponsorLevels.backbone') },
    { value: 'core', label: t('admin.monitors.sponsorLevels.core') },
  ];

  const updateField = <K extends keyof MonitorConfig>(key: K, value: MonitorConfig[K]) => {
    setConfig(prev => ({ ...prev, [key]: value }));
  };

  const addChild = () => setChildren(prev => [...prev, { ...EMPTY_CHILD }]);

  const removeChild = (index: number) => {
    setChildren(prev => prev.filter((_, i) => i !== index));
  };

  const updateChild = (index: number, field: keyof ChildDraft, value: string) => {
    setChildren(prev => prev.map((c, i) => i === index ? { ...c, [field]: value } : c));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!config.provider || !config.service || !config.channel) {
      setError(t('admin.monitors.form.requiredFields'));
      return;
    }

    setIsSaving(true);
    try {
      const parentPath = `${config.provider}/${config.service}/${config.channel}`;
      const monitors: MonitorConfig[] = [config];
      for (const child of children) {
        monitors.push({
          provider: '',
          service: '',
          channel: '',
          parent: parentPath,
          model: child.model.trim() || undefined,
          template: child.template || undefined,
          base_url: child.base_url || undefined,
          api_key: child.api_key || undefined,
        });
      }
      const file: MonitorFile = {
        metadata: { source: 'admin', revision: 0, created_at: '', updated_at: '' },
        monitors,
      };
      await onSave(file);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-bold text-primary">{t('admin.monitors.form.title')}</h2>
        <button
          type="button"
          onClick={onCancel}
          className="text-accent hover:text-accent-strong text-sm transition"
        >
          {t('admin.detail.cancel')}
        </button>
      </div>

      {error && (
        <div className="p-3 bg-danger/10 border border-danger/20 rounded-lg text-danger text-sm">
          {error}
        </div>
      )}

      {/* 身份标识 */}
      <fieldset className="bg-surface rounded-lg border border-default p-4 space-y-3">
        <legend className="text-sm font-semibold text-primary px-1">
          {t('admin.monitors.form.identity')}
        </legend>
        <div className="grid grid-cols-2 gap-4">
          <FormField
            label={`${t('admin.monitors.field.provider')} *`}
            value={config.provider || ''}
            onChange={v => updateField('provider', v)}
          />
          <FormField
            label={t('admin.monitors.field.providerName')}
            value={config.provider_name || ''}
            onChange={v => updateField('provider_name', v)}
          />
          <FormField
            label={`${t('admin.monitors.field.service')} *`}
            value={config.service || ''}
            onChange={v => updateField('service', v)}
          />
          <FormField
            label={`${t('admin.monitors.field.channel')} *`}
            value={config.channel || ''}
            onChange={v => updateField('channel', v)}
          />
          <FormField
            label={t('admin.monitors.field.channelName')}
            value={config.channel_name || ''}
            onChange={v => updateField('channel_name', v)}
          />
        </div>
      </fieldset>

      {/* 探测配置 */}
      <fieldset className="bg-surface rounded-lg border border-default p-4 space-y-3">
        <legend className="text-sm font-semibold text-primary px-1">
          {t('admin.monitors.form.probeConfig')}
        </legend>
        <div className="grid grid-cols-2 gap-4">
          <SelectField
            label={t('admin.monitors.field.template')}
            value={config.template || ''}
            onChange={v => updateField('template', v)}
            options={[
              { value: '', label: t('admin.monitors.templateNone') },
              ...templateOptions.map(name => ({ value: name, label: name })),
            ]}
          />
          <FormField
            label={t('admin.monitors.field.baseUrl')}
            value={config.base_url || ''}
            onChange={v => updateField('base_url', v)}
          />
          <FormField
            label={t('admin.monitors.field.apiKey')}
            value={config.api_key || ''}
            onChange={v => updateField('api_key', v)}
            type="password"
          />
          <FormField
            label={t('admin.monitors.field.interval')}
            value={config.interval || ''}
            onChange={v => updateField('interval', v)}
          />
        </div>
      </fieldset>

      {/* 业务属性 */}
      <fieldset className="bg-surface rounded-lg border border-default p-4 space-y-3">
        <legend className="text-sm font-semibold text-primary px-1">
          {t('admin.monitors.form.attributes')}
        </legend>
        <div className="grid grid-cols-2 gap-4">
          <SelectField
            label={t('admin.monitors.field.category')}
            value={config.category || 'commercial'}
            onChange={v => updateField('category', v)}
            options={[
              { value: 'commercial', label: t('admin.monitors.categoryCommercial') },
              { value: 'public', label: t('admin.monitors.categoryPublic') },
            ]}
          />
          <SelectField
            label={t('admin.monitors.field.sponsorLevel')}
            value={config.sponsor_level || ''}
            onChange={v => updateField('sponsor_level', v)}
            options={sponsorLevelOptions}
          />
          <SelectField
            label={t('admin.monitors.field.board')}
            value={config.board || 'hot'}
            onChange={v => updateField('board', v)}
            options={[
              { value: 'hot', label: t('admin.monitors.boardHot') },
              { value: 'secondary', label: t('admin.monitors.boardSecondary') },
              { value: 'cold', label: t('admin.monitors.boardCold') },
            ]}
          />
          <FormField
            label={t('admin.monitors.field.listedSince')}
            value={config.listed_since || ''}
            onChange={v => updateField('listed_since', v)}
          />
        </div>
      </fieldset>

      {/* 子通道（多模型） */}
      <fieldset className="bg-surface rounded-lg border border-default p-4 space-y-3">
        <legend className="text-sm font-semibold text-primary px-1">
          {t('admin.monitors.form.childChannels')}
        </legend>
        <p className="text-xs text-muted">{t('admin.monitors.form.childHint')}</p>

        {children.map((child, i) => (
          <div key={i} className="grid grid-cols-[1fr_1fr_1fr_1fr_auto] gap-3 items-end">
            <FormField
              label={t('admin.monitors.field.model')}
              value={child.model}
              onChange={v => updateChild(i, 'model', v)}
              placeholder={t('admin.monitors.form.modelPlaceholder')}
            />
            <FormField
              label={t('admin.monitors.field.template')}
              value={child.template}
              onChange={v => updateChild(i, 'template', v)}
            />
            <FormField
              label={t('admin.monitors.field.baseUrl')}
              value={child.base_url}
              onChange={v => updateChild(i, 'base_url', v)}
            />
            <FormField
              label={t('admin.monitors.field.apiKey')}
              value={child.api_key}
              onChange={v => updateChild(i, 'api_key', v)}
              type="password"
            />
            <button
              type="button"
              onClick={() => removeChild(i)}
              className="px-2 py-2 text-danger hover:text-danger/80 text-sm transition"
              title={t('admin.monitors.removeChild')}
            >
              &times;
            </button>
          </div>
        ))}

        <button
          type="button"
          onClick={addChild}
          className="px-3 py-1.5 rounded-lg border border-dashed border-accent/40 text-accent text-xs hover:bg-accent/5 transition"
        >
          + {t('admin.monitors.addChild')}
        </button>
      </fieldset>

      {/* 提交 */}
      <div className="flex gap-3 justify-end">
        <button
          type="button"
          onClick={onCancel}
          className="px-4 py-2 rounded-lg border border-default text-secondary text-sm hover:text-primary transition"
        >
          {t('admin.detail.cancel')}
        </button>
        <button
          type="submit"
          disabled={isSaving}
          className="px-4 py-2 rounded-lg bg-accent/10 text-accent text-sm font-medium hover:bg-accent/20 transition disabled:opacity-50"
        >
          {isSaving ? t('admin.detail.saving') : t('admin.monitors.form.create')}
        </button>
      </div>
    </form>
  );
}

function FormField({
  label,
  value,
  onChange,
  type = 'text',
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  placeholder?: string;
}) {
  return (
    <div>
      <label className="block text-xs text-muted mb-1">{label}</label>
      <input
        type={type}
        value={value}
        onChange={e => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full px-3 py-2 rounded-lg bg-elevated border border-default text-primary text-sm placeholder:text-muted/50 focus:outline-none focus:border-accent"
      />
    </div>
  );
}

function SelectField({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <div>
      <label className="block text-xs text-muted mb-1">{label}</label>
      <select
        value={value}
        onChange={e => onChange(e.target.value)}
        className="w-full px-3 py-2 rounded-lg bg-elevated border border-default text-primary text-sm"
      >
        {options.map(opt => (
          <option key={opt.value} value={opt.value}>{opt.label}</option>
        ))}
      </select>
    </div>
  );
}

function mergeTemplateNames(names: string[], current?: string): string[] {
  const merged = new Set(names.filter(Boolean));
  if (current) merged.add(current);
  return Array.from(merged).sort();
}
