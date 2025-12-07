/**
 * 移除数字末尾多余的零
 */
function formatNum(n: number): string {
  const str = n.toFixed(3);
  return str.replace(/\.?0+$/, '');
}

/**
 * 格式化承诺倍率显示（简单字符串版本）
 * @param priceMin 倍率下限
 * @param priceMax 倍率上限
 * @returns 格式化字符串，如 "0.125" 或 "0.05~0.2"
 */
export function formatPriceRatio(
  priceMin: number | null | undefined,
  priceMax: number | null | undefined
): string {
  if (priceMin == null && priceMax == null) return '-';

  // 只有下限
  if (priceMin != null && priceMax == null) {
    return formatNum(priceMin);
  }

  // 只有上限
  if (priceMin == null && priceMax != null) {
    return formatNum(priceMax);
  }

  // 两者都有
  if (priceMin === priceMax) {
    return formatNum(priceMin!);
  }

  // 显示区间
  return `${formatNum(priceMin!)}~${formatNum(priceMax!)}`;
}

/**
 * 格式化承诺倍率（结构化版本，支持中心值+区间显示）
 * @param priceMin 倍率下限
 * @param priceMax 倍率上限
 * @returns { base: string, range?: string } 或 null
 */
export function formatPriceRatioStructured(
  priceMin: number | null | undefined,
  priceMax: number | null | undefined
): { base: string; range?: string } | null {
  if (priceMin == null && priceMax == null) return null;

  // 只有下限
  if (priceMin != null && priceMax == null) {
    return { base: formatNum(priceMin) };
  }

  // 只有上限
  if (priceMin == null && priceMax != null) {
    return { base: formatNum(priceMax) };
  }

  // 两者都有
  if (priceMin === priceMax) {
    // 相等时只显示单个值，无区间
    return { base: formatNum(priceMin!) };
  }

  // 显示中心值 + 区间
  const center = (priceMin! + priceMax!) / 2;
  return {
    base: formatNum(center),
    range: `${formatNum(priceMin!)}~${formatNum(priceMax!)}`,
  };
}
