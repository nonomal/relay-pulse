/**
 * 板块切换器组件（下拉菜单）
 *
 * 功能：
 * - 触发按钮显示当前板块的 emoji + 短标签 + 数量
 * - 点击/悬浮展开下拉列表，每项显示 emoji + 完整标签 + 数量
 * - board_counts 缺失时（旧后端）不显示数量
 *
 * 交互说明：
 * - 大屏（lg+）：支持 hover 展开菜单；同时仍支持 click 切换
 * - 小屏（<lg）：仅通过 click 展开菜单
 */

import { useState, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import type { BoardCounts, BoardFilter } from '../types';

const BOARD_EMOJI: Record<string, string> = {
  hot: '🔥',
  secondary: '📊',
  cold: '❄️',
  all: '🌐',
};

/**
 * 获取板块 emoji
 */
function BoardEmoji({ board }: { board: BoardFilter }) {
  return <span className="leading-none">{BOARD_EMOJI[board] ?? '🔥'}</span>;
}

const BOARDS: readonly BoardFilter[] = ['hot', 'secondary', 'cold', 'all'];

interface BoardSwitcherProps {
  board: BoardFilter;
  onBoardChange: (board: BoardFilter) => void;
  enabled: boolean;
  boardCounts?: BoardCounts;
}

function BoardSwitcherComponent({ board, onBoardChange, enabled, boardCounts }: BoardSwitcherProps) {
  const { t } = useTranslation();
  const [showMenu, setShowMenu] = useState(false);
  const [isHovering, setIsHovering] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  const isMenuVisible = showMenu || isHovering;

  // 点击外部关闭菜单
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

  // ESC 关闭菜单
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

  const showCounts = boardCounts !== undefined;
  const allCount = showCounts ? boardCounts.hot + boardCounts.secondary + boardCounts.cold : 0;
  const countMap: Record<string, number> = showCounts
    ? { hot: boardCounts.hot, secondary: boardCounts.secondary, cold: boardCounts.cold, all: allCount }
    : {};

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
        onMouseEnter={() => setIsHovering(true)}
        onMouseLeave={() => setIsHovering(false)}
      >
        {/* 触发按钮：emoji + 短标签 + 数量 */}
        <button
          onClick={() => setShowMenu(!showMenu)}
          className="h-8 flex items-center gap-1 px-2 rounded-lg bg-elevated/50 hover:bg-muted/50 transition-all duration-200 focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none text-xs"
          aria-label={t('controls.boards.selectBoard')}
          aria-expanded={isMenuVisible}
          aria-haspopup="listbox"
        >
          <BoardEmoji board={board} />
          <span className="font-medium text-primary">{t(`controls.boards.${board}Short`)}</span>
          {showCounts && (
            <span className="text-[10px] tabular-nums text-secondary">
              {countMap[board as string] ?? ''}
            </span>
          )}
        </button>

        {/* 下拉菜单 */}
        <div
          className={`
            absolute top-full left-0 mt-1 z-50
            bg-elevated border border-default rounded-lg shadow-xl py-1
            transition-all duration-200
            ${showMenu ? 'opacity-100 visible translate-y-0' : 'opacity-0 invisible -translate-y-2'}
            lg:group-hover:opacity-100 lg:group-hover:visible lg:group-hover:translate-y-0
          `}
          role="listbox"
          aria-label={t('controls.boards.selectBoard')}
        >
          {BOARDS.map((b) => (
            <button
              key={b}
              onClick={() => handleBoardChange(b)}
              className={`
                w-full px-3 py-1.5 flex items-center gap-1.5 text-xs whitespace-nowrap
                transition-colors cursor-pointer
                focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none
                ${board === b ? 'bg-accent/15 text-accent' : 'text-primary hover:bg-muted/50'}
              `}
              role="option"
              aria-selected={board === b}
            >
              <BoardEmoji board={b} />
              <span className="font-medium">{t(`controls.boards.${b}`)}</span>
              {showCounts && (
                <span className={`text-[10px] tabular-nums ${board === b ? 'text-accent/70' : 'text-secondary'}`}>
                  {countMap[b]}
                </span>
              )}
            </button>
          ))}
        </div>
      </div>

      {/* 分隔线 */}
      <div className="w-px h-5 bg-muted mx-1"></div>
    </>
  );
}

// 导出组件和 Emoji 子组件（移动端 filter drawer 使用）
export const BoardSwitcher = Object.assign(BoardSwitcherComponent, {
  Icon: BoardEmoji,
});

export default BoardSwitcher;
