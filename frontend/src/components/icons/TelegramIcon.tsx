import * as React from 'react';

export interface TelegramIconProps extends React.SVGProps<SVGSVGElement> {
  size?: number | string;
  strokeWidth?: number | string;
}

/**
 * Telegram 图标组件（线性纸飞机风格，与 lucide-react 一致）
 */
export const TelegramIcon = React.forwardRef<SVGSVGElement, TelegramIconProps>(
  (
    {
      size = 24,
      strokeWidth = 2,
      className,
      ...props
    },
    ref
  ) => {
    return (
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        {...props}
        ref={ref}
        width={size}
        height={size}
        strokeWidth={strokeWidth}
        className={className}
      >
        {/* 纸飞机外形 */}
        <path d="M22 2L2 9l9 4 4 9 7-20Z" />
        {/* 内部折线 */}
        <path d="M22 2L11 13" />
      </svg>
    );
  }
);

TelegramIcon.displayName = 'TelegramIcon';

export default TelegramIcon;
