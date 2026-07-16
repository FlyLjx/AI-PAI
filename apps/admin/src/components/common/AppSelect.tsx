'use client';

import * as Select from '@radix-ui/react-select';
import { Check, ChevronDown, ChevronUp } from 'lucide-react';

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
}: AppSelectProps) {
  const emptyOption = options.find((option) => option.value === '');
  const selectableOptions = options.filter((option) => option.value !== '');

  return (
    <Select.Root
      name={name}
      value={value}
      onValueChange={onValueChange}
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
          <Select.Viewport className="app-select-viewport">
            {selectableOptions.map((option) => (
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
          </Select.Viewport>
          <Select.ScrollDownButton className="app-select-scroll-button">
            <ChevronDown size={13} aria-hidden="true" />
          </Select.ScrollDownButton>
        </Select.Content>
      </Select.Portal>
    </Select.Root>
  );
}
