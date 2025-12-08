/**
 * 主题管理 Hook
 *
 * 功能：
 * - 管理主题状态
 * - localStorage 持久化
 * - 更新 DOM 属性
 */

import { useState, useEffect, useCallback } from 'react';

export type ThemeId = 'default-dark' | 'night-dark' | 'light-cool' | 'light-warm';

export interface Theme {
  id: ThemeId;
  nameKey: string; // i18n key
  isDark: boolean;
}

export const THEMES: Theme[] = [
  { id: 'default-dark', nameKey: 'theme.defaultDark', isDark: true },
  { id: 'night-dark', nameKey: 'theme.nightDark', isDark: true },
  { id: 'light-cool', nameKey: 'theme.lightCool', isDark: false },
  { id: 'light-warm', nameKey: 'theme.lightWarm', isDark: false },
];

const STORAGE_KEY = 'relay-pulse-theme';
const DEFAULT_THEME: ThemeId = 'default-dark';

/**
 * 从 localStorage 获取保存的主题
 */
function getStoredTheme(): ThemeId {
  if (typeof window === 'undefined') return DEFAULT_THEME;

  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored && THEMES.some((t) => t.id === stored)) {
      return stored as ThemeId;
    }
  } catch {
    // localStorage 不可用（隐私模式/安全策略）
  }
  return DEFAULT_THEME;
}

/**
 * 应用主题到 DOM
 */
function applyTheme(themeId: ThemeId): void {
  const root = document.documentElement;
  const theme = THEMES.find((t) => t.id === themeId);

  // 设置 data-theme 属性
  root.setAttribute('data-theme', themeId);

  // 设置 color-scheme（影响浏览器原生控件）
  root.style.colorScheme = theme?.isDark ? 'dark' : 'light';
}

/**
 * 主题管理 Hook
 */
export function useTheme() {
  const [theme, setThemeState] = useState<ThemeId>(getStoredTheme);

  // 主题变化时应用到 DOM 和 localStorage
  useEffect(() => {
    applyTheme(theme);
    try {
      localStorage.setItem(STORAGE_KEY, theme);
    } catch {
      // localStorage 不可用（隐私模式/安全策略），降级为内存状态
    }
    // 触发自定义事件，供 color.ts 监听
    window.dispatchEvent(new CustomEvent('theme-change', { detail: theme }));
  }, [theme]);

  // setTheme 只更新 state，副作用由 useEffect 处理
  const setTheme = useCallback((newTheme: ThemeId) => {
    setThemeState(newTheme);
  }, []);

  const currentTheme = THEMES.find((t) => t.id === theme) || THEMES[0];

  return {
    theme,
    setTheme,
    themes: THEMES,
    currentTheme,
    isDark: currentTheme.isDark,
  };
}

/**
 * 获取当前主题 ID（非 Hook，用于工具函数）
 */
export function getCurrentThemeId(): ThemeId {
  if (typeof window === 'undefined') return DEFAULT_THEME;
  return (document.documentElement.getAttribute('data-theme') as ThemeId) || DEFAULT_THEME;
}

/**
 * 检查当前是否为暗色主题
 */
export function isDarkTheme(): boolean {
  const themeId = getCurrentThemeId();
  const theme = THEMES.find((t) => t.id === themeId);
  return theme?.isDark ?? true;
}
