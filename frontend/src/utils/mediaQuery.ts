/**
 * 媒体查询工具 - 统一管理响应式断点
 * 兼容 Safari ≤13 (使用 addListener/removeListener 回退)
 */

export type Breakpoint = 'mobile' | 'tablet' | 'desktop';

/**
 * 响应式断点定义
 * mobile: < 768px (Tooltip 底部 Sheet vs 悬浮)
 * tablet: < 1024px (StatusTable 卡片 vs 表格，与 Tailwind lg: 断点一致)
 */
export const BREAKPOINTS = {
  mobile: '(max-width: 767px)',
  tablet: '(max-width: 1023px)',
} as const;

/**
 * 兼容 Safari ≤13 的 matchMedia 监听器包装
 */
export function addMediaQueryListener(
  mediaQuery: MediaQueryList,
  handler: (e: MediaQueryListEvent | MediaQueryList) => void
): () => void {
  // 现代浏览器使用 addEventListener
  if (mediaQuery.addEventListener) {
    mediaQuery.addEventListener('change', handler);
    return () => mediaQuery.removeEventListener('change', handler);
  }

  // Safari ≤13 回退到 addListener（现代 TypeScript 已支持）
  if (mediaQuery.addListener) {
    mediaQuery.addListener(handler);
    return () => mediaQuery.removeListener(handler);
  }

  // 降级方案：返回空清理函数
  console.warn('matchMedia listeners not supported');
  return () => {};
}

/**
 * 创建响应式断点 Hook 辅助函数
 *
 * @param breakpoint 断点类型
 * @param callback 断点变化回调
 * @returns 清理函数
 */
export function createMediaQueryEffect(
  breakpoint: keyof typeof BREAKPOINTS,
  callback: (matches: boolean) => void
): () => void {
  const mediaQuery = window.matchMedia(BREAKPOINTS[breakpoint]);

  // 立即调用一次
  callback(mediaQuery.matches);

  // 添加监听器
  const cleanup = addMediaQueryListener(mediaQuery, (e) => {
    callback(e.matches);
  });

  return cleanup;
}
