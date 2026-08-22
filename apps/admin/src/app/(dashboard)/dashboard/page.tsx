'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Activity,
  Cable,
  ChartNoAxesCombined,
  CircleDollarSign,
  HeartPulse,
  Loader2,
  RefreshCw,
  Server,
  TriangleAlert,
  Users,
  Wallet,
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
import { AdminMetricCard } from '@/components/common/AdminMetricCard';
import { SortableHeader, sortItems, type SortState, type TableHeader } from '@/components/common/DataTable';
import { portalApi, type OpenAIImageStatusSnapshot, type StabilitySnapshot } from '@/lib/admin-api';
import { formatCNY, formatDate } from '@/lib/common/utils';
import {
  ADMIN_CHART_ACTIVE_DOT,
  ADMIN_CHART_AXIS,
  ADMIN_CHART_DOT,
  ADMIN_CHART_MARGIN,
} from '@/lib/chart-theme';

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

const recentTaskHeaders: TableHeader<UsageRow>[] = [
  { key: 'user', label: '客户', sortValue: (row) => row.userEmail || row.userId || '' },
  { key: 'model', label: '模型', sortValue: (row) => row.modelDisplayName || row.modelName || row.modelId || '' },
  { key: 'quantity', label: '数量', sortValue: (row) => Number(row.quantity || 0) },
  { key: 'status', label: '状态', sortValue: (row) => row.status || '' },
  { key: 'createdAt', label: '时间', sortValue: (row) => Date.parse(row.createdAt || '') || 0 },
];

const recentOrderHeaders: TableHeader<RechargeRow>[] = [
  { key: 'user', label: '客户', sortValue: (row) => row.userEmail || row.userId || '' },
  { key: 'orderType', label: '类型', sortValue: (row) => row.orderType || '' },
  { key: 'amount', label: '金额', sortValue: (row) => Number(row.amount || 0) },
  { key: 'status', label: '状态', sortValue: (row) => row.status || '' },
  { key: 'createdAt', label: '时间', sortValue: (row) => Date.parse(row.createdAt || '') || 0 },
];

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
  countedFailed?: number;
  excluded?: number;
};

type DashboardData = {
  today?: {
    users?: number;
    orders?: number;
    paidAmount?: number;
    tasks?: number;
    failedTasks?: number;
    successfulTasks?: number;
    countedFailedTasks?: number;
    countedTasks?: number;
    excludedTasks?: number;
    balanceConsumed?: number;
  };
  yesterday?: {
    users?: number;
    orders?: number;
    paidAmount?: number;
    tasks?: number;
    runningTasks?: number;
    failedTasks?: number;
    successfulTasks?: number;
    countedFailedTasks?: number;
    countedTasks?: number;
    excludedTasks?: number;
    balanceConsumed?: number;
  };
  users?: { total?: number; active?: number; totalBalance?: number };
  revenue?: { totalPaidAmount?: number };
  taskStats?: {
    total?: number;
    queued?: number;
    pending?: number;
    processing?: number;
    success?: number;
    failed?: number;
    canceled?: number;
    counted?: number;
    countedFailed?: number;
    excluded?: number;
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
  };
  recentOrders?: RechargeRow[];
  recentTasks?: UsageRow[];
  taskTrend?: TaskTrendPoint[];
  taskTrendHourly?: TaskTrendPoint[];
};

const TASK_TREND_COLORS = {
  success: '#35C982',
  failed: '#F27979',
} as const;
const TASK_TREND_GRID = '#D5E2DA';
const TASK_TREND_BORDER = '#C1D2C6';

// Keep the admin runtime health colors aligned with the user status page:
// error rate < 5% is healthy, 5%-<10% is a warning, and >= 10% is danger.
const ERROR_RATE_WARNING_THRESHOLD = 5;
const ERROR_RATE_DANGER_THRESHOLD = 10;
type UpstreamStatusTone = 'healthy' | 'warning' | 'danger' | 'pending';

function shortDate(value: string): string {
  const [, month = '', day = ''] = value.split('-');
  return `${month}/${day}`;
}

