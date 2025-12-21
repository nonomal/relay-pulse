import { useState, useCallback, useRef, useEffect } from 'react';

export interface TooltipPosition {
  x: number;
  y: number;
  placement: 'top' | 'bottom';
}

/**
 * 徽标 Tooltip 悬停管理 Hook
 * - 700ms 延迟显示
 * - 自动计算位置（优先向上，空间不足向下）
 * - 支持强制指定方向
 */
export function useBadgeTooltip(
  triggerRef: React.RefObject<HTMLElement | null>,
  preferredPlacement: 'top' | 'bottom' | 'auto' = 'auto'
) {
  const [isOpen, setIsOpen] = useState(false);
  const [position, setPosition] = useState<TooltipPosition>({ x: 0, y: 0, placement: 'top' });
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const calculatePosition = useCallback(() => {
    if (!triggerRef.current) return null;

    const rect = triggerRef.current.getBoundingClientRect();

    // 计算上方空间，决定方向：优先向上，空间不足（< 40px）时向下
    const spaceAbove = rect.top;
    let placement: 'top' | 'bottom';
    if (preferredPlacement !== 'auto') {
      placement = preferredPlacement;
    } else {
      placement = spaceAbove >= 40 ? 'top' : 'bottom';
    }

    return {
      x: rect.left + rect.width / 2,
      y: placement === 'top' ? rect.top : rect.bottom,
      placement,
    };
  }, [triggerRef, preferredPlacement]);

  const handleMouseEnter = useCallback(() => {
    // 清除之前的定时器
    if (timerRef.current) {
      clearTimeout(timerRef.current);
    }

    // 700ms 延迟显示
    timerRef.current = setTimeout(() => {
      const pos = calculatePosition();
      if (pos) {
        setPosition(pos);
        setIsOpen(true);
      }
    }, 700);
  }, [calculatePosition]);

  const handleMouseLeave = useCallback(() => {
    // 清除定时器
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    setIsOpen(false);
  }, []);

  // 清理定时器
  useEffect(() => {
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
    };
  }, []);

  return {
    isOpen,
    position,
    handleMouseEnter,
    handleMouseLeave,
  };
}
