'use client';

import { useEffect, useMemo, useState } from 'react';
import { CalendarRange, ChartNoAxesCombined, LoaderCircle, RefreshCw, Search } from 'lucide-react';
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { APIError, portalApi, type PortalUser, type UsageTrendPoint } from '@/lib/portal-api';

type RangePreset = 7 | 15 | 30 | 'custom';
type DateRange = { startDate: string; endDate: string };

const DAY_MS = 24 * 60 * 60 * 1000;

function localDateValue(date: Date): string {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 10);
}

function rangeForDays(days: number): DateRange {
  const end = new Date();
  const start = new Date(end);
  start.setDate(end.getDate() - days + 1);
  return { startDate: localDateValue(start), endDate: localDateValue(end) };
}

function inclusiveDays(startDate: string, endDate: string): number {
  const start = Date.parse(`${startDate}T00:00:00Z`);
  const end = Date.parse(`${endDate}T00:00:00Z`);
  return Math.floor((end - start) / DAY_MS) + 1;
}

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : '趋势数据加载失败';
}

function shortDate(value: string): string {
  const [, month = '', day = ''] = value.split('-');
  return `${month}/${day}`;
}

export function UsageTrendPanel({ user, refreshSignal }: { user: PortalUser | null; refreshSignal: number }) {
  const initialRange = useMemo(() => rangeForDays(7), []);
  const [preset, setPreset] = useState<RangePreset>(7);
  const [range, setRange] = useState<DateRange>(initialRange);
  const [customStart, setCustomStart] = useState(initialRange.startDate);
  const [customEnd, setCustomEnd] = useState(initialRange.endDate);
  const [data, setData] = useState<UsageTrendPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [requestNonce, setRequestNonce] = useState(0);
  const today = localDateValue(new Date());

  useEffect(() => {
    if (!user) return;
    let active = true;
    void Promise.resolve()
      .then(() => {
        if (active) {
          setLoading(true);
          setError('');
          setData([]);
        }
        return portalApi.usageTrend(user, range.startDate, range.endDate);
      })
      .then((response) => {
        if (active) setData(response.data || []);
      })
      .catch((loadError) => {
        if (active) setError(errorMessage(loadError));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => { active = false; };
  }, [range.endDate, range.startDate, refreshSignal, requestNonce, user]);

  const summary = useMemo(() => data.reduce(
    (result, point) => ({
      total: result.total + Number(point.total || 0),
      success: result.success + Number(point.success || 0),
      failed: result.failed + Number(point.failed || 0),
    }),
    { total: 0, success: 0, failed: 0 },
  ), [data]);

  const selectPreset = (days: 7 | 15 | 30) => {
    const nextRange = rangeForDays(days);
    setPreset(days);
    setCustomStart(nextRange.startDate);
    setCustomEnd(nextRange.endDate);
    setRange(nextRange);
    setRequestNonce((current) => current + 1);
  };

  const applyCustomRange = () => {
    if (!customStart || !customEnd) {
      setError('请选择完整的开始和结束日期');
      return;
    }
    const days = inclusiveDays(customStart, customEnd);
    if (days < 1) {
      setError('开始日期不能晚于结束日期');
      return;
    }
    if (days > 366) {
      setError('单次最多查询 366 天数据');
      return;
    }
    setError('');
    setRange({ startDate: customStart, endDate: customEnd });
    setRequestNonce((current) => current + 1);
  };

  const series = [
    { key: 'total', label: '调用量', value: summary.total, color: '#2563eb' },
    { key: 'success', label: '成功', value: summary.success, color: '#0f7a4b' },
    { key: 'failed', label: '失败', value: summary.failed, color: '#d92d20' },
  ] as const;

  return (
    <section className="section-panel min-w-0 w-full max-w-full overflow-hidden" aria-labelledby="usage-trend-title">
      <header className="flex min-h-[50px] flex-col gap-3 border-b border-[#edf0ee] px-4 py-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex items-center gap-2.5">
          <span className="grid size-8 shrink-0 place-items-center rounded-[7px] bg-blue-50 text-blue-600"><ChartNoAxesCombined size={16} /></span>
          <div>
            <strong id="usage-trend-title" className="block text-[13px]">调用趋势</strong>
            <small className="mt-0.5 block text-[10px] text-zinc-500">{range.startDate} 至 {range.endDate}</small>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <div className="inline-flex rounded-[7px] border border-[#dce4df] bg-[#f7f8f6] p-0.5" role="group" aria-label="趋势时间范围">
            {[7, 15, 30].map((days) => (
              <button
                key={days}
                type="button"
                className={`h-7 min-w-11 rounded-[5px] border-0 px-2 text-[10px] font-bold ${preset === days ? 'bg-white text-[#087443] shadow-sm' : 'bg-transparent text-zinc-500 hover:text-zinc-800'}`}
                onClick={() => selectPreset(days as 7 | 15 | 30)}
                aria-pressed={preset === days}
              >
                {days}天
              </button>
            ))}
            <button
              type="button"
              className={`h-7 rounded-[5px] border-0 px-2.5 text-[10px] font-bold ${preset === 'custom' ? 'bg-white text-[#087443] shadow-sm' : 'bg-transparent text-zinc-500 hover:text-zinc-800'}`}
              onClick={() => setPreset('custom')}
              aria-pressed={preset === 'custom'}
            >
              <span className="inline-flex items-center gap-1"><CalendarRange size={11} />自定义</span>
            </button>
          </div>
        </div>
      </header>

      {preset === 'custom' && (
        <div className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-2 border-b border-[#edf0ee] bg-[#fafbf9] px-4 py-3 sm:flex sm:justify-end">
          <label className="min-w-0">
            <span className="sr-only">开始日期</span>
            <input className="h-8 w-full rounded-[7px] border border-[#dce4df] bg-white px-2 text-[11px] outline-none focus:border-[#86efac] sm:w-[142px]" type="date" value={customStart} max={customEnd || today} onChange={(event) => setCustomStart(event.target.value)} />
          </label>
          <span className="text-[10px] text-zinc-400">至</span>
          <label className="min-w-0">
            <span className="sr-only">结束日期</span>
            <input className="h-8 w-full rounded-[7px] border border-[#dce4df] bg-white px-2 text-[11px] outline-none focus:border-[#86efac] sm:w-[142px]" type="date" value={customEnd} min={customStart} max={today} onChange={(event) => setCustomEnd(event.target.value)} />
          </label>
          <button className="btn primary col-span-3 w-full sm:col-span-1 sm:w-auto" type="button" onClick={applyCustomRange}>
            <Search size={13} />查询
          </button>
        </div>
      )}

      <div className="flex flex-wrap items-center gap-x-5 gap-y-2 border-b border-[#edf0ee] px-4 py-3" aria-label="趋势汇总">
        {series.map((item) => (
          <span key={item.key} className="inline-flex items-center gap-2 text-[10px] text-zinc-500">
            <i className="size-2 rounded-full" style={{ backgroundColor: item.color }} aria-hidden="true" />
            {item.label}<strong className="mono text-[12px] text-[#17201b]">{item.value.toLocaleString()}</strong>
          </span>
        ))}
        {loading && <span className="ml-auto inline-flex items-center gap-1.5 text-[10px] text-zinc-400" role="status" aria-live="polite"><LoaderCircle size={12} className="animate-spin" />更新中</span>}
        {!loading && error && <span className="ml-auto text-[10px] font-semibold text-[#b42318]" role="alert">{error}</span>}
      </div>

      <p className="sr-only" id="usage-trend-description">
        {loading ? '正在更新调用趋势。' : error ? `调用趋势加载失败：${error}` : `当前范围共调用 ${summary.total} 次，成功 ${summary.success} 次，失败 ${summary.failed} 次。`}
      </p>
      <div className="relative h-[260px] w-full px-1 pb-3 pt-3 sm:h-[300px] sm:px-3" aria-describedby="usage-trend-description">
        {!loading && error ? (
          <div className="grid h-full place-items-center text-center">
            <div>
              <p className="text-[11px] font-semibold text-[#b42318]">趋势数据加载失败</p>
              <button className="btn mt-3" type="button" onClick={() => setRequestNonce((current) => current + 1)}><RefreshCw size={13} />重试</button>
            </div>
          </div>
        ) : !loading && data.length === 0 ? (
          <div className="grid h-full place-items-center text-[11px] text-zinc-400">当前范围暂无调用数据</div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <LineChart accessibilityLayer data={data} margin={{ top: 8, right: 14, left: -8, bottom: 0 }}>
              <CartesianGrid stroke="#edf0ee" strokeDasharray="3 3" vertical={false} />
              <XAxis dataKey="date" tickFormatter={shortDate} tick={{ fill: '#778079', fontSize: 9 }} tickLine={false} axisLine={{ stroke: '#dce4df' }} minTickGap={22} />
              <YAxis allowDecimals={false} tick={{ fill: '#778079', fontSize: 9 }} tickLine={false} axisLine={false} width={44} />
              <Tooltip
                formatter={(value, name) => [Number(value || 0).toLocaleString(), String(name)]}
                labelFormatter={(label) => `日期 ${String(label)}`}
                contentStyle={{ border: '1px solid #dce4df', borderRadius: 7, boxShadow: '0 8px 24px rgba(23,32,27,.08)', fontSize: 10 }}
              />
              <Line type="monotone" dataKey="total" name="调用量" stroke="#2563eb" strokeWidth={2} dot={data.length <= 15 ? { r: 2 } : false} activeDot={{ r: 4 }} />
              <Line type="monotone" dataKey="success" name="成功" stroke="#0f7a4b" strokeWidth={2} dot={data.length <= 15 ? { r: 2 } : false} activeDot={{ r: 4 }} />
              <Line type="monotone" dataKey="failed" name="失败" stroke="#d92d20" strokeWidth={2} dot={data.length <= 15 ? { r: 2 } : false} activeDot={{ r: 4 }} />
            </LineChart>
          </ResponsiveContainer>
        )}
      </div>
    </section>
  );
}
