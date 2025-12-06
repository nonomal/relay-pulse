import { useState } from 'react';
import { Activity, CheckCircle, AlertTriangle, Sparkles, Globe, Bookmark, Share2, Filter, RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useLocation } from 'react-router-dom';
import { FEEDBACK_URLS } from '../constants';
import { SUPPORTED_LANGUAGES, LANGUAGE_PATH_MAP, isSupportedLanguage, type SupportedLanguage } from '../i18n';
import { FlagIcon } from './FlagIcon';
import { useToast } from './Toast';
import { shareCurrentPage, getBookmarkShortcut } from '../utils/share';

interface HeaderProps {
  stats: {
    total: number;
    healthy: number;
    issues: number;
  };
  // 移动端筛选/刷新相关（可选，用于合并到 Header）
  onFilterClick?: () => void;
  onRefresh?: () => void;
  loading?: boolean;
  refreshCooldown?: boolean;
  activeFiltersCount?: number;
}

export function Header({ stats, onFilterClick, onRefresh, loading, refreshCooldown, activeFiltersCount = 0 }: HeaderProps) {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { showToast } = useToast();

  // 移动端语言下拉菜单状态
  const [showMobileLangMenu, setShowMobileLangMenu] = useState(false);

  // 获取当前语言，使用类型守卫确保类型安全
  const currentLang: SupportedLanguage = isSupportedLanguage(i18n.language) ? i18n.language : 'zh-CN';

  // 处理收藏按钮点击
  const handleBookmark = () => {
    const shortcut = getBookmarkShortcut();
    showToast(t('share.bookmarkHint', { shortcut }), 'info');
  };

  // 处理分享按钮点击
  const handleShare = async () => {
    const result = await shareCurrentPage();
    if (result.method === 'cancelled') {
      // 用户取消分享，静默处理
      return;
    }
    if (result.success) {
      if (result.method === 'copy') {
        showToast(t('share.linkCopied'), 'success');
      }
      // Web Share API 成功时不需要提示，系统会处理
    } else {
      showToast(t('share.copyFailed'), 'error');
    }
  };

  // 语言简称显示（按钮和下拉项共用）
  const getLanguageShortLabel = (lang: string): string => {
    switch (lang) {
      case 'zh-CN':
        return 'CN';
      case 'en-US':
        return 'EN';
      case 'ru-RU':
        return 'RU';
      case 'ja-JP':
        return 'JA';
      default:
        return lang;
    }
  };

  /**
   * 处理语言切换
   *
   * 逻辑：
   * 1. 移除当前语言的路径前缀（如果有）
   * 2. 添加新语言的路径前缀（中文除外）
   * 3. 保留查询参数和 hash
   * 4. 导航到新路径并更新 i18n 语言状态
   *
   * 示例：
   * - 中文 → 英文：/ → /en/
   * - 英文 → 俄语：/en/docs → /ru/docs
   * - 俄语 → 中文：/ru/docs → /docs
   */
  const handleLanguageChange = (newLang: SupportedLanguage) => {
    // 获取当前语言，使用类型守卫确保类型安全
    const rawLang = i18n.language;
    const currentLang: SupportedLanguage = isSupportedLanguage(rawLang) ? rawLang : 'zh-CN';

    // 构建新路径
    let newPath = location.pathname;
    const queryString = location.search + location.hash;

    // 移除当前语言前缀（如果有）
    const currentPrefix = LANGUAGE_PATH_MAP[currentLang];
    if (currentPrefix && newPath.startsWith(`/${currentPrefix}`)) {
      newPath = newPath.substring(`/${currentPrefix}`.length) || '/';
    }

    // 添加新语言前缀（中文除外）
    const newPrefix = LANGUAGE_PATH_MAP[newLang];
    if (newPrefix) {
      newPath = `/${newPrefix}${newPath === '/' ? '' : newPath}`;
    }

    // 更新 i18n 语言状态
    i18n.changeLanguage(newLang);

    // 导航到新路径
    navigate(newPath + queryString);
  };

  return (
    <header className="flex flex-col gap-1 sm:gap-2 mb-3 border-b border-slate-800/50 pb-2">
      {/* 第一行：Logo + 标题 + 操作按钮（桌面端右侧完整显示） */}
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2 sm:gap-3">
            <div className="p-1.5 sm:p-2 bg-cyan-500/10 rounded-lg border border-cyan-500/20 flex-shrink-0">
              <Activity className="w-5 h-5 sm:w-6 sm:h-6 text-cyan-400" />
            </div>
            <h1 className="text-2xl sm:text-3xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-cyan-400 via-blue-400 to-purple-400">
              RelayPulse
            </h1>
          </div>
          {/* 移动端 Tagline - 作为副标题 */}
          <p className="sm:hidden text-[10px] text-slate-500 mt-1 flex items-center gap-1.5 pl-1">
            <span className="inline-block w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse flex-shrink-0"></span>
            <span className="truncate">{t('header.tagline')}</span>
          </p>
        </div>

        {/* 移动端：紧凑操作按钮（语言国旗 + 收藏 + 分享） */}
        <div className="flex items-center gap-1 sm:hidden flex-shrink-0">
          {/* 语言切换器 - 点击展开 */}
          <div className="relative">
            <button
              onClick={() => setShowMobileLangMenu(!showMobileLangMenu)}
              className="p-2 rounded-lg border border-slate-700 bg-slate-800/50 hover:bg-slate-700/50 hover:border-slate-600 transition-all duration-200"
              aria-label={t('accessibility.changeLanguage')}
              aria-expanded={showMobileLangMenu}
            >
              <FlagIcon language={currentLang} className="w-5 h-auto" />
            </button>
            {/* 下拉菜单 */}
            {showMobileLangMenu && (
              <>
                {/* 点击外部关闭 */}
                <div
                  className="fixed inset-0 z-40"
                  onClick={() => setShowMobileLangMenu(false)}
                />
                <div className="absolute right-0 mt-2 w-24 py-2 bg-slate-800 border border-slate-700 rounded-lg shadow-xl z-50">
                  {SUPPORTED_LANGUAGES.map((lang) => (
                    <button
                      key={lang}
                      onClick={() => {
                        handleLanguageChange(lang);
                        setShowMobileLangMenu(false);
                      }}
                      className={`w-full px-3 py-2 text-left flex items-center gap-2 hover:bg-slate-700/50 transition-colors ${
                        currentLang === lang ? 'bg-slate-700/30' : ''
                      }`}
                    >
                      <FlagIcon language={lang} className="w-5 h-auto flex-shrink-0" />
                      <span className="text-xs font-medium text-slate-300">{getLanguageShortLabel(lang)}</span>
                    </button>
                  ))}
                </div>
              </>
            )}
          </div>
          <button
            onClick={handleBookmark}
            className="p-2 rounded-lg border border-slate-700 bg-slate-800/50 text-slate-400 hover:text-slate-200 hover:bg-slate-700/50 hover:border-slate-600 transition-all duration-200"
            aria-label={t('share.bookmark')}
          >
            <Bookmark size={16} />
          </button>
          <button
            onClick={handleShare}
            className="p-2 rounded-lg border border-slate-700 bg-slate-800/50 text-slate-400 hover:text-slate-200 hover:bg-slate-700/50 hover:border-slate-600 transition-all duration-200"
            aria-label={t('share.share')}
          >
            <Share2 size={16} />
          </button>
        </div>

        {/* 桌面端：右侧完整操作区（语言 + 收藏 + 分享 + 推荐 + 统计卡片） */}
        <div className="hidden sm:flex items-center gap-3 flex-shrink-0">
          {/* 语言切换器 */}
          <div className="relative inline-block group">
            <button
              className="inline-flex items-center gap-2 px-3 py-2 rounded-lg border border-slate-700 bg-slate-800/50 text-slate-300 hover:bg-slate-700/50 hover:border-slate-600 transition-all duration-200"
              aria-label={t('accessibility.changeLanguage')}
            >
              <Globe size={16} className="text-slate-400" />
              <span className="text-sm font-medium">
                {getLanguageShortLabel(currentLang)}
              </span>
              <svg className="w-4 h-4 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
              </svg>
            </button>
            <div className="absolute left-0 mt-2 w-full py-2 bg-slate-800 border border-slate-700 rounded-lg shadow-xl opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-200 z-50">
              {SUPPORTED_LANGUAGES.map((lang) => (
                <button
                  key={lang}
                  onClick={() => handleLanguageChange(lang)}
                  className={`w-full px-3 py-2.5 text-left flex items-center gap-2.5 hover:bg-slate-700/50 transition-colors ${
                    currentLang === lang ? 'bg-slate-700/30 text-cyan-400' : 'text-slate-300'
                  }`}
                >
                  <FlagIcon language={lang} className="w-5 h-auto flex-shrink-0" />
                  <span className="text-sm font-medium leading-none">{getLanguageShortLabel(lang)}</span>
                  {currentLang === lang && (
                    <svg className="w-3.5 h-3.5 ml-auto flex-shrink-0 text-cyan-400" fill="currentColor" viewBox="0 0 20 20">
                      <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                    </svg>
                  )}
                </button>
              ))}
            </div>
          </div>

          {/* 收藏和分享按钮 */}
          <div className="flex gap-1">
            <button
              onClick={handleBookmark}
              className="p-2 rounded-lg border border-slate-700 bg-slate-800/50 text-slate-400 hover:text-slate-200 hover:bg-slate-700/50 hover:border-slate-600 transition-all duration-200"
              aria-label={t('share.bookmark')}
              title={t('share.bookmark')}
            >
              <Bookmark size={16} />
            </button>
            <button
              onClick={handleShare}
              className="p-2 rounded-lg border border-slate-700 bg-slate-800/50 text-slate-400 hover:text-slate-200 hover:bg-slate-700/50 hover:border-slate-600 transition-all duration-200"
              aria-label={t('share.share')}
              title={t('share.share')}
            >
              <Share2 size={16} />
            </button>
          </div>

          {/* 推荐按钮 */}
          <a
            href={FEEDBACK_URLS.PROVIDER_SUGGESTION}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 px-4 py-2 rounded-xl border border-cyan-500/40 bg-cyan-500/10 text-cyan-200 font-semibold tracking-wide shadow-[0_0_12px_rgba(6,182,212,0.25)] hover:bg-cyan-500/20 transition"
          >
            <Sparkles size={14} />
            {t('header.recommendBtn')}
          </a>

          {/* 统计卡片 */}
          <div className="flex gap-4">
            {/* 正常运行 */}
            <div className="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-slate-900/50 border border-slate-800 backdrop-blur-sm shadow-lg">
              <div className="p-1.5 rounded-full bg-emerald-500/10 text-emerald-400">
                <CheckCircle size={16} />
              </div>
              <span className="font-mono font-bold text-emerald-400 text-base">{stats.healthy}</span>
              <span className="text-slate-400 text-xs">{t('header.stats.healthy')}</span>
            </div>
            {/* 异常告警 */}
            <div className="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-slate-900/50 border border-slate-800 backdrop-blur-sm shadow-lg">
              <div className="p-1.5 rounded-full bg-rose-500/10 text-rose-400">
                <AlertTriangle size={16} />
              </div>
              <span className="font-mono font-bold text-rose-400 text-base">{stats.issues}</span>
              <span className="text-slate-400 text-xs">{t('header.stats.issues')}</span>
            </div>
          </div>
        </div>
      </div>

      {/* 桌面端 Tagline */}
      <p className="hidden sm:flex text-slate-400 text-sm items-center gap-2">
        <span className="inline-block w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
        {t('header.tagline')}
      </p>

      {/* 移动端：统计卡片 + 筛选/刷新 + 推荐按钮 */}
      <div className="flex items-center gap-1.5 sm:hidden">
        {/* 统计卡片 - 移动端极简模式（仅图标+数字） */}
        <div className="flex gap-1.5">
          {/* 正常运行 */}
          <div className="flex items-center gap-1 px-1.5 py-1 rounded-lg bg-slate-900/50 border border-slate-800 backdrop-blur-sm shadow-lg"
               title={t('header.stats.healthy')}>
            <div className="p-0.5 rounded-full bg-emerald-500/10 text-emerald-400">
              <CheckCircle size={12} />
            </div>
            <span className="font-mono font-bold text-emerald-400 text-xs">{stats.healthy}</span>
          </div>
          {/* 异常告警 */}
          <div className="flex items-center gap-1 px-1.5 py-1 rounded-lg bg-slate-900/50 border border-slate-800 backdrop-blur-sm shadow-lg"
               title={t('header.stats.issues')}>
            <div className="p-0.5 rounded-full bg-rose-500/10 text-rose-400">
              <AlertTriangle size={12} />
            </div>
            <span className="font-mono font-bold text-rose-400 text-xs">{stats.issues}</span>
          </div>
        </div>

        {/* 移动端：筛选按钮 */}
        {onFilterClick && (
          <button
            onClick={onFilterClick}
            className="flex items-center gap-1 px-2 py-1 bg-slate-800 text-slate-300 rounded-lg border border-slate-700 hover:bg-slate-750 transition-colors text-xs"
            title={t('controls.mobile.filterBtn')}
          >
            <Filter size={12} />
            <span>{t('controls.mobile.filterBtnShort')}</span>
            {activeFiltersCount > 0 && (
              <span className="px-1 py-0.5 bg-cyan-500 text-white text-[10px] rounded-full leading-none">
                {activeFiltersCount}
              </span>
            )}
          </button>
        )}

        {/* 移动端：刷新按钮 */}
        {onRefresh && (
          <div className="relative">
            <button
              onClick={onRefresh}
              className="p-1.5 rounded-lg bg-cyan-500/10 text-cyan-400 hover:bg-cyan-500/20 transition-colors border border-cyan-500/20"
              title={t('common.refresh')}
            >
              <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
            </button>
            {refreshCooldown && (
              <div className="absolute top-full left-1/2 -translate-x-1/2 mt-1 px-2 py-1 bg-slate-800 text-slate-300 text-[10px] rounded whitespace-nowrap shadow-lg border border-slate-700 z-50">
                {t('common.refreshCooldown')}
              </div>
            )}
          </div>
        )}

        {/* 推荐按钮 - 移动端紧凑版 */}
        <a
          href={FEEDBACK_URLS.PROVIDER_SUGGESTION}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-1 px-2 py-1 rounded-lg border border-cyan-500/40 bg-cyan-500/10 text-cyan-200 text-xs font-medium shadow-[0_0_8px_rgba(6,182,212,0.2)] hover:bg-cyan-500/20 transition whitespace-nowrap ml-auto"
        >
          <Sparkles size={12} />
          {t('header.recommendBtnShort')}
        </a>
      </div>
    </header>
  );
}
