'use client';

import React from 'react';
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

  return (
    <div className={`group bg-white border border-[#DCE4DF] rounded-md overflow-hidden flex flex-col justify-between shadow-sm hover:shadow-md ${hoverBorder} transition-all duration-300 relative`}>
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
          <span className="font-sans text-2xl font-bold tracking-tight text-[#17201B] leading-none">
            {value}
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
