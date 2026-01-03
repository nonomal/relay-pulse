import { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import type { SelfTestFormData, TestType } from '../types/selftest';

interface SelfTestFormProps {
  onSubmit: (data: SelfTestFormData) => void;
  isSubmitting: boolean;
  disabled?: boolean;
}

export const SelfTestForm: React.FC<SelfTestFormProps> = ({ onSubmit, isSubmitting, disabled }) => {
  const { t } = useTranslation();
  const [testTypes, setTestTypes] = useState<TestType[]>([]);
  const [formData, setFormData] = useState<SelfTestFormData>({
    testType: 'cc',
    apiUrl: '',
    apiKey: '',
  });

  // 加载测试类型列表
  useEffect(() => {
    fetch('/api/selftest/types')
      .then((res) => res.json())
      .then((data: TestType[]) => {
        setTestTypes(data);
        if (data.length > 0 && !formData.testType) {
          setFormData((prev) => ({ ...prev, testType: data[0].id }));
        }
      })
      .catch((err) => {
        console.error('Failed to load test types:', err);
      });
  }, []);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!formData.testType || !formData.apiUrl || !formData.apiKey) {
      return;
    }
    onSubmit(formData);
  };

  const isDisabled = disabled || isSubmitting;

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      {/* 测试类型选择 */}
      <div>
        <label htmlFor="testType" className="block text-sm font-medium text-primary mb-2">
          {t('selftest.form.testType')}
        </label>
        <select
          id="testType"
          value={formData.testType}
          onChange={(e) => setFormData({ ...formData, testType: e.target.value })}
          disabled={isDisabled}
          className="w-full px-4 py-2 bg-surface border border-muted rounded-lg text-primary focus:outline-none focus:ring-2 focus:ring-accent disabled:opacity-50"
        >
          {testTypes.map((type) => (
            <option key={type.id} value={type.id}>
              {type.name}
            </option>
          ))}
        </select>
        {/* 显示描述：优先后端返回，其次 i18n fallback */}
        {(() => {
          const selectedType = testTypes.find((t) => t.id === formData.testType);
          const description =
            selectedType?.description ||
            t(`selftest.testTypeDescriptions.${formData.testType}`, { defaultValue: '' });
          return description ? (
            <p className="mt-1 text-sm text-secondary">{description}</p>
          ) : null;
        })()}
      </div>

      {/* API URL 输入 */}
      <div>
        <label htmlFor="apiUrl" className="block text-sm font-medium text-primary mb-2">
          {t('selftest.form.apiUrl')}
        </label>
        <input
          id="apiUrl"
          type="url"
          placeholder={t('selftest.form.apiUrlPlaceholder')}
          value={formData.apiUrl}
          onChange={(e) => setFormData({ ...formData, apiUrl: e.target.value })}
          disabled={isDisabled}
          required
          className="w-full px-4 py-2 bg-surface border border-muted rounded-lg text-primary placeholder-muted focus:outline-none focus:ring-2 focus:ring-accent disabled:opacity-50"
        />
        <p className="mt-1 text-sm text-secondary">{t('selftest.form.apiUrlHint')}</p>
      </div>

      {/* API Key 输入 */}
      <div>
        <label htmlFor="apiKey" className="block text-sm font-medium text-primary mb-2">
          {t('selftest.form.apiKey')}
        </label>
        <input
          id="apiKey"
          type="password"
          placeholder={t('selftest.form.apiKeyPlaceholder')}
          value={formData.apiKey}
          onChange={(e) => setFormData({ ...formData, apiKey: e.target.value })}
          disabled={isDisabled}
          required
          className="w-full px-4 py-2 bg-surface border border-muted rounded-lg text-primary placeholder-muted focus:outline-none focus:ring-2 focus:ring-accent disabled:opacity-50"
        />
        <p className="mt-1 text-sm text-secondary">{t('selftest.form.apiKeyHint')}</p>
      </div>

      {/* 提交按钮 */}
      <button
        type="submit"
        disabled={isDisabled}
        className="w-full px-6 py-3 bg-accent text-white rounded-lg font-medium hover:bg-accent-strong transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {isSubmitting ? t('selftest.form.submitting') : t('selftest.form.submit')}
      </button>
    </form>
  );
};
