/**
 * 移除数字末尾多余的零
 */
function formatNum(n: number): string {
  const str = n.toFixed(3);
  return str.replace(/\.?0+$/, '');
}

/**
 * 格式化参考倍率显示（简单字符串版本）
 * @param priceMin 倍率下限
 * @param priceMax 倍率上限
 * @returns 格式化字符串，如 "0.2" 或 "≤0.2 · 0.05~"
 */
export function formatPriceRatio(
  priceMin: number | null | undefined,
  priceMax: number | null | undefined
): string {
  if (priceMin == null && priceMax == null) return '-';

  // 只有下限
  if (priceMin != null && priceMax == null) {
    return `${formatNum(priceMin)}~`;
  }

  // 只有上限
  if (priceMin == null && priceMax != null) {
    return `≤${formatNum(priceMax)}`;
  }

  // 两者都有且相等
  if (priceMin === priceMax) {
    return formatNum(priceMin!);
  }

  // 两者都有且不同：上限 · 下限起点
  return `≤${formatNum(priceMax!)} · ${formatNum(priceMin!)}~`;
}

/**
 * 格式化参考倍率（结构化版本，上限为主、下限为辅）
 * @param priceMin 倍率下限
 * @param priceMax 倍率上限
 * @returns { base: string, sub?: string } 或 null
 *   - base: 主显示（上限，如 "≤0.2"）
 *   - sub: 辅助显示（下限起点，如 "0.05~"）
 */
export function formatPriceRatioStructured(
  priceMin: number | null | undefined,
  priceMax: number | null | undefined
): { base: string; sub?: string } | null {
  if (priceMin == null && priceMax == null) return null;

  // 只有下限
  if (priceMin != null && priceMax == null) {
    return { base: `${formatNum(priceMin)}~` };
  }

  // 只有上限
  if (priceMin == null && priceMax != null) {
    return { base: `≤${formatNum(priceMax)}` };
  }

  // 两者都有且相等
  if (priceMin === priceMax) {
    return { base: formatNum(priceMin!) };
  }

  // 两者都有且不同：上限为主，下限起点为辅
  return {
    base: `≤${formatNum(priceMax!)}`,
    sub: `${formatNum(priceMin!)}~`,
  };
}
