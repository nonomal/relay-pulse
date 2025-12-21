import { createPortal } from 'react-dom';
import type { ReactNode } from 'react';
import type { TooltipPosition } from '../../hooks/useBadgeTooltip';

interface BadgeTooltipProps {
  isOpen: boolean;
  position: TooltipPosition;
  children: ReactNode;
}

/**
 * 徽标 Tooltip 组件
 * - Portal 渲染到 body，脱离父容器 overflow 限制
 * - 支持向上/向下两种方向
 * - 700ms 延迟由 useBadgeTooltip 控制
 */
export function BadgeTooltip({ isOpen, position, children }: BadgeTooltipProps) {
  if (!isOpen) return null;

  const { x, y, placement } = position;

  // 向上时：tooltip 底部对齐触发器顶部
  // 向下时：tooltip 顶部对齐触发器底部
  const style: React.CSSProperties = {
    left: x,
    top: y,
    transform: placement === 'top'
      ? 'translate(-50%, -100%) translateY(-4px)'  // 向上 + 4px 间距
      : 'translate(-50%, 0) translateY(4px)',       // 向下 + 4px 间距
  };

  return createPortal(
    <div
      className="fixed z-[9999] pointer-events-none"
      style={style}
    >
      <div className="bg-elevated text-primary text-xs px-2 py-1 rounded whitespace-nowrap shadow-lg border border-default">
        {children}
      </div>
    </div>,
    document.body
  );
}
