import { useEffect, useState } from 'react';
import { X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { TooltipState } from '../types';
import { availabilityToColor, latencyToColor } from '../utils/color';
import { createMediaQueryEffect } from '../utils/mediaQuery';

interface TooltipProps {
  tooltip: TooltipState;
  slowLatencyMs: number;
  timeRange: string;
  onClose?: () => void;
}

// 时间块粒度（毫秒）
const BUCKET_DURATION: Record<string, number> = {
  '24h': 60 * 60 * 1000,       // 1 小时
  '1d': 60 * 60 * 1000,        // 1 小时
  '7d': 24 * 60 * 60 * 1000,   // 1 天
  '30d': 24 * 60 * 60 * 1000,  // 1 天
};

// 两位数补零
const pad2 = (n: number) => n.toString().padStart(2, '0');

// 格式化时间段显示
function formatTimeRange(timestampSec: number, timeRange: string): string {
  const startMs = timestampSec * 1000;
  const duration = BUCKET_DURATION[timeRange] || BUCKET_DURATION['24h'];
  const endMs = startMs + duration;

  const start = new Date(startMs);
  const end = new Date(endMs);

  // 24h/1d: 显示 HH:MM - HH:MM
  if (timeRange === '24h' || timeRange === '1d') {
    return `${pad2(start.getHours())}:${pad2(start.getMinutes())} - ${pad2(end.getHours())}:${pad2(end.getMinutes())}`;
  }

  // 7d/30d: 显示 MM-DD - MM-DD
  return `${pad2(start.getMonth() + 1)}-${pad2(start.getDate())} - ${pad2(end.getMonth() + 1)}-${pad2(end.getDate())}`;
}

export function Tooltip({ tooltip, slowLatencyMs, timeRange, onClose }: TooltipProps) {
  const { t } = useTranslation();
  const [isMobile, setIsMobile] = useState(false);

  // 检测是否为移动端（兼容 Safari ≤13）
  useEffect(() => {
    const cleanup = createMediaQueryEffect('mobile', setIsMobile);
    return cleanup;
  }, []);

  if (!tooltip.show || !tooltip.data) return null;

  // 状态计数统计（向后兼容）
  const counts = tooltip.data.statusCounts ?? {
    available: 0,
    degraded: 0,
    unavailable: 0,
    missing: 0,
    slow_latency: 0,
    rate_limit: 0,
    server_error: 0,
    client_error: 0,
    auth_error: 0,
    invalid_request: 0,
    network_error: 0,
    content_mismatch: 0,
  };

  // 状态统计
  const statusSummary = [
    { key: 'available', emoji: '🟢', label: t('status.available'), value: counts.available },
    { key: 'degraded', emoji: '🟡', label: t('status.degraded'), value: counts.degraded },
    { key: 'unavailable', emoji: '🔴', label: t('status.unavailable'), value: counts.unavailable },
  ];

  // 黄色波动细分
  const degradedSubstatus = [
    { key: 'slow_latency', label: t('subStatus.slow_latency'), value: counts.slow_latency },
  ].filter(item => item.value > 0);

  // 红色不可用细分
  const unavailableSubstatus = [
    { key: 'server_error', label: t('subStatus.server_error'), value: counts.server_error },
    { key: 'client_error', label: t('subStatus.client_error'), value: counts.client_error },
    { key: 'auth_error', label: t('subStatus.auth_error'), value: counts.auth_error },
    { key: 'invalid_request', label: t('subStatus.invalid_request'), value: counts.invalid_request },
    { key: 'network_error', label: t('subStatus.network_error'), value: counts.network_error },
    { key: 'rate_limit', label: t('subStatus.rate_limit'), value: counts.rate_limit },
    { key: 'content_mismatch', label: t('subStatus.content_mismatch'), value: counts.content_mismatch },
  ].filter(item => item.value > 0);

  // Tooltip 内容（桌面和移动端共用）
  const TooltipContent = () => (
    <>
      <div className="text-slate-400 text-center">
        {formatTimeRange(tooltip.data!.timestampNum, timeRange)}
      </div>
      {tooltip.data!.availability >= 0 && (
        <div
          className="font-medium text-center text-sm md:text-xs"
          style={{ color: availabilityToColor(tooltip.data!.availability) }}
        >
          {t('tooltip.uptime')} {tooltip.data!.availability.toFixed(2)}%
        </div>
      )}
      {tooltip.data!.latency > 0 && (
        <div className="text-[10px] text-center">
          <span className="text-slate-500">{t('tooltip.latency')} </span>
          <span style={{ color: latencyToColor(tooltip.data!.latency, slowLatencyMs) }}>
            {tooltip.data!.latency}ms
          </span>
        </div>
      )}

      {/* 状态统计 */}
      <div className="flex flex-col gap-1 pt-2 border-t border-slate-700/50">
        {statusSummary.map((item) => (
          <div key={item.key} className="flex justify-between items-center gap-3 text-[11px]">
            <span className="text-slate-300">
              {item.emoji} {item.label}
            </span>
            <span className="text-slate-100 font-semibold tabular-nums">
              {item.value} {t('tooltip.count')}
            </span>
          </div>
        ))}
      </div>

      {/* 黄色波动细分 */}
      {degradedSubstatus.length > 0 && (
        <div className="flex flex-col gap-1 pt-2 border-t border-slate-700/50">
          <div className="text-[10px] text-slate-400 mb-0.5">{t('tooltip.degradedTitle')}</div>
          {degradedSubstatus.map((item) => (
            <div key={item.key} className="flex justify-between items-center gap-3 text-[10px] pl-2">
              <span className="text-slate-400">• {item.label}</span>
              <span className="text-slate-200 tabular-nums">{item.value}</span>
            </div>
          ))}
        </div>
      )}

      {/* 红色不可用细分 */}
      {unavailableSubstatus.length > 0 && (
        <div className="flex flex-col gap-1 pt-2 border-t border-slate-700/50">
          <div className="text-[10px] text-slate-400 mb-0.5">{t('tooltip.unavailableTitle')}</div>
          {unavailableSubstatus.map((item) => (
            <div key={item.key} className="flex justify-between items-center gap-3 text-[10px] pl-2">
              <span className="text-slate-400">• {item.label}</span>
              <span className="text-slate-200 tabular-nums">{item.value}</span>
            </div>
          ))}
        </div>
      )}
    </>
  );

  // 移动端：底部 Sheet
  if (isMobile) {
    return (
      <div
        className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm"
        onClick={onClose}
      >
        <div
          className="absolute bottom-0 left-0 right-0 bg-slate-900 border-t border-slate-700 rounded-t-2xl p-4 pb-6 animate-slide-up"
          onClick={(e) => e.stopPropagation()}
          style={{
            animation: 'slideUp 0.2s ease-out',
          }}
        >
          {/* 拖动指示条 */}
          <div className="flex justify-center mb-3">
            <div className="w-10 h-1 bg-slate-700 rounded-full" />
          </div>

          {/* 头部 */}
          <div className="flex justify-between items-center mb-3">
            <h3 className="text-sm font-semibold text-slate-200">{t('tooltip.title')}</h3>
            <button
              onClick={onClose}
              className="p-1.5 rounded-lg bg-slate-800 text-slate-400 hover:text-slate-200 transition-colors"
              aria-label={t('common.close')}
            >
              <X size={16} />
            </button>
          </div>

          {/* 内容 */}
          <div className="flex flex-col gap-2 text-xs">
            <TooltipContent />
          </div>
        </div>

        {/* CSS 动画 */}
        <style>{`
          @keyframes slideUp {
            from {
              transform: translateY(100%);
            }
            to {
              transform: translateY(0);
            }
          }
        `}</style>
      </div>
    );
  }

  // 桌面端：悬浮 Tooltip
  return (
    <div
      className="fixed z-50 pointer-events-none transition-opacity duration-200"
      style={{
        left: tooltip.x,
        top: tooltip.y,
        transform: 'translate(-50%, -100%)',
      }}
    >
      <div className="bg-slate-900/95 backdrop-blur-md text-slate-200 text-xs p-3 rounded-lg border border-slate-700 shadow-[0_10px_40px_-10px_rgba(0,0,0,0.8)] flex flex-col gap-2 min-w-[180px]">
        <TooltipContent />

        {/* 小三角箭头 */}
        <div className="absolute -bottom-1.5 left-1/2 -translate-x-1/2 w-3 h-3 bg-slate-900 border-r border-b border-slate-700 transform rotate-45"></div>
      </div>
    </div>
  );
}
