/**
 * 板块切换器组件
 *
 * 功能：
 * - 显示当前板块图标
 * - 点击/悬浮展开板块列表
 * - 支持桌面端 hover 和移动端 click
 *
 * 交互说明：
 * - 大屏（lg+）：支持 hover 展开菜单；同时仍支持 click 切换（用于键盘/无 hover 场景）
 * - 小屏（<lg）：仅通过 click 展开菜单（触控设备没有可靠 hover）
 */

import { useState, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Flame, BarChart2, Snowflake, Globe } from 'lucide-react';
import type { BoardFilter } from '../types';

/**
 * 获取板块图标
 */
function BoardIcon({ board, size = 14 }: { board: BoardFilter; size?: number }) {
  switch (board) {
    case 'hot':
      return <Flame size={size} />;
    case 'secondary':
      return <BarChart2 size={size} />;
    case 'cold':
      return <Snowflake size={size} />;
    case 'all':
      return <Globe size={size} />;
    default:
      return <Flame size={size} />;
  }
}

interface BoardSwitcherProps {
  board: BoardFilter;
  onBoardChange: (board: BoardFilter) => void;
  enabled: boolean;
}

function BoardSwitcherComponent({ board, onBoardChange, enabled }: BoardSwitcherProps) {
  const { t } = useTranslation();
  const [showMenu, setShowMenu] = useState(false);
  const [isHovering, setIsHovering] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // 菜单"实际可见"状态（用于 aria-expanded 与 ESC/outside-click 逻辑一致）
  // - click 打开：showMenu=true
  // - 桌面 hover 打开：isHovering=true（容器 mouseenter）
  const isMenuVisible = showMenu || isHovering;

  // 点击外部关闭菜单（点击态/hover 态都应可关闭）
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setShowMenu(false);
        setIsHovering(false);
      }
    }

    if (isMenuVisible) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [isMenuVisible]);

  // ESC 关闭菜单（点击态/hover 态都应可关闭）
  useEffect(() => {
    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setShowMenu(false);
        setIsHovering(false);
      }
    }

    if (isMenuVisible) {
      document.addEventListener('keydown', handleEscape);
      return () => document.removeEventListener('keydown', handleEscape);
    }
  }, [isMenuVisible]);

  if (!enabled) {
    return null;
  }

  const boards: BoardFilter[] = ['hot', 'secondary', 'cold', 'all'];

  const handleBoardChange = (newBoard: BoardFilter) => {
    onBoardChange(newBoard);
    setShowMenu(false);
    setIsHovering(false);
  };

  return (
    <>
      <div
        ref={menuRef}
        className="relative group"
        // 用 JS 维护 hover 状态：确保 aria-expanded 与"视觉展开"一致
        // 仅在 lg+ 断点下菜单会因为 CSS hover 展开；这里的状态对小屏不会产生负作用
        onMouseEnter={() => setIsHovering(true)}
        onMouseLeave={() => setIsHovering(false)}
      >
        {/* 触发按钮 - 仅图标 */}
        <button
          onClick={() => setShowMenu(!showMenu)}
          className="h-8 w-8 flex items-center justify-center rounded-lg bg-elevated/50 hover:bg-muted/50 transition-all duration-200 focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
          aria-label={t('controls.boards.selectBoard')}
          aria-expanded={isMenuVisible}
          aria-haspopup="listbox"
        >
          <BoardIcon board={board} size={14} />
        </button>

        {/* 下拉菜单 - 桌面端 hover 显示，移动端 click 显示 */}
        <div
          className={`
            absolute top-full right-0 mt-1 z-50
            bg-elevated border border-default rounded-lg shadow-xl
            transition-all duration-200
            ${showMenu ? 'opacity-100 visible translate-y-0' : 'opacity-0 invisible -translate-y-2'}
            lg:group-hover:opacity-100 lg:group-hover:visible lg:group-hover:translate-y-0
          `}
          role="listbox"
          aria-label={t('controls.boards.selectBoard')}
        >
          {boards.map((b) => (
            <button
              key={b}
              onClick={() => handleBoardChange(b)}
              className={`
                w-8 h-8 flex items-center justify-center
                transition-colors cursor-pointer first:rounded-t-lg last:rounded-b-lg
                focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none
                ${board === b ? 'bg-accent/20 text-accent' : 'text-primary hover:bg-muted/50'}
              `}
              role="option"
              aria-selected={board === b}
              aria-label={t(`controls.boards.${b}`)}
              title={t(`controls.boards.${b}`)}
            >
              <BoardIcon board={b} size={14} />
            </button>
          ))}
        </div>
      </div>

      {/* 分隔线 */}
      <div className="w-px h-5 bg-muted mx-1"></div>
    </>
  );
}

// 导出组件和 Icon 子组件
export const BoardSwitcher = Object.assign(BoardSwitcherComponent, {
  Icon: BoardIcon,
});

export default BoardSwitcher;
