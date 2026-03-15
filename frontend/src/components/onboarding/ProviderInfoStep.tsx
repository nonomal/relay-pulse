import { useMemo, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { ChevronRight, ExternalLink } from 'lucide-react';
import type { OnboardingFormData, OnboardingMeta, IdentityType } from '../../types/onboarding';

interface ProviderInfoStepProps {
  formData: OnboardingFormData;
  updateField: <K extends keyof OnboardingFormData>(key: K, value: OnboardingFormData[K]) => void;
  meta: OnboardingMeta | null;
  onNext: () => void;
}

/** Step 1: Provider information and channel configuration. */
export function ProviderInfoStep({ formData, updateField, meta, onNext }: ProviderInfoStepProps) {
  const { t } = useTranslation();

  const channelCode = useMemo(() => {
    if (!formData.channelType || !formData.channelSource) return '';
    return `${formData.channelType}-${formData.channelSource}`;
  }, [formData.channelType, formData.channelSource]);

  const handleIdentityChange = useCallback((identity: IdentityType) => {
    updateField('identity', identity);
    if (identity === 'publicOwner') {
      updateField('category', 'public');
      updateField('sponsorLevel', 'public');
    } else if (identity === 'commercialOwner') {
      updateField('category', 'commercial');
      updateField('sponsorLevel', 'pulse');
    } else if (identity === 'personal') {
      updateField('category', 'commercial');
      updateField('sponsorLevel', 'signal');
    }
  }, [updateField]);

  const canProceed = useMemo(() => {
    return (
      formData.agreementAccepted &&
      formData.identity !== '' &&
      formData.providerName.trim().length > 0 &&
      formData.websiteUrl.trim().length > 0 &&
      formData.serviceType.length > 0 &&
      formData.channelType.length > 0 &&
      formData.channelSource.length > 0
    );
  }, [
    formData.agreementAccepted, formData.identity, formData.providerName, formData.websiteUrl,
    formData.serviceType, formData.channelType, formData.channelSource,
  ]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (canProceed) onNext();
  };

  if (!meta) {
    return (
      <div className="bg-surface border border-muted rounded-lg p-8 text-center">
        <p className="text-secondary">{t('onboarding.loading')}</p>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="bg-surface border border-muted rounded-lg p-6 space-y-6">
      <h2 className="text-xl font-semibold text-primary">
        {t('onboarding.providerInfo.title')}
      </h2>

      {/* Agreement checkbox */}
      <div className="p-4 bg-elevated rounded-lg space-y-3">
        <label className="flex items-start gap-3 cursor-pointer">
          <input
            type="checkbox"
            checked={formData.agreementAccepted}
            onChange={(e) => updateField('agreementAccepted', e.target.checked)}
            className="mt-1 w-4 h-4 rounded border-muted accent-accent"
          />
          <span className="text-sm text-secondary leading-relaxed">
            {t('onboarding.providerInfo.agreementText')}{' '}
            <a
              href="https://github.com/chaosgoo/relay-pulse/blob/main/docs/user/sponsorship.md"
              target="_blank"
              rel="noopener noreferrer"
              className="text-accent hover:text-accent-strong underline inline-flex items-center gap-0.5"
            >
              {t('onboarding.providerInfo.agreementLink')}
              <ExternalLink className="w-3 h-3" />
            </a>
          </span>
        </label>
      </div>

      {/* Provider name */}
      <div>
        <label htmlFor="ob-provider-name" className="block text-sm font-medium text-primary mb-2">
          {t('onboarding.providerInfo.providerName')}
          <span className="text-danger ml-0.5">*</span>
        </label>
        <input
          id="ob-provider-name"
          type="text"
          required
          value={formData.providerName}
          onChange={(e) => updateField('providerName', e.target.value)}
          placeholder={t('onboarding.providerInfo.providerNamePlaceholder')}
          className="w-full px-4 py-2 bg-surface border border-muted rounded-lg text-primary placeholder-muted focus:outline-none focus:ring-2 focus:ring-accent disabled:opacity-50"
        />
      </div>

      {/* Website URL */}
      <div>
        <label htmlFor="ob-website-url" className="block text-sm font-medium text-primary mb-2">
          {t('onboarding.providerInfo.websiteUrl')}
          <span className="text-danger ml-0.5">*</span>
        </label>
        <input
          id="ob-website-url"
          type="url"
          required
          value={formData.websiteUrl}
          onChange={(e) => updateField('websiteUrl', e.target.value)}
          placeholder="https://example.com"
          className="w-full px-4 py-2 bg-surface border border-muted rounded-lg text-primary placeholder-muted focus:outline-none focus:ring-2 focus:ring-accent disabled:opacity-50"
        />
      </div>

      {/* Identity - radio group */}
      <fieldset>
        <legend className="block text-sm font-medium text-primary mb-2">
          {t('onboarding.providerInfo.identity')}
          <span className="text-danger ml-0.5">*</span>
        </legend>
        <div className="space-y-2">
          {(['publicOwner', 'commercialOwner', 'personal'] as const).map((id) => (
            <label key={id} className="flex items-start gap-3 cursor-pointer p-3 rounded-lg border border-muted hover:border-accent/40 transition-colors has-[:checked]:border-accent has-[:checked]:bg-accent/5">
              <input
                type="radio"
                name="identity"
                value={id}
                checked={formData.identity === id}
                onChange={() => handleIdentityChange(id)}
                className="mt-0.5 w-4 h-4 accent-accent"
              />
              <div>
                <span className="text-sm font-medium text-primary">
                  {t(`onboarding.providerInfo.identities.${id}`)}
                </span>
                <p className="text-xs text-secondary mt-0.5">
                  {t(`onboarding.providerInfo.identities.${id}Desc`)}
                </p>
              </div>
            </label>
          ))}
        </div>
      </fieldset>

      {/* Service type - select */}
      <div>
        <label htmlFor="ob-service-type" className="block text-sm font-medium text-primary mb-2">
          {t('onboarding.providerInfo.serviceType')}
          <span className="text-danger ml-0.5">*</span>
        </label>
        <select
          id="ob-service-type"
          value={formData.serviceType}
          onChange={(e) => updateField('serviceType', e.target.value)}
          className="w-full px-4 py-2 bg-surface border border-muted rounded-lg text-primary focus:outline-none focus:ring-2 focus:ring-accent disabled:opacity-50"
        >
          {meta.service_types.map((st) => (
            <option key={st} value={st}>
              {t(`onboarding.providerInfo.serviceTypes.${st}`, { defaultValue: st.toUpperCase() })}
            </option>
          ))}
        </select>
      </div>

      {/* Channel type - card radio group */}
      <fieldset>
        <legend className="block text-sm font-medium text-primary mb-2">
          {t('onboarding.providerInfo.channelType')}
          <span className="text-danger ml-0.5">*</span>
        </legend>
        <div className="space-y-2">
          {meta.channel_types.map((ct) => (
            <label
              key={ct.value}
              className="flex items-start gap-3 cursor-pointer p-3 rounded-lg border border-muted hover:border-accent/40 transition-colors has-[:checked]:border-accent has-[:checked]:bg-accent/5"
            >
              <input
                type="radio"
                name="channelType"
                value={ct.value}
                checked={formData.channelType === ct.value}
                onChange={() => {
                  updateField('channelType', ct.value);
                  if (ct.value === 'O') {
                    updateField('channelSource', meta.channel_sources[0] || 'API');
                  } else {
                    updateField('channelSource', '');
                  }
                }}
                className="mt-0.5 w-4 h-4 accent-accent"
              />
              <div>
                <span className="text-sm font-medium text-primary">
                  {t(`onboarding.providerInfo.channelTypes.${ct.value}`, { defaultValue: ct.label })}
                </span>
                <p className="text-xs text-secondary mt-0.5">
                  {t(`onboarding.providerInfo.channelTypes.${ct.value}Desc`, { defaultValue: '' })}
                </p>
              </div>
            </label>
          ))}
        </div>
      </fieldset>

      {/* Channel source - conditional rendering based on channel type */}
      <div>
        <label htmlFor="ob-channel-source" className="block text-sm font-medium text-primary mb-2">
          {t('onboarding.providerInfo.channelSource')}
          <span className="text-danger ml-0.5">*</span>
        </label>
        {formData.channelType === 'O' ? (
          <select
            id="ob-channel-source"
            value={formData.channelSource}
            onChange={(e) => updateField('channelSource', e.target.value)}
            className="w-full px-4 py-2 bg-surface border border-muted rounded-lg text-primary focus:outline-none focus:ring-2 focus:ring-accent disabled:opacity-50"
          >
            {meta.channel_sources.map((src) => (
              <option key={src} value={src}>{src}</option>
            ))}
          </select>
        ) : (
          <input
            id="ob-channel-source"
            type="text"
            value={formData.channelSource}
            onChange={(e) => updateField('channelSource', e.target.value.toUpperCase().replace(/[^A-Z0-9]/g, ''))}
            placeholder={t('onboarding.providerInfo.channelSourceCustomPlaceholder')}
            maxLength={10}
            className="w-full px-4 py-2 bg-surface border border-muted rounded-lg text-primary placeholder-muted focus:outline-none focus:ring-2 focus:ring-accent"
          />
        )}
      </div>

      {/* Channel code preview */}
      {channelCode && (
        <div className="flex items-center gap-3 p-3 bg-elevated rounded-lg">
          <span className="text-sm text-secondary">{t('onboarding.providerInfo.channelCodePreview')}</span>
          <code className="px-3 py-1 bg-accent/10 border border-accent/30 rounded text-accent font-mono font-bold text-lg">
            {channelCode}
          </code>
        </div>
      )}

      {/* Next button */}
      <div className="flex justify-end pt-2">
        <button
          type="submit"
          disabled={!canProceed}
          className="flex items-center gap-2 px-6 py-3 bg-accent text-white rounded-lg font-medium hover:bg-accent-strong transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {t('onboarding.next')}
          <ChevronRight className="w-4 h-4" />
        </button>
      </div>
    </form>
  );
}
