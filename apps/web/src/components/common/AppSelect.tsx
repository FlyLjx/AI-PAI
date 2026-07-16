'use client';

import * as Select from '@radix-ui/react-select';
import { Check, ChevronDown, ChevronUp } from 'lucide-react';

const EMPTY_VALUE = '__app_select_empty__';

export type AppSelectOption = {
  value: string;
  label: string;
  disabled?: boolean;
};

function encodeValue(value: string): string {
  return value === '' ? EMPTY_VALUE : value;
}

function decodeValue(value: string): string {
  return value === EMPTY_VALUE ? '' : value;
}

export function AppSelect({
  id,
  value,
  options,
  onValueChange,
  disabled = false,
  className = '',
}: {
  id?: string;
  value: string;
  options: readonly AppSelectOption[];
  onValueChange: (value: string) => void;
  disabled?: boolean;
  className?: string;
}) {
  const selectedLabel = options.find((option) => option.value === value)?.label || '';

  return (
    <Select.Root value={encodeValue(value)} onValueChange={(nextValue) => onValueChange(decodeValue(nextValue))} disabled={disabled}>
      <Select.Trigger id={id} className={`app-select-trigger ${className}`.trim()}>
        <Select.Value>{selectedLabel}</Select.Value>
        <Select.Icon className="app-select-icon">
          <ChevronDown size={14} aria-hidden="true" />
        </Select.Icon>
      </Select.Trigger>
      <Select.Portal>
        <Select.Content className="app-select-content" position="popper" sideOffset={5} collisionPadding={10}>
          <Select.ScrollUpButton className="app-select-scroll-button">
            <ChevronUp size={13} aria-hidden="true" />
          </Select.ScrollUpButton>
          <Select.Viewport className="app-select-viewport">
            {options.map((option) => (
              <Select.Item
                key={option.value || EMPTY_VALUE}
                className="app-select-item"
                value={encodeValue(option.value)}
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
