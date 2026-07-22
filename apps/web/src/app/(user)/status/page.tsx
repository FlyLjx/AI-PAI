'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Activity,
  CheckCircle2,
  Clock3,
  HeartPulse,
  LoaderCircle,
  RefreshCw,
  Server,
  ShieldCheck,
  XCircle,
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
import { PageHeader } from '@/components/common/PageHeader';
import { StatBlock } from '@/components/common/StatBlock';
import {
  APIError,
  portalApi,
  type OpenAIImageStatusSnapshot,
  type StabilityRecentWindow,
  type StabilitySnapshot,
} from '@/lib/portal-api';
import { formatDate } from '@/lib/common/utils';

const REFRESH_INTERVAL_MS = 30_000;
const TREND_COLORS = {
  success: '#3F9274',
  failed: '#D06F69',
} as const;

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : '接口状态加载失败';
}

function percentage(value: number | undefined): string {
  const normalized = Number(value || 0);
  return `${normalized.toFixed(1)}%`;
}

function durationLabel(value: number | undefined): string {
  const seconds = Math.max(0, Number(value || 0));
  if (seconds < 1) return `${Math.round(seconds * 1000)}ms`;
  if (seconds < 60) return `${seconds.toFixed(1)}秒`;
  const minutes = Math.floor(seconds / 60);
  const remainder = Math.round(seconds % 60);
  return `${minutes}分${remainder}秒`;
}

function timeLabel(value: string): string {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
}

type TaskBreakdown = {
  key: keyof Pick<StabilityRecentWindow, 'success' | 'failed' | 'canceled' | 'rejected' | 'running' | 'other'>;
  label: string;
  color: string;
};

const TASK_BREAKDOWN: readonly TaskBreakdown[] = [
  { key: 'success', label: '成功', color: '#3F9274' },
  { key: 'failed', label: '失败', color: '#D06F69' },
  { key: 'canceled', label: '已取消', color: '#D69A45' },
  { key: 'rejected', label: '已拒绝', color: '#8A7FB0' },
  { key: 'running', label: '运行中', color: '#587FA3' },
  { key: 'other', label: '其他', color: '#9AA29D' },
];

