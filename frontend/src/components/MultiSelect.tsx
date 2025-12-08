import { useState, useMemo, useRef, useEffect, useCallback } from 'react';
import { Check, ChevronDown, Search, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

export interface MultiSelectOption {
  value: string;
  label: string;
}

interface MultiSelectProps {
  value: string[];
  options: MultiSelectOption[];
  onChange: (values: string[]) => void;
  placeholder?: string;
  searchable?: boolean;
  disabled?: boolean;
  className?: string;
}

/**
 * 多选下拉组件
 * - 空数组表示"全部"
 * - 支持搜索过滤
 * - 支持键盘操作
 * - 点击外部自动关闭
 */
export function MultiSelect({
  value,
  options,
  onChange,
  placeholder,
  searchable = true,
  disabled = false,
  className = '',
}: MultiSelectProps) {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = useState(false);
  const [search, setSearch] = useState('');
  const containerRef = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);

  // 默认占位符
  const displayPlaceholder = placeholder ?? t('controls.filters.provider');

  // 搜索过滤
  const filteredOptions = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) return options;
    return options.filter(opt =>
      opt.label.toLowerCase().includes(term) ||
      opt.value.toLowerCase().includes(term)
    );
  }, [options, search]);

  // 点击外部关闭
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false);
        setSearch('');
      }
    };

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [isOpen]);

  // 打开时聚焦搜索框
  useEffect(() => {
    if (isOpen && searchable && searchInputRef.current) {
      searchInputRef.current.focus();
    }
  }, [isOpen, searchable]);

  // 切换选中状态
  const toggleOption = useCallback((optionValue: string) => {
    const isSelected = value.includes(optionValue);
    if (isSelected) {
      onChange(value.filter(v => v !== optionValue));
    } else {
      onChange([...value, optionValue]);
    }
  }, [value, onChange]);

  // 全选/全不选
  const handleSelectAll = useCallback(() => {
    onChange([]);  // 空数组 = 全部
    setIsOpen(false);
    setSearch('');
  }, [onChange]);

  // 清空选择
  const handleClear = useCallback((e: React.MouseEvent | React.KeyboardEvent) => {
    e.stopPropagation();
    onChange([]);
  }, [onChange]);

  // 键盘操作
  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      setIsOpen(false);
      setSearch('');
    } else if (e.key === 'Enter' && !isOpen) {
      setIsOpen(true);
    }
  }, [isOpen]);

  // 显示文本
  const displayText = useMemo(() => {
    if (value.length === 0) {
      return displayPlaceholder;
    }
    if (value.length === 1) {
      const opt = options.find(o => o.value === value[0]);
      return opt?.label ?? value[0];
    }
    return t('controls.multiSelect.selectedCount', { count: value.length });
  }, [value, options, displayPlaceholder, t]);

  const isAllSelected = value.length === 0;

  return (
    <div
      ref={containerRef}
      className={`relative overflow-visible ${className}`}
      onKeyDown={handleKeyDown}
    >
      {/* 触发按钮 */}
      <div
        role="button"
        tabIndex={disabled ? -1 : 0}
        onClick={() => !disabled && setIsOpen(!isOpen)}
        onKeyDown={(e) => {
          if (!disabled && (e.key === 'Enter' || e.key === ' ')) {
            e.preventDefault();
            setIsOpen(!isOpen);
          }
        }}
        aria-haspopup="listbox"
        aria-expanded={isOpen}
        aria-disabled={disabled}
        className={`
          flex items-center justify-between gap-2 w-full lg:w-auto
          bg-elevated text-primary text-sm rounded-lg
          border border-default p-2 outline-none
          transition-all hover:bg-muted
          focus:ring-2 focus:ring-accent focus:border-transparent
          ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
          ${!isAllSelected ? 'border-accent/50' : ''}
        `}
      >
        <span className={`truncate ${isAllSelected ? 'text-secondary' : 'text-primary'}`}>
          {displayText}
        </span>
        <div className="flex items-center gap-1 flex-shrink-0">
          {!isAllSelected && (
            <button
              type="button"
              onClick={handleClear}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  handleClear(e);
                }
              }}
              className="p-0.5 hover:bg-muted rounded transition-colors focus:outline-none focus:ring-1 focus:ring-accent"
              aria-label={t('common.clear')}
              tabIndex={0}
            >
              <X size={14} className="text-secondary hover:text-primary" />
            </button>
          )}
          <ChevronDown
            size={16}
            className={`text-secondary transition-transform ${isOpen ? 'rotate-180' : ''}`}
          />
        </div>
      </div>

      {/* 下拉面板 */}
      {isOpen && (
        <div
          role="listbox"
          aria-multiselectable="true"
          className="
            absolute z-50 mt-1 min-w-full w-max
            bg-elevated border border-default rounded-lg shadow-xl
            max-h-[320px] flex flex-col
          "
        >
          {/* 搜索框 */}
          {searchable && (
            <div className="p-2 border-b border-default flex-shrink-0">
              <div className="relative">
                <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted" />
                <input
                  ref={searchInputRef}
                  type="text"
                  value={search}
                  onChange={e => setSearch(e.target.value)}
                  placeholder={t('controls.multiSelect.searchPlaceholder')}
                  className="
                    w-full pl-8 pr-3 py-1.5 text-sm
                    bg-surface text-primary rounded-md
                    border border-default outline-none
                    focus:ring-1 focus:ring-accent focus:border-accent
                    placeholder:text-muted
                  "
                />
              </div>
            </div>
          )}

          {/* 选项列表 */}
          <div className="overflow-y-auto flex-1 py-1">
            {/* "全部"选项 - 仅在没有搜索时显示 */}
            {!search.trim() && (
              <button
                type="button"
                role="option"
                aria-selected={isAllSelected}
                onClick={handleSelectAll}
                className={`
                  w-full flex items-center gap-2 px-3 py-2 text-sm text-left
                  transition-colors
                  ${isAllSelected
                    ? 'bg-accent/10 text-accent'
                    : 'text-secondary hover:bg-muted/50'
                  }
                `}
              >
                <span className={`
                  w-4 h-4 rounded border flex items-center justify-center flex-shrink-0
                  ${isAllSelected
                    ? 'bg-accent border-accent'
                    : 'border-strong'
                  }
                `}>
                  {isAllSelected && <Check size={12} className="text-inverse" />}
                </span>
                <span className="truncate">{displayPlaceholder}</span>
              </button>
            )}

            {/* 分隔线 - 仅在没有搜索时显示 */}
            {!search.trim() && <div className="h-px bg-muted mx-2 my-1" />}

            {/* 服务商选项 */}
            {filteredOptions.length > 0 ? (
              filteredOptions.map(option => {
                const isSelected = value.includes(option.value);
                return (
                  <button
                    key={option.value}
                    type="button"
                    role="option"
                    aria-selected={isSelected}
                    onClick={() => toggleOption(option.value)}
                    className={`
                      w-full flex items-center gap-2 px-3 py-2 text-sm text-left
                      transition-colors
                      ${isSelected
                        ? 'bg-accent/10 text-accent'
                        : 'text-secondary hover:bg-muted/50'
                      }
                    `}
                  >
                    <span className={`
                      w-4 h-4 rounded border flex items-center justify-center flex-shrink-0
                      ${isSelected
                        ? 'bg-accent border-accent'
                        : 'border-strong'
                      }
                    `}>
                      {isSelected && <Check size={12} className="text-inverse" />}
                    </span>
                    <span className="truncate">{option.label}</span>
                  </button>
                );
              })
            ) : (
              <div className="px-3 py-4 text-sm text-muted text-center">
                {t('controls.multiSelect.noResults')}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
