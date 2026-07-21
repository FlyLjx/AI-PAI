'use client';

import Link from 'next/link';
import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Activity,
  ImageIcon,
  ListOrdered,
  Loader2,
  Radio,
  RefreshCw,
  Timer,
  TriangleAlert,
  Users,
} from 'lucide-react';
import { AppSelect } from '@/components/common/AppSelect';
import { PageHeader } from '@/components/common/PageHeader';
import {
  portalApi,
  type AdminOperationsMetric,
  type AdminOperationsRange,
  type AdminOperationsSnapshot,
} from '@/lib/admin-api';
import { formatDate } from '@/lib/common/utils';

const METRIC_OPTIONS = [
  { value: 'requests', label: '按调用次数' },
  { value: 'images', label: '按输出图片' },
  { value: 'credits', label: '按余额消费' },
  { value: 'failures', label: '按失败次数' },
  { value: 'duration', label: '按平均耗时' },
] as const;

const EMPTY_OPERATIONS: AdminOperationsSnapshot = {
  range: 'today',
  metric: 'requests',
  activeUsers: 0,
  activeRequests: 0,
  queuedRequests: 0,
  processingRequests: 0,
  slowRequests: 0,
  averageElapsedSeconds: 0,
  topUsers: [],
  activeCalls: [],
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

export default function AdminAPIOperationsPage() {
  const [operations, setOperations] = useState<AdminOperationsSnapshot>(EMPTY_OPERATIONS);
  const [range, setRange] = useState<AdminOperationsRange>('today');
  const [metric, setMetric] = useState<AdminOperationsMetric>('requests');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async (quiet = false) => {
    if (!quiet) setLoading(true);
    try {
      const response = await portalApi.adminOperations(range, metric);
      setOperations(response.data);
      setError('');
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : '实时 API 运营数据加载失败');
    } finally {
      if (!quiet) setLoading(false);
    }
  }, [metric, range]);

  useEffect(() => {
    const firstLoad = window.setTimeout(() => void load(), 0);
    const timer = window.setInterval(() => {
      if (!document.hidden) void load(true);
    }, 5_000);
    return () => {
      window.clearTimeout(firstLoad);
      window.clearInterval(timer);
    };
  }, [load]);

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
    }>();

    operations.activeCalls.forEach((call) => {
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
      };

      group.taskCount += 1;
      group.imageCount += Math.max(0, Number(call.quantity || 0));
      group.maxElapsedSeconds = Math.max(group.maxElapsedSeconds, call.elapsedSeconds);
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

    return Array.from(groups.values())
      .sort((left, right) => left.firstCreatedAt.localeCompare(right.firstCreatedAt))
      .map((group) => {
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
        };
      });
  }, [operations.activeCalls]);

  return (
    <div className="space-y-5">
      <PageHeader title="API 实时运营" description="查看高用量客户、正在调用的用户与当前任务压力。">
        <button type="button" onClick={() => void load()} disabled={loading} className="inline-flex h-8 items-center gap-2 rounded-md border border-[#DCE4DF] bg-white px-3 text-xs font-semibold text-[#17201B] hover:border-[#12B76A] disabled:opacity-50">
          <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />刷新
        </button>
      </PageHeader>

      <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white" aria-labelledby="api-operations-panel-title">
        <header className="flex flex-col gap-3 border-b border-[#EDF0EE] px-4 py-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex min-w-0 items-center gap-3">
            <span className="grid h-9 w-9 shrink-0 place-items-center rounded-md bg-emerald-50 text-[#047857]"><Radio className="h-4.5 w-4.5" /></span>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2"><h2 id="api-operations-panel-title" className="text-sm font-semibold text-[#17201B]">实时调用状态</h2><span className="inline-flex h-5 items-center gap-1.5 rounded border border-emerald-200 bg-emerald-50 px-1.5 text-[10px] font-semibold text-emerald-700"><Radio className="h-3 w-3" />5秒刷新</span></div>
              <p className="mt-0.5 text-[11px] text-zinc-500">排行筛选不会影响右侧实时任务</p>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="inline-flex rounded-md border border-[#DCE4DF] bg-[#F7F8F6] p-0.5" role="group" aria-label="排行时间范围">
              {([['today', '今日'], ['7d', '7天'], ['15d', '15天'], ['30d', '30天']] as const).map(([value, label]) => (
                <button key={value} type="button" onClick={() => setRange(value)} aria-pressed={range === value} className={`h-7 min-w-11 rounded px-2 text-[11px] font-semibold ${range === value ? 'bg-white text-[#047857] shadow-sm' : 'text-zinc-500 hover:text-zinc-800'}`}>{label}</button>
              ))}
            </div>
            <AppSelect compact value={metric} options={METRIC_OPTIONS} onValueChange={(value) => setMetric(value as AdminOperationsMetric)} ariaLabel="用户排行指标" className="min-w-[132px]" />
          </div>
        </header>

        {error && (
          <div className="flex items-center justify-between gap-3 border-b border-red-100 bg-red-50 px-4 py-2.5 text-[11px] text-red-700" role="alert">
            <span className="flex min-w-0 items-center gap-2"><TriangleAlert className="h-3.5 w-3.5 shrink-0" /><span className="truncate">{error}</span></span>
            <button type="button" onClick={() => void load()} className="shrink-0 font-semibold underline">重试</button>
          </div>
        )}

        {loading && !operations.generatedAt ? (
          <div className="grid min-h-[360px] place-items-center"><Loader2 className="h-5 w-5 animate-spin text-[#12B76A]" /></div>
        ) : (
          <>
            <div className="grid grid-cols-2 divide-x divide-y divide-[#EDF0EE] border-b border-[#EDF0EE] sm:grid-cols-3 xl:grid-cols-5 xl:divide-y-0">
              {[
                { label: '调用用户', value: operations.activeUsers, note: '当前有任务的客户', icon: Users, tone: 'bg-blue-50 text-blue-700' },
                { label: '进行中任务', value: operations.activeRequests, note: `${operations.processingRequests} 个上游处理中`, icon: Activity, tone: 'bg-emerald-50 text-emerald-700' },
                { label: '排队任务', value: operations.queuedRequests, note: '等待并发执行', icon: ListOrdered, tone: 'bg-amber-50 text-amber-700' },
                { label: '平均已用时间', value: durationLabel(operations.averageElapsedSeconds), note: '当前任务实时均值', icon: Timer, tone: 'bg-zinc-100 text-zinc-600' },
                { label: '超过 120 秒', value: operations.slowRequests, note: '需要优先关注', icon: TriangleAlert, tone: operations.slowRequests ? 'bg-red-50 text-red-700' : 'bg-zinc-100 text-zinc-500' },
              ].map((item) => {
                const Icon = item.icon;
                return <div key={item.label} className="flex min-w-0 items-center gap-3 px-4 py-3.5"><span className={`grid h-8 w-8 shrink-0 place-items-center rounded-md ${item.tone}`}><Icon className="h-4 w-4" /></span><span className="min-w-0"><span className="block text-[10px] font-semibold text-zinc-500">{item.label}</span><strong className="mt-0.5 block truncate font-mono text-base text-[#17201B]">{typeof item.value === 'number' ? item.value.toLocaleString('zh-CN') : item.value}</strong><small className="block truncate text-[9px] text-zinc-400">{item.note}</small></span></div>;
              })}
            </div>

            <div className="grid min-w-0 grid-cols-1 xl:grid-cols-[minmax(0,1.2fr)_minmax(340px,.8fr)]">
              <div className="min-w-0 border-b border-[#EDF0EE] xl:border-b-0 xl:border-r">
                <div className="flex min-h-11 items-center justify-between gap-3 border-b border-[#EDF0EE] px-4 py-2.5"><div><h3 className="text-xs font-semibold text-[#17201B]">用户用量 Top 10</h3><p className="mt-0.5 text-[10px] text-zinc-400">点击客户进入用户管理</p></div><ImageIcon className="h-4 w-4 text-zinc-400" /></div>
                <div className="overflow-x-auto">
                  <table className="w-full min-w-[760px] text-left text-[11px]">
                    <thead className="bg-[#F7F8F6] text-[10px] text-zinc-500"><tr><th className="w-11 px-3 py-2 text-center">#</th><th className="px-3 py-2">客户</th><th className="px-3 py-2 text-right">调用</th><th className="px-3 py-2 text-right">成功率</th><th className="px-3 py-2 text-right">图片</th><th className="px-3 py-2 text-right">余额消费</th><th className="px-3 py-2 text-right">平均耗时</th></tr></thead>
                    <tbody className="divide-y divide-[#EDF0EE]">
                      {operations.topUsers.map((user, index) => (
                        <tr key={user.userId} className="hover:bg-[#FAFBFA]">
                          <td className="px-3 py-2.5 text-center"><span className={`inline-grid h-5 w-5 place-items-center rounded font-mono text-[10px] font-bold ${index < 3 ? 'bg-emerald-50 text-emerald-700' : 'bg-zinc-100 text-zinc-500'}`}>{index + 1}</span></td>
                          <td className="max-w-[210px] px-3 py-2.5"><Link href={`/users?search=${encodeURIComponent(user.userEmail || user.userId)}`} className="block truncate font-semibold text-[#17201B] hover:text-[#047857] hover:underline">{user.userEmail || user.userId}</Link><small className="mt-0.5 block text-[9px] text-zinc-400">{billingModeLabel(user.billingMode)} · {formatDate(user.lastRequestAt)}</small></td>
                          <td className="px-3 py-2.5 text-right font-mono font-semibold">{user.requestCount.toLocaleString('zh-CN')}</td>
                          <td className={`px-3 py-2.5 text-right font-mono font-semibold ${user.successRate >= 95 ? 'text-emerald-700' : user.successRate >= 80 ? 'text-amber-700' : 'text-red-700'}`}>{user.successRate.toFixed(1)}%</td>
                          <td className="px-3 py-2.5 text-right font-mono">{user.imageCount.toLocaleString('zh-CN')}</td>
                          <td className="px-3 py-2.5 text-right font-mono">{user.creditsSpent.toLocaleString('zh-CN', { maximumFractionDigits: 4 })}</td>
                          <td className="px-3 py-2.5 text-right font-mono">{user.averageDurationSeconds.toFixed(1)}s</td>
                        </tr>
                      ))}
                      {!operations.topUsers.length && <tr><td colSpan={7} className="px-4 py-10 text-center text-zinc-400">当前范围暂无 API 调用</td></tr>}
                    </tbody>
                  </table>
                </div>
              </div>

              <div className="min-w-0">
                <div className="flex min-h-11 items-center justify-between gap-3 border-b border-[#EDF0EE] px-4 py-2.5"><div><h3 className="text-xs font-semibold text-[#17201B]">正在调用 API</h3><p className="mt-0.5 text-[10px] text-zinc-400">按用户聚合，按最早任务排序</p></div><span className={`rounded border px-2 py-0.5 text-[10px] font-semibold ${operations.activeRequests ? 'border-blue-200 bg-blue-50 text-blue-700' : 'border-emerald-200 bg-emerald-50 text-emerald-700'}`}>{operations.activeRequests ? `${activeCallUserGroups.length} 个用户 / ${operations.activeRequests} 个任务` : '当前空闲'}</span></div>
                <div className="max-h-[520px] divide-y divide-[#EDF0EE] overflow-y-auto">
                  {activeCallUserGroups.slice(0, 20).map((group) => {
                    const status = activeCallStatus(group.representativeStatus);
                    const elapsed = elapsedTimeMeta(group.maxElapsedSeconds);
                    const concurrencyBusy = group.concurrencyUsed >= group.concurrencyLimit;
                    return (
                      <div key={group.groupKey} className="px-4 py-3 hover:bg-[#FAFBFA]">
                        <div className="flex items-start justify-between gap-3"><div className="min-w-0"><Link href={`/users?search=${encodeURIComponent(group.userEmail || group.userId || group.groupKey)}`} className="block truncate text-[11px] font-semibold text-[#17201B] hover:text-[#047857] hover:underline">{group.userEmail || group.userId || group.groupKey}</Link><span className="mt-0.5 block truncate text-[9px] text-zinc-400">{group.keyLabel} · {billingModeLabel(group.billingMode)}</span></div><div className="flex shrink-0 items-center gap-1.5"><span className={`rounded border px-1.5 py-0.5 text-[9px] font-semibold ${status.className}`}>{status.label}</span><span className={`min-w-[55px] rounded border px-1.5 py-0.5 text-center font-mono text-[9px] font-semibold ${elapsed.className}`}>{elapsed.label}</span></div></div>
                        <div className="mt-2 flex items-center justify-between gap-3 rounded bg-[#F7F8F6] px-2 py-1.5 text-[10px]"><span className="min-w-0 truncate font-medium text-zinc-600">{group.modelLabel}</span><span className={`shrink-0 font-mono ${concurrencyBusy ? 'text-red-700' : 'text-zinc-500'}`}>并发 {group.concurrencyUsed}/{group.concurrencyLimit}</span></div>
                        <div className="mt-2 flex flex-wrap items-center gap-1.5 text-[9px] text-zinc-500">
                          <span className="rounded border border-[#DCE4DF] bg-white px-1.5 py-0.5">任务数量 <strong className="font-mono text-[#17201B]">{group.taskCount}</strong> 个</span>
                          <span className="rounded border border-emerald-100 bg-emerald-50 px-1.5 py-0.5 text-emerald-700">处理中 {group.processingCount}</span>
                          <span className="rounded border border-amber-100 bg-amber-50 px-1.5 py-0.5 text-amber-700">排队 {group.queuedCount}</span>
                          <span className="rounded border border-zinc-200 bg-white px-1.5 py-0.5">图片 {group.imageCount}</span>
                          {group.slowCount > 0 && <span className="rounded border border-red-100 bg-red-50 px-1.5 py-0.5 text-red-700">超时关注 {group.slowCount}</span>}
                        </div>
                      </div>
                    );
                  })}
                  {!activeCallUserGroups.length && <div className="grid min-h-[240px] place-items-center px-4 text-center"><div><Radio className="mx-auto h-5 w-5 text-emerald-500" /><p className="mt-2 text-[11px] font-semibold text-zinc-600">当前没有进行中的 API 请求</p><span className="mt-1 block text-[10px] text-zinc-400">新任务进入后会自动显示</span></div></div>}
                </div>
              </div>
            </div>

            <footer className="flex flex-wrap items-center justify-between gap-2 border-t border-[#EDF0EE] bg-[#FAFBFA] px-4 py-2 text-[9px] text-zinc-400"><span>排行按当前筛选范围重新统计，实时任务不受时间范围影响</span><span>同步时间：{operations.generatedAt ? formatDate(operations.generatedAt) : '-'}</span></footer>
          </>
        )}
      </section>
    </div>
  );
}
