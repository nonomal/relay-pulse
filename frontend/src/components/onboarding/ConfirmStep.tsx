import { useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { ChevronLeft, Copy, Check, RotateCcw } from 'lucide-react';
import type { OnboardingFormData, SubmitOnboardingResponse } from '../../types/onboarding';

interface ConfirmStepProps {
  formData: OnboardingFormData;
  submitResult: SubmitOnboardingResponse | null;
  isSubmitting: boolean;
  onSubmit: () => void;
  onBack: () => void;
  onReset: () => void;
}

/** A single row in the summary table. */
function SummaryRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 py-2">
      <span className="text-sm text-secondary flex-shrink-0">{label}</span>
      <span className="text-sm text-primary text-right break-all">{value}</span>
    </div>
  );
}

/** Copyable text with a feedback icon. */
function CopyableText({ text }: { text: string }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback: select text in a temp input
      const input = document.createElement('input');
      input.value = text;
      document.body.appendChild(input);
      input.select();
      document.execCommand('copy');
      document.body.removeChild(input);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }, [text]);

  return (
    <div className="flex items-center gap-2">
      <code className="px-3 py-1.5 bg-accent/10 border border-accent/30 rounded text-accent font-mono text-sm select-all">
        {text}
      </code>
      <button
        type="button"
        onClick={handleCopy}
        className="p-1.5 text-muted hover:text-accent transition-colors"
        aria-label={t('onboarding.confirm.copy')}
      >
        {copied ? <Check className="w-4 h-4 text-success" /> : <Copy className="w-4 h-4" />}
      </button>
    </div>
  );
}

/** Step 3: Review summary and submit. */
export function ConfirmStep({ formData, submitResult, isSubmitting, onSubmit, onBack, onReset }: ConfirmStepProps) {
  const { t } = useTranslation();

  const channelCode = formData.channelType && formData.channelSource
    ? `${formData.channelType}-${formData.channelSource}`
    : '';

  const maskApiKey = (key: string): string => {
    if (key.length <= 8) return '****';
    return `${key.slice(0, 4)}${'*'.repeat(Math.min(key.length - 8, 20))}${key.slice(-4)}`;
  };

  // After successful submission
  if (submitResult) {
    return (
      <div className="bg-surface border border-muted rounded-lg p-6 space-y-6">
        <div className="text-center space-y-3">
          <div className="inline-flex items-center justify-center w-16 h-16 bg-success/10 rounded-full">
            <Check className="w-8 h-8 text-success" />
          </div>
          <h2 className="text-xl font-semibold text-primary">
            {t('onboarding.confirm.successTitle')}
          </h2>
          <p className="text-secondary">{t('onboarding.confirm.successDescription')}</p>
        </div>

        {/* Public ID */}
        <div className="bg-elevated rounded-lg p-4 space-y-3">
          <div>
            <p className="text-sm font-medium text-secondary mb-1">
              {t('onboarding.confirm.publicId')}
            </p>
            <CopyableText text={submitResult.public_id} />
          </div>
          <p className="text-xs text-muted">{t('onboarding.confirm.publicIdHint')}</p>
        </div>

        {/* Contact info with copy template */}
        {submitResult.contact_info && (
          <div className="bg-elevated rounded-lg p-4 space-y-3">
            <p className="text-sm font-medium text-secondary">
              {t('onboarding.confirm.contactLabel')}
            </p>
            <p className="text-sm text-primary">{submitResult.contact_info}</p>
            <div>
              <p className="text-xs text-muted mb-1">{t('onboarding.confirm.copyTemplateHint')}</p>
              <CopyableText
                text={t('onboarding.confirm.contactTemplate', {
                  id: submitResult.public_id,
                  provider: formData.providerName,
                })}
              />
            </div>
          </div>
        )}

        {/* Reset button */}
        <div className="flex justify-center pt-2">
          <button
            type="button"
            onClick={onReset}
            className="flex items-center gap-2 px-6 py-3 bg-accent/10 border border-accent/40 text-accent rounded-lg font-medium hover:bg-accent/20 transition-colors"
          >
            <RotateCcw className="w-4 h-4" />
            {t('onboarding.confirm.newSubmission')}
          </button>
        </div>
      </div>
    );
  }

  // Pre-submission: review summary
  return (
    <div className="bg-surface border border-muted rounded-lg p-6 space-y-6">
      <h2 className="text-xl font-semibold text-primary">
        {t('onboarding.confirm.title')}
      </h2>
      <p className="text-sm text-secondary">{t('onboarding.confirm.description')}</p>

      {/* Provider info summary */}
      <div className="bg-elevated rounded-lg p-4 space-y-1 divide-y divide-muted/20">
        <h3 className="text-sm font-semibold text-primary pb-2">
          {t('onboarding.confirm.sectionProvider')}
        </h3>
        <SummaryRow
          label={t('onboarding.providerInfo.providerName')}
          value={formData.providerName}
        />
        <SummaryRow
          label={t('onboarding.providerInfo.websiteUrl')}
          value={formData.websiteUrl}
        />
        <SummaryRow
          label={t('onboarding.providerInfo.category')}
          value={t(`onboarding.providerInfo.categories.${formData.category}`)}
        />
        <SummaryRow
          label={t('onboarding.providerInfo.serviceType')}
          value={t(`onboarding.providerInfo.serviceTypes.${formData.serviceType}`, { defaultValue: formData.serviceType.toUpperCase() })}
        />
        <SummaryRow
          label={t('onboarding.providerInfo.sponsorLevel')}
          value={t(`onboarding.providerInfo.sponsorLevels.${formData.sponsorLevel}`, { defaultValue: formData.sponsorLevel })}
        />
        <SummaryRow
          label={t('onboarding.providerInfo.channelCodePreview')}
          value={
            <code className="px-2 py-0.5 bg-accent/10 border border-accent/30 rounded text-accent font-mono font-bold">
              {channelCode}
            </code>
          }
        />
      </div>

      {/* Connection info summary */}
      <div className="bg-elevated rounded-lg p-4 space-y-1 divide-y divide-muted/20">
        <h3 className="text-sm font-semibold text-primary pb-2">
          {t('onboarding.confirm.sectionConnection')}
        </h3>
        <SummaryRow
          label={t('onboarding.connectionTest.baseUrl')}
          value={formData.baseUrl}
        />
        <SummaryRow
          label={t('onboarding.connectionTest.apiKey')}
          value={
            <span className="font-mono text-xs">{maskApiKey(formData.apiKey)}</span>
          }
        />
        <SummaryRow
          label={t('onboarding.connectionTest.testType')}
          value={formData.testVariant || formData.testType}
        />
      </div>

      {/* Navigation buttons */}
      <div className="flex justify-between pt-2">
        <button
          type="button"
          onClick={onBack}
          className="flex items-center gap-2 px-6 py-3 bg-surface border border-muted text-secondary rounded-lg hover:bg-elevated transition-colors"
        >
          <ChevronLeft className="w-4 h-4" />
          {t('onboarding.back')}
        </button>
        <button
          type="button"
          onClick={onSubmit}
          disabled={isSubmitting}
          className="px-6 py-3 bg-accent text-white rounded-lg font-medium hover:bg-accent-strong transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {isSubmitting ? t('onboarding.confirm.submitting') : t('onboarding.confirm.submit')}
        </button>
      </div>
    </div>
  );
}
