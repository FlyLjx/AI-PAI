'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { Activity, Check, CircleStop, Clipboard, Eye, Gauge, Image as ImageIcon, KeyRound, Loader2, RefreshCw, Trash2, X } from 'lucide-react';
import { toast } from 'sonner';
import { AdminMetricCard } from '@/components/common/AdminMetricCard';
import { AppSelect } from '@/components/common/AppSelect';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { DataTable, type SortState } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { StatusBadge } from '@/components/common/StatusBadge';
import { type APIKey, type APIKeyBillingMode, type DynamicConcurrencyConfig, type UsageLog, type UsageSummary, portalApi } from '@/lib/admin-api';
import { formatDate } from '@/lib/common/utils';

type DetailedUsageLog = UsageLog & {
  apiKeyId?: string;
};

const logPageSize = 30;
const keyPageSize = 20;
const searchDebounceMs = 450;
const KEY_STATUS_OPTIONS = [
  { value: 'all', label: '全部状态' },
  { value: 'active', label: '已启用' },
  { value: 'disabled', label: '已禁用' },
] as const;
const LOG_STATUS_OPTIONS = [
  { value: 'all', label: '全部状态' },
  { value: 'queued', label: '排队中' },
  { value: 'processing', label: '处理中' },
  { value: 'success', label: '成功' },
  { value: 'failed', label: '失败' },
  { value: 'canceled', label: '已取消' },
] as const;

const defaultDynamicConcurrency: DynamicConcurrencyConfig = {
  enabled: true,
  windowValue: 1,
  windowUnit: 'hour',
  requestStep: 50,
  increment: 5,
};

function dynamicWindowLabel(config: DynamicConcurrencyConfig) {
  return `${config.windowValue} ${config.windowUnit === 'minute' ? '分钟' : '小时'}`;
}

function dynamicConcurrencySummary(key: APIKey, config: DynamicConcurrencyConfig) {
  if (!config.enabled) return '动态扩容已关闭';
  const requestCount = Number(key.windowRequestCount ?? key.hourlyRequestCount ?? 0);
  const bonus = Number(key.dynamicConcurrencyBonus || 0);
  return `${dynamicWindowLabel(config)} ${requestCount} 次${bonus > 0 ? ` · +${bonus}` : ''}`;
}

function logStatus(status: string) {
  if (status === 'success' || status === 'succeeded') return { label: '成功', badge: 'succeeded' as const };
  if (status === 'failed') return { label: '失败', badge: 'failed' as const };
  if (status === 'canceled' || status === 'cancelled') return { label: '已取消', badge: 'canceled' as const };
  if (status === 'processing') return { label: '处理中', badge: 'processing' as const };
  return { label: '排队中', badge: 'queued' as const };
}

function generationDurationMeta(log: UsageLog) {
  if (!['success', 'succeeded', 'failed'].includes(log.status.toLowerCase())) {
    return { label: '--', className: 'border-zinc-200 bg-zinc-50 text-zinc-500' };
  }
  const seconds = Number(log.durationSeconds);
  if (!Number.isFinite(seconds) || seconds < 0) {
    return { label: '--', className: 'border-zinc-200 bg-zinc-50 text-zinc-500' };
  }
  if (seconds <= 65) return { label: `${seconds.toFixed(2)}s`, className: 'border-emerald-200 bg-emerald-50 text-emerald-700' };
  if (seconds < 120) return { label: `${seconds.toFixed(2)}s`, className: 'border-amber-200 bg-amber-50 text-amber-800' };
  return { label: `${seconds.toFixed(2)}s`, className: 'border-red-200 bg-red-50 text-red-700' };
}

function GenerationDurationBadge({ log }: { log: UsageLog }) {
  const meta = generationDurationMeta(log);
  return <span className={`inline-flex h-6 min-w-[58px] items-center justify-center whitespace-nowrap rounded border px-2 font-mono text-[10px] font-semibold ${meta.className}`}>{meta.label}</span>;
}

function billingModeMeta(mode?: APIKeyBillingMode | null) {
  if (mode === 'subscription') return { label: '订阅额度', className: 'border-amber-200 bg-amber-50 text-amber-800' };
  if (mode === 'balance') return { label: '账户余额', className: 'border-blue-200 bg-blue-50 text-blue-700' };
  return { label: '自动兼容', className: 'border-zinc-200 bg-zinc-50 text-zinc-600' };
}

function BillingModeBadge({ mode }: { mode?: APIKeyBillingMode | null }) {
  const meta = billingModeMeta(mode);
  return <span className={`inline-flex h-6 items-center whitespace-nowrap rounded border px-2 text-[11px] font-semibold ${meta.className}`}>{meta.label}</span>;
}

