import { useState, useRef, useEffect, useMemo, useCallback } from 'react';
import { Clock, ChevronDown, Check } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { getTimeFilterPresets, TIME_START_OPTIONS, TIME_END_OPTIONS } from '../constants';

interface TimeFilterPickerProps {
  value: string | null;  // null = 全天, "01:00-09:00" = UTC 时间范围
  disabled?: boolean;    // 24h 周期时禁用
  onChange: (filter: string | null) => void;
}

export function TimeFilterPicker({ value, disabled = false, onChange }: TimeFilterPickerProps) {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = useState(false);
  const [customMode, setCustomMode] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // 获取用户时区偏移（分钟），正值表示 UTC 以西，负值表示 UTC 以东
  // 例如 UTC+8 的 offset 是 -480
  const timezoneOffset = useMemo(() => new Date().getTimezoneOffset(), []);

  // 将 UTC 时间转换为本地时间
  const utcToLocal = useCallback((utcTime: string): string => {
    const [hours, minutes] = utcTime.split(':').map(Number);
    const totalMinutes = hours * 60 + minutes - timezoneOffset;
    const localHours = Math.floor(((totalMinutes % 1440) + 1440) % 1440 / 60);
    const localMins = ((totalMinutes % 60) + 60) % 60;
    return `${String(localHours).padStart(2, '0')}:${String(localMins).padStart(2, '0')}`;
  }, [timezoneOffset]);

  // 将本地时间转换为 UTC 时间
  const localToUtc = useCallback((localTime: string): string => {
    const [hours, minutes] = localTime.split(':').map(Number);
    const totalMinutes = hours * 60 + minutes + timezoneOffset;
    const utcHours = Math.floor(((totalMinutes % 1440) + 1440) % 1440 / 60);
    const utcMins = ((totalMinutes % 60) + 60) % 60;
    return `${String(utcHours).padStart(2, '0')}:${String(utcMins).padStart(2, '0')}`;
  }, [timezoneOffset]);

  // 将本地时间范围转换为 UTC 时间范围
  const localRangeToUtc = useCallback((localRange: string): string => {
    const [localStart, localEnd] = localRange.split('-');
    return `${localToUtc(localStart)}-${localToUtc(localEnd)}`;
  }, [localToUtc]);

  // 将 UTC 时间范围转换为本地时间范围
  // 特殊处理：结束时间 00:00 显示为 24:00（表示当天结束）
  const utcRangeToLocal = useCallback((utcRange: string): string => {
    const [utcStart, utcEnd] = utcRange.split('-');
    const localStart = utcToLocal(utcStart);
    let localEnd = utcToLocal(utcEnd);
    if (localEnd === '00:00') {
      localEnd = '24:00';
    }
    return `${localStart}-${localEnd}`;
  }, [utcToLocal]);

  // 从 UTC value 解析并转换为本地时间
  // 特殊处理：结束时间 00:00 应显示为 24:00（表示当天结束）
  const parseValueToLocal = useCallback((val: string | null): { start: string; end: string } => {
    if (val && val.includes('-')) {
      const [utcStart, utcEnd] = val.split('-');
      if (utcStart && utcEnd) {
        const localStart = utcToLocal(utcStart);
        let localEnd = utcToLocal(utcEnd);
        // 结束时间 00:00 显示为 24:00（午夜作为一天的结束而非开始）
        if (localEnd === '00:00') {
          localEnd = '24:00';
        }
        return { start: localStart, end: localEnd };
      }
    }
    // 默认值：本地时间 09:00-17:00
    return { start: '09:00', end: '17:00' };
  }, [utcToLocal]);

  // 自定义时间段状态（本地时间），惰性初始化
  const [customStart, setCustomStart] = useState(() => parseValueToLocal(value).start);
  const [customEnd, setCustomEnd] = useState(() => parseValueToLocal(value).end);

  // 预设选项（value 是本地时间，显示时直接使用）
  const presets = useMemo(() => {
    return getTimeFilterPresets(t).map(preset => {
      if (!preset.value) {
        // "全天"不需要显示时间
        return { ...preset, utcValue: null as string | null };
      }
      // preset.value 是本地时间，计算对应的 UTC value
      const utcValue = localRangeToUtc(preset.value);
      return {
        ...preset,
        label: `${preset.label} ${preset.value}`,  // 直接显示本地时间
        utcValue,  // 存储 UTC 值用于匹配和发送
      };
    });
  }, [t, localRangeToUtc]);

  // 检查当前 UTC value 是否匹配某个预设
  const matchedPreset = useMemo(() => {
    if (!value) {
      return presets.find((p) => p.utcValue === null);
    }
    return presets.find((p) => p.utcValue === value);
  }, [presets, value]);

  // 进入自定义模式时同步当前值（转换为本地时间）
  const enterCustomMode = () => {
    const { start, end } = parseValueToLocal(value);
    setCustomStart(start);
    setCustomEnd(end);
    setCustomMode(true);
  };

  // 点击外部关闭下拉菜单
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
        setCustomMode(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // 显示文本（显示本地时间）
  const displayText = useMemo(() => {
    if (!value) return t('timeFilter.presets.all');
    if (matchedPreset) return matchedPreset.label;
    // 自定义时段：将 UTC 转换为本地时间显示
    return utcRangeToLocal(value);
  }, [value, matchedPreset, t, utcRangeToLocal]);

  // 验证自定义时段（本地时间，允许跨午夜）
  // 注意：只要 start 和 end 不同就有效，跨午夜的情况（如 22:00-06:00）也是有效的
  const isCustomValid = customStart && customEnd && customStart !== customEnd;

  // 应用自定义时段（将本地时间转换为 UTC）
  const applyCustom = () => {
    if (isCustomValid) {
      const utcStart = localToUtc(customStart);
      const utcEnd = localToUtc(customEnd);
      onChange(`${utcStart}-${utcEnd}`);
      setIsOpen(false);
      setCustomMode(false);
    }
  };

  // 处理预设选择（将本地时间转换为 UTC 发送给 onChange）
  const handlePresetSelect = (preset: typeof presets[number]) => {
    onChange(preset.utcValue);
    setIsOpen(false);
  };

  // 获取用户时区名称
  const timezoneName = useMemo(() => {
    try {
      return Intl.DateTimeFormat().resolvedOptions().timeZone;
    } catch {
      // 回退到 UTC±N 格式
      const offsetHours = -timezoneOffset / 60;
      const sign = offsetHours >= 0 ? '+' : '';
      return `UTC${sign}${offsetHours}`;
    }
  }, [timezoneOffset]);

  return (
    <div className="relative" ref={dropdownRef}>
      {/* 触发按钮 */}
      <button
        type="button"
        disabled={disabled}
        onClick={() => !disabled && setIsOpen(!isOpen)}
        className={`
          flex items-center gap-2 px-3 py-2 rounded-xl text-xs font-medium
          transition-all duration-200 whitespace-nowrap
          ${disabled
            ? 'bg-slate-800/50 text-slate-500 cursor-not-allowed'
            : value
              ? 'bg-gradient-to-br from-cyan-500 to-blue-600 text-white shadow-lg shadow-cyan-500/25'
              : 'bg-slate-800/80 text-slate-300 hover:bg-slate-700/80'
          }
        `}
        title={disabled ? t('timeFilter.disabled24h') : undefined}
      >
        <Clock className="w-3.5 h-3.5" />
        <span className="max-w-[120px] truncate">{displayText}</span>
        <ChevronDown className={`w-3.5 h-3.5 transition-transform ${disabled ? 'opacity-0' : ''} ${isOpen ? 'rotate-180' : ''}`} />
      </button>

      {/* 下拉菜单 */}
      {isOpen && !disabled && (
        <div className="absolute top-full left-0 mt-2 z-[9999] min-w-[200px] bg-slate-800 rounded-xl border border-slate-700 shadow-xl overflow-hidden">
          {/* 预设选项 */}
          {!customMode && (
            <div className="py-1">
              {presets.map((preset) => (
                <button
                  key={preset.id}
                  type="button"
                  onClick={() => handlePresetSelect(preset)}
                  className={`
                    w-full flex items-center justify-between px-4 py-2.5 text-sm
                    transition-colors
                    ${preset.utcValue === value || (preset.utcValue === null && value === null)
                      ? 'bg-cyan-600/20 text-cyan-400'
                      : 'text-slate-300 hover:bg-slate-700/50'
                    }
                  `}
                >
                  <span>{preset.label}</span>
                  {(preset.utcValue === value || (preset.utcValue === null && value === null)) && <Check className="w-4 h-4" />}
                </button>
              ))}

              {/* 分隔线 */}
              <div className="border-t border-slate-700 my-1" />

              {/* 自定义按钮 */}
              <button
                type="button"
                onClick={enterCustomMode}
                className={`
                  w-full flex items-center justify-between px-4 py-2.5 text-sm
                  transition-colors
                  ${!matchedPreset && value
                    ? 'bg-cyan-600/20 text-cyan-400'
                    : 'text-slate-300 hover:bg-slate-700/50'
                  }
                `}
              >
                <span>{t('timeFilter.presets.custom')}</span>
                {!matchedPreset && value && <Check className="w-4 h-4" />}
              </button>
            </div>
          )}

          {/* 自定义时间选择器（使用本地时间） */}
          {customMode && (
            <div className="p-4 space-y-4">
              <div className="text-sm text-slate-400 mb-2">
                {t('timeFilter.customRange.title')} ({timezoneName})
              </div>

              <div className="flex items-center gap-2">
                <select
                  value={customStart}
                  onChange={(e) => setCustomStart(e.target.value)}
                  className="flex-1 bg-slate-700 border border-slate-600 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:ring-2 focus:ring-cyan-500"
                >
                  {TIME_START_OPTIONS.map((time) => (
                    <option key={time} value={time}>
                      {time}
                    </option>
                  ))}
                </select>

                <span className="text-slate-400">—</span>

                <select
                  value={customEnd}
                  onChange={(e) => setCustomEnd(e.target.value)}
                  className="flex-1 bg-slate-700 border border-slate-600 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:ring-2 focus:ring-cyan-500"
                >
                  {TIME_END_OPTIONS.map((time) => (
                    <option key={time} value={time}>
                      {time}
                    </option>
                  ))}
                </select>
              </div>

              {/* 验证提示 */}
              {customStart === customEnd && (
                <div className="text-xs text-rose-400">
                  {t('timeFilter.validation.startBeforeEnd')}
                </div>
              )}

              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => setCustomMode(false)}
                  className="flex-1 px-3 py-2 text-sm text-slate-400 hover:text-slate-200 transition-colors"
                >
                  {t('common.cancel')}
                </button>
                <button
                  type="button"
                  disabled={!isCustomValid}
                  onClick={applyCustom}
                  className={`
                    flex-1 px-3 py-2 text-sm rounded-lg transition-colors
                    ${isCustomValid
                      ? 'bg-cyan-600 text-white hover:bg-cyan-500'
                      : 'bg-slate-700 text-slate-500 cursor-not-allowed'
                    }
                  `}
                >
                  {t('common.apply')}
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
