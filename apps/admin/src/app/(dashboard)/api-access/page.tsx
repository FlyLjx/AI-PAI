'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Activity, Check, CircleStop, KeyRound, Loader2, RefreshCw, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { AppSelect } from '@/components/common/AppSelect';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { DataTable } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { StatusBadge } from '@/components/common/StatusBadge';
import { type APIKey, type APIKeyBillingMode, type UsageLog, portalApi } from '@/lib/admin-api';
import { formatDate } from '@/lib/common/utils';

type DetailedUsageLog = UsageLog & {
  apiKeyId?: string;
  prompt?: string;
  responseFormat?: string;
};

const logPageSize = 30;
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

function logStatus(status: string) {
  if (status === 'success' || status === 'succeeded') return { label: '成功', badge: 'succeeded' as const };
  if (status === 'failed') return { label: '失败', badge: 'failed' as const };
  if (status === 'canceled' || status === 'cancelled') return { label: '已取消', badge: 'canceled' as const };
  if (status === 'processing') return { label: '处理中', badge: 'processing' as const };
  return { label: '排队中', badge: 'queued' as const };
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

export default function AdminAPIAccessPage() {
  const [tab, setTab] = useState<'keys' | 'logs'>('keys');
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [stats, setStats] = useState<Record<string, number>>({});
  const [logs, setLogs] = useState<DetailedUsageLog[]>([]);
  const [keysLoading, setKeysLoading] = useState(true);
  const [logsLoading, setLogsLoading] = useState(true);
  const [error, setError] = useState('');
  const [keySearch, setKeySearch] = useState('');
  const [keyStatus, setKeyStatus] = useState('all');
  const [logSearch, setLogSearch] = useState('');
  const [logStatusFilter, setLogStatusFilter] = useState('all');
  const [logPage, setLogPage] = useState(1);
  const [logTotal, setLogTotal] = useState(0);
  const [concurrencyDraft, setConcurrencyDraft] = useState<Record<string, number>>({});
  const [actionId, setActionId] = useState('');
  const [deleteCandidate, setDeleteCandidate] = useState<APIKey | null>(null);
  const [cancelCandidate, setCancelCandidate] = useState<DetailedUsageLog | null>(null);
  const [cancelingTaskId, setCancelingTaskId] = useState('');

  const loadKeys = useCallback(async () => {
    setKeysLoading(true);
    setError('');
    try {
      const response = await portalApi.adminKeys();
      setKeys(response.data.items || []);
      setStats(response.data.stats || {});
      setConcurrencyDraft(Object.fromEntries((response.data.items || []).map((key) => [key.id, Number(key.concurrencyLimit || 10)])));
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'API Key 加载失败');
    } finally {
      setKeysLoading(false);
    }
  }, []);

  const loadLogs = useCallback(async (page = 1) => {
    setLogsLoading(true);
    setError('');
    try {
      const response = await portalApi.adminUsage(page);
      setLogs(response.data as DetailedUsageLog[]);
      setLogTotal(response.pagination?.total || response.data.length);
      setLogPage(page);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'API 调用日志加载失败');
    } finally {
      setLogsLoading(false);
    }
  }, []);

  const refreshAll = useCallback(async () => {
    await Promise.all([loadKeys(), loadLogs(logPage)]);
  }, [loadKeys, loadLogs, logPage]);

  useEffect(() => {
    const timer = window.setTimeout(() => void Promise.all([loadKeys(), loadLogs(1)]), 0);
    return () => window.clearTimeout(timer);
  }, [loadKeys, loadLogs]);

  const filteredKeys = useMemo(() => {
    const keyword = keySearch.trim().toLowerCase();
    return keys.filter((key) => {
      const matchesKeyword = !keyword || `${key.userEmail || key.userId} ${key.name} ${key.keyPrefix}`.toLowerCase().includes(keyword);
      return matchesKeyword && (keyStatus === 'all' || key.status === keyStatus);
    });
  }, [keySearch, keyStatus, keys]);

  const filteredLogs = useMemo(() => {
    const keyword = logSearch.trim().toLowerCase();
    return logs.filter((log) => {
      const matchesKeyword = !keyword || `${log.userEmail || log.userId} ${log.keyName || log.keyPrefix || ''} ${log.endpoint} ${log.model} ${log.prompt || ''}`.toLowerCase().includes(keyword);
      return matchesKeyword && (logStatusFilter === 'all' || log.status === logStatusFilter);
    });
  }, [logSearch, logStatusFilter, logs]);

  const toggleKey = async (key: APIKey) => {
    setActionId(key.id);
    try {
      const nextStatus = key.status === 'active' ? 'disabled' : 'active';
      await portalApi.updateAdminKey(key.id, { status: nextStatus });
      toast.success(nextStatus === 'active' ? 'API Key 已启用' : 'API Key 已禁用');
      await loadKeys();
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
      toast.success('并发上限已保存');
      await loadKeys();
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
      await loadKeys();
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

  const keyActions = (key: APIKey) => (
    <div className="flex items-center justify-end gap-1">
      <button type="button" onClick={() => void toggleKey(key)} disabled={actionId === key.id} className={`rounded px-2 py-1 text-[11px] font-semibold ${key.status === 'active' ? 'text-amber-700 hover:bg-amber-50' : 'text-emerald-700 hover:bg-emerald-50'} disabled:opacity-40`}>{key.status === 'active' ? '禁用' : '启用'}</button>
      <button type="button" onClick={() => setDeleteCandidate(key)} title="删除 Key" className="rounded p-1.5 text-red-600 hover:bg-red-50"><Trash2 className="h-3.5 w-3.5" /></button>
    </div>
  );

  return (
    <div className="space-y-5">
      <PageHeader title="API 调用" description="管理客户 API Key、并发上限和 OpenAI 图片接口调用记录。">
        <button type="button" onClick={() => void refreshAll()} disabled={keysLoading || logsLoading} className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[#DCE4DF] bg-white px-3 text-xs font-semibold hover:border-[#12B76A] disabled:opacity-50"><RefreshCw className={`h-4 w-4 ${keysLoading || logsLoading ? 'animate-spin' : ''}`} />刷新</button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
        {[
          ['Key 总数', stats.totalKeys ?? keys.length, '用户创建'],
          ['启用 Key', stats.activeKeys ?? keys.filter((key) => key.status === 'active').length, '可正常调用'],
          ['今日请求', stats.todayRequests ?? 0, 'OpenAI 图片接口'],
          ['今日成功', stats.todaySuccess ?? 0, '完成请求'],
          ['今日图片', stats.todayImageCount ?? 0, '返回图片数'],
        ].map(([label, value, note]) => <div key={String(label)} className="rounded-md border border-[#DCE4DF] bg-white p-3.5"><span className="text-[11px] font-semibold text-zinc-500">{label}</span><strong className="mt-1.5 block text-xl">{Number(value || 0).toLocaleString('zh-CN')}</strong><small className="mt-1 block text-[11px] text-zinc-400">{note}</small></div>)}
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
            { key: 'key', label: 'Key 名称 / 前缀' },
            { key: 'status', label: '状态' },
            { key: 'billingMode', label: '计费方式' },
            { key: 'concurrency', label: '并发上限' },
            { key: 'requests', label: '请求统计' },
            { key: 'images', label: '图片数' },
            { key: 'used', label: '最近使用' },
            { key: 'actions', label: '操作', className: 'text-right' },
          ]}
          data={filteredKeys}
          searchPlaceholder="搜索用户、Key 名称或前缀"
          searchValue={keySearch}
          onSearchChange={setKeySearch}
          filterControls={<><AppSelect value={keyStatus} options={KEY_STATUS_OPTIONS} onValueChange={setKeyStatus} compact ariaLabel="筛选 API Key 状态" /><span className="text-[11px] text-zinc-400">{filteredKeys.length} 条</span></>}
          emptyState={<EmptyState title="暂无 API Key" description="客户在开发者工作台创建 Key 后会显示在这里。" icon={KeyRound} />}
          renderRow={(key) => (
            <tr key={key.id} className="hover:bg-[#FAFBFA]">
              <td className="px-4 py-3"><strong className="block max-w-[190px] truncate font-medium">{key.userEmail || key.userId}</strong><small className="font-mono text-[10px] text-zinc-400">{key.userId}</small></td>
              <td className="px-4 py-3"><strong className="block max-w-[150px] truncate text-[12px] font-medium">{key.name}</strong><code className="text-[10px] text-zinc-400">{key.keyPrefix}••••••</code></td>
              <td className="px-4 py-3"><StatusBadge status={key.status === 'active' ? 'active' : 'disabled'} /></td>
              <td className="px-4 py-3"><BillingModeBadge mode={key.billingMode} /></td>
              <td className="px-4 py-3"><div className="flex items-center gap-1"><input aria-label={`${key.name} 并发上限`} min={1} step={1} type="number" value={concurrencyDraft[key.id] ?? key.concurrencyLimit} onChange={(event) => setConcurrencyDraft((current) => ({ ...current, [key.id]: Number(event.target.value) }))} className="h-7 w-20 rounded border border-[#DCE4DF] px-2 font-mono text-[11px]" /><button type="button" onClick={() => void saveConcurrency(key)} disabled={actionId === key.id} title="保存并发" className="grid h-7 w-7 place-items-center rounded border border-[#86EFAC] bg-[#F0FDF4] text-[#047857] disabled:opacity-40"><Check className="h-3.5 w-3.5" /></button></div></td>
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
              <div className="mt-3 grid grid-cols-3 divide-x divide-[#EDF0EE] border-y border-[#EDF0EE] py-2 text-center"><span><small className="block text-[10px] text-zinc-400">请求</small><strong className="text-[12px]">{key.requestCount}</strong></span><span><small className="block text-[10px] text-zinc-400">图片</small><strong className="text-[12px]">{key.imageCount}</strong></span><span><small className="block text-[10px] text-zinc-400">并发</small><strong className="text-[12px]">{key.concurrencyLimit}</strong></span></div>
              <div className="mt-2 flex justify-end">{keyActions(key)}</div>
            </article>
          )}
        />
      ))}

      {tab === 'logs' && (logsLoading ? (
        <div className="grid min-h-[300px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <DataTable
          headers={[
            { key: 'time', label: '请求时间' },
            { key: 'user', label: 'API 客户 / Key' },
            { key: 'endpoint', label: '接口' },
            { key: 'model', label: '模型' },
            { key: 'params', label: '参数' },
            { key: 'status', label: '状态' },
            { key: 'error', label: '错误信息' },
            { key: 'actions', label: '操作', className: 'text-right' },
          ]}
          data={filteredLogs}
          searchPlaceholder="搜索用户、Key、接口、模型或提示词"
          searchValue={logSearch}
          onSearchChange={setLogSearch}
          filterControls={<><AppSelect value={logStatusFilter} options={LOG_STATUS_OPTIONS} onValueChange={setLogStatusFilter} compact ariaLabel="筛选调用状态" /><span className="text-[11px] text-zinc-400">本页 {filteredLogs.length} 条</span></>}
          currentPage={logPage}
          totalPages={Math.max(1, Math.ceil(logTotal / logPageSize))}
          onPageChange={(page) => void loadLogs(page)}
          emptyState={<EmptyState title="暂无 API 调用" description="客户通过 OpenAI 图片接口发起请求后会生成调用日志。" icon={Activity} />}
          renderRow={(log) => { const status = logStatus(log.status); return (
            <tr key={log.id} className="hover:bg-[#FAFBFA]">
              <td className="whitespace-nowrap px-4 py-3 text-zinc-500">{formatDate(log.createdAt)}</td>
              <td className="px-4 py-3"><strong className="block max-w-[170px] truncate font-medium">{log.userEmail || log.userId}</strong><small className="block max-w-[150px] truncate text-[10px] text-zinc-400">{log.keyName || log.keyPrefix || '-'}</small></td>
              <td className="max-w-[150px] truncate px-4 py-3 font-mono text-[11px]">{log.endpoint}</td>
              <td className="max-w-[160px] truncate px-4 py-3">{log.model || '-'}</td>
              <td className="px-4 py-3 text-[11px] text-zinc-500">{log.size || '-'} · {log.quality || '-'} · {log.imageCount || log.quantity || 0} 张</td>
              <td className="px-4 py-3"><StatusBadge status={status.badge} customLabel={status.label} /></td>
              <td className="max-w-[220px] truncate px-4 py-3 text-[11px] text-red-600" title={log.errorMessage || log.prompt || ''}>{log.errorMessage || '-'}</td>
              <td className="px-4 py-3 text-right">{canCancelTask(log) && <button type="button" onClick={() => setCancelCandidate(log)} disabled={cancelingTaskId === log.taskId} className="inline-flex h-7 items-center gap-1 rounded border border-red-200 bg-red-50 px-2 text-[11px] font-semibold text-red-700 hover:bg-red-100 disabled:opacity-40"><CircleStop className="h-3.5 w-3.5" />取消</button>}</td>
            </tr>
          ); }}
          renderMobileItem={(log) => { const status = logStatus(log.status); return (
            <article key={log.id} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
              <div className="flex items-start justify-between gap-3"><div className="min-w-0"><strong className="block truncate text-sm">{log.model || log.endpoint}</strong><small className="block truncate text-[10px] text-zinc-400">{log.userEmail || log.userId} · {log.keyName || log.keyPrefix || '-'}</small></div><StatusBadge status={status.badge} customLabel={status.label} /></div>
              <p className="mt-3 truncate rounded bg-[#F6F8F6] px-2 py-1.5 font-mono text-[11px]">{log.endpoint}</p>
              <div className="mt-2 flex items-center justify-between text-[10px] text-zinc-400"><span>{log.size || '-'} · {log.imageCount || log.quantity || 0} 张</span><span>{formatDate(log.createdAt)}</span></div>
              {log.errorMessage && <p className="mt-2 line-clamp-2 text-[11px] text-red-600">{log.errorMessage}</p>}
              {canCancelTask(log) && <div className="mt-2 flex justify-end"><button type="button" onClick={() => setCancelCandidate(log)} disabled={cancelingTaskId === log.taskId} className="inline-flex h-7 items-center gap-1 rounded border border-red-200 bg-red-50 px-2 text-[11px] font-semibold text-red-700 disabled:opacity-40"><CircleStop className="h-3.5 w-3.5" />取消任务</button></div>}
            </article>
          ); }}
        />
      ))}

      <ConfirmDialog isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)} onConfirm={() => void deleteKey()} title="删除 API Key" description={`确定删除 ${deleteCandidate?.name || '该 Key'} 吗？删除后对应客户端将立即失去调用权限。`} confirmText="删除" type="danger" />
      <ConfirmDialog isOpen={Boolean(cancelCandidate)} onClose={() => setCancelCandidate(null)} onConfirm={() => void cancelTask()} title="取消生成任务" description="确定取消该任务吗？正在进行的上游请求会被中断，任务不会扣除余额或订阅额度。" confirmText="确认取消" type="warning" />
    </div>
  );
}