function requestParameters(log: DetailedUsageLog): Record<string, unknown> {
  if (log.requestParameters && Object.keys(log.requestParameters).length > 0) return log.requestParameters;
  return {
    model: log.model,
    prompt: log.prompt || '',
    size: log.size,
    quality: log.quality,
    n: log.quantity,
    response_format: log.responseFormat || 'url',
  };
}

function wantsBase64Response(format?: string | null): boolean {
  return ['b64_json', 'base64', 'b64'].includes(String(format || '').trim().toLowerCase());
}

function isCountedFailure(log: UsageLog): boolean {
  const status = log.status.toLowerCase();
  return ['failed', 'canceled', 'cancelled'].includes(status);
}

function isCountedRequest(log: UsageLog): boolean {
  return ['success', 'succeeded'].includes(log.status.toLowerCase()) || isCountedFailure(log);
}

function responseParameters(log: DetailedUsageLog): Record<string, unknown> {
  if (log.responseParameters && Object.keys(log.responseParameters).length > 0) return log.responseParameters;
  const normalizedStatus = log.status.toLowerCase();
  if (['failed', 'canceled', 'cancelled'].includes(normalizedStatus)) {
    return {
      error: {
        message: log.errorMessage || (normalizedStatus === 'failed' ? '图片生成失败' : '任务已取消'),
        type: log.errorMessage?.includes('用户余额不足') ? 'insufficient_quota' : 'api_error',
        param: null,
        code: log.errorCode || null,
      },
    };
  }
  if (['success', 'succeeded'].includes(normalizedStatus) && log.taskId) {
    const imageCount = Math.max(0, Number(log.imageCount || log.quantity || 0));
    const finishedAt = Date.parse(log.finishedAt || log.createdAt);
    const asBase64 = wantsBase64Response(log.responseFormat);
    return {
      created: Number.isFinite(finishedAt) ? Math.floor(finishedAt / 1000) : 0,
      data: Array.from({ length: imageCount }, (_, index) => (
        asBase64
          ? { b64_json: '[base64 image data omitted from logs]' }
          : { url: `/api/tasks/${log.taskId}/images/${index}` }
      )),
    };
  }
  return {
    status: normalizedStatus || 'queued',
    message: '请求尚未完成，暂无返回参数',
  };
}

function isAbortError(error: unknown) {
  return error instanceof Error && error.name === 'AbortError';
}

