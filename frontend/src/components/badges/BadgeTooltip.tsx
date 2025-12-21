import { createPortal } from 'react-dom';
import { useRef, useLayoutEffect, type ReactNode } from 'react';
import type { TooltipPosition } from '../../hooks/useBadgeTooltip';

interface BadgeTooltipProps {
  isOpen: boolean;
  position: TooltipPosition;
  children: ReactNode;
}

const VIEWPORT_PADDING = 8; // 距离视口边缘最小间距

/**
 * 徽标 Tooltip 组件
 * - Portal 渲染到 body，脱离父容器 overflow 限制
 * - 支持向上/向下两种方向
 * - 自动检测水平边界，防止超出视口
 * - 700ms 延迟由 useBadgeTooltip 控制
 */
export function BadgeTooltip({ isOpen, position, children }: BadgeTooltipProps) {
  const tooltipRef = useRef<HTMLDivElement>(null);

  const { x, y, placement } = position;

  // 测量 tooltip 宽度并直接调整 DOM 位置（useLayoutEffect 在 paint 前执行）
  useLayoutEffect(() => {
    if (!isOpen || !tooltipRef.current) return;

    const tooltip = tooltipRef.current;
    const tooltipWidth = tooltip.offsetWidth;
    const viewportWidth = window.innerWidth;

    // 计算居中后的左边界位置
    const centeredLeft = x - tooltipWidth / 2;
    const centeredRight = x + tooltipWidth / 2;

    let adjustedX = x;

    // 检查左边界
    if (centeredLeft < VIEWPORT_PADDING) {
      adjustedX = VIEWPORT_PADDING + tooltipWidth / 2;
    }
    // 检查右边界
    else if (centeredRight > viewportWidth - VIEWPORT_PADDING) {
      adjustedX = viewportWidth - VIEWPORT_PADDING - tooltipWidth / 2;
    }

    // 直接修改 DOM，避免 re-render
    if (adjustedX !== x) {
      tooltip.style.left = `${adjustedX}px`;
    }
  }, [isOpen, x]);

  if (!isOpen) return null;

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
      ref={tooltipRef}
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
