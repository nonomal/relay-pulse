/**
 * 格式化承诺倍率显示
 * @param ratio 基础倍率
 * @param variance 浮动范围（可选）
 * @returns 格式化字符串，如 "0.8" 或 "0.8±0.2"
 */
export function formatPriceRatio(
  ratio: number | null | undefined,
  variance: number | null | undefined
): string {
  if (ratio == null) return '-';

  // 移除末尾多余的零，保持简洁
  const formatNum = (n: number) => {
    const str = n.toFixed(3);
    return str.replace(/\.?0+$/, '');
  };

  const base = formatNum(ratio);
  if (variance != null && variance > 0) {
    return `${base}±${formatNum(variance)}`;
  }

  return base;
}
