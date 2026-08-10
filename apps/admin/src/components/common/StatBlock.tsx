'use client';

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { ArrowDownRight, ArrowUpRight, LucideIcon, Minus } from 'lucide-react';

interface StatBlockProps {
  title: string;
  value: string | number;
  subtext?: string;
  trend?: {
    value: string;
    type: 'positive' | 'negative' | 'neutral';
    label?: string;
  };
  icon?: LucideIcon;
  color?: 'green' | 'cyan' | 'amber' | 'neutral';
}

type AnimatedValueParts = {
  target: number;
  prefix: string;
  suffix: string;
  decimals: number;
  useGrouping: boolean;
};

const numericPattern = /[-+]?\d[\d,]*(?:\.\d+)?/g;

function parseAnimatedValue(value: string | number): AnimatedValueParts | null {
  if (typeof value === 'number') {
    if (!Number.isFinite(value)) return null;
    return {
      target: value,
      prefix: '',
      suffix: '',
      decimals: Number.isInteger(value) ? 0 : Math.min(4, String(value).split('.')[1]?.length || 0),
      useGrouping: Math.abs(value) >= 1000,
    };
  }

  const raw = String(value);
  const matches = Array.from(raw.matchAll(numericPattern));
  if (matches.length !== 1) return null;

  const match = matches[0];
  const numericText = match[0];
  const target = Number(numericText.replace(/,/g, ''));
  if (!Number.isFinite(target)) return null;

  return {
    target,
    prefix: raw.slice(0, match.index || 0),
    suffix: raw.slice((match.index || 0) + numericText.length),
    decimals: numericText.includes('.') ? numericText.split('.')[1].length : 0,
    useGrouping: numericText.includes(',') || Math.abs(target) >= 1000,
  };
}

function formatAnimatedValue(value: number, parts: AnimatedValueParts): string {
  const fixed = parts.decimals > 0 ? value.toFixed(parts.decimals) : String(Math.round(value));
  const negative = fixed.startsWith('-');
  const unsigned = negative ? fixed.slice(1) : fixed;
  const [integerPart, decimalPart] = unsigned.split('.');
  const integerText = parts.useGrouping
    ? Number(integerPart || 0).toLocaleString('zh-CN', { maximumFractionDigits: 0 })
    : integerPart;
  return `${parts.prefix}${negative ? '-' : ''}${integerText}${decimalPart !== undefined ? `.${decimalPart}` : ''}${parts.suffix}`;
}

function useUpdatePulse(signature: string) {
  const [pulseToken, setPulseToken] = useState(0);
  const previousSignature = useRef<string | null>(null);

  useEffect(() => {
    if (previousSignature.current === null) {
      previousSignature.current = signature;
      return;
    }
    if (previousSignature.current === signature) return;
    previousSignature.current = signature;
    const frame = window.requestAnimationFrame(() => setPulseToken((current) => current + 1));
    return () => window.cancelAnimationFrame(frame);
  }, [signature]);

  return pulseToken;
}

