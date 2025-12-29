import { useMemo } from 'react';
import {
  LANGUAGE_PATH_MAP,
  isSupportedLanguage,
  type SupportedLanguage,
} from '../i18n';

/** Hreflang 语言标签类型 */
export type SeoHreflang = 'zh-CN' | 'en' | 'ru' | 'ja' | 'x-default';

/** Hreflang 备用链接 */
export interface SeoAlternateLink {
  hreflang: SeoHreflang;
  href: string;
}

/** SEO Meta Hook 返回值 */
export interface SeoMetaResult {
  /** 用于 <html lang="..."> 的语言码 */
  htmlLang: SupportedLanguage;
  /** canonical URL（去除查询参数） */
  canonical: string;
  /** hreflang 备用链接 */
  alternates: SeoAlternateLink[];
}

/** SupportedLanguage → hreflang 标签映射 */
const HREFLANG_MAP: Record<SupportedLanguage, Exclude<SeoHreflang, 'x-default'>> = {
  'zh-CN': 'zh-CN',
  'en-US': 'en',
  'ru-RU': 'ru',
  'ja-JP': 'ja',
};

/** 有序语言列表（用于生成 hreflang） */
const ORDERED_LANGUAGES: SupportedLanguage[] = ['zh-CN', 'en-US', 'ru-RU', 'ja-JP'];

/** 已知的语言路径前缀集合 */
const KNOWN_PATH_PREFIXES = new Set(
  Object.values(LANGUAGE_PATH_MAP).filter((p): p is string => Boolean(p))
);

/**
 * 清理路径名：去除查询参数和 hash
 */
function sanitizePathname(input: string): string {
  const noQuery = input.split('?')[0].split('#')[0];
  if (!noQuery) return '/';
  return noQuery.startsWith('/') ? noQuery : `/${noQuery}`;
}

/**
 * 去除路径中的语言前缀
 * @example '/en/p/foo' → '/p/foo'
 * @example '/en' → '/'
 * @example '/p/foo' → '/p/foo'（无前缀）
 */
function stripLanguagePrefix(pathname: string): string {
  const segments = pathname.split('/').filter(Boolean);
  if (segments.length === 0) return '/';

  const first = segments[0];
  if (!KNOWN_PATH_PREFIXES.has(first)) return pathname;

  const rest = segments.slice(1).join('/');
  return rest ? `/${rest}` : '/';
}

/**
 * 规范化语言根路径的尾斜杠
 * @example '/en' → '/en/'
 * @example '/en/p/foo' → '/en/p/foo'（子路径不变）
 * @example '/' → '/'
 */
function normalizeTrailingSlash(pathname: string): string {
  // 检查是否是语言根路径（如 /en, /ru, /ja）
  const segments = pathname.split('/').filter(Boolean);
  if (segments.length === 1 && KNOWN_PATH_PREFIXES.has(segments[0])) {
    return `/${segments[0]}/`;
  }
  return pathname;
}

/**
 * 为指定语言构建路径（语言前缀路径统一使用尾斜杠）
 * @example ('en-US', '/') → '/en/'
 * @example ('en-US', '/p/foo') → '/en/p/foo'
 * @example ('zh-CN', '/p/foo') → '/p/foo'（中文无前缀）
 */
function buildPathForLanguage(lang: SupportedLanguage, basePath: string): string {
  const prefix = LANGUAGE_PATH_MAP[lang];
  if (!prefix) return basePath;
  // 语言根路径使用尾斜杠（如 /en/），子路径不加（如 /en/p/foo）
  return basePath === '/' ? `/${prefix}/` : `/${prefix}${basePath}`;
}

/**
 * 将路径转换为绝对 URL
 */
function toAbsolute(origin: string, path: string): string {
  return origin ? `${origin}${path}` : path;
}

/**
 * SEO Meta 生成 Hook
 *
 * 生成 canonical URL 和 hreflang 备用链接，用于多语言 SEO 优化。
 *
 * @param params.pathname - 当前页面路径（必传，通常使用 location.pathname）
 * @param params.language - 当前语言（可选，默认 'zh-CN'）
 *
 * @example
 * const seo = useSeoMeta({ pathname: location.pathname, language: i18n.language });
 * // seo.canonical → 'https://relaypulse.top/en/'
 * // seo.alternates → [{ hreflang: 'zh-CN', href: '...' }, ...]
 */
export function useSeoMeta(params: { pathname: string; language?: string }): SeoMetaResult {
  const { pathname, language } = params;

  return useMemo(() => {
    const rawLang = language ?? '';
    const htmlLang: SupportedLanguage = isSupportedLanguage(rawLang) ? rawLang : 'zh-CN';

    const origin =
      typeof window === 'undefined'
        ? ''
        : window.location.origin;

    // 规范化路径：去除查询参数 + 语言根路径加尾斜杠
    const canonicalPath = normalizeTrailingSlash(sanitizePathname(pathname));

    const basePath = stripLanguagePrefix(canonicalPath);

    const alternates: SeoAlternateLink[] = ORDERED_LANGUAGES.map((lng) => ({
      hreflang: HREFLANG_MAP[lng],
      href: toAbsolute(origin, buildPathForLanguage(lng, basePath)),
    }));

    // x-default 指向默认语言（zh-CN，无前缀）
    alternates.push({
      hreflang: 'x-default',
      href: toAbsolute(origin, buildPathForLanguage('zh-CN', basePath)),
    });

    return {
      htmlLang,
      canonical: toAbsolute(origin, canonicalPath),
      alternates,
    };
  }, [pathname, language]);
}
