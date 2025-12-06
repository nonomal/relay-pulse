import { useState } from 'react';
import { Github, Tag, ChevronDown, ChevronUp, Bug, Zap, Handshake, ShieldCheck, AlertTriangle } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useVersionInfo } from '../hooks/useVersionInfo';
import { FEEDBACK_URLS } from '../constants';

export function Footer() {
  const { t } = useTranslation();
  const { versionInfo } = useVersionInfo();
  const [expanded, setExpanded] = useState(false);

  const notices = [
    {
      icon: Zap,
      title: t('footer.disclaimer.dataReference.title'),
      text: t('footer.disclaimer.dataReference.text'),
    },
    {
      icon: Handshake,
      title: t('footer.disclaimer.neutralMaintenance.title'),
      text: t('footer.disclaimer.neutralMaintenance.text'),
    },
    {
      icon: ShieldCheck,
      title: t('footer.disclaimer.monitoringScope.title'),
      text: t('footer.disclaimer.monitoringScope.text'),
    },
    {
      icon: AlertTriangle,
      title: t('footer.disclaimer.liability.title'),
      text: t('footer.disclaimer.liability.text'),
    },
  ];

  return (
    <footer className="mt-4 bg-slate-900/60 border border-slate-800 rounded-2xl p-4 sm:p-5 text-slate-400">
      {/* 免责声明标题 - 移动端可折叠 */}
      <button
        onClick={() => setExpanded(!expanded)}
        className="sm:hidden w-full flex items-center justify-between text-sm font-semibold text-slate-200 mb-2"
      >
        <span>{t('footer.disclaimer.title')}</span>
        {expanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
      </button>
      <div className="hidden sm:block text-sm font-semibold text-slate-200 mb-3">{t('footer.disclaimer.title')}</div>

      {/* 免责声明内容 - 移动端折叠 */}
      <div className={`${expanded ? 'block' : 'hidden'} sm:block`}>
        <div className="grid gap-2 sm:gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {notices.map(({ icon: Icon, title, text }) => (
            <div
              key={title}
              className="flex items-start gap-2 sm:gap-3 bg-slate-900/40 border border-slate-800 rounded-xl p-2.5 sm:p-3"
            >
              <div className="p-1.5 sm:p-2 rounded-lg bg-slate-800/80 text-cyan-300 flex-shrink-0">
                <Icon size={14} className="sm:w-4 sm:h-4" />
              </div>
              <div className="text-[11px] sm:text-xs leading-relaxed">
                <div className="font-semibold text-slate-200 mb-0.5 sm:mb-1">{title}</div>
                <p className="text-slate-500">{text}</p>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* GitHub 链接与版本信息 */}
      <div className={`${expanded ? 'mt-4 pt-4' : 'mt-2 pt-2 sm:mt-4 sm:pt-4'} border-t border-slate-800/50 flex flex-col sm:flex-row items-center justify-center gap-2 text-xs`}>
        <div className="flex items-center gap-2 flex-wrap justify-center">
          <a
            href="https://github.com/prehisle/relay-pulse"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800/50 border border-slate-700/50 text-slate-300 hover:text-cyan-300 hover:border-cyan-500/30 transition min-h-[36px]"
          >
            <Github size={14} />
            <span>GitHub</span>
          </a>
          <span className="hidden sm:inline text-slate-600">·</span>
          <a
            href={FEEDBACK_URLS.BUG_REPORT}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800/50 border border-slate-700/50 text-slate-300 hover:text-rose-300 hover:border-rose-500/30 transition min-h-[36px]"
          >
            <Bug size={14} />
            <span>{t('footer.issuesBtn')}</span>
          </a>
          <span className="hidden sm:inline text-slate-600">·</span>
          <span className="text-slate-500 text-[11px] sm:text-xs">{t('footer.openSourceLabel')}</span>
        </div>
        {versionInfo && (
          <>
            <span className="hidden sm:inline text-slate-600">·</span>
            <div
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800/50 border border-slate-700/50 text-slate-400"
              title={`Commit: ${versionInfo.git_commit} | Built: ${versionInfo.build_time}`}
            >
              <Tag size={14} className="text-slate-500" />
              <span className="text-slate-400">{versionInfo.version}</span>
            </div>
          </>
        )}
      </div>
    </footer>
  );
}
