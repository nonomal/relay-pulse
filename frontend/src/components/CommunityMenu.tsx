/**
 * 加入社群菜单组件
 *
 * 交互设计：
 * - 桌面端（lg+）：点击/hover 展开菜单，hover 菜单项时右侧显示二维码预览
 * - 移动端：点击展开菜单，点击菜单项展开手风琴显示二维码
 *
 * 可扩展：支持多个社群（QQ群、微信群、Telegram等）
 */

import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Users, ChevronRight, ChevronDown, ExternalLink } from 'lucide-react';
import { COMMUNITY_LIST } from '../constants';

export function CommunityMenu() {
  const { t } = useTranslation();
  const [showMenu, setShowMenu] = useState(false);
  // 桌面端：当前 hover/focus 的社群 ID（用于显示预览）
  const [hoveredId, setHoveredId] = useState<string | null>(COMMUNITY_LIST[0]?.id ?? null);
  // 移动端：当前展开的手风琴项
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  // 获取当前预览的社群项
  const activeItem = useMemo(() => {
    return COMMUNITY_LIST.find((item) => item.id === hoveredId) || COMMUNITY_LIST[0] || null;
  }, [hoveredId]);

  // 点击外部关闭菜单
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setShowMenu(false);
        setExpandedId(null);
      }
    }

    if (showMenu) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [showMenu]);

  // ESC 关闭菜单
  useEffect(() => {
    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setShowMenu(false);
        setExpandedId(null);
      }
    }

    if (showMenu) {
      document.addEventListener('keydown', handleEscape);
      return () => document.removeEventListener('keydown', handleEscape);
    }
  }, [showMenu]);

  // 鼠标离开容器时重置手风琴状态，避免状态残留
  const handleMouseLeave = () => {
    setExpandedId(null);
  };

  return (
    <div ref={menuRef} className="relative group" onMouseLeave={handleMouseLeave}>
      {/* 触发按钮 */}
      <button
        onClick={() => setShowMenu((v) => !v)}
        className="p-2 rounded-lg bg-elevated/50 text-secondary hover:text-primary hover:bg-muted/50 transition-all duration-200 focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none"
        aria-label={t('community.title')}
        aria-expanded={showMenu}
        aria-haspopup="menu"
        title={t('community.title')}
      >
        <Users size={16} />
      </button>

      {/* 下拉菜单 */}
      <div
        className={`
          absolute top-full mt-1 z-50
          right-0 lg:right-0
          w-[min(85vw,18rem)] lg:w-auto
          bg-elevated border border-default rounded-lg shadow-xl
          transition-all duration-200
          ${showMenu ? 'opacity-100 visible translate-y-0' : 'opacity-0 invisible -translate-y-2'}
          lg:group-hover:opacity-100 lg:group-hover:visible lg:group-hover:translate-y-0
        `}
        aria-label={t('community.title')}
      >
        <div className="flex flex-col lg:flex-row">
          {/* 左侧：社群列表 */}
          <div className="w-full lg:w-auto lg:min-w-28">
            {COMMUNITY_LIST.map((item) => {
              const isExpanded = expandedId === item.id;
              const groupName = t(item.nameKey);

              return (
                <div key={item.id} className="first:rounded-t-lg last:rounded-b-lg lg:last:rounded-r-none">
                  {/* 菜单项按钮 */}
                  <button
                    onClick={() => {
                      // 移动端：切换手风琴
                      setExpandedId((prev) => (prev === item.id ? null : item.id));
                      // 同步更新桌面端预览
                      setHoveredId(item.id);
                    }}
                    onMouseEnter={() => setHoveredId(item.id)}
                    onFocus={() => setHoveredId(item.id)}
                    className={`
                      w-full flex items-center justify-between gap-2 px-3 py-2 text-left
                      transition-colors cursor-pointer
                      focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none focus-visible:ring-inset
                      ${hoveredId === item.id ? 'bg-muted/50 text-primary' : 'text-primary hover:bg-muted/50'}
                    `}
                    aria-expanded={isExpanded}
                    aria-controls={`community-panel-${item.id}`}
                  >
                    <span className="text-sm font-medium truncate">{groupName}</span>
                    {/* 桌面端：右箭头 */}
                    <ChevronRight size={14} className="hidden lg:block text-muted flex-shrink-0" />
                    {/* 移动端：展开/收起箭头 */}
                    <ChevronDown
                      size={14}
                      className={`lg:hidden text-muted flex-shrink-0 transition-transform duration-200 ${isExpanded ? 'rotate-180' : ''}`}
                    />
                  </button>

                  {/* 移动端：手风琴展开内容 */}
                  <div
                    id={`community-panel-${item.id}`}
                    className={`
                      lg:hidden overflow-hidden transition-all duration-200 ease-out
                      ${isExpanded ? 'max-h-[70vh] opacity-100' : 'max-h-0 opacity-0'}
                    `}
                  >
                    <div className="px-3 pb-3">
                      {/* 二维码 */}
                      {item.qrImageSrc && (
                        <div className="mt-2 rounded-lg bg-surface/40 border border-default p-3 flex justify-center">
                          <img
                            src={item.qrImageSrc}
                            alt={t('community.qrCodeAlt', { name: groupName })}
                            className="max-w-full max-h-[50vh] rounded-md bg-white object-contain"
                            loading="lazy"
                          />
                        </div>
                      )}
                      {/* 加入链接 */}
                      {item.joinUrl && (
                        <a
                          href={item.joinUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          onClick={() => setShowMenu(false)}
                          className="mt-2 inline-flex items-center gap-1 text-sm text-accent hover:underline focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none rounded"
                        >
                          {t('community.joinLink')}
                          <ExternalLink size={14} />
                        </a>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>

          {/* 右侧：桌面端预览面板（二维码已包含群名和群号，此处不再重复显示） */}
          <div className="hidden lg:block w-72 border-l border-default">
            {activeItem && (
              <div className="p-3">
                {/* 二维码 */}
                {activeItem.qrImageSrc && (
                  <div className="rounded-lg bg-surface/40 border border-default p-2">
                    <img
                      src={activeItem.qrImageSrc}
                      alt={t('community.qrCodeAlt', { name: t(activeItem.nameKey) })}
                      className="w-full rounded-md bg-white"
                      loading="lazy"
                    />
                  </div>
                )}
                {/* 加入链接 */}
                {activeItem.joinUrl && (
                  <a
                    href={activeItem.joinUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={() => setShowMenu(false)}
                    className="mt-2 inline-flex items-center gap-1 text-sm text-accent hover:underline focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none rounded"
                  >
                    {t('community.joinLink')}
                    <ExternalLink size={14} />
                  </a>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

export default CommunityMenu;
