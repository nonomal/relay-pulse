/**
 * 空收藏状态提示组件
 *
 * 当用户开启"仅显示收藏"但没有收藏任何项目时显示
 */

import { useTranslation } from 'react-i18next';
import { Star } from 'lucide-react';

export interface EmptyFavoritesProps {
  /** 关闭"仅显示收藏"筛选的回调 */
  onClearFilter?: () => void;
}

export function EmptyFavorites({ onClearFilter }: EmptyFavoritesProps) {
  const { t } = useTranslation();

  return (
    <div className="flex flex-col items-center justify-center py-16 px-4">
      {/* 图标 */}
      <div className="mb-4 p-4 rounded-full bg-muted/30">
        <Star size={32} className="text-muted" />
      </div>

      {/* 标题 */}
      <h3 className="text-lg font-medium text-primary mb-2">
        {t('favorites.empty.title')}
      </h3>

      {/* 描述 */}
      <p className="text-secondary text-center max-w-sm mb-4">
        {t('favorites.empty.description')}
      </p>

      {/* 操作按钮 */}
      {onClearFilter && (
        <button
          type="button"
          onClick={onClearFilter}
          className="
            px-4 py-2 rounded-lg
            bg-accent/10 text-accent
            hover:bg-accent/20
            transition-colors duration-150
            focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:outline-none
          "
        >
          {t('favorites.empty.showAll')}
        </button>
      )}
    </div>
  );
}

export default EmptyFavorites;
