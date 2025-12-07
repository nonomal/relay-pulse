/**
 * 移除数字末尾多余的零
 */
function formatNum(n: number): string {
  const str = n.toFixed(3);
  return str.replace(/\.?0+$/, '');
}

/**
 * 格式化承诺倍率显示（简单字符串版本）
 * @param ratio 基础倍率
 * @param variance 浮动范围（可选）
 * @returns 格式化字符串，如 "0.8" 或 "0.8±0.2"
 */
export function formatPriceRatio(
  ratio: number | null | undefined,
  variance: number | null | undefined
): string {
  if (ratio == null) return '-';

  const base = formatNum(ratio);
  if (variance != null && variance > 0) {
    return `${base}±${formatNum(variance)}`;
  }

  return base;
}

/**
 * 格式化承诺倍率（结构化版本，支持中心值+区间显示）
 * @returns { base: string, range?: string } 或 null
 */
export function formatPriceRatioStructured(
  ratio: number | null | undefined,
  variance: number | null | undefined
): { base: string; range?: string } | null {
  if (ratio == null) return null;

  const base = formatNum(ratio);

  if (variance != null && variance > 0) {
    const min = formatNum(Math.max(0, ratio - variance));
    const max = formatNum(ratio + variance);
    return { base, range: `${min}~${max}` };
  }

  return { base };
}
