'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import * as Select from '@radix-ui/react-select';
import { Check, ChevronDown, ChevronUp, Search } from 'lucide-react';

export type AppSelectOption = {
  value: string;
  label: string;
  disabled?: boolean;
};

type AppSelectProps = {
  id?: string;
  name?: string;
  value: string;
  options: readonly AppSelectOption[];
  onValueChange?: (value: string) => void;
  placeholder?: string;
  ariaLabel?: string;
  disabled?: boolean;
  required?: boolean;
  compact?: boolean;
  className?: string;
  searchable?: boolean;
};

export function AppSelect({
  id,
  name,
  value,
  options,
  onValueChange,
  placeholder,
  ariaLabel,
  disabled = false,
  required = false,
  compact = false,
  className = '',
  searchable = true,
}: AppSelectProps) {
  const [open, setOpen] = useState(false);
  const [searchText, setSearchText] = useState('');
  const searchInputRef = useRef<HTMLInputElement>(null);
  const emptyOption = options.find((option) => option.value === '');
  const selectableOptions = options.filter((option) => option.value !== '');
  const filteredOptions = useMemo(() => {
    const keyword = searchText.trim().toLocaleLowerCase();
    if (!keyword || !searchable) return selectableOptions;
    return selectableOptions.filter((option) => `${option.label} ${option.value}`.toLocaleLowerCase().includes(keyword));
  }, [searchText, searchable, selectableOptions]);

  useEffect(() => {
    if (!open || !searchable) return;
    const focusTimer = window.setTimeout(() => searchInputRef.current?.focus(), 0);
    return () => window.clearTimeout(focusTimer);
  }, [open, searchable]);

  return (
    <Select.Root
      name={name}
      value={value}
      onValueChange={onValueChange}
      open={open}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen);
        if (!nextOpen) setSearchText('');
      }}
      disabled={disabled}
      required={required}
    >
      <Select.Trigger
        id={id}
        className={`app-select-trigger${compact ? ' is-compact' : ''} ${className}`.trim()}
        aria-label={ariaLabel}
      >
        <Select.Value placeholder={placeholder || emptyOption?.label || '请选择'} />
        <Select.Icon className="app-select-icon">
          <ChevronDown size={14} aria-hidden="true" />
        </Select.Icon>
      </Select.Trigger>
      <Select.Portal>
        <Select.Content
          className="app-select-content"
          position="popper"
          sideOffset={5}
          collisionPadding={10}
        >
          <Select.ScrollUpButton className="app-select-scroll-button">
            <ChevronUp size={13} aria-hidden="true" />
          </Select.ScrollUpButton>
          {searchable && (
            <label className="app-select-search" onPointerDown={(event) => event.stopPropagation()}>
              <Search size={13} aria-hidden="true" />
              <input
                ref={searchInputRef}
                value={searchText}
                onChange={(event) => setSearchText(event.target.value)}
                onKeyDown={(event) => event.stopPropagation()}
                placeholder="搜索选项"
                aria-label="搜索选项"
              />
            </label>
          )}
          <Select.Viewport className="app-select-viewport">
            {filteredOptions.map((option) => (
              <Select.Item
                key={option.value}
                className="app-select-item"
                value={option.value}
                disabled={option.disabled}
              >
                <Select.ItemText>{option.label}</Select.ItemText>
                <Select.ItemIndicator className="app-select-indicator">
                  <Check size={13} strokeWidth={2.5} aria-hidden="true" />
                </Select.ItemIndicator>
              </Select.Item>
            ))}
            {filteredOptions.length === 0 && <div className="app-select-empty">没有匹配选项</div>}
          </Select.Viewport>
          <Select.ScrollDownButton className="app-select-scroll-button">
            <ChevronDown size={13} aria-hidden="true" />
          </Select.ScrollDownButton>
        </Select.Content>
      </Select.Portal>
    </Select.Root>
  );
}
