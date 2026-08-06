'use client';

import Link from 'next/link';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  ChartNoAxesCombined,
  ChevronDown,
  ChevronRight,
  Loader2,
  Radio,
  RefreshCw,
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
import { AppSelect } from '@/components/common/AppSelect';
import { PageHeader } from '@/components/common/PageHeader';
import {
  portalApi,
  type AdminOperationsActiveCall,
  type AdminOperationsLiveSnapshot,
  type AdminOperationsMetric,
  type AdminOperationsRange,
  type AdminOperationsRankingSnapshot,
  type AdminOperationsTopUser,
  type AdminOperationsTrendSnapshot,
  type StabilitySnapshot,
} from '@/lib/admin-api';
import { formatCNY, formatDate } from '@/lib/common/utils';

const METRIC_OPTIONS = [
  { value: 'credits', label: '按余额消费' },
  { value: 'requests', label: '按调用次数' },
  { value: 'images', label: '按输出图片' },
  { value: 'failures', label: '按失败次数' },
  { value: 'duration', label: '按平均耗时' },
] as const;

const EMPTY_LIVE: AdminOperationsLiveSnapshot = {
  activeUsers: 0,
  activeRequests: 0,
  queuedRequests: 0,
  processingRequests: 0,
  slowRequests: 0,
  averageElapsedSeconds: 0,
  activeCalls: [],
  generatedAt: '',
};

const EMPTY_RANKING: AdminOperationsRankingSnapshot = {
  range: 'today',
  metric: 'credits',
  topUsers: [],
  generatedAt: '',
};

const EMPTY_TREND: AdminOperationsTrendSnapshot = {
  minutes: 60,
  points: [],
  generatedAt: '',
};

type ActiveCallUserGroup = {
  groupKey: string;
  userId: string;
  userEmail?: string;
  billingMode: string;
  keyLabel: string;
  modelLabel: string;
  representativeStatus: string;
  taskCount: number;
  queuedCount: number;
  processingCount: number;
  slowCount: number;
  imageCount: number;
  maxElapsedSeconds: number;
  concurrencyUsed: number;
  concurrencyLimit: number;
  firstCreatedAt: string;
  calls: AdminOperationsActiveCall[];
};

