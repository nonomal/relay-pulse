/**
 * 格式化数字，移除末尾多余的零
 */
export function formatNum(n: number): string {
  const str = n.toFixed(3);
  return str.replace(/\.?0+$/, '');
}

/**
 * 格式化承诺倍率显示（纯字符串版本，用于简单场景）
 * @param ratio 基础倍率
 * @param variance 浮动范围（可选）
 * @param emptyText 空值时的显示文本（默认 "-"）
 * @returns 格式化字符串，如 "0.8x" 或 "0.8x±0.1"
 */
export function formatPriceRatio(
  ratio: number | null | undefined,
  variance: number | null | undefined,
  emptyText = '-'
): string {
  if (ratio == null) return emptyText;

  const base = formatNum(ratio);
  if (variance != null && variance > 0) {
    return `${base}x±${formatNum(variance)}`;
  }

  return `${base}x`;
}

/**
 * 解析倍率为组件可用的部分
 * @returns null 表示无数据，否则返回 { base, variance? }
 */
export function parsePriceRatio(
  ratio: number | null | undefined,
  variance: number | null | undefined
): { base: string; variance?: string } | null {
  if (ratio == null) return null;

  const base = formatNum(ratio);
  if (variance != null && variance > 0) {
    return { base, variance: formatNum(variance) };
  }

  return { base };
}