export default function AdminAPIAccessPage() {
  const [tab, setTab] = useState<'keys' | 'logs'>('keys');
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [stats, setStats] = useState<Record<string, number>>({});
  const [dynamicConcurrency, setDynamicConcurrency] = useState<DynamicConcurrencyConfig>(defaultDynamicConcurrency);
  const [logs, setLogs] = useState<DetailedUsageLog[]>([]);
  const [keysLoading, setKeysLoading] = useState(true);
  const [logsLoading, setLogsLoading] = useState(true);
  const [error, setError] = useState('');
  const [keySearch, setKeySearch] = useState('');
  const [keyStatus, setKeyStatus] = useState('all');
  const [keyPage, setKeyPage] = useState(1);
  const [keyTotal, setKeyTotal] = useState(0);
  const [keySort, setKeySort] = useState<SortState>({ key: 'lastUsedAt', direction: 'desc' });
  const [logSearch, setLogSearch] = useState('');
  const [logStatusFilter, setLogStatusFilter] = useState('all');
  const [logPage, setLogPage] = useState(1);
  const [logTotal, setLogTotal] = useState(0);
  const [logSort, setLogSort] = useState<SortState>({ key: 'createdAt', direction: 'desc' });
  const [logSummary, setLogSummary] = useState<UsageSummary>({ total: 0, counted: 0, success: 0, failed: 0, imageCount: 0 });
  const [concurrencyDraft, setConcurrencyDraft] = useState<Record<string, number>>({});
  const [actionId, setActionId] = useState('');
  const [deleteCandidate, setDeleteCandidate] = useState<APIKey | null>(null);
  const [cancelCandidate, setCancelCandidate] = useState<DetailedUsageLog | null>(null);
  const [cancelingTaskId, setCancelingTaskId] = useState('');
  const [detailLog, setDetailLog] = useState<DetailedUsageLog | null>(null);
  const [detailTab, setDetailTab] = useState<'request' | 'response'>('request');
  const keyRequestSequence = useRef(0);
  const logRequestSequence = useRef(0);
  const loadedLogPage = useRef(1);
  const keyRequestController = useRef<AbortController | null>(null);
  const logRequestController = useRef<AbortController | null>(null);

  const loadKeys = useCallback(async (page = 1) => {
    const requestSequence = ++keyRequestSequence.current;
    keyRequestController.current?.abort();
    const controller = new AbortController();
    keyRequestController.current = controller;
    setKeysLoading(true);
    setError('');
    try {
      const response = await portalApi.adminKeys({
        page,
        pageSize: keyPageSize,
        keyword: keySearch.trim() || undefined,
        status: keyStatus === 'all' ? undefined : keyStatus,
        sortBy: keySort.key,
        sortOrder: keySort.direction,
      }, controller.signal);
      if (requestSequence !== keyRequestSequence.current) return;
      setKeys(response.data.items || []);
      setStats(response.data.stats || {});
      setDynamicConcurrency(response.data.dynamicConcurrency || defaultDynamicConcurrency);
      setConcurrencyDraft(Object.fromEntries((response.data.items || []).map((key) => [key.id, Number(key.baseConcurrencyLimit || key.concurrencyLimit || 10)])));
      setKeyTotal(response.pagination?.total ?? response.data.items?.length ?? 0);
      setKeyPage(page);
    } catch (requestError) {
      if (isAbortError(requestError)) return;
      if (requestSequence !== keyRequestSequence.current) return;
      setError(requestError instanceof Error ? requestError.message : 'API Key 加载失败');
    } finally {
      if (requestSequence === keyRequestSequence.current) setKeysLoading(false);
    }
  }, [keySearch, keySort.direction, keySort.key, keyStatus]);

  const loadLogs = useCallback(async (page = 1) => {
    const requestSequence = ++logRequestSequence.current;
    logRequestController.current?.abort();
    const controller = new AbortController();
    logRequestController.current = controller;
    setLogPage(page);
    setLogsLoading(true);
    setError('');
    try {
      const response = await portalApi.adminUsage({
        page,
        pageSize: logPageSize,
        keyword: logSearch.trim() || undefined,
        status: logStatusFilter === 'all' ? undefined : logStatusFilter,
        sortBy: logSort.key,
        sortOrder: logSort.direction,
      }, controller.signal);
      if (requestSequence !== logRequestSequence.current) return;
      setLogs(response.data as DetailedUsageLog[]);
      const responseTotal = response.pagination?.total ?? response.data.length;
      setLogTotal(responseTotal);
      setLogSummary(response.summary || {
        total: responseTotal,
        counted: response.data.filter(isCountedRequest).length,
        success: response.data.filter((log) => ['success', 'succeeded'].includes(log.status.toLowerCase())).length,
        failed: response.data.filter(isCountedFailure).length,
        imageCount: response.data.reduce((total, log) => total + Number(log.imageCount || 0), 0),
      });
      const resolvedPage = response.pagination?.page ?? page;
      loadedLogPage.current = resolvedPage;
      setLogPage(resolvedPage);
    } catch (requestError) {
      if (isAbortError(requestError)) return;
      if (requestSequence !== logRequestSequence.current) return;
      setLogPage(loadedLogPage.current);
      setError(requestError instanceof Error ? requestError.message : 'API 调用日志加载失败');
    } finally {
      if (requestSequence === logRequestSequence.current) setLogsLoading(false);
    }
  }, [logSearch, logSort.direction, logSort.key, logStatusFilter]);

  const refreshAll = useCallback(async () => {
    if (tab === 'logs') {
      await Promise.all([loadKeys(keyPage), loadLogs(logPage)]);
      return;
    }
    await loadKeys(keyPage);
  }, [keyPage, loadKeys, loadLogs, logPage, tab]);

  useEffect(() => {
    const timer = window.setTimeout(() => void loadKeys(1), searchDebounceMs);
    return () => window.clearTimeout(timer);
  }, [loadKeys]);

  useEffect(() => {
    if (tab !== 'logs') return;
    const timer = window.setTimeout(() => void loadLogs(1), searchDebounceMs);
    return () => window.clearTimeout(timer);
  }, [loadLogs, tab]);

  useEffect(() => () => {
    keyRequestController.current?.abort();
    logRequestController.current?.abort();
  }, []);

  useEffect(() => {
    if (!detailLog) return;
    const previousOverflow = document.body.style.overflow;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setDetailLog(null);
    };
    document.body.style.overflow = 'hidden';
    window.addEventListener('keydown', closeOnEscape);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener('keydown', closeOnEscape);
    };
  }, [detailLog]);

  const keyTotalPages = Math.max(1, Math.ceil(keyTotal / keyPageSize));
  const effectiveKeyPage = Math.min(keyPage, keyTotalPages);
  const countedRequests = Number(logSummary.counted ?? logSummary.success + logSummary.failed);
  const logSuccessRate = countedRequests > 0 ? `${((logSummary.success / countedRequests) * 100).toFixed(1)}%` : '0.0%';

  const toggleKey = async (key: APIKey) => {
    setActionId(key.id);
    try {
      const nextStatus = key.status === 'active' ? 'disabled' : 'active';
      await portalApi.updateAdminKey(key.id, { status: nextStatus });
      toast.success(nextStatus === 'active' ? 'API Key 已启用' : 'API Key 已禁用');
      await loadKeys(keyPage);
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : 'Key 状态更新失败');
    } finally {
      setActionId('');
    }
  };

  const saveConcurrency = async (key: APIKey) => {
    const value = Math.floor(Number(concurrencyDraft[key.id] || 0));
    if (!Number.isSafeInteger(value) || value < 1) return toast.error('请输入大于 0 的整数并发值');
    setActionId(key.id);
    try {
      await portalApi.updateAdminKey(key.id, { concurrencyLimit: value });
      toast.success('基础并发已保存');
      await loadKeys(keyPage);
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '并发上限保存失败');
    } finally {
      setActionId('');
    }
  };

  const deleteKey = async () => {
    if (!deleteCandidate) return;
    try {
      await portalApi.deleteAdminKey(deleteCandidate.id);
      toast.success('API Key 已删除');
      setDeleteCandidate(null);
      await loadKeys(Math.min(keyPage, Math.max(1, Math.ceil(Math.max(0, keyTotal - 1) / keyPageSize))));
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : 'API Key 删除失败');
    }
  };

  const cancelTask = async () => {
    const taskId = cancelCandidate?.taskId;
    if (!taskId) return;
    setCancelingTaskId(taskId);
    try {
      await portalApi.cancelTask(taskId);
      toast.success('任务已取消');
      await loadLogs(logPage);
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '任务取消失败');
    } finally {
      setCancelingTaskId('');
    }
  };

  const canCancelTask = (log: DetailedUsageLog) => Boolean(log.taskId && ['queued', 'pending', 'processing'].includes(log.status));

  const openLogDetail = (log: DetailedUsageLog) => {
    setDetailTab('request');
    setDetailLog(log);
  };

  const copyDetailParameters = async () => {
    if (!detailLog) return;
    const parameters = detailTab === 'request' ? requestParameters(detailLog) : responseParameters(detailLog);
    try {
      await navigator.clipboard.writeText(JSON.stringify(parameters, null, 2));
      toast.success(`${detailTab === 'request' ? '请求' : '返回'}参数已复制`);
    } catch {
      toast.error('复制失败，请手动选择内容');
    }
  };

  const keyActions = (key: APIKey) => (
    <div className="flex items-center justify-end gap-1">
      <button type="button" onClick={() => void toggleKey(key)} disabled={actionId === key.id} className={`rounded px-2 py-1 text-[11px] font-semibold ${key.status === 'active' ? 'text-amber-700 hover:bg-amber-50' : 'text-emerald-700 hover:bg-emerald-50'} disabled:opacity-40`}>{key.status === 'active' ? '禁用' : '启用'}</button>
      <button type="button" onClick={() => setDeleteCandidate(key)} title="删除 Key" className="rounded p-1.5 text-red-600 hover:bg-red-50"><Trash2 className="h-3.5 w-3.5" /></button>
    </div>
  );

  return (
    <div className="space-y-5">
      <PageHeader title="API 调用" description="管理客户 API Key、动态并发和 OpenAI 图片接口调用记录。">
        <button type="button" onClick={() => void refreshAll()} disabled={keysLoading || (tab === 'logs' && logsLoading)} className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[#DCE4DF] bg-white px-3 text-xs font-semibold hover:border-[#12B76A] disabled:opacity-50"><RefreshCw className={`h-4 w-4 ${keysLoading || (tab === 'logs' && logsLoading) ? 'animate-spin' : ''}`} />刷新</button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
        {(tab === 'logs' ? [
          { title: '筛选请求', value: logSummary.total, note: '全部匹配记录', icon: Activity, tone: 'blue' as const },
          { title: '成功请求', value: logSummary.success, note: '全部匹配记录', icon: Check, tone: 'green' as const },
          { title: '失败请求', value: logSummary.failed, note: '全部匹配记录', icon: CircleStop, tone: 'red' as const },
          { title: '成功率', value: logSuccessRate, note: '成功 / 纳入统计', icon: Gauge, tone: 'green' as const },
          { title: '输出图片', value: logSummary.imageCount, note: '全部匹配记录', icon: ImageIcon, tone: 'amber' as const },
        ] : [
          { title: 'Key 总数', value: stats.totalKeys ?? keyTotal, note: '用户创建', icon: KeyRound, tone: 'blue' as const },
          { title: '启用 Key', value: stats.activeKeys ?? 0, note: '可正常调用', icon: Check, tone: 'green' as const },
          { title: '今日请求', value: stats.todayRequests ?? 0, note: 'OpenAI 图片接口', icon: Activity, tone: 'blue' as const },
          { title: '今日成功', value: stats.todaySuccess ?? 0, note: '完成请求', icon: Check, tone: 'green' as const },
          { title: '今日图片', value: stats.todayImageCount ?? 0, note: '返回图片数', icon: ImageIcon, tone: 'amber' as const },
        ]).map((metric) => <AdminMetricCard key={metric.title} title={metric.title} value={typeof metric.value === 'number' ? Number(metric.value || 0).toLocaleString('zh-CN') : metric.value} note={metric.note} icon={metric.icon} tone={metric.tone} />)}
      </div>

      <div className="inline-flex rounded-md border border-[#DCE4DF] bg-[#F6F8F6] p-0.5">
        <button type="button" onClick={() => setTab('keys')} className={`inline-flex h-8 items-center gap-1.5 rounded px-3 text-xs font-semibold ${tab === 'keys' ? 'bg-white text-[#047857] shadow-sm' : 'text-zinc-500'}`}><KeyRound className="h-3.5 w-3.5" />API Key</button>
        <button type="button" onClick={() => setTab('logs')} className={`inline-flex h-8 items-center gap-1.5 rounded px-3 text-xs font-semibold ${tab === 'logs' ? 'bg-white text-[#047857] shadow-sm' : 'text-zinc-500'}`}><Activity className="h-3.5 w-3.5" />调用日志</button>
      </div>

      {error && <div className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700">{error}</div>}

      {tab === 'keys' && (keysLoading ? (
        <div className="grid min-h-[300px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <DataTable
          headers={[
            { key: 'user', label: 'API 客户' },
            { key: 'name', label: 'Key 名称 / 前缀' },
            { key: 'status', label: '状态' },
            { key: 'billingMode', label: '计费方式' },
            { key: 'concurrencyLimit', label: '基础 / 当前并发' },
            { key: 'requestCount', label: '请求统计' },
            { key: 'imageCount', label: '图片数' },
            { key: 'lastUsedAt', label: '最近使用' },
            { key: 'actions', label: '操作', sortable: false, className: 'text-right' },
          ]}
          data={keys}
          searchPlaceholder="搜索用户、Key 名称或前缀"
          searchValue={keySearch}
          onSearchChange={(value) => { setKeySearch(value); setKeyPage(1); }}
          filterControls={<><AppSelect value={keyStatus} options={KEY_STATUS_OPTIONS} onValueChange={(value) => { setKeyStatus(value); setKeyPage(1); }} compact ariaLabel="筛选 API Key 状态" /><span className="text-[11px] text-zinc-400">{keysLoading ? '正在查询...' : `共 ${keyTotal} 条 · 本页 ${keys.length} 条`}</span></>}
          currentPage={effectiveKeyPage}
          totalPages={keyTotalPages}
          totalItems={keyTotal}
          onPageChange={(page) => void loadKeys(page)}
          sortKey={keySort.key}
          sortDirection={keySort.direction}
          onSort={(key, direction) => { setKeySort({ key, direction }); setKeyPage(1); }}
          serverSideSorting
          emptyState={<EmptyState title="暂无 API Key" description="客户在开发者工作台创建 Key 后会显示在这里。" icon={KeyRound} />}
          renderRow={(key) => (
            <tr key={key.id} className="hover:bg-[#FAFBFA]">
              <td className="px-4 py-3"><strong className="block max-w-[190px] truncate font-medium">{key.userEmail || key.userId}</strong><small className="font-mono text-[10px] text-zinc-400">{key.userId}</small></td>
              <td className="px-4 py-3"><strong className="block max-w-[150px] truncate text-[12px] font-medium">{key.name}</strong><code className="text-[10px] text-zinc-400">{key.keyPrefix}••••••</code></td>
              <td className="px-4 py-3"><StatusBadge status={key.status === 'active' ? 'active' : 'disabled'} /></td>
              <td className="px-4 py-3"><BillingModeBadge mode={key.billingMode} /></td>
              <td className="px-4 py-3"><div className="flex items-center gap-1"><input aria-label={`${key.name} 基础并发`} min={1} step={1} type="number" value={concurrencyDraft[key.id] ?? key.baseConcurrencyLimit ?? key.concurrencyLimit} onChange={(event) => setConcurrencyDraft((current) => ({ ...current, [key.id]: Number(event.target.value) }))} className="h-7 w-16 rounded border border-[#DCE4DF] px-2 font-mono text-[11px]" /><button type="button" onClick={() => void saveConcurrency(key)} disabled={actionId === key.id} title="保存基础并发" className="grid h-7 w-7 place-items-center rounded border border-[#86EFAC] bg-[#F0FDF4] text-[#047857] disabled:opacity-40"><Check className="h-3.5 w-3.5" /></button></div><small className="mt-1 block whitespace-nowrap text-[10px] text-zinc-400">当前 <strong className="font-mono text-[#047857]">{key.concurrencyLimit}</strong> · {dynamicConcurrencySummary(key, dynamicConcurrency)}</small></td>
              <td className="px-4 py-3 font-mono text-[11px]"><span className="text-emerald-700">{key.successCount}</span> / <span className="text-red-600">{key.failedCount}</span><small className="mt-0.5 block text-[10px] text-zinc-400">共 {key.requestCount}</small></td>
              <td className="px-4 py-3 font-mono">{Number(key.imageCount || 0).toLocaleString('zh-CN')}</td>
              <td className="whitespace-nowrap px-4 py-3 text-zinc-500">{key.lastUsedAt ? formatDate(key.lastUsedAt) : '未使用'}</td>
              <td className="px-4 py-3">{keyActions(key)}</td>
            </tr>
          )}
          renderMobileItem={(key) => (
            <article key={key.id} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
              <div className="flex items-start justify-between gap-3"><div className="min-w-0"><strong className="block truncate text-sm">{key.name}</strong><small className="font-mono text-[10px] text-zinc-400">{key.keyPrefix}••••••</small></div><div className="flex shrink-0 flex-col items-end gap-1.5"><StatusBadge status={key.status === 'active' ? 'active' : 'disabled'} /><BillingModeBadge mode={key.billingMode} /></div></div>
              <p className="mt-2 truncate text-[11px] text-zinc-500">{key.userEmail || key.userId}</p>
              <div className="mt-3 grid grid-cols-3 divide-x divide-[#EDF0EE] border-y border-[#EDF0EE] py-2 text-center"><span><small className="block text-[10px] text-zinc-400">请求</small><strong className="text-[12px]">{key.requestCount}</strong></span><span><small className="block text-[10px] text-zinc-400">图片</small><strong className="text-[12px]">{key.imageCount}</strong></span><span><small className="block text-[10px] text-zinc-400">当前并发</small><strong className="text-[12px] text-[#047857]">{key.concurrencyLimit}</strong><small className="block text-[9px] text-zinc-400">基础 {key.baseConcurrencyLimit || 10} · {dynamicConcurrencySummary(key, dynamicConcurrency)}</small></span></div>
              <div className="mt-2 flex justify-end">{keyActions(key)}</div>
            </article>
          )}
        />
      ))}

      {tab === 'logs' && (logsLoading && logs.length === 0 ? (
        <div className="grid min-h-[300px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <DataTable
          headers={[
            { key: 'createdAt', label: '请求时间' },
            { key: 'user', label: 'API 客户 / Key' },
            { key: 'endpoint', label: '接口' },
            { key: 'model', label: '模型' },
            { key: 'imageCount', label: '参数 / 图片数' },
            { key: 'chargedCredits', label: '扣费 / 成本' },
            { key: 'durationSeconds', label: '生图时间' },
            { key: 'status', label: '状态' },
            { key: 'error', label: '错误信息', sortable: false },
            { key: 'actions', label: '操作', sortable: false, className: 'text-right' },
          ]}
          data={logs}
          searchPlaceholder="搜索用户、Key、接口、模型或提示词"
          searchValue={logSearch}
          onSearchChange={(value) => { setLogSearch(value); setLogPage(1); }}
          filterControls={<><AppSelect value={logStatusFilter} options={LOG_STATUS_OPTIONS} onValueChange={(value) => { setLogStatusFilter(value); setLogPage(1); }} compact ariaLabel="筛选调用状态" /><span className="text-[11px] text-zinc-400">{logsLoading ? '正在查询...' : `共 ${logTotal} 条 · 本页 ${logs.length} 条`}</span></>}
          currentPage={logPage}
          totalPages={Math.max(1, Math.ceil(logTotal / logPageSize))}
          totalItems={logTotal}
          onPageChange={(page) => void loadLogs(page)}
          paginationLoading={logsLoading}
          sortKey={logSort.key}
          sortDirection={logSort.direction}
          onSort={(key, direction) => { setLogSort({ key, direction }); setLogPage(1); }}
          serverSideSorting
          emptyState={<EmptyState title="暂无 API 调用" description="客户通过 OpenAI 图片接口发起请求后会生成调用日志。" icon={Activity} />}
          renderRow={(log) => { const status = logStatus(log.status); return (
            <tr key={log.id} className="hover:bg-[#FAFBFA]">
              <td className="whitespace-nowrap px-4 py-3 text-zinc-500">{formatDate(log.createdAt)}</td>
              <td className="px-4 py-3"><strong className="block max-w-[170px] truncate font-medium">{log.userEmail || log.userId}</strong><small className="block max-w-[150px] truncate text-[10px] text-zinc-400">{log.keyName || log.keyPrefix || '-'}</small></td>
              <td className="max-w-[150px] truncate px-4 py-3 font-mono text-[11px]">{log.endpoint}</td>
              <td className="max-w-[160px] truncate px-4 py-3">{log.model || '-'}</td>
              <td className="px-4 py-3 text-[11px] text-zinc-500">{log.size || '-'} · {log.quality || '-'} · {log.imageCount || log.quantity || 0} 张</td>
              <td className="whitespace-nowrap px-4 py-3"><span className="block font-mono text-[11px] text-blue-700">{Number(log.chargedCredits || 0).toFixed(4)}</span><small className="mt-0.5 block font-mono text-[9px] text-zinc-400">成本 {Number(log.modelCostCredits || 0).toFixed(4)}</small></td>
              <td className="px-4 py-3"><GenerationDurationBadge log={log} /></td>
              <td className="px-4 py-3"><StatusBadge status={status.badge} customLabel={status.label} /></td>
              <td className="max-w-[220px] truncate px-4 py-3 text-[11px] text-red-600" title={log.errorMessage || log.prompt || ''}>{log.errorMessage || '-'}</td>
              <td className="px-4 py-3 text-right"><div className="flex items-center justify-end gap-1"><button type="button" onClick={() => openLogDetail(log)} title="查看调用详情" className="grid h-7 w-7 place-items-center rounded border border-[#DCE4DF] bg-white text-zinc-600 hover:border-[#86EFAC] hover:text-[#047857]"><Eye className="h-3.5 w-3.5" /></button>{canCancelTask(log) && <button type="button" onClick={() => setCancelCandidate(log)} disabled={cancelingTaskId === log.taskId} className="inline-flex h-7 items-center gap-1 rounded border border-red-200 bg-red-50 px-2 text-[11px] font-semibold text-red-700 hover:bg-red-100 disabled:opacity-40"><CircleStop className="h-3.5 w-3.5" />取消</button>}</div></td>
            </tr>
          ); }}
          renderMobileItem={(log) => { const status = logStatus(log.status); return (
            <article key={log.id} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
              <div className="flex items-start justify-between gap-3"><div className="min-w-0"><strong className="block truncate text-sm">{log.model || log.endpoint}</strong><small className="block truncate text-[10px] text-zinc-400">{log.userEmail || log.userId} · {log.keyName || log.keyPrefix || '-'}</small></div><StatusBadge status={status.badge} customLabel={status.label} /></div>
              <p className="mt-3 truncate rounded bg-[#F6F8F6] px-2 py-1.5 font-mono text-[11px]">{log.endpoint}</p>
              <div className="mt-2 flex items-center justify-between text-[10px] text-zinc-400"><span>{log.size || '-'} · {log.imageCount || log.quantity || 0} 张</span><span>{formatDate(log.createdAt)}</span></div>
              <div className="mt-2 flex items-center justify-between border-t border-[#EEF1EF] pt-2 text-[10px] text-zinc-400"><span>扣费 / 成本</span><span className="font-mono text-zinc-600">{Number(log.chargedCredits || 0).toFixed(4)} / {Number(log.modelCostCredits || 0).toFixed(4)}</span></div>
              <div className="mt-2 flex items-center justify-between border-t border-[#EEF1EF] pt-2 text-[10px] text-zinc-400"><span>生图时间</span><GenerationDurationBadge log={log} /></div>
              {log.errorMessage && <p className="mt-2 line-clamp-2 text-[11px] text-red-600">{log.errorMessage}</p>}
              <div className="mt-2 flex justify-end gap-1.5"><button type="button" onClick={() => openLogDetail(log)} className="inline-flex h-7 items-center gap-1 rounded border border-[#DCE4DF] bg-white px-2 text-[11px] font-semibold text-zinc-600"><Eye className="h-3.5 w-3.5" />调用详情</button>{canCancelTask(log) && <button type="button" onClick={() => setCancelCandidate(log)} disabled={cancelingTaskId === log.taskId} className="inline-flex h-7 items-center gap-1 rounded border border-red-200 bg-red-50 px-2 text-[11px] font-semibold text-red-700 disabled:opacity-40"><CircleStop className="h-3.5 w-3.5" />取消任务</button>}</div>
            </article>
          ); }}
        />
      ))}

      {detailLog && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/40 p-4 sm:grid sm:place-items-center" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setDetailLog(null); }}>
          <section className="w-full max-w-3xl overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl" role="dialog" aria-modal="true" aria-labelledby="api-call-detail-title">
            <header className="flex items-start justify-between gap-4 border-b border-[#DCE4DF] px-5 py-4">
              <div className="min-w-0"><h2 id="api-call-detail-title" className="text-sm font-semibold">API 调用详情</h2><p className="mt-1 truncate text-[11px] text-zinc-500">{detailLog.userEmail || detailLog.userId} · {detailLog.endpoint}</p></div>
              <button type="button" onClick={() => setDetailLog(null)} title="关闭" className="grid h-7 w-7 shrink-0 place-items-center rounded text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button>
            </header>
            <div className="grid grid-cols-2 border-b border-[#EDF0EE] bg-[#FAFBFA] sm:grid-cols-4">
              <div className="border-b border-r border-[#EDF0EE] px-4 py-3 sm:border-b-0"><span className="block text-[10px] text-zinc-400">请求时间</span><strong className="mt-1 block whitespace-nowrap text-[11px] font-medium">{formatDate(detailLog.createdAt)}</strong></div>
              <div className="border-b border-[#EDF0EE] px-4 py-3 sm:border-b-0 sm:border-r"><span className="block text-[10px] text-zinc-400">模型</span><strong className="mt-1 block truncate text-[11px] font-medium" title={detailLog.model}>{detailLog.model || '-'}</strong></div>
              <div className="border-r border-[#EDF0EE] px-4 py-3"><span className="block text-[10px] text-zinc-400">API Key</span><strong className="mt-1 block truncate font-mono text-[11px] font-medium">{detailLog.keyName || detailLog.keyPrefix || '-'}</strong></div>
              <div className="px-4 py-3"><span className="block text-[10px] text-zinc-400">日志 ID</span><strong className="mt-1 block truncate font-mono text-[11px] font-medium" title={detailLog.id}>{detailLog.id}</strong></div>
            </div>
            <div className="border-b border-[#DCE4DF] px-4 pt-3 sm:px-5" role="tablist" aria-label="调用参数类型">
              <button type="button" role="tab" aria-selected={detailTab === 'request'} onClick={() => setDetailTab('request')} className={`mr-5 border-b-2 px-0.5 pb-2.5 text-xs font-semibold ${detailTab === 'request' ? 'border-[#12B76A] text-[#047857]' : 'border-transparent text-zinc-500 hover:text-zinc-800'}`}>请求参数</button>
              <button type="button" role="tab" aria-selected={detailTab === 'response'} onClick={() => setDetailTab('response')} className={`border-b-2 px-0.5 pb-2.5 text-xs font-semibold ${detailTab === 'response' ? 'border-[#12B76A] text-[#047857]' : 'border-transparent text-zinc-500 hover:text-zinc-800'}`}>返回参数</button>
            </div>
            <div className="p-4 sm:p-5"><pre className="min-h-[260px] max-h-[55vh] overflow-auto rounded-md border border-[#DCE4DF] bg-[#111814] p-4 font-mono text-[11px] leading-5 text-[#DDF5E6]">{JSON.stringify(detailTab === 'request' ? requestParameters(detailLog) : responseParameters(detailLog), null, 2)}</pre></div>
            <footer className="flex items-center justify-end gap-2 border-t border-[#DCE4DF] bg-[#F6F8F6] px-5 py-3.5"><button type="button" onClick={() => void copyDetailParameters()} className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[#BDE8CC] bg-white px-3 text-xs font-semibold text-[#047857] hover:bg-[#F0FDF4]"><Clipboard className="h-3.5 w-3.5" />复制 JSON</button></footer>
          </section>
        </div>
      )}

      <ConfirmDialog isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)} onConfirm={() => void deleteKey()} title="删除 API Key" description={`确定删除 ${deleteCandidate?.name || '该 Key'} 吗？删除后对应客户端将立即失去调用权限。`} confirmText="删除" type="danger" />
      <ConfirmDialog isOpen={Boolean(cancelCandidate)} onClose={() => setCancelCandidate(null)} onConfirm={() => void cancelTask()} title="取消生成任务" description="确定取消该任务吗？正在进行的上游请求会被中断，任务不会扣除余额或订阅额度。" confirmText="确认取消" type="warning" />
    </div>
  );
}
