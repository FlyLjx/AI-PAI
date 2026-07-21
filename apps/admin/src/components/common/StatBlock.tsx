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
          iconBg: 'bg-[#12B76A]/8 text-[#12B76A]',
          accentBar: 'bg-[#12B76A]',
          hoverBorder: 'hover:border-[#12B76A]'
        };
      case 'cyan':
        return {
          iconBg: 'bg-[#0891B2]/8 text-[#0891B2]',
          accentBar: 'bg-[#0891B2]',
          hoverBorder: 'hover:border-[#0891B2]'
        };
      case 'amber':
        return {
          iconBg: 'bg-[#D97706]/8 text-[#D97706]',
          accentBar: 'bg-[#D97706]',
          hoverBorder: 'hover:border-[#D97706]'
        };
      default:
        return {
          iconBg: 'bg-[#17201B]/5 text-[#17201B]/60',
          accentBar: 'bg-[#DCE4DF]',
          hoverBorder: 'hover:border-[#12B76A]'
        };
    }
  };

  const { iconBg, accentBar, hoverBorder } = getColorClasses();
  const TrendIcon = trend?.type === 'positive' ? ArrowUpRight : trend?.type === 'negative' ? ArrowDownRight : Minus;
  const pulseToken = useUpdatePulse(`${String(value)}|${subtext || ''}|${trend?.value || ''}|${trend?.type || ''}`);

  return (
    <div className={`stat-block-card group bg-white border border-[#DCE4DF] rounded-md overflow-hidden flex flex-col justify-between shadow-sm hover:shadow-md ${hoverBorder} transition-all duration-300 relative`}>
      {pulseToken > 0 && <span key={pulseToken} className="stat-block-update-glow" aria-hidden="true" />}
      {/* Subtle top accent bar */}
      <div className={`h-[3px] w-full ${accentBar}`} />

      <div className="p-4 flex flex-col justify-between flex-1">
        <div className="flex items-center justify-between gap-3">
          <span className="text-[11px] font-bold text-[#17201B]/50 uppercase tracking-wider">
            {title}
          </span>
          {Icon && (
            <div className={`p-1.5 rounded-md ${iconBg} transition-transform duration-300 group-hover:scale-105`}>
              <Icon className="w-3.5 h-3.5" />
            </div>
          )}
        </div>

        <div className="mt-3 flex items-baseline gap-2">
          <span className="stat-block-value font-sans text-2xl font-bold tracking-tight text-[#17201B] leading-none tabular-nums" aria-label={String(value)}>
            <AnimatedStatValue value={value} pulseToken={pulseToken} />
          </span>
        </div>

        {(subtext || trend) && (
          <div className="mt-3 min-h-[54px] border-t border-[#F0F3F1] pt-2.5 text-[11px]">
            {trend && (
              <div className="flex items-center justify-between gap-2" aria-label={`${trend.label || '较昨日'} ${trend.value}`}>
                <span className="text-zinc-400">{trend.label || '较昨日'}</span>
                <span className={`inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 font-mono font-bold ${
                  trend.type === 'positive' ? 'bg-[#12B76A]/8 text-[#079455]' :
                  trend.type === 'negative' ? 'bg-[#DC2626]/8 text-[#DC2626]' : 'bg-zinc-100 text-zinc-500'
                }`}>
                  <TrendIcon className="h-3 w-3" aria-hidden="true" />
                  {trend.value}
                </span>
              </div>
            )}
            {subtext && (
              <div className={`${trend ? 'mt-2 border-t border-[#F6F8F6] pt-2' : ''} flex items-center justify-between gap-2`}>
                <span className="min-w-0 truncate font-medium text-zinc-500">
                  {subtext}
                </span>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
