import { createPortal } from 'react-dom';
import type { TooltipPosition } from '../../hooks/useAnnotationTooltip';

interface AnnotationTooltipProps {
  isOpen: boolean;
  position: TooltipPosition;
  children: React.ReactNode;
}

/**
 * 注解 Tooltip（Portal 渲染，避免 overflow 裁剪）
 */
export function AnnotationTooltip({ isOpen, position, children }: AnnotationTooltipProps) {
  if (!isOpen) return null;

  const isTop = position.placement === 'top';

  return createPortal(
    <div
      className="fixed z-[9999] pointer-events-none"
      style={{
        left: position.x,
        top: position.y,
        transform: isTop
          ? 'translate(-50%, -100%) translateY(-6px)'
          : 'translate(-50%, 0) translateY(6px)',
      }}
    >
      <div className="px-2.5 py-1.5 bg-elevated border border-default rounded-lg shadow-xl text-xs text-primary whitespace-nowrap max-w-xs">
        {children}
      </div>
    </div>,
    document.body
  );
}
