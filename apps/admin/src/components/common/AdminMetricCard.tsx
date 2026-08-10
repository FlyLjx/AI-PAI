'use client';

import { ArrowDownRight, ArrowUpRight, LucideIcon, Minus } from 'lucide-react';

export type AdminMetricTone = 'green' | 'blue' | 'amber' | 'red' | 'neutral';

export type AdminMetricTrend = {
  value: string;
  type: 'positive' | 'negative' | 'neutral';
  label?: string;
};

type AdminMetricCardProps = {
  title: string;
  value: string | number;
  note?: string;
  trend?: AdminMetricTrend;
  icon?: LucideIcon;
  tone?: AdminMetricTone;
  className?: string;
};

const toneClasses: Record<AdminMetricTone, { icon: string; iconBg: string; trend: string }> = {
  green: { icon: 'text-[#12B76A]', iconBg: 'bg-[#EAF8F0]', trend: 'text-[#079455]' },
  blue: { icon: 'text-[#2B82CF]', iconBg: 'bg-[#EDF6FF]', trend: 'text-[#2B82CF]' },
  amber: { icon: 'text-[#D99217]', iconBg: 'bg-[#FFF6E7]', trend: 'text-[#B7791F]' },
  red: { icon: 'text-[#F0444C]', iconBg: 'bg-[#FFF0F1]', trend: 'text-[#D92D3F]' },
  neutral: { icon: 'text-[#68756D]', iconBg: 'bg-[#F0F3F1]', trend: 'text-[#68756D]' },
};

export function AdminMetricCard({ title, value, note, trend, icon: Icon, tone = 'neutral', className = '' }: AdminMetricCardProps) {
  const colors = toneClasses[tone];
  const TrendIcon = trend?.type === 'positive' ? ArrowUpRight : trend?.type === 'negative' ? ArrowDownRight : Minus;

  return (
    <article className={`min-w-0 min-h-[108px] flex items-center gap-3 rounded-[10px] border border-[#DCE4DF] bg-white px-4 py-3.5 shadow-[0_1px_2px_rgba(23,32,27,0.025)] transition-colors hover:border-[#B7D5C2] ${className}`.trim()}>
      <span className={`grid h-12 w-12 shrink-0 place-items-center rounded-[10px] ${colors.iconBg} ${colors.icon}`}>
        {Icon ? <Icon className="h-5 w-5" strokeWidth={1.8} aria-hidden="true" /> : <span className="h-2 w-2 rounded-full bg-current" aria-hidden="true" />}
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[11px] font-semibold leading-[1.25] text-[#58655D]">{title}</span>
        <strong className="mt-1.5 block truncate font-sans text-[23px] font-extrabold leading-none tracking-[-0.03em] text-[#17201B] tabular-nums">{value}</strong>
        {(trend || note) && (
          <span className="mt-2.5 flex min-w-0 items-center gap-1.5 truncate text-[9px] leading-none">
            {trend ? (
              <span className={`inline-flex shrink-0 items-center gap-0.5 font-mono font-bold ${trend.type === 'positive' ? 'text-[#079455]' : trend.type === 'negative' ? 'text-[#D92D3F]' : 'text-[#68756D]'}`}>
                <TrendIcon className="h-3 w-3" strokeWidth={2.3} aria-hidden="true" />
                {trend.value}
              </span>
            ) : null}
            <span className="min-w-0 truncate text-[#8A948D]">{trend?.label || note}</span>
          </span>
        )}
      </span>
    </article>
  );
}
