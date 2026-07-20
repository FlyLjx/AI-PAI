'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Activity,
  CheckCircle2,
  Clock3,
  Gauge,
  ImageIcon,
  LoaderCircle,
  RefreshCw,
  RotateCcw,
  Search,
  XCircle,
  type LucideIcon,
} from 'lucide-react';
import { DataTable } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { AppSelect, type AppSelectOption } from '@/components/common/AppSelect';
import { PageHeader } from '@/components/common/PageHeader';
import { StatBlock } from '@/components/common/StatBlock';
import { APIError, getSession, portalApi, type UsageLog, type UsageSummary } from '@/lib/portal-api';
import { formatDate } from '@/lib/common/utils';

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : '调用记录加载失败';
}

function statusMeta(status: string): { label: string; className: string; icon: LucideIcon } {
  switch (status.toLowerCase()) {
    case 'success':
    case 'succeeded':
      return { label: '成功', className: 'success', icon: CheckCircle2 };
    case 'failed':
      return { label: '失败', className: 'failed', icon: XCircle };
    case 'processing':
      return { label: '处理中', className: 'processing', icon: LoaderCircle };
    default:
      return { label: '排队中', className: 'queued', icon: Clock3 };
  }
}

function durationMeta(log: UsageLog): { label: string; className: string } {
  if (!['success', 'succeeded', 'failed'].includes(log.status.toLowerCase())) {
    return { label: '--', className: '' };
  }
  const seconds = Number(log.durationSeconds);
  if (!Number.isFinite(seconds) || seconds < 0) return { label: '--', className: '' };
  if (seconds <= 65) return { label: `${seconds.toFixed(2)}s`, className: 'active' };
  if (seconds < 120) return { label: `${seconds.toFixed(2)}s`, className: 'processing' };
  return { label: `${seconds.toFixed(2)}s`, className: 'failed' };
}

const STATUS_OPTIONS: readonly AppSelectOption[] = [
  { value: '', label: '全部状态' },
  { value: 'success', label: '成功' },
  { value: 'failed', label: '失败' },
  { value: 'processing', label: '处理中' },
  { value: 'queued', label: '排队中' },
];

const PAGE_SIZE_OPTIONS: readonly AppSelectOption[] = [
  { value: '10', label: '10 条' },
  { value: '20', label: '20 条' },
  { value: '50', label: '50 条' },
];