function AnimatedStatValue({ value, pulseToken }: { value: string | number; pulseToken: number }) {
  const parts = useMemo(() => parseAnimatedValue(value), [value]);
  const [displayNumber, setDisplayNumber] = useState<number | null>(() => (parts ? 0 : null));
  const displayNumberRef = useRef<number | null>(parts ? 0 : null);
  const lastTargetRef = useRef<number | null>(null);

  useEffect(() => {
    if (!parts) {
      lastTargetRef.current = null;
      return;
    }

    const prefersReducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
    const from = lastTargetRef.current === null ? 0 : displayNumberRef.current ?? lastTargetRef.current;
    const to = parts.target;
    lastTargetRef.current = to;
    const updateDisplayNumber = (nextValue: number) => {
      displayNumberRef.current = nextValue;
      setDisplayNumber(nextValue);
    };

    if (prefersReducedMotion || Math.abs(to - from) < 0.000001) {
      const frame = window.requestAnimationFrame(() => updateDisplayNumber(to));
      return () => window.cancelAnimationFrame(frame);
    }

    const startedAt = performance.now();
    const duration = 850;
    let frame = 0;

    const tick = (now: number) => {
      const progress = Math.min(1, (now - startedAt) / duration);
      const eased = 1 - Math.pow(1 - progress, 3);
      updateDisplayNumber(from + (to - from) * eased);
      if (progress < 1) {
        frame = window.requestAnimationFrame(tick);
      } else {
        updateDisplayNumber(to);
      }
    };

    frame = window.requestAnimationFrame(tick);
    return () => window.cancelAnimationFrame(frame);
  }, [parts]);

  return (
    <span key={pulseToken} className={pulseToken > 0 ? 'stat-value-jump' : undefined}>
      {parts && displayNumber !== null ? formatAnimatedValue(displayNumber, parts) : value}
    </span>
  );
}

export function StatBlock({ title, value, subtext, trend, icon: Icon, color = 'neutral' }: StatBlockProps) {
  const getColorClasses = () => {
    switch (color) {
      case 'green':
        return {
          iconBg: 'bg-[#EAF8F0] text-[#12B76A]',
        };
      case 'cyan':
        return {
          iconBg: 'bg-[#EDF6FF] text-[#2B82CF]',
        };
      case 'amber':
        return {
          iconBg: 'bg-[#FFF6E7] text-[#D99217]',
        };
      default:
        return {
          iconBg: 'bg-[#F0F3F1] text-[#68756D]',
        };
    }
  };

  const { iconBg } = getColorClasses();
  const TrendIcon = trend?.type === 'positive' ? ArrowUpRight : trend?.type === 'negative' ? ArrowDownRight : Minus;
  const pulseToken = useUpdatePulse(`${String(value)}|${subtext || ''}|${trend?.value || ''}|${trend?.type || ''}`);

  return (
    <div className="stat-block-card group relative flex min-h-[108px] min-w-0 items-center gap-3 overflow-hidden rounded-[10px] border border-[#DCE4DF] bg-white px-4 py-3.5 shadow-[0_1px_2px_rgba(23,32,27,0.025)] transition-colors hover:border-[#B7D5C2]">
      {pulseToken > 0 && <span key={pulseToken} className="stat-block-update-glow" aria-hidden="true" />}
      <span className={`grid h-12 w-12 shrink-0 place-items-center rounded-[10px] ${iconBg}`}>
        {Icon ? <Icon className="h-5 w-5" strokeWidth={1.8} aria-hidden="true" /> : <span className="h-2 w-2 rounded-full bg-current" aria-hidden="true" />}
      </span>
      <div className="min-w-0 flex-1">
        <span className="block truncate text-[11px] font-semibold leading-[1.25] text-[#58655D]">{title}</span>
        <span className="stat-block-value mt-1.5 block truncate font-sans text-[23px] font-extrabold leading-none tracking-[-0.03em] text-[#17201B] tabular-nums" aria-label={String(value)}>
            <AnimatedStatValue value={value} pulseToken={pulseToken} />
        </span>
        {(subtext || trend) && (
          <div className="mt-2.5 flex min-w-0 items-center gap-1.5 truncate text-[9px] leading-none">
            {trend && (
              <span className={`inline-flex shrink-0 items-center gap-0.5 font-mono font-bold ${trend.type === 'positive' ? 'text-[#079455]' : trend.type === 'negative' ? 'text-[#D92D3F]' : 'text-[#68756D]'}`} aria-label={`${trend.label || '较昨日'} ${trend.value}`}>
                  <TrendIcon className="h-3 w-3" aria-hidden="true" />
                  {trend.value}
              </span>
            )}
            <span className="min-w-0 truncate text-[#8A948D]">{trend?.label || subtext}</span>
          </div>
        )}
      </div>
    </div>
  );
}