export default function StatusPage() {
  const [snapshot, setSnapshot] = useState<StabilitySnapshot | null>(null);
  const [openAIStatus, setOpenAIStatus] = useState<OpenAIImageStatusSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [openAIError, setOpenAIError] = useState('');
  const [autoRefresh, setAutoRefresh] = useState(true);

  const loadStatus = useCallback(async () => {
    setLoading(true);
    setError('');
    setOpenAIError('');
    try {
      const [stabilityResponse, openAIResponse] = await Promise.allSettled([
        portalApi.stability(),
        portalApi.openAIImageStatus(),
      ]);
      if (stabilityResponse.status === 'fulfilled') {
        setSnapshot(stabilityResponse.value.data);
      } else {
        throw stabilityResponse.reason;
      }
      if (openAIResponse.status === 'fulfilled') {
        setOpenAIStatus(openAIResponse.value.data);
      } else {
        setOpenAIError(errorMessage(openAIResponse.reason));
      }
    } catch (loadError) {
      setError(errorMessage(loadError));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => void loadStatus(), 0);
    return () => window.clearTimeout(timer);
  }, [loadStatus]);

  useEffect(() => {
    if (!autoRefresh) return;
    const timer = window.setInterval(() => void loadStatus(), REFRESH_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [autoRefresh, loadStatus]);

  const runtime = snapshot?.runtime;
  const recent = snapshot?.recent_60;
  const upstreamCode = Number(snapshot?.upstream_status_code || 0);
  const reachable = Boolean(snapshot?.reachable && upstreamCode >= 200 && upstreamCode < 300);
  const degraded = reachable && Number(runtime?.error_rate || 0) > 0;
  const overallLabel = reachable ? degraded ? '接口在线，近期存在波动' : '接口运行正常' : '接口连接异常';
  const overallDetail = reachable
    ? `上游 HTTP ${upstreamCode} · 最近 ${runtime?.window_minutes || 60} 分钟成功率 ${percentage(runtime?.success_rate)}`
    : snapshot?.error || '状态服务暂时不可达';
  const overallTone = reachable ? degraded ? 'bg-amber-500' : 'bg-[#3F9274]' : 'bg-[#D06F69]';
  const overallIconTone = reachable ? degraded ? 'bg-amber-50 text-amber-700' : 'bg-[#eaf8ef] text-[#087443]' : 'bg-red-50 text-red-700';
  const openAISeverity = openAIStatus?.severity || 'ok';
  const openAITone = openAIError || openAISeverity === 'critical'
    ? 'bg-red-50 text-red-700'
    : openAISeverity === 'warning'
      ? 'bg-amber-50 text-amber-700'
      : 'bg-[#eaf8ef] text-[#087443]';
  const openAIBar = openAIError || openAISeverity === 'critical' ? 'bg-[#D06F69]' : openAISeverity === 'warning' ? 'bg-amber-500' : 'bg-[#3F9274]';

  const trendData = useMemo(() => (runtime?.series || []).map((point) => ({
    label: point.label || timeLabel(point.time),
    success: Number(point.success || 0),
    failed: Number(point.failed || 0),
  })), [runtime?.series]);

  const errorReasons = runtime?.error_reasons || [];
  const taskTotal = Math.max(0, Number(recent?.total || 0));
  const taskRows = TASK_BREAKDOWN.map((item) => ({
    ...item,
    value: Math.max(0, Number(recent?.[item.key] || 0)),
  })).filter((item) => item.value > 0 || item.key === 'success' || item.key === 'failed');

  return (
    <div className="page-stack">
      <PageHeader title="接口状态" description="实时查看上游 API 的可用性、成功率与响应耗时">
        <button
          className="btn"
          type="button"
          role="switch"
          aria-checked={autoRefresh}
          onClick={() => setAutoRefresh((enabled) => !enabled)}
        >
          <span className={`relative h-4 w-7 shrink-0 overflow-hidden rounded-full transition-colors ${autoRefresh ? 'bg-[#3F9274]' : 'bg-zinc-300'}`} aria-hidden="true">
            <span className={`absolute left-0.5 top-0.5 z-10 h-3 w-3 rounded-full bg-white shadow-sm ring-1 ring-black/5 transition-transform ${autoRefresh ? 'translate-x-3' : 'translate-x-0'}`} />
          </span>
          自动刷新
        </button>
        <button className="btn" type="button" onClick={() => void loadStatus()} disabled={loading}>
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />刷新
        </button>
      </PageHeader>

      {error && <div className="notice border-red-200 bg-red-50 text-red-700" role="alert">{error}</div>}

      <section className="section-panel overflow-hidden" aria-labelledby="overall-status-title">
        <div className={`h-1 w-full ${overallTone}`} />
        <div className="flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 items-center gap-3">
            <span className={`grid h-11 w-11 shrink-0 place-items-center rounded-[7px] ${overallIconTone}`}>
              {reachable ? <HeartPulse size={21} /> : <XCircle size={21} />}
            </span>
            <div className="min-w-0">
              <span className="text-[10px] font-bold text-zinc-400">总体状态</span>
              <strong id="overall-status-title" className="mt-0.5 block text-[16px]">{loading && !snapshot ? '正在检测接口状态' : overallLabel}</strong>
              <p className="mt-1 text-[11px] text-zinc-500">{loading && !snapshot ? '正在连接状态服务...' : overallDetail}</p>
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-3 text-right">
            <div>
              <span className="block text-[10px] text-zinc-400">数据更新时间</span>
              <strong className="mono mt-1 block text-[11px] text-[#526059]">{snapshot?.fetched_at ? formatDate(snapshot.fetched_at) : '-'}</strong>
            </div>
            {loading && <LoaderCircle size={16} className="animate-spin text-zinc-400" aria-label="更新中" />}
          </div>
        </div>
        {snapshot?.error && <div className="border-t border-red-100 bg-red-50 px-5 py-2.5 text-[11px] text-red-700">{snapshot.error}</div>}
      </section>

      <section className="section-panel overflow-hidden" aria-labelledby="openai-image-status-title">
        <div className={`h-1 w-full ${openAIBar}`} />
        <div className="flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 items-center gap-3">
            <span className={`grid h-11 w-11 shrink-0 place-items-center rounded-[7px] ${openAITone}`}>
              <HeartPulse size={21} />
            </span>
            <div className="min-w-0">
              <span className="text-[10px] font-bold text-zinc-400">OpenAI Image</span>
              <strong id="openai-image-status-title" className="mt-0.5 block text-[16px]">{loading && !openAIStatus ? '正在检测 OAI 图像状态' : openAIError || openAIStatus?.statusLabel || '状态未知'}</strong>
              <p className="mt-1 text-[11px] text-zinc-500">{openAIStatus?.summary || '订阅 OpenAI 官方状态源，仅展示 Image / Image Generation 相关事件。'}</p>
            </div>
          </div>
          <div className="shrink-0 text-right">
            <span className="block text-[10px] text-zinc-400">RSS 更新时间</span>
            <strong className="mono mt-1 block text-[11px] text-[#526059]">{openAIStatus?.fetchedAt ? formatDate(openAIStatus.fetchedAt) : '-'}</strong>
          </div>
        </div>
      </section>

      <section className="metric-grid" aria-label="接口状态关键指标">
        <StatBlock
          title="上游状态"
          value={loading && !snapshot ? '--' : reachable ? '在线' : '异常'}
          subtext={upstreamCode ? `HTTP ${upstreamCode}` : '等待状态码'}
          icon={Server}
          color={reachable ? 'green' : 'neutral'}
        />
        <StatBlock
          title="最近任务成功率"
          value={loading && !snapshot ? '--' : percentage(recent?.success_rate)}
          subtext={`${Number(recent?.success || 0).toLocaleString()} 成功 / ${Number(recent?.availability_total || 0).toLocaleString()} 已完成`}
          icon={ShieldCheck}
          color="green"
        />
        <StatBlock
          title="成功平均耗时"
          value={loading && !snapshot ? '--' : durationLabel(recent?.average_success_duration_secs)}
          subtext={`失败平均 ${durationLabel(recent?.average_failure_duration_secs)}`}
          icon={Clock3}
          color="cyan"
        />
        <StatBlock
          title="近一小时请求"
          value={loading && !snapshot ? '--' : Number(runtime?.total || 0).toLocaleString()}
          subtext={`${Number(runtime?.totals?.success || 0).toLocaleString()} 成功 · ${Number(runtime?.totals?.failed || 0).toLocaleString()} 失败`}
          icon={Activity}
          color="amber"
        />
      </section>

      <section className="section-panel min-w-0 overflow-hidden" aria-labelledby="stability-trend-title">
        <header className="flex min-h-[50px] flex-col gap-2 border-b border-[#edf0ee] px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-2.5">
            <span className="grid size-8 shrink-0 place-items-center rounded-[7px] bg-blue-50 text-blue-600"><Activity size={16} /></span>
            <div>
              <strong id="stability-trend-title" className="block text-[14px]">最近 60 分钟稳定性</strong>
              <small className="mt-0.5 block text-[11px] text-zinc-500">按分钟统计成功与失败请求</small>
            </div>
          </div>
          <div className="flex items-center gap-4 text-[11px] text-zinc-500" aria-label="趋势图例">
            <span className="inline-flex items-center gap-1.5"><i className="size-2 rounded-full bg-[#3F9274]" />成功</span>
            <span className="inline-flex items-center gap-1.5"><i className="size-2 rounded-full bg-[#D06F69]" />失败</span>
            <strong className="mono text-[#17201b]">成功率 {percentage(runtime?.success_rate)}</strong>
          </div>
        </header>

        <div className="relative h-[270px] w-full px-1 pb-3 pt-3 sm:h-[310px] sm:px-3">
          {loading && !snapshot ? (
            <div className="grid h-full place-items-center text-[12px] text-zinc-400"><span className="inline-flex items-center gap-2"><LoaderCircle size={14} className="animate-spin" />正在读取趋势</span></div>
          ) : trendData.length === 0 ? (
            <div className="grid h-full place-items-center text-[12px] text-zinc-400">当前窗口暂无趋势数据</div>
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <LineChart accessibilityLayer data={trendData} margin={{ top: 8, right: 14, left: -8, bottom: 0 }}>
                <CartesianGrid stroke="#edf0ee" strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="label" tick={{ fill: '#778079', fontSize: 9 }} tickLine={false} axisLine={{ stroke: '#dce4df' }} minTickGap={28} />
                <YAxis allowDecimals={false} tick={{ fill: '#778079', fontSize: 9 }} tickLine={false} axisLine={false} width={40} />
                <Tooltip
                  formatter={(value, name) => [Number(value || 0).toLocaleString(), String(name)]}
                  labelFormatter={(label) => `时间 ${String(label)}`}
                  contentStyle={{ border: '1px solid #dce4df', borderRadius: 7, boxShadow: '0 8px 24px rgba(23,32,27,.08)', fontSize: 10 }}
                />
                <Line type="monotone" dataKey="success" name="成功" stroke={TREND_COLORS.success} strokeWidth={2.25} dot={false} activeDot={{ r: 4, fill: '#fff', strokeWidth: 2.25 }} />
                <Line type="monotone" dataKey="failed" name="失败" stroke={TREND_COLORS.failed} strokeWidth={2.25} strokeDasharray="5 4" dot={false} activeDot={{ r: 4, fill: '#fff', strokeWidth: 2.25 }} />
              </LineChart>
            </ResponsiveContainer>
          )}
        </div>
      </section>

      <section className="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div className="section-panel overflow-hidden" aria-labelledby="task-window-title">
          <header className="section-head">
            <div><strong id="task-window-title">最近 {Number(recent?.limit || 60)} 个任务</strong><small className="ml-2">状态分布</small></div>
            <span className="status-pill">共 {taskTotal.toLocaleString()}</span>
          </header>
          <div className="grid gap-3 p-4">
            {taskRows.map((item) => {
              const width = taskTotal > 0 ? Math.min(100, (item.value / taskTotal) * 100) : 0;
              return (
                <div key={item.key} className="grid grid-cols-[58px_minmax(0,1fr)_42px] items-center gap-3 text-[11px]">
                  <span className="text-zinc-500">{item.label}</span>
                  <span className="h-2 overflow-hidden rounded-full bg-[#f1f3f2]">
                    <i className="block h-full rounded-full" style={{ width: `${width}%`, backgroundColor: item.color }} />
                  </span>
                  <strong className="mono text-right text-[#17201b]">{item.value.toLocaleString()}</strong>
                </div>
              );
            })}
          </div>
          <div className="grid grid-cols-2 border-t border-[#edf0ee] bg-[#fafbf9] text-[11px] sm:grid-cols-4">
            <div className="px-4 py-3"><span className="block text-zinc-400">失败率</span><strong className="mono mt-1 block">{percentage(recent?.failure_rate)}</strong></div>
            <div className="border-l border-[#edf0ee] px-4 py-3"><span className="block text-zinc-400">平均耗时</span><strong className="mono mt-1 block">{durationLabel(recent?.average_duration_secs)}</strong></div>
            <div className="border-l-0 border-t border-[#edf0ee] px-4 py-3 sm:border-l sm:border-t-0"><span className="block text-zinc-400">已取消</span><strong className="mono mt-1 block">{Number(recent?.canceled || 0).toLocaleString()}</strong></div>
            <div className="border-l border-t border-[#edf0ee] px-4 py-3 sm:border-t-0"><span className="block text-zinc-400">已拒绝</span><strong className="mono mt-1 block">{Number(recent?.rejected || 0).toLocaleString()}</strong></div>
          </div>
        </div>

        <aside className="section-panel overflow-hidden" aria-labelledby="error-reasons-title">
          <header className="section-head">
            <div><strong id="error-reasons-title">错误原因</strong><small className="ml-2">最近一小时</small></div>
            {errorReasons.length > 0 ? <XCircle size={16} className="text-[#D06F69]" /> : <CheckCircle2 size={16} className="text-[#3F9274]" />}
          </header>
          <div className="p-4">
            {errorReasons.length === 0 ? (
              <div className="grid min-h-[112px] place-items-center text-center">
                <div><CheckCircle2 size={24} className="mx-auto text-[#3F9274]" /><strong className="mt-2 block text-[12px]">没有已记录的错误原因</strong><p className="mt-1 text-[10px] text-zinc-400">最近窗口内未返回错误明细</p></div>
              </div>
            ) : (
              <ul className="grid gap-2.5">
                {errorReasons.map((reason) => (
                  <li key={reason.label} className="flex items-start justify-between gap-3 border-b border-[#edf0ee] pb-2.5 last:border-0 last:pb-0">
                    <span className="min-w-0 break-words text-[11px] leading-4 text-zinc-600">{reason.label}</span>
                    <strong className="mono shrink-0 text-[11px] text-[#b42318]">{Number(reason.value || 0).toLocaleString()}</strong>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <footer className="border-t border-[#edf0ee] bg-[#fafbf9] px-4 py-3 text-[10px] text-zinc-400">
            数据源 <code className="mono text-[#526059]">/health/stability</code>
          </footer>
        </aside>
      </section>
    </div>
  );
}
