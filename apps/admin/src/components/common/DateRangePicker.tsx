'use client';

import { useCallback, useId, useLayoutEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { DayPicker, type DateRange } from '@daypicker/react';
import { zhCN } from '@daypicker/react/locale';
import { ArrowRight, CalendarDays, ChevronDown } from 'lucide-react';

const POPOVER_WIDTH = 320;
const POPOVER_HEIGHT = 390;
const VIEWPORT_GAP = 12;

function dateFromValue(value: string): Date | undefined {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return undefined;
  const [year, month, day] = value.split('-').map(Number);
  const date = new Date(year, month - 1, day, 12);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

function valueFromDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function displayDate(value: string): string {
  return value ? value.replaceAll('-', '/') : '选择日期';
}

function selectedRange(startDate: string, endDate: string): DateRange | undefined {
  const from = dateFromValue(startDate);
  const to = dateFromValue(endDate);
  return from ? { from, to } : undefined;
}

type PickerPosition = { left: number; top: number; width: number };

export function DateRangePicker({
  startDate,
  endDate,
  maxDate,
  onChange,
}: {
  startDate: string;
  endDate: string;
  maxDate: string;
  onChange: (startDate: string, endDate: string) => void;
}) {
  const dialogId = useId();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<DateRange | undefined>(() => selectedRange(startDate, endDate));
  const [position, setPosition] = useState<PickerPosition>({ left: VIEWPORT_GAP, top: VIEWPORT_GAP, width: POPOVER_WIDTH });
  const max = dateFromValue(maxDate) || new Date();

  const closePicker = useCallback(() => {
    setOpen(false);
    window.requestAnimationFrame(() => triggerRef.current?.focus());
  }, []);

  const openPicker = () => {
    setDraft(selectedRange(startDate, endDate));
    setOpen(true);
  };

  useLayoutEffect(() => {
    if (!open) return;
    const updatePosition = () => {
      const trigger = triggerRef.current;
      if (!trigger) return;
      const anchor = trigger.getBoundingClientRect();
      const width = Math.min(POPOVER_WIDTH, window.innerWidth - VIEWPORT_GAP * 2);
      const height = popoverRef.current?.offsetHeight || POPOVER_HEIGHT;
      const left = Math.min(Math.max(VIEWPORT_GAP, anchor.left), window.innerWidth - width - VIEWPORT_GAP);
      const below = anchor.bottom + 8;
      const top = below + height <= window.innerHeight - VIEWPORT_GAP
        ? below
        : Math.max(VIEWPORT_GAP, anchor.top - height - 8);
      setPosition({ left, top, width });
    };
    updatePosition();
    window.addEventListener('resize', updatePosition);
    window.addEventListener('scroll', updatePosition, true);
    return () => {
      window.removeEventListener('resize', updatePosition);
      window.removeEventListener('scroll', updatePosition, true);
    };
  }, [open]);

  useLayoutEffect(() => {
    if (!open) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') closePicker();
    };
    document.addEventListener('keydown', handleKeyDown);
    popoverRef.current?.focus();
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [closePicker, open]);

  const applyDraft = () => {
    if (!draft?.from || !draft.to) return;
    onChange(valueFromDate(draft.from), valueFromDate(draft.to));
    closePicker();
  };

  const draftLabel = draft?.from
    ? `${displayDate(valueFromDate(draft.from))} - ${draft.to ? displayDate(valueFromDate(draft.to)) : '请选择结束日期'}`
    : '请选择日期范围';

  return (
    <>
      <button
        ref={triggerRef}
        className="admin-date-picker-trigger"
        type="button"
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-controls={open ? dialogId : undefined}
        onClick={open ? closePicker : openPicker}
      >
        <CalendarDays size={15} className="text-[#0f7a4b]" aria-hidden="true" />
        <span className="admin-date-picker-trigger-label">
          <span className="truncate">{displayDate(startDate)}</span>
          <ArrowRight size={11} className="shrink-0 text-zinc-400" aria-hidden="true" />
          <span className="truncate">{displayDate(endDate)}</span>
        </span>
        <ChevronDown size={13} className={`text-zinc-400 transition-transform ${open ? 'rotate-180' : ''}`} aria-hidden="true" />
      </button>

      {open && createPortal(
        <>
          <button className="fixed inset-0 z-[90] cursor-default bg-transparent" type="button" aria-label="关闭日期选择器" onClick={closePicker} />
          <div
            ref={popoverRef}
            id={dialogId}
            className="admin-date-picker-popover fixed z-[100]"
            style={{ left: position.left, top: position.top, width: position.width }}
            role="dialog"
            aria-modal="true"
            aria-label="选择收益日期范围"
            tabIndex={-1}
          >
            <div className="admin-date-picker">
              <DayPicker
                mode="range"
                required={false}
                selected={draft}
                onSelect={setDraft}
                defaultMonth={draft?.from || max}
                endMonth={max}
                disabled={{ after: max }}
                locale={zhCN}
                weekStartsOn={1}
                fixedWeeks
                showOutsideDays
                navLayout="around"
                resetOnSelect
                max={366}
                formatters={{ formatCaption: (date) => `${date.getFullYear()}年${date.getMonth() + 1}月` }}
              />
            </div>
            <div className="admin-date-picker-footer">
              <div className="min-w-0">
                <span>已选范围</span>
                <strong>{draftLabel}</strong>
              </div>
              <div className="flex shrink-0 items-center gap-1.5">
                <button className="admin-date-picker-action" type="button" onClick={closePicker}>取消</button>
                <button className="admin-date-picker-action primary" type="button" disabled={!draft?.from || !draft.to} onClick={applyDraft}>应用</button>
              </div>
            </div>
          </div>
        </>,
        document.body,
      )}
    </>
  );
}
