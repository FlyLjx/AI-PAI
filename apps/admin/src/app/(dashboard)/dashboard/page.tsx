'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Activity,
  Cable,
  ChartNoAxesCombined,
  CircleDollarSign,
  Clock3,
  HeartPulse,
  Loader2,
  RefreshCw,
  Server,
  TriangleAlert,
  Users,
} from 'lucide-react';
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { toast } from 'sonner';
import { PageHeader } from '@/components/common/PageHeader';
import { StatBlock } from '@/components/common/StatBlock';
import { portalApi, type StabilitySnapshot } from '@/lib/admin-api';
import { formatCNY, formatDate } from '@/lib/common/utils';

type RechargeRow = {
  id: string;
  userEmail?: string;
  userId?: string;
  orderType?: string;
  amount?: number;
  status?: string;
  createdAt?: string;
};

type UsageRow = {
  id: string;
  userEmail?: string;
  userId?: string;
  modelDisplayName?: string;
  modelName?: string;
  modelId?: string;
  quantity?: number;
  status?: string;
  createdAt?: string;
};

type TaskTrendPoint = {
  date: string;
  total: number;
  queued: number;
  pending: number;
  processing: number;
  running: number;
  success: number;
  failed: number;
  canceled: number;
};

type DashboardData = {
  today?: {
    users?: number;
    orders?: number;
    paidAmount?: number;
    tasks?: number;
    runningTasks?: number;
    failedTasks?: number;
    successfulTasks?: number;
  };
  yesterday?: {
    users?: number;
    orders?: number;
    paidAmount?: number;
    tasks?: number;
    runningTasks?: number;
    failedTasks?: number;
    successfulTasks?: number;
  };
  users?: { total?: number; active?: number };
  orders?: { all?: number; paid?: number; pending?: number; failed?: number; closed?: number };
  taskStats?: {
    total?: number;
    queued?: number;
    pending?: number;
    processing?: number;
    success?: number;
    failed?: number;
    canceled?: number;
    totalImages?: number;
  };
  pending?: { pendingOrders?: number; runningTasks?: number; recentFailedTasks?: number };
  system?: {
    api?: string;
    database?: string;
    activeProviders?: number;
    disabledProviders?: number;
    activeModels?: number;
    disabledModels?: number;
    lastTaskAt?: string | null;
  };
  recentOrders?: RechargeRow[];
  recentTasks?: UsageRow[];
  taskTrend?: TaskTrendPoint[];
};

const TASK_TREND_COLORS = {
  total: '#587FA3',
  success: '#3F9274',
  failed: '#D06F69',
  running: '#D69A45',
  canceled: '#8A7FB0',
} as const;

function shortDate(value: string): string {
  const [, month = '', day = ''] = value.split('-');
  return `${month}/${day}`;
}

function percentage(value: number | undefined): string {
  return `${Number(value || 0).toFixed(1)}%`;
}