export default function UsagePage() {
  const [logs, setLogs] = useState<UsageLog[]>([]);
  const [summary, setSummary] = useState<UsageSummary>({ total: 0, success: 0, failed: 0, imageCount: 0 });
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [keyword, setKeyword] = useState('');
  const [draftKeyword, setDraftKeyword] = useState('');
  const [status, setStatus] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const loadUsage = useCallback(async () => {
    const current = getSession();
    if (!current) {
      setError('登录状态已失效，请重新登录');
      setLoading(false);
      return;
    }
    setLoading(true);
    setError('');
    try {
      const response = await portalApi.usage(current, page, pageSize, keyword, status);
      const items = response.data || [];
      const responseTotal = response.pagination?.total || 0;
      setLogs(items);
      setTotal(responseTotal);
      setSummary(response.summary || {
        total: responseTotal,
        success: items.filter((log) => ['success', 'succeeded'].includes(log.status.toLowerCase())).length,
        failed: items.filter((log) => log.status.toLowerCase() === 'failed').length,
        imageCount: items.reduce((sum, log) => sum + Number(log.imageCount || 0), 0),
      });
    } catch (loadError) {
      setError(errorMessage(loadError));
    } finally {
      setLoading(false);
    }
  }, [keyword, page, pageSize, status]);

  useEffect(() => {
    const timer = window.setTimeout(() => void loadUsage(), 0);
    return () => window.clearTimeout(timer);
  }, [loadUsage]);

  const successRate = useMemo(() => summary.total > 0 ? `${((summary.success / summary.total) * 100).toFixed(1)}%` : '0.0%', [summary.success, summary.total]);
  const pending = Math.max(0, summary.total - summary.success - summary.failed);

  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  const applyFilters = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setPage(1);
    setKeyword(draftKeyword.trim());
  };

  const resetFilters = () => {
    setDraftKeyword('');
    setKeyword('');
    setStatus('');
    setPage(1);
  };

  const headers = [
    { key: 'status', label: '状态' },
    { key: 'endpoint', label: '接口' },
    { key: 'key', label: 'API Key' },
    { key: 'model', label: '模型' },
    { key: 'prompt', label: '提示词' },
    { key: 'spec', label: '规格' },
    { key: 'quantity', label: '请求 / 输出' },
    { key: 'chargedCredits', label: '扣费金额' },
    { key: 'durationSeconds', label: '响应时间' },
    { key: 'createdAt', label: '请求时间' },
  ];

  return (
    <div className="page-stack">
      <PageHeader title="用量记录" description="查询 API 请求、输出数量与执行状态">
        <button className="btn" type="button" onClick={() => void loadUsage()} disabled={loading}>
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />刷新
        </button>
      </PageHeader>

      <section className="metric-grid">
        <StatBlock title="请求总数" value={summary.total.toLocaleString()} subtext="当前筛选范围" icon={Activity} color="green" />
        <StatBlock title="成功请求" value={summary.success.toLocaleString()} subtext={`${summary.failed.toLocaleString()} 次失败`} icon={CheckCircle2} color="cyan" />
        <StatBlock title="成功率" value={successRate} subtext={pending > 0 ? `${pending.toLocaleString()} 个尚未完成` : '成功请求 / 全部请求'} icon={Gauge} color="amber" />
        <StatBlock title="输出图片" value={summary.imageCount.toLocaleString()} subtext={`第 ${page} / ${totalPages} 页`} icon={ImageIcon} color="neutral" />
      </section>

      <form className="section-panel section-body" onSubmit={applyFilters}>
        <div className="grid gap-3 md:grid-cols-[minmax(220px,1fr)_160px_120px_auto] md:items-end">
          <div className="field">
            <label htmlFor="usage-keyword">搜索</label>
            <div className="input-leading-icon">
              <Search size={14} aria-hidden="true" />
              <input
                id="usage-keyword"
                value={draftKeyword}
                onChange={(event) => setDraftKeyword(event.target.value)}
                placeholder="接口、模型、提示词或 Key 名称"
              />
            </div>
          </div>
          <div className="field">
            <label htmlFor="usage-status">状态</label>
            <AppSelect id="usage-status" value={status} options={STATUS_OPTIONS} onValueChange={(nextStatus) => { setStatus(nextStatus); setPage(1); }} />
          </div>
          <div className="field">
            <label htmlFor="usage-page-size">每页条数</label>
            <AppSelect id="usage-page-size" value={String(pageSize)} options={PAGE_SIZE_OPTIONS} onValueChange={(nextPageSize) => { setPageSize(Number(nextPageSize)); setPage(1); }} />
          </div>
          <div className="action-row md:justify-end">
            <button className="btn" type="button" onClick={resetFilters}><RotateCcw size={13} />重置</button>
            <button className="btn primary" type="submit"><Search size={13} />查询</button>
          </div>
        </div>
      </form>

      {error && <div className="notice" role="alert">{error}</div>}

      {loading && logs.length === 0 ? (
        <div className="section-panel empty-row">正在读取调用记录...</div>
      ) : (
        <DataTable
          headers={headers}
          data={logs}
          currentPage={page}
          totalPages={totalPages}
          onPageChange={setPage}
          renderRow={(log) => {
            const meta = statusMeta(log.status);
            const duration = durationMeta(log);
            const StatusIcon = meta.icon;
            return (
              <tr key={log.id}>
                <td className="px-4 py-3"><span className={`status-pill gap-1 ${meta.className}`}><StatusIcon size={11} className={meta.className === 'processing' ? 'animate-spin' : ''} />{meta.label}</span></td>
                <td className="px-4 py-3 mono truncate-cell" title={log.endpoint}>{log.endpoint || '-'}</td>
                <td className="px-4 py-3">
                  <strong className="block max-w-[150px] truncate text-[12px]">{log.keyName || '已删除 Key'}</strong>
                  <small className="mono text-[10px] text-zinc-400">{log.keyPrefix || '-'}</small>
                </td>
                <td className="px-4 py-3 truncate-cell" title={log.model}>{log.model || '-'}</td>
                <td className="min-w-[220px] max-w-[300px] px-4 py-3" title={log.prompt || ''}>
                  <p className="truncate text-[12px] text-zinc-600">{log.prompt || '-'}</p>
                </td>
                <td className="px-4 py-3 mono">{log.size || '-'}{log.quality ? ` · ${log.quality}` : ''}</td>
                <td className="px-4 py-3 mono">{Number(log.quantity || 0)} / {Number(log.imageCount || 0)}</td>
                <td className="px-4 py-3 mono text-[#047857]">{Number(log.chargedCredits || 0).toFixed(4)}</td>
                <td className="px-4 py-3"><span className={`status-pill mono min-w-[58px] justify-center ${duration.className}`}>{duration.label}</span></td>
                <td className="px-4 py-3 mono text-zinc-500">{formatDate(log.createdAt)}</td>
              </tr>
            );
          }}
          renderMobileItem={(log) => {
            const meta = statusMeta(log.status);
            const duration = durationMeta(log);
            const StatusIcon = meta.icon;
            return (
              <article key={log.id} className="section-panel p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <strong className="block truncate text-xs">{log.model || '未知模型'}</strong>
                    <code className="mt-1 block truncate text-[10px] text-zinc-500">{log.endpoint || '-'}</code>
                  </div>
                  <span className={`status-pill gap-1 ${meta.className}`}><StatusIcon size={11} className={meta.className === 'processing' ? 'animate-spin' : ''} />{meta.label}</span>
                </div>
                <dl className="mt-3 grid grid-cols-2 gap-x-3 gap-y-2 border-t border-[#edf0ee] pt-3 text-[11px]">
                  <div className="col-span-2">
                    <dt className="text-zinc-400">提示词</dt>
                    <dd className="mt-1 line-clamp-3 whitespace-pre-wrap break-words leading-5 text-[#3f4943]" title={log.prompt || ''}>{log.prompt || '-'}</dd>
                  </div>
                  <div><dt className="text-zinc-400">API Key</dt><dd className="mt-0.5 truncate">{log.keyName || log.keyPrefix || '-'}</dd></div>
                  <div><dt className="text-zinc-400">规格</dt><dd className="mono mt-0.5">{log.size || '-'} · {log.quality || '-'}</dd></div>
                  <div><dt className="text-zinc-400">请求 / 输出</dt><dd className="mono mt-0.5">{Number(log.quantity || 0)} / {Number(log.imageCount || 0)}</dd></div>
                  <div><dt className="text-zinc-400">扣费金额</dt><dd className="mono mt-0.5 text-[#047857]">{Number(log.chargedCredits || 0).toFixed(4)}</dd></div>
                  <div><dt className="text-zinc-400">响应时间</dt><dd className="mt-1"><span className={`status-pill mono min-w-[58px] justify-center ${duration.className}`}>{duration.label}</span></dd></div>
                  <div><dt className="text-zinc-400">时间</dt><dd className="mono mt-0.5">{formatDate(log.createdAt)}</dd></div>
                </dl>
                {log.errorMessage && (
                  <p className="mt-3 flex gap-1.5 rounded-md bg-red-50 p-2 text-[11px] text-red-700">
                    <XCircle size={13} className="shrink-0" />{log.errorMessage}
                  </p>
                )}
              </article>
            );
          }}
          emptyState={(
            <EmptyState
              title="暂无调用记录"
              description={keyword || status ? '当前筛选条件没有匹配结果。' : 'API 请求发起后会在这里显示。'}
              icon={Activity}
            />
          )}
        />
      )}
    </div>
  );
}