function shortHour(value: string): string {
  return value.split(' ')[1] || value;
}

function trendRangePoint(value: string, hourly: boolean): string {
  if (!hourly) return value;
  const [date = '', hour = ''] = value.split(' ');
  return `${shortDate(date)} ${hour}`;
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

function dailySuccessRate(success: number | undefined, countedFailed: number | undefined): number | null {
  const successCount = Number(success || 0);
  const failedCount = Number(countedFailed || 0);
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
  const [openAIStatus, setOpenAIStatus] = useState<OpenAIImageStatusSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [stabilityLoading, setStabilityLoading] = useState(true);
  const [error, setError] = useState('');
  const [stabilityError, setStabilityError] = useState('');
  const [openAIStatusError, setOpenAIStatusError] = useState('');
  const [lastUpdated, setLastUpdated] = useState('');
  const [trendDays, setTrendDays] = useState<1 | 7 | 15 | 30>(7);
  const [activityView, setActivityView] = useState<'tasks' | 'orders'>('tasks');
  const [recentTasksSort, setRecentTasksSort] = useState<SortState>({ key: 'createdAt', direction: 'desc' });
  const [recentOrdersSort, setRecentOrdersSort] = useState<SortState>({ key: 'createdAt', direction: 'desc' });

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
    setOpenAIStatusError('');
    try {
      const [stabilityResponse, openAIResponse] = await Promise.allSettled([
        portalApi.stability(),
        portalApi.openAIImageStatus(),
      ]);
      if (stabilityResponse.status === 'fulfilled') {
        setStability(stabilityResponse.value.data);
      } else {
        throw stabilityResponse.reason;
      }
      if (openAIResponse.status === 'fulfilled') {
        setOpenAIStatus(openAIResponse.value.data);
      } else {
        const reason = openAIResponse.reason;
        setOpenAIStatusError(reason instanceof Error ? reason.message : 'OpenAI Image 状态加载失败');
      }
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
  const totalSuccessRate = completed ? Math.round((successful / completed) * 100) : null;
  const todaySuccessRate = dailySuccessRate(data.today?.successfulTasks, data.today?.failedTasks);
  const yesterdaySuccessRate = dailySuccessRate(data.yesterday?.successfulTasks, data.yesterday?.failedTasks);
  const revenueTrend = dayOverDayTrend(Number(data.today?.paidAmount || 0), Number(data.yesterday?.paidAmount || 0));
  const requestTrend = dayOverDayTrend(Number(data.today?.tasks || 0), Number(data.yesterday?.tasks || 0));
  const successRateTrend = dayOverDayTrend(todaySuccessRate, yesterdaySuccessRate);
  const customerTrend = dayOverDayTrend(Number(data.today?.users || 0), Number(data.yesterday?.users || 0));
  const balanceConsumptionTrend = dayOverDayTrend(Number(data.today?.balanceConsumed || 0), Number(data.yesterday?.balanceConsumed || 0));
  const runningTasks = Number(data.pending?.runningTasks || 0);
  const recentOrders = useMemo(() => data.recentOrders || [], [data.recentOrders]);
  const recentTasks = useMemo(() => data.recentTasks || [], [data.recentTasks]);
  const sortedRecentTasks = useMemo(() => {
    const header = recentTaskHeaders.find((item) => item.key === recentTasksSort.key) || recentTaskHeaders[4];
    return sortItems(recentTasks, header, recentTasksSort.direction);
  }, [recentTasks, recentTasksSort.direction, recentTasksSort.key]);
  const sortedRecentOrders = useMemo(() => {
    const header = recentOrderHeaders.find((item) => item.key === recentOrdersSort.key) || recentOrderHeaders[4];
    return sortItems(recentOrders, header, recentOrdersSort.direction);
  }, [recentOrders, recentOrdersSort.direction, recentOrdersSort.key]);
  const hourlyTrend = trendDays === 1;
  const taskTrend = useMemo(() => hourlyTrend
    ? (data.taskTrendHourly || []).slice(-24)
    : (data.taskTrend || []).slice(-trendDays), [data.taskTrend, data.taskTrendHourly, hourlyTrend, trendDays]);
  const taskTrendSummary = useMemo(() => taskTrend.reduce((summary, point) => ({
    total: summary.total + Number(point.total || 0),
    success: summary.success + Number(point.success || 0),
    failed: summary.failed + Number(point.failed || 0),
    running: summary.running + Number(point.running || 0),
    canceled: summary.canceled + Number(point.canceled || 0),
  }), { total: 0, success: 0, failed: 0, running: 0, canceled: 0 }), [taskTrend]);
  const taskTrendCompleted = taskTrendSummary.success + taskTrendSummary.failed + taskTrendSummary.canceled;
  const taskTrendSuccessRate = taskTrendCompleted > 0 ? (taskTrendSummary.success / taskTrendCompleted) * 100 : null;
  const taskTrendSeries = [
    { key: 'success', label: '成功', value: taskTrendSummary.success, color: TASK_TREND_COLORS.success },
    { key: 'failed', label: '失败', value: taskTrendSummary.failed, color: TASK_TREND_COLORS.failed },
  ] as const;

  const upstreamCode = Number(stability?.upstream_status_code || 0);
  const upstreamReachable = Boolean(stability?.reachable && upstreamCode >= 200 && upstreamCode < 300);
  const upstreamRuntime = stability?.runtime;
  const upstreamRecent = stability?.recent_60;
  const upstreamPending = stabilityLoading && !stability;
  const upstreamErrorRate = Math.max(0, Number(upstreamRuntime?.error_rate || 0));
  const upstreamStatusTone: UpstreamStatusTone = upstreamPending
    ? 'pending'
    : !upstreamReachable
      ? 'danger'
      : upstreamErrorRate >= ERROR_RATE_DANGER_THRESHOLD
        ? 'danger'
        : upstreamErrorRate >= ERROR_RATE_WARNING_THRESHOLD ? 'warning' : 'healthy';
  const upstreamLabel = upstreamStatusTone === 'danger'
    ? upstreamReachable ? '错误率较高' : '连接异常'
    : upstreamStatusTone === 'warning' ? '运行波动' : upstreamStatusTone === 'pending' ? '检测中' : '运行正常';
  const upstreamTone = upstreamStatusTone === 'healthy'
    ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
    : upstreamStatusTone === 'warning'
      ? 'border-amber-200 bg-amber-50 text-amber-700'
      : upstreamStatusTone === 'pending'
        ? 'border-zinc-200 bg-zinc-100 text-zinc-500'
        : 'border-red-200 bg-red-50 text-red-700';
  const upstreamIconTone = upstreamStatusTone === 'healthy'
    ? 'bg-emerald-50 text-emerald-700'
    : upstreamStatusTone === 'warning'
      ? 'bg-amber-50 text-amber-700'
      : upstreamStatusTone === 'pending'
        ? 'bg-zinc-100 text-zinc-500'
        : 'bg-red-50 text-red-700';
  const upstreamSuccessRateTone = upstreamStatusTone === 'danger'
    ? 'text-red-700'
    : upstreamStatusTone === 'warning'
      ? 'text-amber-700'
      : upstreamStatusTone === 'healthy' ? 'text-emerald-700' : 'text-zinc-700';
  const openAISeverity = openAIStatus?.severity || 'ok';
  const openAITextTone = openAIStatusError
    ? 'text-red-700'
    : openAISeverity === 'critical'
      ? 'text-red-700'
      : openAISeverity === 'warning'
        ? 'text-amber-700'
        : 'text-emerald-700';
  const openAIStatusDot = openAIStatusError || openAISeverity === 'critical' ? 'bg-red-500' : openAISeverity === 'warning' ? 'bg-amber-500' : 'bg-emerald-500';

  const summaryMetrics = [
    { key: 'revenue', label: '今日实收', value: formatCNY(Number(data.today?.paidAmount || 0)), note: `累计 ${formatCNY(Number(data.revenue?.totalPaidAmount || 0))}`, trend: revenueTrend, icon: CircleDollarSign, tone: 'green' as const },
    { key: 'balance', label: '余额消耗', value: formatCNY(Number(data.today?.balanceConsumed || 0), 4), note: `总余额 ${formatCNY(Number(data.users?.totalBalance || 0), 4)}`, trend: balanceConsumptionTrend, icon: Wallet, tone: 'amber' as const },
    { key: 'requests', label: 'API 请求', value: Number(data.today?.tasks || 0).toLocaleString('zh-CN'), note: `累计 ${Number(stats.total || 0).toLocaleString('zh-CN')} 次`, trend: requestTrend, icon: Activity, tone: 'blue' as const },
    { key: 'success-rate', label: '请求成功率', value: todaySuccessRate === null ? '--' : `${Math.round(todaySuccessRate)}%`, note: `累计 ${totalSuccessRate === null ? '--' : `${totalSuccessRate}%`} · ${Number(stats.totalImages || 0).toLocaleString('zh-CN')} 张`, trend: successRateTrend, icon: Cable, tone: todaySuccessRate !== null && todaySuccessRate < 95 ? 'amber' as const : 'green' as const },
    { key: 'customers', label: '新增客户', value: Number(data.today?.users || 0).toLocaleString('zh-CN'), note: `累计 ${Number(data.users?.total || 0).toLocaleString('zh-CN')} · 启用 ${Number(data.users?.active || 0).toLocaleString('zh-CN')}`, trend: customerTrend, icon: Users, tone: 'neutral' as const },
  ];
  const alertItems = useMemo(() => [
    { key: 'pending-orders', label: '待支付充值单', value: Number(data.pending?.pendingOrders || 0), dotTone: 'bg-amber-500', valueTone: 'text-amber-800' },
    { key: 'recent-failures', label: '24 小时失败', value: Number(data.pending?.recentFailedTasks || 0), dotTone: 'bg-red-500', valueTone: 'text-red-700' },
  ].filter((item) => item.value > 0), [data.pending]);

  return (
    <div className="space-y-5">
      <PageHeader title="经营概览" description="API 中转业务、订阅收入和上游运行状态。">
        <div className="flex items-center gap-3">
          <span className="hidden text-[10px] text-zinc-400 sm:inline">更新于 {lastUpdated ? formatDate(lastUpdated) : '-'}</span>
          <button
            type="button"
            onClick={() => void refreshAll()}
            disabled={loading || stabilityLoading}
            className="inline-flex h-8 items-center gap-2 rounded-md border border-[#DCE4DF] bg-white px-3 text-xs font-semibold text-[#17201B] hover:border-[#12B76A] disabled:opacity-50"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading || stabilityLoading ? 'animate-spin' : ''}`} />
            刷新
          </button>
        </div>
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
          <section aria-label="今日经营摘要" className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
            {summaryMetrics.map((metric) => <AdminMetricCard key={metric.key} title={metric.label} value={metric.value} note={metric.note} trend={metric.trend} icon={metric.icon} tone={metric.tone} />)}
          </section>

          {alertItems.length > 0 && (
            <section aria-label="待关注事项" className="flex flex-wrap items-center gap-x-5 gap-y-2 border-y border-[#E8D8D2] bg-[#FFFDFC] px-4 py-2.5">
              <span className="inline-flex items-center gap-1.5 text-[11px] font-semibold text-zinc-600"><TriangleAlert className="h-3.5 w-3.5 text-amber-600" />待关注</span>
              {alertItems.map((item) => (
                <span key={item.key} className="inline-flex items-center gap-2 text-[11px] text-zinc-500"><i className={`h-1.5 w-1.5 rounded-full ${item.dotTone}`} />{item.label}<strong className={`font-mono text-xs ${item.valueTone}`}>{item.value.toLocaleString('zh-CN')}</strong></span>
              ))}
            </section>
          )}

          <div className="grid min-w-0 grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1.55fr)_minmax(320px,.7fr)]">
          <section className="min-w-0 overflow-hidden rounded-md border border-[#DCE4DF] bg-white" aria-labelledby="task-trend-title">
            <header className="flex min-h-[54px] flex-col gap-3 border-b border-[#EDF0EE] px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-2.5">
                <span className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-blue-50 text-[#1E5F91]"><ChartNoAxesCombined className="h-4 w-4" /></span>
                <div>
                  <div className="flex flex-wrap items-center gap-2"><h2 id="task-trend-title" className="text-sm font-semibold text-[#17201B]">任务处理趋势</h2>{runningTasks > 0 && <span className="rounded border border-amber-200 bg-amber-50 px-1.5 py-0.5 text-[9px] font-semibold text-amber-700">{runningTasks} 个处理中</span>}</div>
                  <p className="mt-0.5 text-[11px] text-zinc-500">{taskTrend[0]?.date ? trendRangePoint(taskTrend[0].date, hourlyTrend) : '-'} 至 {taskTrend.at(-1)?.date ? trendRangePoint(taskTrend.at(-1)!.date, hourlyTrend) : '-'}</p>
                </div>
              </div>
              <div className="inline-flex self-start rounded-md border border-[#DCE4DF] bg-[#F7F8F6] p-0.5 sm:self-auto" role="group" aria-label="任务趋势时间范围">
                {[1, 7, 15, 30].map((days) => (
                  <button key={days} type="button" onClick={() => setTrendDays(days as 1 | 7 | 15 | 30)} aria-pressed={trendDays === days} className={`h-7 min-w-12 rounded px-2 text-[11px] font-semibold ${trendDays === days ? 'bg-white text-[#047857] shadow-sm' : 'text-zinc-500 hover:text-zinc-800'}`}>{days}天</button>
                ))}
              </div>
            </header>
            <div className="flex min-h-[38px] flex-wrap items-center gap-x-4 gap-y-1.5 border-b border-[#EDF0EE] px-4 py-2" aria-label="任务趋势图例">
              {taskTrendSeries.map((item) => (
                <span key={item.key} className="inline-flex items-center gap-1.5 text-[10px] text-zinc-500"><i className={`inline-block h-px w-5 ${item.key === 'failed' ? 'border-t border-dashed' : ''}`} style={item.key === 'failed' ? { borderColor: item.color } : { backgroundColor: item.color }} aria-hidden="true" />{item.label}</span>
              ))}
            </div>
            <p className="sr-only" id="task-trend-description">当前范围共 {taskTrendSummary.total} 个任务，成功 {taskTrendSummary.success} 个，失败 {taskTrendSummary.failed} 个，处理中 {taskTrendSummary.running} 个，已取消 {taskTrendSummary.canceled} 个。成功率按成功、失败与已取消的终态任务计算，为 {taskTrendSuccessRate === null ? '暂无数据' : `${Math.round(taskTrendSuccessRate)}%`}。</p>
            <div className="h-[220px] w-full px-1 pb-3 pt-3 sm:h-[260px] sm:px-3" aria-describedby="task-trend-description">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart accessibilityLayer data={taskTrend} margin={ADMIN_CHART_MARGIN}>
                  <CartesianGrid stroke={TASK_TREND_GRID} vertical />
                  <XAxis dataKey="date" tickFormatter={hourlyTrend ? shortHour : shortDate} tick={{ fill: ADMIN_CHART_AXIS, fontSize: 9 }} tickLine={false} axisLine={{ stroke: TASK_TREND_BORDER }} minTickGap={hourlyTrend ? 18 : 22} />
                  <YAxis allowDecimals={false} tick={{ fill: ADMIN_CHART_AXIS, fontSize: 9 }} tickLine={false} axisLine={false} width={44} />
                  <Tooltip formatter={(value, name) => [Number(value || 0).toLocaleString('zh-CN'), String(name)]} labelFormatter={(label) => `${hourlyTrend ? '时间' : '日期'} ${String(label)}`} contentStyle={{ border: `1px solid ${TASK_TREND_BORDER}`, borderRadius: 6, boxShadow: '0 8px 24px rgba(23,32,27,.08)', fontSize: 11 }} />
                  <Line type="monotone" dataKey="success" name="成功" stroke={TASK_TREND_COLORS.success} strokeWidth={2} dot={taskTrend.length <= 15 ? { ...ADMIN_CHART_DOT, fill: TASK_TREND_COLORS.success, strokeWidth: 0 } : false} activeDot={ADMIN_CHART_ACTIVE_DOT} isAnimationActive={false} />
                  <Line type="monotone" dataKey="failed" name="失败" stroke={TASK_TREND_COLORS.failed} strokeWidth={1.8} strokeDasharray="5 4" dot={taskTrend.length <= 15 ? { ...ADMIN_CHART_DOT, fill: TASK_TREND_COLORS.failed, strokeWidth: 0 } : false} activeDot={ADMIN_CHART_ACTIVE_DOT} isAnimationActive={false} />
                </LineChart>
              </ResponsiveContainer>
            </div>
          </section>

          <section className="flex min-w-0 flex-col overflow-hidden rounded-md border border-[#DCE4DF] bg-white" aria-labelledby="upstream-status-title">
            <div className={`h-1 w-full ${upstreamStatusTone === 'healthy' ? 'bg-[#3F9274]' : upstreamStatusTone === 'warning' ? 'bg-amber-500' : upstreamStatusTone === 'pending' ? 'bg-zinc-300' : 'bg-[#D06F69]'}`} />
            <header className="flex items-start justify-between gap-3 border-b border-[#EDF0EE] px-4 py-3">
              <div className="flex min-w-0 items-center gap-3">
                <span className={`grid h-8 w-8 shrink-0 place-items-center rounded-md ${upstreamIconTone}`}>{upstreamReachable ? <HeartPulse className="h-4 w-4" /> : <Server className="h-4 w-4" />}</span>
                <div className="min-w-0">
                  <h2 id="upstream-status-title" className="text-sm font-semibold text-[#17201B]">运行健康</h2>
                  <p className="mt-0.5 truncate text-[10px] text-zinc-500">{stabilityLoading && !stability ? '正在检测上游' : stabilityError || stability?.error || (upstreamCode ? `上游 HTTP ${upstreamCode}` : '暂无状态码')}</p>
                </div>
              </div>
              {stabilityLoading && !stability ? <Loader2 className="mt-1 h-4 w-4 animate-spin text-zinc-400" aria-label="检测中" /> : <span className={`shrink-0 rounded border px-2 py-0.5 text-[10px] font-semibold ${upstreamTone}`}>{upstreamLabel}</span>}
            </header>
            <div className="grid flex-1 grid-cols-2 gap-px bg-[#EDF0EE]">
              {[
                ['近一小时成功率', stabilityLoading && !stability ? '--' : percentage(upstreamRuntime?.success_rate), `${Number(upstreamRuntime?.total || 0).toLocaleString('zh-CN')} 次 · ${Number(upstreamRuntime?.totals?.failed || 0).toLocaleString('zh-CN')} 失败`],
                ['平均耗时', stabilityLoading && !stability ? '--' : durationLabel(upstreamRecent?.average_duration_secs), `成功 ${durationLabel(upstreamRecent?.average_success_duration_secs)}`],
                ['启用上游', Number(data.system?.activeProviders || 0).toLocaleString('zh-CN'), `${Number(data.system?.disabledProviders || 0).toLocaleString('zh-CN')} 个停用`],
                ['启用模型', Number(data.system?.activeModels || 0).toLocaleString('zh-CN'), `${Number(data.system?.disabledModels || 0).toLocaleString('zh-CN')} 个停用`],
              ].map(([label, value, note]) => (
                <div key={label} className="min-w-0 bg-white px-4 py-3">
                  <span className="block text-[10px] font-semibold text-zinc-500">{label}</span>
                  <strong className={`mt-1 block truncate text-base ${label === '近一小时成功率' ? upstreamSuccessRateTone : 'text-[#17201B]'}`}>{value}</strong>
                  <small className="mt-0.5 block truncate text-[10px] text-zinc-400">{note}</small>
                </div>
              ))}
            </div>
            <div className="border-t border-[#EDF0EE] bg-[#FAFBFA] px-4 py-3">
              <div className="min-w-0">
                <div className="flex items-center justify-between gap-3"><span className={`inline-flex min-w-0 items-center gap-1.5 text-[10px] font-semibold ${openAITextTone}`}><i className={`h-1.5 w-1.5 shrink-0 rounded-full ${openAIStatusDot}`} />OpenAI Image：<span className="truncate">{openAIStatusError || openAIStatus?.statusLabel || '检测中'}</span></span><span className="shrink-0 text-[9px] text-zinc-400">{openAIStatus?.fetchedAt ? formatDate(openAIStatus.fetchedAt) : '-'}</span></div>
                {(openAISeverity !== 'ok' || openAIStatusError) && <p className="mt-1 line-clamp-2 text-[10px] text-zinc-500">{openAIStatus?.latestImageIncident?.title || openAIStatus?.summary || openAIStatusError}</p>}
              </div>
            </div>
            {stabilityError && (
              <div className="flex items-center gap-2 border-t border-red-100 bg-red-50 px-4 py-2.5 text-[11px] text-red-700" role="alert">
                <TriangleAlert className="h-3.5 w-3.5 shrink-0" />
                状态检测失败：{stabilityError}
              </div>
            )}
          </section>
          </div>

          <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white" aria-labelledby="recent-activity-title">
            <header className="flex flex-wrap items-center justify-between gap-3 border-b border-[#DCE4DF] px-4 py-3">
              <div><h2 id="recent-activity-title" className="text-sm font-semibold text-[#17201B]">最近活动</h2><p className="mt-0.5 text-[10px] text-zinc-400">仅显示最新记录</p></div>
              <div className="inline-flex rounded-md border border-[#DCE4DF] bg-[#F7F8F6] p-0.5" role="tablist" aria-label="最近活动类型">
                <button type="button" role="tab" aria-selected={activityView === 'tasks'} onClick={() => setActivityView('tasks')} className={`h-7 rounded px-3 text-[10px] font-semibold ${activityView === 'tasks' ? 'bg-white text-[#047857] shadow-sm' : 'text-zinc-500 hover:text-zinc-800'}`}>API 请求</button>
                <button type="button" role="tab" aria-selected={activityView === 'orders'} onClick={() => setActivityView('orders')} className={`h-7 rounded px-3 text-[10px] font-semibold ${activityView === 'orders' ? 'bg-white text-[#047857] shadow-sm' : 'text-zinc-500 hover:text-zinc-800'}`}>充值订阅</button>
              </div>
            </header>

            {activityView === 'tasks' ? (
              <>
                <div className="divide-y divide-[#EDF0EE] sm:hidden">
                  {sortedRecentTasks.map((row) => { const status = statusView(row.status); return (
                    <div key={row.id} className="grid grid-cols-[minmax(0,1fr)_auto] gap-3 px-4 py-3">
                      <span className="min-w-0"><strong className="block truncate text-[11px]">{row.userEmail || row.userId || '-'}</strong><small className="mt-1 block truncate text-[10px] text-zinc-400">{row.modelDisplayName || row.modelName || row.modelId || '-'} · {Number(row.quantity || 0)} 张</small></span>
                      <span className="text-right"><span className={`rounded border px-1.5 py-0.5 text-[9px] ${status.className}`}>{status.label}</span><small className="mt-1.5 block text-[9px] text-zinc-400">{formatDate(row.createdAt || '')}</small></span>
                    </div>
                  ); })}
                  {!recentTasks.length && <p className="px-4 py-8 text-center text-[11px] text-zinc-400">暂无 API 请求</p>}
                </div>
                <div className="hidden overflow-x-auto sm:block">
                  <table className="w-full min-w-[640px] text-left text-xs">
                    <thead className="bg-[#F6F8F6] text-[10px] text-zinc-500"><tr>{recentTaskHeaders.map((header) => <SortableHeader key={header.key} header={header} sortState={recentTasksSort} onSort={(key) => { const direction = recentTasksSort.key === key && recentTasksSort.direction === 'asc' ? 'desc' : 'asc'; setRecentTasksSort({ key, direction }); }} />)}</tr></thead>
                    <tbody className="divide-y divide-[#EDF0EE]">
                      {sortedRecentTasks.map((row) => { const status = statusView(row.status); return (
                        <tr key={row.id}><td className="max-w-[220px] truncate px-4 py-2.5">{row.userEmail || row.userId || '-'}</td><td className="max-w-[220px] truncate px-4 py-2.5">{row.modelDisplayName || row.modelName || row.modelId || '-'}</td><td className="px-4 py-2.5 font-mono">{Number(row.quantity || 0)}</td><td className="px-4 py-2.5"><span className={`rounded border px-1.5 py-0.5 text-[10px] ${status.className}`}>{status.label}</span></td><td className="whitespace-nowrap px-4 py-2.5 text-zinc-500">{formatDate(row.createdAt || '')}</td></tr>
                      ); })}
                      {!recentTasks.length && <tr><td colSpan={5} className="px-4 py-8 text-center text-zinc-400">暂无 API 请求</td></tr>}
                    </tbody>
                  </table>
                </div>
              </>
            ) : (
              <>
                <div className="divide-y divide-[#EDF0EE] sm:hidden">
                  {sortedRecentOrders.map((row) => { const status = statusView(row.status); return (
                    <div key={row.id} className="grid grid-cols-[minmax(0,1fr)_auto] gap-3 px-4 py-3">
                      <span className="min-w-0"><strong className="block truncate text-[11px]">{row.userEmail || row.userId || '-'}</strong><small className="mt-1 block text-[10px] text-zinc-400">{row.orderType === 'subscription' ? '订阅' : '余额'} · {formatCNY(Number(row.amount || 0))}</small></span>
                      <span className="text-right"><span className={`rounded border px-1.5 py-0.5 text-[9px] ${status.className}`}>{status.label}</span><small className="mt-1.5 block text-[9px] text-zinc-400">{formatDate(row.createdAt || '')}</small></span>
                    </div>
                  ); })}
                  {!recentOrders.length && <p className="px-4 py-8 text-center text-[11px] text-zinc-400">暂无充值记录</p>}
                </div>
                <div className="hidden overflow-x-auto sm:block">
                  <table className="w-full min-w-[640px] text-left text-xs">
                    <thead className="bg-[#F6F8F6] text-[10px] text-zinc-500"><tr>{recentOrderHeaders.map((header) => <SortableHeader key={header.key} header={header} sortState={recentOrdersSort} onSort={(key) => { const direction = recentOrdersSort.key === key && recentOrdersSort.direction === 'asc' ? 'desc' : 'asc'; setRecentOrdersSort({ key, direction }); }} />)}</tr></thead>
                    <tbody className="divide-y divide-[#EDF0EE]">
                      {sortedRecentOrders.map((row) => { const status = statusView(row.status); return (
                        <tr key={row.id}><td className="max-w-[220px] truncate px-4 py-2.5">{row.userEmail || row.userId || '-'}</td><td className="px-4 py-2.5">{row.orderType === 'subscription' ? '订阅' : '余额'}</td><td className="px-4 py-2.5 font-mono">{formatCNY(Number(row.amount || 0))}</td><td className="px-4 py-2.5"><span className={`rounded border px-1.5 py-0.5 text-[10px] ${status.className}`}>{status.label}</span></td><td className="whitespace-nowrap px-4 py-2.5 text-zinc-500">{formatDate(row.createdAt || '')}</td></tr>
                      ); })}
                      {!recentOrders.length && <tr><td colSpan={5} className="px-4 py-8 text-center text-zinc-400">暂无充值记录</td></tr>}
                    </tbody>
                  </table>
                </div>
              </>
            )}
          </section>
        </>
      )}
    </div>
  );
}
