import type { Annotation, AnnotationFamily } from '../../types';
import { AnnotationChip } from './AnnotationChip';

interface AnnotationCellProps {
  annotations?: Annotation[];
  className?: string;
  tooltipPlacement?: 'top' | 'bottom';
}

function pickFamily(annotations: Annotation[], family: AnnotationFamily): Annotation[] {
  return annotations.filter((a) => a.family === family);
}

/**
 * 注解单元格 — 统一渲染一组注解
 *
 * 渲染顺序（后端已排序）：
 * 1. positive（正向，绿色）
 * 2. neutral（中性，蓝色）
 * 3. 分隔符 |（仅在正/中性 和 负向 都存在时显示）
 * 4. negative（负向，黄色/红色）
 */
export function AnnotationCell({
  annotations = [],
  className = '',
  tooltipPlacement = 'top',
}: AnnotationCellProps) {
  if (annotations.length === 0) return null;

  // 后端已保证排序（family → priority desc → id asc），这里只按 family 分组
  const positive = pickFamily(annotations, 'positive');
  const neutral = pickFamily(annotations, 'neutral');
  const negative = pickFamily(annotations, 'negative');
  const hasLeading = positive.length > 0 || neutral.length > 0;

  return (
    <div className={`flex items-center gap-0.5 ${className}`}>
      {positive.map((a) => (
        <AnnotationChip key={a.id} annotation={a} tooltipPlacement={tooltipPlacement} />
      ))}

      {neutral.map((a) => (
        <AnnotationChip key={a.id} annotation={a} tooltipPlacement={tooltipPlacement} />
      ))}

      {hasLeading && negative.length > 0 && (
        <span className="text-muted text-xs select-none mx-0.5">|</span>
      )}

      {negative.map((a) => (
        <AnnotationChip key={a.id} annotation={a} tooltipPlacement={tooltipPlacement} />
      ))}
    </div>
  );
}