function durationLabel(value: number | undefined): string {
  const seconds = Math.max(0, Number(value || 0));
  if (seconds < 1) return `${Math.round(seconds * 1000)}ms`;
  if (seconds < 60) return `${seconds.toFixed(1)}秒`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes}分${Math.round(seconds % 60)}秒`;
}

function dayOverDayTrend(currentValue: number | null, previousValue: number | null) {
  if (currentValue === null || previousValue === null || !Number.isFinite(currentValue) || !Number.isFinite(previousValue)) {
    return { value: '--', type: 'neutral' as const, label: '今日较昨日' };
  }
  if (previousValue === 0) {
    if (currentValue === 0) return { value: '0%', type: 'neutral' as const, label: '今日较昨日' };
    return { value: '新增', type: 'positive' as const, label: '今日较昨日' };
  }
  const change = ((currentValue - previousValue) / Math.abs(previousValue)) * 100;
  const rounded = Math.round(Math.abs(change) * 10) / 10;
  const value = `${rounded.toLocaleString('zh-CN', { maximumFractionDigits: 1 })}%`;
  if (change > 0) return { value: `+${value}`, type: 'positive' as const, label: '今日较昨日' };
  if (change < 0) return { value: `-${value}`, type: 'negative' as const, label: '今日较昨日' };
  return { value, type: 'neutral' as const, label: '今日较昨日' };
}

function dailySuccessRate(success: number | undefined, failed: number | undefined): number | null {
  const successCount = Number(success || 0);
  const failedCount = Number(failed || 0);
  const completed = successCount + failedCount;
  return completed > 0 ? (successCount / completed) * 100 : null;
}

function statusView(status = '') {
  if (status === 'paid' || status === 'success') return { label: '成功', className: 'border-emerald-200 bg-emerald-50 text-emerald-700' };
  if (status === 'pending' || status === 'queued' || status === 'processing') return { label: '处理中', className: 'border-amber-200 bg-amber-50 text-amber-700' };
  if (status === 'closed') return { label: '已关闭', className: 'border-zinc-200 bg-zinc-50 text-zinc-500' };
  if (status === 'failed' || status === 'canceled') return { label: '失败', className: 'border-red-200 bg-red-50 text-red-700' };
  return { label: status || '未知', className: 'border-zinc-200 bg-zinc-50 text-zinc-600' };
}

export default function AdminDashboardPage() {
  const [data, setData] = useState<DashboardData>({});
  const [stability, setStability] = useState<StabilitySnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [stabilityLoading, setStabilityLoading] = useState(true);
  const [error, setError] = useState('');
  const [stabilityError, setStabilityError] = useState('');
  const [lastUpdated, setLastUpdated] = useState('');
  const [trendDays, setTrendDays] = useState<7 | 15 | 30>(7);

  const load = useCallback(async (quiet = false) => {
    if (!quiet) setLoading(true);
    setError('');
    try {
      const response = await portalApi.dashboard();
      setData(response.data as DashboardData);
      setLastUpdated(new Date().toISOString());
    } catch (requestError) {
      const message = requestError instanceof Error ? requestError.message : '经营数据加载失败';
      setError(message);
      if (quiet) toast.error(message);
    } finally {
      if (!quiet) setLoading(false);
    }
  }, []);

  const loadStability = useCallback(async () => {
    setStabilityLoading(true);
    setStabilityError('');
    try {
      const response = await portalApi.stability();
      setStability(response.data);
    } catch (requestError) {
      setStabilityError(requestError instanceof Error ? requestError.message : '上游接口状态加载失败');
    } finally {
      setStabilityLoading(false);
    }
  }, []);

  const refreshAll = useCallback(async (quiet = false) => {
    await Promise.allSettled([load(quiet), loadStability()]);
  }, [load, loadStability]);

  useEffect(() => {
    const firstLoad = window.setTimeout(() => void refreshAll(), 0);
    const timer = window.setInterval(() => {
      if (!document.hidden) void refreshAll(true);
    }, 30_000);
    return () => {
      window.clearTimeout(firstLoad);
      window.clearInterval(timer);
    };
  }, [refreshAll]);

  const stats = data.taskStats || {};
  const successful = Number(stats.success || 0);
  const failed = Number(stats.failed || 0) + Number(stats.canceled || 0);
  const completed = successful + failed;
  const successRate = completed ? Math.round((successful / completed) * 100) : 100;
  const revenueTrend = dayOverDayTrend(Number(data.today?.paidAmount || 0), Number(data.yesterday?.paidAmount || 0));
  const requestTrend = dayOverDayTrend(Number(data.today?.tasks || 0), Number(data.yesterday?.tasks || 0));
  const successRateTrend = dayOverDayTrend(
    dailySuccessRate(data.today?.successfulTasks, data.today?.failedTasks),
    dailySuccessRate(data.yesterday?.successfulTasks, data.yesterday?.failedTasks),
  );
  const customerTrend = dayOverDayTrend(Number(data.today?.users || 0), Number(data.yesterday?.users || 0));
  const pendingCount = Number(data.pending?.runningTasks || 0) + Number(data.pending?.pendingOrders || 0);
  const recentOrders = data.recentOrders || [];
  const recentTasks = data.recentTasks || [];
  const taskTrend = useMemo(() => (data.taskTrend || []).slice(-trendDays), [data.taskTrend, trendDays]);
  const taskTrendSummary = useMemo(() => taskTrend.reduce((summary, point) => ({
    total: summary.total + Number(point.total || 0),
    success: summary.success + Number(point.success || 0),
    failed: summary.failed + Number(point.failed || 0),
    running: summary.running + Number(point.running || 0),
    canceled: summary.canceled + Number(point.canceled || 0),
  }), { total: 0, success: 0, failed: 0, running: 0, canceled: 0 }), [taskTrend]);
  const taskTrendSeries = [
    { key: 'total', label: '全部任务', value: taskTrendSummary.total, color: TASK_TREND_COLORS.total },
    { key: 'success', label: '成功', value: taskTrendSummary.success, color: TASK_TREND_COLORS.success },
    { key: 'failed', label: '失败', value: taskTrendSummary.failed, color: TASK_TREND_COLORS.failed },
    { key: 'running', label: '处理中', value: taskTrendSummary.running, color: TASK_TREND_COLORS.running },
    { key: 'canceled', label: '已取消', value: taskTrendSummary.canceled, color: TASK_TREND_COLORS.canceled },
  ] as const;

  const upstreamCode = Number(stability?.upstream_status_code || 0);
  const upstreamReachable = Boolean(stability?.reachable && upstreamCode >= 200 && upstreamCode < 300);
  const upstreamRuntime = stability?.runtime;
  const upstreamRecent = stability?.recent_60;
  const upstreamPending = stabilityLoading && !stability;
  const upstreamDegraded = upstreamReachable && Number(upstreamRuntime?.error_rate || 0) > 0;
  const upstreamLabel = upstreamReachable ? upstreamDegraded ? '运行波动' : '运行正常' : '连接异常';
  const upstreamTone = upstreamReachable
    ? upstreamDegraded
      ? 'border-amber-200 bg-amber-50 text-amber-700'
      : 'border-emerald-200 bg-emerald-50 text-emerald-700'
    : 'border-red-200 bg-red-50 text-red-700';
  const upstreamIconTone = upstreamReachable
    ? upstreamDegraded ? 'bg-amber-50 text-amber-700' : 'bg-emerald-50 text-emerald-700'
    : upstreamPending ? 'bg-zinc-100 text-zinc-500' : 'bg-red-50 text-red-700';

  const attention = useMemo(() => [
    { label: '待支付充值单', value: Number(data.pending?.pendingOrders || 0), note: '等待支付结果同步', tone: 'amber' },
    { label: '运行中 API 请求', value: Number(data.pending?.runningTasks || 0), note: '排队与上游处理中', tone: 'blue' },
    { label: '24 小时失败', value: Number(data.pending?.recentFailedTasks || 0), note: '建议检查上游和密钥', tone: 'red' },
  ], [data.pending]);

  return (
    <div className="space-y-5">
      <PageHeader title="经营概览" description="API 中转业务、订阅收入和上游运行状态。">
        <button
          type="button"
          onClick={() => void refreshAll()}
          disabled={loading || stabilityLoading}
          className="inline-flex h-8 items-center gap-2 rounded-md border border-[#DCE4DF] bg-white px-3 text-xs font-semibold text-[#17201B] hover:border-[#12B76A] disabled:opacity-50"
        >
          <RefreshCw className={`h-3.5 w-3.5 ${loading || stabilityLoading ? 'animate-spin' : ''}`} />
          刷新
        </button>
      </PageHeader>

      {error && !loading && (
        <div className="flex items-center justify-between gap-3 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700">
          <span className="flex items-center gap-2"><TriangleAlert className="h-4 w-4" />{error}</span>
          <button type="button" onClick={() => void load()} className="font-semibold underline">重试</button>
        </div>
      )}

      {loading ? (
        <div className="grid min-h-[320px] place-items-center rounded-md border border-[#DCE4DF] bg-white">
          <Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" />
        </div>
      ) : (
        <>
          <section className="min-w-0 overflow-hidden rounded-md border border-[#DCE4DF] bg-white" aria-labelledby="task-trend-title">
            <header className="flex min-h-[54px] flex-col gap-3 border-b border-[#EDF0EE] px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-2.5">
                <span className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-blue-50 text-[#587FA3]"><ChartNoAxesCombined className="h-4 w-4" /></span>
                <div><h2 id="task-trend-title" className="text-sm font-semibold text-[#17201B]">任务处理趋势</h2><p className="mt-0.5 text-[11px] text-zinc-500">{taskTrend[0]?.date || '-'} 至 {taskTrend.at(-1)?.date || '-'}</p></div>
              </div>
              <div className="inline-flex self-start rounded-md border border-[#DCE4DF] bg-[#F7F8F6] p-0.5 sm:self-auto" role="group" aria-label="任务趋势时间范围">
                {[7, 15, 30].map((days) => (
                  <button key={days} type="button" onClick={() => setTrendDays(days as 7 | 15 | 30)} aria-pressed={trendDays === days} className={`h-7 min-w-12 rounded px-2 text-[11px] font-semibold ${trendDays === days ? 'bg-white text-[#047857] shadow-sm' : 'text-zinc-500 hover:text-zinc-800'}`}>{days}天</button>
                ))}
              </div>
            </header>
            <div className="flex min-h-[45px] flex-wrap items-center gap-x-5 gap-y-2 border-b border-[#EDF0EE] px-4 py-2.5" aria-label="任务趋势汇总">
              {taskTrendSeries.map((item) => (
                <span key={item.key} className="inline-flex items-center gap-2 text-[11px] text-zinc-500"><i className="h-2 w-2 rounded-full" style={{ backgroundColor: item.color }} aria-hidden="true" />{item.label}<strong className="font-mono text-xs text-[#17201B]">{item.value.toLocaleString('zh-CN')}</strong></span>
              ))}
            </div>
            <p className="sr-only" id="task-trend-description">当前范围共 {taskTrendSummary.total} 个任务，成功 {taskTrendSummary.success} 个，失败 {taskTrendSummary.failed} 个，处理中 {taskTrendSummary.running} 个，已取消 {taskTrendSummary.canceled} 个。</p>
            <div className="h-[280px] w-full px-1 pb-3 pt-4 sm:h-[320px] sm:px-3" aria-describedby="task-trend-description">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart accessibilityLayer data={taskTrend} margin={{ top: 6, right: 14, left: -8, bottom: 0 }}>
                  <CartesianGrid stroke="#EDF0EE" strokeDasharray="3 3" vertical={false} />
                  <XAxis dataKey="date" tickFormatter={shortDate} tick={{ fill: '#778079', fontSize: 9 }} tickLine={false} axisLine={{ stroke: '#DCE4DF' }} minTickGap={22} />
                  <YAxis allowDecimals={false} tick={{ fill: '#778079', fontSize: 9 }} tickLine={false} axisLine={false} width={44} />
                  <Tooltip formatter={(value, name) => [Number(value || 0).toLocaleString('zh-CN'), String(name)]} labelFormatter={(label) => `日期 ${String(label)}`} contentStyle={{ border: '1px solid #DCE4DF', borderRadius: 7, boxShadow: '0 8px 24px rgba(23,32,27,.08)', fontSize: 10 }} />
                  <Line type="monotone" dataKey="total" name="全部任务" stroke={TASK_TREND_COLORS.total} strokeWidth={2.25} dot={taskTrend.length <= 15 ? { r: 2.25, fill: '#fff', strokeWidth: 1.75 } : false} activeDot={{ r: 4, fill: '#fff', strokeWidth: 2.25 }} />
                  <Line type="monotone" dataKey="success" name="成功" stroke={TASK_TREND_COLORS.success} strokeWidth={2.25} dot={taskTrend.length <= 15 ? { r: 2.25, fill: '#fff', strokeWidth: 1.75 } : false} activeDot={{ r: 4, fill: '#fff', strokeWidth: 2.25 }} />
                  <Line type="monotone" dataKey="failed" name="失败" stroke={TASK_TREND_COLORS.failed} strokeWidth={2.25} strokeDasharray="5 4" dot={taskTrend.length <= 15 ? { r: 2.25, fill: '#fff', strokeWidth: 1.75 } : false} activeDot={{ r: 4, fill: '#fff', strokeWidth: 2.25 }} />
                  <Line type="monotone" dataKey="running" name="处理中" stroke={TASK_TREND_COLORS.running} strokeWidth={2} dot={false} activeDot={{ r: 4, fill: '#fff', strokeWidth: 2 }} />
                  <Line type="monotone" dataKey="canceled" name="已取消" stroke={TASK_TREND_COLORS.canceled} strokeWidth={1.75} strokeDasharray="3 4" dot={false} activeDot={{ r: 4, fill: '#fff', strokeWidth: 2 }} />
                </LineChart>
              </ResponsiveContainer>
            </div>
          </section>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <StatBlock title="今日实收" value={formatCNY(Number(data.today?.paidAmount || 0))} subtext={`${Number(data.today?.orders || 0)} 笔充值/订阅订单`} trend={revenueTrend} color="green" icon={CircleDollarSign} />
            <StatBlock title="累计 API 请求" value={Number(stats.total || 0).toLocaleString('zh-CN')} subtext={`今日 ${Number(data.today?.tasks || 0).toLocaleString('zh-CN')} 次`} trend={requestTrend} color="cyan" icon={Activity} />
            <StatBlock title="请求成功率" value={`${successRate}%`} subtext={`累计返回 ${Number(stats.totalImages || 0).toLocaleString('zh-CN')} 张图片`} trend={successRateTrend} color={successRate >= 95 ? 'green' : 'amber'} icon={Cable} />
            <StatBlock title="API 客户" value={Number(data.users?.total || 0).toLocaleString('zh-CN')} subtext={`启用 ${Number(data.users?.active || 0)}，今日新增 ${Number(data.today?.users || 0)}`} trend={customerTrend} color="neutral" icon={Users} />
          </div>

          <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white" aria-labelledby="upstream-status-title">
            <div className={`h-1 w-full ${upstreamReachable ? upstreamDegraded ? 'bg-amber-500' : 'bg-[#3F9274]' : upstreamPending ? 'bg-zinc-300' : 'bg-[#D06F69]'}`} />
            <header className="flex flex-col gap-3 border-b border-[#EDF0EE] px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex min-w-0 items-center gap-3">
                <span className={`grid h-9 w-9 shrink-0 place-items-center rounded-md ${upstreamIconTone}`}>
                  {upstreamReachable ? <HeartPulse className="h-4.5 w-4.5" /> : <Server className="h-4.5 w-4.5" />}
                </span>
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 id="upstream-status-title" className="text-sm font-semibold text-[#17201B]">上游接口运行状况</h2>
                    {!stabilityLoading || stability ? <span className={`rounded border px-2 py-0.5 text-[11px] font-semibold ${upstreamTone}`}>{upstreamLabel}</span> : null}
                  </div>
                  <p className="mt-0.5 text-[11px] text-zinc-500">
                    {stabilityLoading && !stability ? '正在连接上游状态服务...' : stabilityError || stability?.error || `最近 ${Number(upstreamRuntime?.window_minutes || 60)} 分钟运行统计`}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-2 text-[11px] text-zinc-400">
                {stabilityLoading && <Loader2 className="h-3.5 w-3.5 animate-spin" aria-label="检测中" />}
                <span>检测时间：{stability?.fetched_at ? formatDate(stability.fetched_at) : '-'}</span>
              </div>
            </header>
            <div className="grid grid-cols-2 divide-x divide-y divide-[#EDF0EE] sm:grid-cols-3 sm:divide-y-0 xl:grid-cols-6">
              {[
                ['接口状态', stabilityLoading && !stability ? '--' : upstreamReachable ? '在线' : '异常', upstreamCode ? `HTTP ${upstreamCode}` : '暂无状态码'],
                ['近一小时成功率', stabilityLoading && !stability ? '--' : percentage(upstreamRuntime?.success_rate), `${Number(upstreamRuntime?.total || 0).toLocaleString('zh-CN')} 次请求`],
                ['成功请求', Number(upstreamRuntime?.totals?.success || 0).toLocaleString('zh-CN'), '最近 60 分钟'],
                ['失败请求', Number(upstreamRuntime?.totals?.failed || 0).toLocaleString('zh-CN'), `错误率 ${percentage(upstreamRuntime?.error_rate)}`],
                ['平均耗时', stabilityLoading && !stability ? '--' : durationLabel(upstreamRecent?.average_duration_secs), `成功 ${durationLabel(upstreamRecent?.average_success_duration_secs)}`],
                ['最近任务', Number(upstreamRecent?.total || 0).toLocaleString('zh-CN'), `${Number(upstreamRecent?.success || 0)} 成功 / ${Number(upstreamRecent?.failed || 0)} 失败`],
              ].map(([label, value, note]) => (
                <div key={label} className="min-w-0 px-4 py-3.5">
                  <span className="block text-[11px] font-semibold text-zinc-500">{label}</span>
                  <strong className="mt-1.5 block truncate text-base text-[#17201B]">{value}</strong>
                  <small className="mt-0.5 block truncate text-[10px] text-zinc-400">{note}</small>
                </div>
              ))}
            </div>
            {stabilityError && (
              <div className="flex items-center gap-2 border-t border-red-100 bg-red-50 px-4 py-2.5 text-[11px] text-red-700" role="alert">
                <TriangleAlert className="h-3.5 w-3.5 shrink-0" />
                状态检测失败：{stabilityError}
              </div>
            )}
          </section>

          <div className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1.4fr)_minmax(300px,.6fr)]">
            <section className="rounded-md border border-[#DCE4DF] bg-white">
              <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[#DCE4DF] px-4 py-3">
                <div>
                  <h2 className="text-sm font-semibold text-[#17201B]">今日运行</h2>
                  <p className="mt-0.5 text-[12px] text-zinc-500">从本地时区 00:00 开始统计</p>
                </div>
                <span className={`rounded border px-2 py-1 text-[11px] font-semibold ${pendingCount ? 'border-amber-200 bg-amber-50 text-amber-700' : 'border-emerald-200 bg-emerald-50 text-emerald-700'}`}>
                  {pendingCount ? `${pendingCount} 项处理中` : '队列平稳'}
                </span>
              </div>
              <div className="grid grid-cols-2 divide-x divide-y divide-[#EDF0EE] sm:grid-cols-4 sm:divide-y-0">
                {[
                  ['API 请求', data.today?.tasks],
                  ['运行中', data.today?.runningTasks],
                  ['失败请求', data.today?.failedTasks],
                  ['新增客户', data.today?.users],
                ].map(([label, value]) => (
                  <div key={String(label)} className="p-4">
                    <span className="text-[11px] font-semibold text-zinc-500">{label}</span>
                    <strong className="mt-2 block text-xl text-[#17201B]">{Number(value || 0).toLocaleString('zh-CN')}</strong>
                  </div>
                ))}
              </div>
              <div className="grid grid-cols-1 gap-3 border-t border-[#DCE4DF] bg-[#FAFBFA] p-4 sm:grid-cols-3">
                {attention.map((item) => (
                  <div key={item.label} className="flex items-center gap-3 rounded-md border border-[#E5E9E6] bg-white px-3 py-2.5">
                    <span className={`h-2 w-2 rounded-full ${item.tone === 'red' ? 'bg-red-500' : item.tone === 'amber' ? 'bg-amber-500' : 'bg-blue-500'}`} />
                    <span className="min-w-0 flex-1">
                      <span className="block text-[11px] text-zinc-500">{item.label}</span>
                      <strong className="block text-base text-[#17201B]">{item.value}</strong>
                      <small className="block truncate text-[11px] text-zinc-400">{item.note}</small>
                    </span>
                  </div>
                ))}
              </div>
            </section>

            <section className="rounded-md border border-[#DCE4DF] bg-white">
              <div className="border-b border-[#DCE4DF] px-4 py-3">
                <h2 className="text-sm font-semibold text-[#17201B]">资源状态</h2>
                <p className="mt-0.5 text-[12px] text-zinc-500">接口与模型可用性</p>
              </div>
              <div className="divide-y divide-[#EDF0EE] px-4">
                {[
                  ['启用上游', data.system?.activeProviders, `${Number(data.system?.disabledProviders || 0)} 个停用`],
                  ['启用模型', data.system?.activeModels, `${Number(data.system?.disabledModels || 0)} 个停用`],
                  ['累计充值单', data.orders?.all, `${Number(data.orders?.paid || 0)} 笔已支付`],
                ].map(([label, value, note]) => (
                  <div key={String(label)} className="flex items-center justify-between gap-4 py-3 text-xs">
                    <span className="text-zinc-600">{label}</span>
                    <span className="text-right"><strong className="block text-[#17201B]">{Number(value || 0)}</strong><small className="text-[11px] text-zinc-400">{note}</small></span>
                  </div>
                ))}
              </div>
              <div className="flex items-center gap-2 border-t border-[#DCE4DF] bg-[#FAFBFA] px-4 py-3 text-[11px] text-zinc-500">
                <Clock3 className="h-3.5 w-3.5" />
                最近请求：{data.system?.lastTaskAt ? formatDate(data.system.lastTaskAt) : '暂无记录'}
              </div>
            </section>
          </div>

          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white">
              <div className="border-b border-[#DCE4DF] px-4 py-3"><h2 className="text-sm font-semibold">最近充值与订阅</h2></div>
              <div className="overflow-x-auto">
                <table className="w-full min-w-[560px] text-left text-xs">
                  <thead className="bg-[#F6F8F6] text-[11px] text-zinc-500"><tr><th className="px-4 py-2">客户</th><th className="px-4 py-2">类型</th><th className="px-4 py-2">金额</th><th className="px-4 py-2">状态</th><th className="px-4 py-2">时间</th></tr></thead>
                  <tbody className="divide-y divide-[#EDF0EE]">
                    {recentOrders.map((row) => { const status = statusView(row.status); return (
                      <tr key={row.id}><td className="max-w-[180px] truncate px-4 py-2.5">{row.userEmail || row.userId || '-'}</td><td className="px-4 py-2.5">{row.orderType === 'subscription' ? '订阅' : '余额'}</td><td className="px-4 py-2.5 font-mono">{formatCNY(Number(row.amount || 0))}</td><td className="px-4 py-2.5"><span className={`rounded border px-1.5 py-0.5 text-[11px] ${status.className}`}>{status.label}</span></td><td className="px-4 py-2.5 text-zinc-500">{formatDate(row.createdAt || '')}</td></tr>
                    ); })}
                    {!recentOrders.length && <tr><td colSpan={5} className="px-4 py-8 text-center text-zinc-400">暂无充值记录</td></tr>}
                  </tbody>
                </table>
              </div>
            </section>

            <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white">
              <div className="border-b border-[#DCE4DF] px-4 py-3"><h2 className="text-sm font-semibold">最近 API 请求</h2></div>
              <div className="overflow-x-auto">
                <table className="w-full min-w-[560px] text-left text-xs">
                  <thead className="bg-[#F6F8F6] text-[11px] text-zinc-500"><tr><th className="px-4 py-2">客户</th><th className="px-4 py-2">模型</th><th className="px-4 py-2">数量</th><th className="px-4 py-2">状态</th><th className="px-4 py-2">时间</th></tr></thead>
                  <tbody className="divide-y divide-[#EDF0EE]">
                    {recentTasks.map((row) => { const status = statusView(row.status); return (
                      <tr key={row.id}><td className="max-w-[160px] truncate px-4 py-2.5">{row.userEmail || row.userId || '-'}</td><td className="max-w-[160px] truncate px-4 py-2.5">{row.modelDisplayName || row.modelName || row.modelId || '-'}</td><td className="px-4 py-2.5 font-mono">{Number(row.quantity || 0)}</td><td className="px-4 py-2.5"><span className={`rounded border px-1.5 py-0.5 text-[11px] ${status.className}`}>{status.label}</span></td><td className="px-4 py-2.5 text-zinc-500">{formatDate(row.createdAt || '')}</td></tr>
                    ); })}
                    {!recentTasks.length && <tr><td colSpan={5} className="px-4 py-8 text-center text-zinc-400">暂无 API 请求</td></tr>}
                  </tbody>
                </table>
              </div>
            </section>
          </div>

          <p className="text-right text-[11px] text-zinc-400">最后同步：{lastUpdated ? formatDate(lastUpdated) : '-'}</p>
        </>
      )}
    </div>
  );
}