function durationLabel(value: number): string {
  const seconds = Math.max(0, Number(value || 0));
  if (seconds < 1) return `${Math.round(seconds * 1000)}ms`;
  if (seconds < 60) return `${seconds.toFixed(1)}秒`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes}分${Math.round(seconds % 60)}秒`;
}

function activeCallStatus(status = '') {
  if (status === 'processing') return { label: '处理中', className: 'border-blue-200 bg-blue-50 text-blue-700' };
  return { label: '排队中', className: 'border-amber-200 bg-amber-50 text-amber-700' };
}

function elapsedTimeMeta(value: number) {
  const seconds = Math.max(0, Number(value || 0));
  if (seconds <= 65) return { label: `${seconds.toFixed(1)}s`, className: 'border-emerald-200 bg-emerald-50 text-emerald-700' };
  if (seconds < 120) return { label: `${seconds.toFixed(1)}s`, className: 'border-amber-200 bg-amber-50 text-amber-800' };
  return { label: `${seconds.toFixed(1)}s`, className: 'border-red-200 bg-red-50 text-red-700' };
}

function billingModeLabel(mode = '') {
  if (mode === 'subscription') return '订阅额度';
  if (mode === 'balance') return '账户余额';
  if (mode === 'mixed') return '混合计费';
  return '自动兼容';
}

function sizeTierLabel(value = '') {
  const normalized = value.trim();
  return normalized ? normalized.toUpperCase() : '默认规格';
}

function summarizeLabels(labels: string[], emptyLabel: string, maxVisible = 2) {
  const unique = Array.from(new Set(labels.map((item) => item.trim()).filter(Boolean)));
  if (!unique.length) return emptyLabel;
  if (unique.length <= maxVisible) return unique.join('、');
  return `${unique.slice(0, maxVisible).join('、')} 等 ${unique.length} 项`;
}

function shortClock(value = '') {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}

function rankingMetricMeta(user: AdminOperationsTopUser, metric: AdminOperationsMetric) {
  if (metric === 'images') return { label: '输出图片', value: user.imageCount.toLocaleString('zh-CN') };
  if (metric === 'credits') return { label: '余额消费', value: formatCNY(user.creditsSpent) };
  if (metric === 'failures') return { label: '失败次数', value: user.failedCount.toLocaleString('zh-CN') };
  if (metric === 'duration') return { label: '平均耗时', value: durationLabel(user.averageDurationSeconds) };
  return { label: '调用次数', value: user.requestCount.toLocaleString('zh-CN') };
}

function operationPanelId(value: string) {
  return `active-operation-${value.replace(/[^a-zA-Z0-9_-]/g, '-')}`;
}

export default function AdminAPIOperationsPage() {
  const [live, setLive] = useState<AdminOperationsLiveSnapshot>(EMPTY_LIVE);
  const [ranking, setRanking] = useState<AdminOperationsRankingSnapshot>(EMPTY_RANKING);
  const [trend, setTrend] = useState<AdminOperationsTrendSnapshot>(EMPTY_TREND);
  const [stability, setStability] = useState<StabilitySnapshot | null>(null);
  const [range, setRange] = useState<AdminOperationsRange>('today');
  const [metric, setMetric] = useState<AdminOperationsMetric>('credits');
  const [expandedUser, setExpandedUser] = useState('');
  const [liveLoading, setLiveLoading] = useState(true);
  const [rankingLoading, setRankingLoading] = useState(true);
  const [trendLoading, setTrendLoading] = useState(true);
  const [stabilityLoading, setStabilityLoading] = useState(true);
  const [liveError, setLiveError] = useState('');
  const [rankingError, setRankingError] = useState('');
  const [trendError, setTrendError] = useState('');
  const [stabilityError, setStabilityError] = useState('');
  const liveRequestRef = useRef(false);
  const rankingRequestRef = useRef(0);

  const loadLive = useCallback(async (quiet = false) => {
    if (liveRequestRef.current) return;
    liveRequestRef.current = true;
    if (!quiet) setLiveLoading(true);
    try {
      const response = await portalApi.adminOperationsLive();
      setLive(response.data);
      setLiveError('');
    } catch (requestError) {
      setLiveError(requestError instanceof Error ? requestError.message : '实时任务加载失败');
    } finally {
      liveRequestRef.current = false;
      if (!quiet) setLiveLoading(false);
    }
  }, []);

  const loadRanking = useCallback(async () => {
    const requestId = rankingRequestRef.current + 1;
    rankingRequestRef.current = requestId;
    setRankingLoading(true);
    try {
      const response = await portalApi.adminOperationsRanking(range, metric);
      if (rankingRequestRef.current !== requestId) return;
      setRanking(response.data);
      setRankingError('');
    } catch (requestError) {
      if (rankingRequestRef.current !== requestId) return;
      setRankingError(requestError instanceof Error ? requestError.message : '用户排行加载失败');
    } finally {
      if (rankingRequestRef.current === requestId) setRankingLoading(false);
    }
  }, [metric, range]);

  const loadTrend = useCallback(async () => {
    setTrendLoading(true);
    try {
      const response = await portalApi.adminOperationsTrend();
      setTrend(response.data);
      setTrendError('');
    } catch (requestError) {
      setTrendError(requestError instanceof Error ? requestError.message : '调用趋势加载失败');
    } finally {
      setTrendLoading(false);
    }
  }, []);

  const loadStability = useCallback(async (quiet = false) => {
    if (!quiet) setStabilityLoading(true);
    try {
      const response = await portalApi.stability();
      setStability(response.data);
      setStabilityError('');
    } catch (requestError) {
      setStabilityError(requestError instanceof Error ? requestError.message : '上游状态加载失败');
    } finally {
      if (!quiet) setStabilityLoading(false);
    }
  }, []);

  const refreshAll = useCallback(async () => {
    await Promise.allSettled([loadLive(), loadRanking(), loadTrend(), loadStability()]);
  }, [loadLive, loadRanking, loadStability, loadTrend]);

  useEffect(() => {
    const firstLoad = window.setTimeout(() => void loadLive(), 0);
    const timer = window.setInterval(() => {
      if (!document.hidden) void loadLive(true);
    }, 5_000);
    return () => {
      window.clearTimeout(firstLoad);
      window.clearInterval(timer);
    };
  }, [loadLive]);

  useEffect(() => {
    const firstLoad = window.setTimeout(() => void loadRanking(), 0);
    const timer = window.setInterval(() => {
      if (!document.hidden) void loadRanking();
    }, 60_000);
    return () => {
      window.clearTimeout(firstLoad);
      window.clearInterval(timer);
    };
  }, [loadRanking]);

  useEffect(() => {
    const firstLoad = window.setTimeout(() => {
      void loadTrend();
      void loadStability();
    }, 0);
    const trendTimer = window.setInterval(() => {
      if (!document.hidden) void loadTrend();
    }, 60_000);
    const stabilityTimer = window.setInterval(() => {
      if (!document.hidden) void loadStability(true);
    }, 30_000);
    return () => {
      window.clearTimeout(firstLoad);
      window.clearInterval(trendTimer);
      window.clearInterval(stabilityTimer);
    };
  }, [loadStability, loadTrend]);

  const activeCallUserGroups = useMemo<ActiveCallUserGroup[]>(() => {
    const groups = new Map<string, {
      groupKey: string;
      userId: string;
      userEmail?: string;
      billingModes: string[];
      keyLabels: string[];
      modelLabels: string[];
      representativeStatus: string;
      taskCount: number;
      queuedCount: number;
      processingCount: number;
      slowCount: number;
      imageCount: number;
      maxElapsedSeconds: number;
      firstCreatedAt: string;
      keyConcurrency: Map<string, { used: number; limit: number }>;
      calls: AdminOperationsActiveCall[];
    }>();

    live.activeCalls.forEach((call) => {
      const groupKey = call.userId || call.userEmail || call.apiKeyId || call.taskId;
      const keyLabel = call.keyName || call.keyPrefix || 'API Key';
      const modelLabel = `${call.model || '未知模型'} · ${sizeTierLabel(call.sizeTier)}`;
      const existing = groups.get(groupKey);
      const group = existing || {
        groupKey,
        userId: call.userId,
        userEmail: call.userEmail,
        billingModes: [],
        keyLabels: [],
        modelLabels: [],
        representativeStatus: call.status,
        taskCount: 0,
        queuedCount: 0,
        processingCount: 0,
        slowCount: 0,
        imageCount: 0,
        maxElapsedSeconds: 0,
        firstCreatedAt: call.createdAt,
        keyConcurrency: new Map<string, { used: number; limit: number }>(),
        calls: [],
      };

      group.taskCount += 1;
      group.imageCount += Math.max(0, Number(call.quantity || 0));
      group.maxElapsedSeconds = Math.max(group.maxElapsedSeconds, call.elapsedSeconds);
      group.calls.push(call);
      if (!group.userEmail && call.userEmail) group.userEmail = call.userEmail;
      if (call.billingMode) group.billingModes.push(call.billingMode);
      group.keyLabels.push(keyLabel);
      group.modelLabels.push(modelLabel);
      if (call.status === 'processing') {
        group.processingCount += 1;
        group.representativeStatus = 'processing';
      } else {
        group.queuedCount += 1;
      }
      if (call.elapsedSeconds >= 120) group.slowCount += 1;
      if (call.createdAt && (!group.firstCreatedAt || call.createdAt < group.firstCreatedAt)) group.firstCreatedAt = call.createdAt;

      const apiKeyId = call.apiKeyId || `${groupKey}:${keyLabel}`;
      const stored = group.keyConcurrency.get(apiKeyId);
      group.keyConcurrency.set(apiKeyId, {
        used: Math.max(stored?.used || 0, Math.max(1, call.activeForKey || 0)),
        limit: Math.max(stored?.limit || 0, Math.max(1, call.concurrencyLimit || 0)),
      });
      groups.set(groupKey, group);
    });

    return Array.from(groups.values()).map((group) => {
      const concurrency = Array.from(group.keyConcurrency.values()).reduce((acc, item) => ({
        used: acc.used + item.used,
        limit: acc.limit + item.limit,
      }), { used: 0, limit: 0 });
      const billingModes = Array.from(new Set(group.billingModes));
      return {
        groupKey: group.groupKey,
        userId: group.userId,
        userEmail: group.userEmail,
        billingMode: billingModes.length > 1 ? 'mixed' : billingModes[0] || 'auto',
        keyLabel: summarizeLabels(group.keyLabels, 'API Key'),
        modelLabel: summarizeLabels(group.modelLabels, '未知模型', 2),
        representativeStatus: group.representativeStatus,
        taskCount: group.taskCount,
        queuedCount: group.queuedCount,
        processingCount: group.processingCount,
        slowCount: group.slowCount,
        imageCount: group.imageCount,
        maxElapsedSeconds: group.maxElapsedSeconds,
        concurrencyUsed: concurrency.used || group.taskCount,
        concurrencyLimit: concurrency.limit || 1,
        firstCreatedAt: group.firstCreatedAt,
        calls: group.calls,
      };
    }).sort((left, right) => {
      const slowPriority = Number(right.slowCount > 0) - Number(left.slowCount > 0);
      if (slowPriority) return slowPriority;
      const saturatedPriority = Number(right.concurrencyUsed >= right.concurrencyLimit) - Number(left.concurrencyUsed >= left.concurrencyLimit);
      if (saturatedPriority) return saturatedPriority;
      return left.firstCreatedAt.localeCompare(right.firstCreatedAt);
    });
  }, [live.activeCalls]);

  const upstreamCode = Number(stability?.upstream_status_code || 0);
  const upstreamHealthy = Boolean(stability?.reachable && upstreamCode >= 200 && upstreamCode < 300);
  const runState = stability && !upstreamHealthy
    ? { label: '上游异常', className: 'border-red-200 bg-red-50 text-red-700' }
    : live.slowRequests > 0
      ? { label: '存在慢任务', className: 'border-red-200 bg-red-50 text-red-700' }
      : live.queuedRequests > 0
        ? { label: '任务排队', className: 'border-amber-200 bg-amber-50 text-amber-800' }
        : live.activeRequests > 0
          ? { label: '运行中', className: 'border-emerald-200 bg-emerald-50 text-emerald-700' }
          : { label: '当前空闲', className: 'border-zinc-200 bg-zinc-50 text-zinc-600' };

  const issues = [
    liveError && `实时任务：${liveError}`,
    trendError && `调用趋势：${trendError}`,
    rankingError && `用户排行：${rankingError}`,
    stabilityError && `上游状态：${stabilityError}`,
  ].filter(Boolean) as string[];
  const trendHasData = trend.points.some((point) => point.total > 0 || point.success > 0 || point.failed > 0);

  const summaryItems = [
    { label: '调用用户', value: live.activeUsers.toLocaleString('zh-CN'), note: '当前有任务', tone: 'bg-blue-500' },
    { label: '处理中', value: live.processingRequests.toLocaleString('zh-CN'), note: `${live.activeRequests} 个活动任务`, tone: 'bg-emerald-500' },
    { label: '排队任务', value: live.queuedRequests.toLocaleString('zh-CN'), note: '等待并发执行', tone: live.queuedRequests ? 'bg-amber-500' : 'bg-zinc-300' },
    { label: '平均已用时间', value: durationLabel(live.averageElapsedSeconds), note: '当前任务均值', tone: 'bg-zinc-400' },
    { label: '慢任务', value: live.slowRequests.toLocaleString('zh-CN'), note: '超过 120 秒', tone: live.slowRequests ? 'bg-red-500' : 'bg-zinc-300' },
    { label: '上游状态', value: stabilityLoading && !stability ? '同步中' : upstreamHealthy ? '正常' : stability ? '异常' : '待同步', note: stability ? `${upstreamCode || '-'} · ${Number(stability.stability_percent || 0).toFixed(1)}%` : '等待状态数据', tone: upstreamHealthy ? 'bg-emerald-500' : stability ? 'bg-red-500' : 'bg-zinc-300' },
  ];

  const refreshing = liveLoading || rankingLoading || trendLoading || stabilityLoading;

  return (
    <div className="space-y-5">
      <PageHeader title="API 实时运营" description="实时任务、队列压力、调用趋势与客户排行。">
        <span className={`inline-flex h-8 items-center gap-1.5 rounded-md border px-2.5 text-[11px] font-semibold ${runState.className}`}><Radio className="h-3 w-3" />{runState.label}</span>
        <span className="hidden text-[10px] text-zinc-400 sm:inline">5秒自动刷新 · {live.generatedAt ? shortClock(live.generatedAt) : '-'}</span>
        <button type="button" onClick={() => void refreshAll()} disabled={refreshing} title="刷新全部运营数据" aria-label="刷新全部运营数据" className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white text-zinc-600 hover:border-[#86EFAC] hover:text-[#047857] disabled:opacity-50"><RefreshCw className={`h-3.5 w-3.5 ${refreshing ? 'animate-spin' : ''}`} /></button>
      </PageHeader>

      {issues.length > 0 && (
        <div className="flex items-start gap-2 rounded-md border border-red-200 bg-red-50 px-3 py-2.5 text-[11px] text-red-700" role="alert">
          <TriangleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <div className="min-w-0 space-y-0.5">{issues.map((issue) => <p key={issue} className="break-words">{issue}</p>)}</div>
        </div>
      )}

      <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white" aria-labelledby="operation-summary-title">
        <header className="flex items-center justify-between gap-3 border-b border-[#EDF0EE] px-4 py-2.5">
          <div><h2 id="operation-summary-title" className="text-xs font-semibold text-[#17201B]">当前运行概览</h2><p className="mt-0.5 text-[10px] text-zinc-400">实时任务与上游可用状态</p></div>
          {liveLoading && !live.generatedAt && <Loader2 className="h-4 w-4 animate-spin text-[#12B76A]" aria-label="实时数据加载中" />}
        </header>
        <div className="grid grid-cols-3 gap-px bg-[#EDF0EE] lg:grid-cols-6">
          {summaryItems.map((item) => (
            <div key={item.label} className="min-w-0 bg-white px-3 py-3 sm:px-4">
              <span className="flex items-center gap-1.5 text-[10px] font-semibold text-zinc-500"><i className={`h-1.5 w-1.5 shrink-0 rounded-full ${item.tone}`} />{item.label}</span>
              <strong className="mt-1 block truncate font-mono text-base text-[#17201B]">{item.value}</strong>
              <small className="mt-0.5 block truncate text-[9px] text-zinc-400">{item.note}</small>
            </div>
          ))}
        </div>
      </section>

      <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white" aria-labelledby="operation-trend-title">
        <header className="flex flex-wrap items-center justify-between gap-3 border-b border-[#EDF0EE] px-4 py-3">
          <div className="flex items-center gap-2.5"><span className="grid h-8 w-8 place-items-center rounded-md bg-blue-50 text-blue-700"><ChartNoAxesCombined className="h-4 w-4" /></span><div><h2 id="operation-trend-title" className="text-xs font-semibold text-[#17201B]">近 60 分钟调用趋势</h2><p className="mt-0.5 text-[10px] text-zinc-400">按分钟统计请求完成情况</p></div></div>
          <div className="flex items-center gap-3 text-[10px] text-zinc-500">
            <span className="inline-flex items-center gap-1.5"><i className="h-2 w-2 rounded-full bg-[#1E5F91]" />调用</span>
            <span className="inline-flex items-center gap-1.5"><i className="h-2 w-2 rounded-full bg-[#087A55]" />成功</span>
            <span className="inline-flex items-center gap-1.5"><i className="h-2 w-2 rounded-full bg-[#C43D3D]" />失败</span>
          </div>
        </header>
        <div className="h-[220px] p-4 pl-1">
          {trendLoading && !trend.points.length ? (
            <div className="grid h-full place-items-center"><Loader2 className="h-5 w-5 animate-spin text-[#12B76A]" /></div>
          ) : trendHasData ? (
            <ResponsiveContainer width="100%" height="100%">
              <LineChart accessibilityLayer data={trend.points} margin={{ top: 4, right: 14, bottom: 0, left: -10 }}>
                <CartesianGrid stroke="#EDF0EE" vertical={false} />
                <XAxis dataKey="timestamp" tickFormatter={(value) => shortClock(String(value))} tick={{ fontSize: 10, fill: '#8A938E' }} tickLine={false} axisLine={false} minTickGap={30} />
                <YAxis allowDecimals={false} tick={{ fontSize: 10, fill: '#8A938E' }} tickLine={false} axisLine={false} />
                <Tooltip formatter={(value, name) => [Number(value || 0).toLocaleString('zh-CN'), ({ total: '调用', success: '成功', failed: '失败' } as Record<string, string>)[String(name)] || String(name)]} labelFormatter={(label) => formatDate(String(label))} contentStyle={{ border: '1px solid #DCE4DF', borderRadius: 7, boxShadow: '0 8px 24px rgba(23,32,27,.08)', fontSize: 10 }} />
                <Line type="monotone" dataKey="total" stroke="#1E5F91" strokeWidth={2} dot={false} activeDot={{ r: 3 }} isAnimationActive={false} />
                <Line type="monotone" dataKey="success" stroke="#087A55" strokeWidth={2} dot={false} activeDot={{ r: 3 }} isAnimationActive={false} />
                <Line type="monotone" dataKey="failed" stroke="#C43D3D" strokeWidth={2} dot={false} activeDot={{ r: 3 }} isAnimationActive={false} />
              </LineChart>
            </ResponsiveContainer>
          ) : (
            <div className="grid h-full place-items-center text-[11px] text-zinc-400">近 60 分钟暂无调用记录</div>
          )}
        </div>
      </section>

      <div className="grid min-w-0 grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1.45fr)_minmax(360px,.75fr)]">
        <section className="min-w-0 overflow-hidden rounded-md border border-[#DCE4DF] bg-white" aria-labelledby="active-calls-title">
          <header className="flex min-h-14 flex-wrap items-center justify-between gap-3 border-b border-[#EDF0EE] px-4 py-3">
            <div><h2 id="active-calls-title" className="text-sm font-semibold text-[#17201B]">正在调用 API</h2><p className="mt-0.5 text-[10px] text-zinc-400">异常优先排列，展开可查看单个任务</p></div>
            <span className={`rounded border px-2 py-0.5 text-[10px] font-semibold ${live.activeRequests ? 'border-blue-200 bg-blue-50 text-blue-700' : 'border-emerald-200 bg-emerald-50 text-emerald-700'}`}>{live.activeRequests ? `${activeCallUserGroups.length} 个用户 / ${live.activeRequests} 个任务` : '当前空闲'}</span>
          </header>
          <div className="max-h-[640px] divide-y divide-[#EDF0EE] overflow-y-auto">
            {activeCallUserGroups.map((group) => {
              const status = activeCallStatus(group.representativeStatus);
              const elapsed = elapsedTimeMeta(group.maxElapsedSeconds);
              const concurrencyBusy = group.concurrencyUsed >= group.concurrencyLimit;
              const concurrencyPercent = Math.min(100, Math.round((group.concurrencyUsed / Math.max(1, group.concurrencyLimit)) * 100));
              const expanded = expandedUser === group.groupKey;
              const detailsId = operationPanelId(group.groupKey);
              return (
                <article key={group.groupKey} className="px-4 py-3 hover:bg-[#FAFBFA]">
                  <div className="flex items-start gap-2.5">
                    <button type="button" onClick={() => setExpandedUser(expanded ? '' : group.groupKey)} aria-expanded={expanded} aria-controls={detailsId} title={expanded ? '收起任务明细' : '展开任务明细'} aria-label={`${expanded ? '收起' : '展开'} ${group.userEmail || group.userId || group.groupKey} 的任务明细`} className="mt-0.5 grid h-6 w-6 shrink-0 place-items-center rounded border border-[#DCE4DF] bg-white text-zinc-500 hover:border-[#86EFAC] hover:text-[#047857]">{expanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}</button>
                    <div className="min-w-0 flex-1">
                      <Link href={`/users?search=${encodeURIComponent(group.userEmail || group.userId || group.groupKey)}`} className="block truncate text-[11px] font-semibold text-[#17201B] hover:text-[#047857] hover:underline">{group.userEmail || group.userId || group.groupKey}</Link>
                      <span className="mt-0.5 block truncate text-[9px] text-zinc-400">{group.keyLabel} · {billingModeLabel(group.billingMode)}</span>
                    </div>
                    <div className="flex shrink-0 items-center gap-1.5"><span className={`rounded border px-1.5 py-0.5 text-[9px] font-semibold ${status.className}`}>{status.label}</span><span className={`min-w-[55px] rounded border px-1.5 py-0.5 text-center font-mono text-[9px] font-semibold ${elapsed.className}`}>{elapsed.label}</span></div>
                  </div>

                  <div className="mt-2.5 grid grid-cols-3 gap-2 text-[9px] sm:grid-cols-[auto_auto_auto_minmax(120px,1fr)]">
                    <span className="rounded border border-[#DCE4DF] bg-white px-2 py-1 text-zinc-500">任务 <strong className="font-mono text-[#17201B]">{group.taskCount}</strong></span>
                    <span className="rounded border border-emerald-100 bg-emerald-50 px-2 py-1 text-emerald-700">处理中 {group.processingCount}</span>
                    <span className="rounded border border-amber-100 bg-amber-50 px-2 py-1 text-amber-700">排队 {group.queuedCount}</span>
                    <div className="col-span-3 flex min-w-0 items-center gap-2 rounded bg-[#F7F8F6] px-2 py-1 sm:col-span-1">
                      <span className="min-w-0 flex-1 truncate text-zinc-500">{group.modelLabel}</span>
                      <span className={`shrink-0 font-mono ${concurrencyBusy ? 'text-red-700' : 'text-zinc-600'}`}>并发 {group.concurrencyUsed}/{group.concurrencyLimit}</span>
                      <span className="h-1.5 w-12 overflow-hidden rounded-full bg-zinc-200"><i className={`block h-full rounded-full ${concurrencyBusy ? 'bg-red-500' : 'bg-emerald-500'}`} style={{ width: `${concurrencyPercent}%` }} /></span>
                    </div>
                  </div>

                  {expanded && (
                    <div id={detailsId} className="mt-3 divide-y divide-[#E5E9E6] overflow-hidden rounded-md border border-[#DCE4DF] bg-white">
                      {group.calls.map((call) => {
                        const callStatus = activeCallStatus(call.status);
                        const callElapsed = elapsedTimeMeta(call.elapsedSeconds);
                        return (
                          <div key={call.taskId} className="px-3 py-2.5">
                            <div className="flex items-start justify-between gap-3"><div className="min-w-0"><strong className="block truncate font-mono text-[10px] text-[#17201B]" title={call.taskId}>{call.taskId}</strong><small className="mt-0.5 block truncate text-[9px] text-zinc-400">{call.model || '未知模型'} · {sizeTierLabel(call.sizeTier)}{call.size ? ` · ${call.size}` : ''}</small></div><div className="flex shrink-0 gap-1"><span className={`rounded border px-1.5 py-0.5 text-[9px] font-semibold ${callStatus.className}`}>{callStatus.label}</span><span className={`rounded border px-1.5 py-0.5 font-mono text-[9px] font-semibold ${callElapsed.className}`}>{callElapsed.label}</span></div></div>
                            <div className="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-[9px] text-zinc-500 sm:grid-cols-4"><span><small className="block text-zinc-400">API Key</small>{call.keyName || call.keyPrefix || 'API Key'}</span><span><small className="block text-zinc-400">计费方式</small>{billingModeLabel(call.billingMode)}</span><span><small className="block text-zinc-400">数量 / 并发</small>{call.quantity} 张 · {call.activeForKey}/{call.concurrencyLimit}</span><span><small className="block text-zinc-400">提交时间</small>{formatDate(call.createdAt)}</span></div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </article>
              );
            })}
            {!activeCallUserGroups.length && (
              <div className="grid min-h-[280px] place-items-center px-4 text-center"><div><Radio className="mx-auto h-5 w-5 text-emerald-500" /><p className="mt-2 text-[11px] font-semibold text-zinc-600">当前没有进行中的 API 请求</p><span className="mt-1 block text-[10px] text-zinc-400">新任务进入后会自动显示</span></div></div>
            )}
          </div>
        </section>

        <section className="min-w-0 overflow-hidden rounded-md border border-[#DCE4DF] bg-white" aria-labelledby="operation-ranking-title">
          <header className="border-b border-[#EDF0EE] px-4 py-3">
            <div className="flex items-center justify-between gap-3"><div><h2 id="operation-ranking-title" className="text-sm font-semibold text-[#17201B]">用户用量 Top 10</h2><p className="mt-0.5 text-[10px] text-zinc-400">历史排行，不影响实时任务</p></div>{rankingLoading && <Loader2 className="h-4 w-4 animate-spin text-[#12B76A]" aria-label="排行加载中" />}</div>
            <div className="mt-3 flex flex-wrap items-center gap-2">
              <div className="inline-flex rounded-md border border-[#DCE4DF] bg-[#F7F8F6] p-0.5" role="group" aria-label="排行时间范围">
                {([['today', '今日'], ['7d', '7天'], ['15d', '15天'], ['30d', '30天']] as const).map(([value, label]) => (
                  <button key={value} type="button" onClick={() => setRange(value)} aria-pressed={range === value} className={`h-7 min-w-11 rounded px-2 text-[10px] font-semibold ${range === value ? 'bg-white text-[#047857] shadow-sm' : 'text-zinc-500 hover:text-zinc-800'}`}>{label}</button>
                ))}
              </div>
              <AppSelect compact value={metric} options={METRIC_OPTIONS} onValueChange={(value) => setMetric(value as AdminOperationsMetric)} ariaLabel="用户排行指标" className="min-w-[126px] flex-1" />
            </div>
          </header>
          <div className="max-h-[640px] divide-y divide-[#EDF0EE] overflow-y-auto">
            {ranking.topUsers.map((user, index) => {
              const primary = rankingMetricMeta(user, metric);
              const rankTone = index === 0 ? 'bg-amber-50 text-amber-700' : index === 1 ? 'bg-zinc-100 text-zinc-600' : index === 2 ? 'bg-orange-50 text-orange-700' : 'bg-zinc-100 text-zinc-500';
              const successTone = user.successRate >= 95 ? 'text-emerald-700' : user.successRate >= 80 ? 'text-amber-700' : 'text-red-700';
              return (
                <article key={user.userId} className="px-4 py-3 hover:bg-[#FAFBFA]">
                  <div className="flex items-start gap-2.5"><span className={`grid h-5 w-5 shrink-0 place-items-center rounded font-mono text-[10px] font-bold ${rankTone}`}>{index + 1}</span><div className="min-w-0 flex-1"><Link href={`/users?search=${encodeURIComponent(user.userEmail || user.userId)}`} className="block truncate text-[11px] font-semibold text-[#17201B] hover:text-[#047857] hover:underline">{user.userEmail || user.userId}</Link><small className="mt-0.5 block truncate text-[9px] text-zinc-400">{billingModeLabel(user.billingMode)} · {formatDate(user.lastRequestAt)}</small></div><div className="shrink-0 text-right"><strong className="block font-mono text-xs text-[#17201B]">{primary.value}</strong><small className="text-[9px] text-zinc-400">{primary.label}</small></div></div>
                  <div className="mt-2 grid grid-cols-2 gap-1.5 pl-7 text-[9px] sm:flex sm:flex-wrap">
                    <span className="min-w-0 rounded border border-emerald-100 bg-emerald-50 px-1.5 py-1 text-emerald-700">可用余额 <strong className="font-mono">{formatCNY(user.availableBalance)}</strong></span>
                    <span className="min-w-0 rounded border border-amber-100 bg-amber-50 px-1.5 py-1 text-amber-700">今日已用余额 <strong className="font-mono">{formatCNY(user.todayCreditsSpent)}</strong></span>
                  </div>
                  <div className="mt-1.5 flex flex-wrap items-center gap-x-2.5 gap-y-1 pl-7 text-[9px] text-zinc-500"><span>调用 <strong className="font-mono text-zinc-700">{user.requestCount}</strong></span><span className={successTone}>成功率 <strong className="font-mono">{user.successRate.toFixed(1)}%</strong></span><span>图片 <strong className="font-mono text-zinc-700">{user.imageCount}</strong></span><span>平均 <strong className="font-mono text-zinc-700">{durationLabel(user.averageDurationSeconds)}</strong></span></div>
                </article>
              );
            })}
            {!rankingLoading && !ranking.topUsers.length && <div className="grid min-h-[280px] place-items-center px-4 text-center"><div><Users className="mx-auto h-5 w-5 text-zinc-300" /><p className="mt-2 text-[11px] font-semibold text-zinc-600">当前范围暂无 API 调用</p></div></div>}
          </div>
        </section>
      </div>
    </div>
  );
}
