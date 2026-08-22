'use client';

import { useCallback, useEffect, useState, type KeyboardEvent as ReactKeyboardEvent } from 'react';
import {
  CheckCircle2,
  Clipboard,
  Clock3,
  CircleDollarSign,
  Download,
  ImageIcon,
  LoaderCircle,
  RefreshCw,
  Search,
  KeyRound,
  UploadCloud,
  X,
  XCircle,
  type LucideIcon,
} from 'lucide-react';
import { toast } from 'sonner';
import { AppSelect, type AppSelectOption } from '@/components/common/AppSelect';
import { DataTable } from '@/components/common/DataTable';
import { TrendDateRangePicker } from '@/components/dashboard/TrendDateRangePicker';
import {
  APIError,
  getSession,
  portalApi,
  type UsageLog,
  type UsageSummary,
} from '@/lib/portal-api';
import { formatCNY, formatDate } from '@/lib/common/utils';

const PERIOD_OPTIONS = [
  { days: 1, label: '1天' },
  { days: 7, label: '7天' },
  { days: 30, label: '30天' },
] as const;

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

const USAGE_TABLE_HEADERS = [
  { key: 'endpoint', label: '接口地址' },
  { key: 'task-id', label: '任务 ID' },
  { key: 'type', label: '请求类型' },
  { key: 'model', label: '渠道 / 模型' },
  { key: 'resolution', label: '分辨率' },
  { key: 'charge', label: '扣费金额' },
  { key: 'time', label: '请求时间' },
  { key: 'status', label: '状态' },
];

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : '调用记录加载失败';
}

function dateKey(value: Date): string {
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, '0');
  const day = String(value.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function periodRange(days: number): { startDate: string; endDate: string } {
  const end = new Date();
  const start = new Date(end);
  start.setHours(0, 0, 0, 0);
  start.setDate(start.getDate() - Math.max(0, days - 1));
  return { startDate: dateKey(start), endDate: dateKey(end) };
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
  if (!['success', 'succeeded', 'failed'].includes(log.status.toLowerCase())) return { label: '--', className: '' };
  const seconds = Number(log.durationSeconds);
  if (!Number.isFinite(seconds) || seconds < 0) return { label: '--', className: '' };
  if (seconds <= 65) return { label: `${seconds.toFixed(2)}s`, className: 'active' };
  if (seconds < 120) return { label: `${seconds.toFixed(2)}s`, className: 'processing' };
  return { label: `${seconds.toFixed(2)}s`, className: 'failed' };
}

function chargeLabel(log: UsageLog): string {
  const amount = Number(log.chargedCredits);
  if (!Number.isFinite(amount)) return '--';
  return amount.toFixed(4);
}

function requestParameters(log: UsageLog): Record<string, unknown> {
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

function responseParameters(log: UsageLog): Record<string, unknown> {
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
  return { status: normalizedStatus || 'queued', message: '请求尚未完成，暂无返回参数' };
}

function requestType(log: UsageLog): string {
  return String(log.endpoint || '').toLowerCase().includes('/edits') ? '图生图' : '文生图';
}

function UsageMetricCard({
  label,
  value,
  icon: Icon,
  tone,
  trend,
  trendTone,
  trendNote = '较昨日',
}: {
  label: string;
  value: string;
  icon: LucideIcon;
  tone: 'green' | 'mint' | 'danger' | 'balance' | 'charge';
  trend: string;
  trendTone: 'up' | 'down' | 'neutral';
  trendNote?: string;
}) {
  return (
    <article className={`usage-metric-card ${tone}`}>
      <div className="usage-metric-card-head">
        <span className="usage-metric-icon"><Icon size={24} strokeWidth={1.7} /></span>
        <div className="usage-metric-copy">
          <strong>{label}</strong>
          <div className="usage-metric-value">{value}</div>
          <div className={`usage-metric-trend ${trendTone}`}>
            <span>{trendTone === 'down' ? '↓' : trendTone === 'up' ? '↑' : '•'} {trend}</span>
            {trendNote && <small>{trendNote}</small>}
          </div>
        </div>
      </div>
    </article>
  );
}

export default function UsagePage() {
  const [logs, setLogs] = useState<UsageLog[]>([]);
  const [summary, setSummary] = useState<UsageSummary>({ total: 0, counted: 0, success: 0, failed: 0, imageCount: 0, chargedCredits: 0 });
  const [balance, setBalance] = useState<number | null>(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [dateRange, setDateRange] = useState(() => periodRange(1));
  const [draftStartDate, setDraftStartDate] = useState(dateRange.startDate);
  const [draftEndDate, setDraftEndDate] = useState(dateRange.endDate);
  const [periodDays, setPeriodDays] = useState<number | null>(1);
  const [keyword, setKeyword] = useState('');
  const [draftKeyword, setDraftKeyword] = useState('');
  const [status, setStatus] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [detailLog, setDetailLog] = useState<UsageLog | null>(null);
  const [detailTab, setDetailTab] = useState<'request' | 'response'>('request');
  const [exporting, setExporting] = useState(false);

  const loadUsage = useCallback(async () => {
    const current = getSession();
    if (!current) {
      setError('登录状态已失效，请重新登录');
      setLoading(false);
      return;
    }
    setBalance(Number(current.credits || 0));
    setLoading(true);
    setError('');
    try {
      const summaryRequest = keyword || status
        ? portalApi.usage(current, 1, 1, '', '', dateRange.startDate, dateRange.endDate)
        : null;
      const [response, summaryResponse] = await Promise.all([
        portalApi.usage(current, page, pageSize, keyword, status, dateRange.startDate, dateRange.endDate),
        summaryRequest,
      ]);
      const items = response.data || [];
      const responseTotal = response.pagination?.total || 0;
      const fallbackSummary: UsageSummary = {
        total: responseTotal,
        counted: items.filter((log) => ['success', 'succeeded'].includes(log.status.toLowerCase()) || isCountedFailure(log)).length,
        success: items.filter((log) => ['success', 'succeeded'].includes(log.status.toLowerCase())).length,
        failed: items.filter(isCountedFailure).length,
        imageCount: items.reduce((sum, log) => sum + Number(log.imageCount || 0), 0),
        chargedCredits: items.reduce((sum, log) => sum + Number(log.chargedCredits || 0), 0),
      };
      setLogs(items);
      setTotal(responseTotal);
      setSummary(summaryResponse?.summary || response.summary || fallbackSummary);
    } catch (loadError) {
      setError(errorMessage(loadError));
    } finally {
      setLoading(false);
    }
  }, [dateRange.endDate, dateRange.startDate, keyword, page, pageSize, status]);

  useEffect(() => {
    const timer = window.setTimeout(() => void loadUsage(), 0);
    return () => window.clearTimeout(timer);
  }, [loadUsage]);

  useEffect(() => {
    if (!detailLog) return;
    const previousOverflow = document.body.style.overflow;
    const closeOnEscape = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') setDetailLog(null);
    };
    document.body.style.overflow = 'hidden';
    window.addEventListener('keydown', closeOnEscape);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener('keydown', closeOnEscape);
    };
  }, [detailLog]);

  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const rangeText = periodDays
    ? `近 ${periodDays} 天 · 共 ${summary.total.toLocaleString()} 次请求`
    : `${dateRange.startDate} 至 ${dateRange.endDate} · 共 ${summary.total.toLocaleString()} 次请求`;

  const applyFilters = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const today = dateKey(new Date());
    if (!draftStartDate || !draftEndDate) {
      setError('请选择开始日期和结束日期');
      return;
    }
    if (draftStartDate > draftEndDate) {
      setError('开始日期不能晚于结束日期');
      return;
    }
    if (draftEndDate > today) {
      setError('结束日期不能晚于今天');
      return;
    }
    setPage(1);
    setKeyword(draftKeyword.trim());
    setDateRange({ startDate: draftStartDate, endDate: draftEndDate });
    setPeriodDays(null);
    setError('');
  };

  const selectPeriod = (days: number) => {
    const nextRange = periodRange(days);
    setPeriodDays(days);
    setDateRange(nextRange);
    setDraftStartDate(nextRange.startDate);
    setDraftEndDate(nextRange.endDate);
    setPage(1);
  };

  const exportUsage = async () => {
    const current = getSession();
    if (!current) {
      toast.error('登录状态已失效，请重新登录');
      return;
    }
    setExporting(true);
    try {
      const blob = await portalApi.exportUsage(current, keyword, status, dateRange.startDate, dateRange.endDate);
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = `客户端用量-${dateRange.startDate}-${dateRange.endDate}.xlsx`;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      URL.revokeObjectURL(url);
      toast.success('Excel 已开始下载');
    } catch (exportError) {
      toast.error(errorMessage(exportError));
    } finally {
      setExporting(false);
    }
  };

  const openDetail = (log: UsageLog) => {
    setDetailTab('request');
    setDetailLog(log);
  };

  const handleRowKeyDown = (event: ReactKeyboardEvent<HTMLTableRowElement>, log: UsageLog) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      openDetail(log);
    }
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

  return (
    <div className="usage-page page-stack">
      <header className="usage-page-head">
        <div className="usage-page-actions">
          <span className="usage-period-label">统计周期：</span>
          <div className="usage-period-switch" role="group" aria-label="统计周期">
            {PERIOD_OPTIONS.map((option) => (
              <button
                key={option.days}
                type="button"
                className={periodDays === option.days ? 'is-active' : ''}
                aria-pressed={periodDays === option.days}
                onClick={() => selectPeriod(option.days)}
              >
                {option.label}
              </button>
            ))}
          </div>
          <div className="usage-date-range-control usage-custom-period-control">
            <TrendDateRangePicker
              startDate={draftStartDate}
              endDate={draftEndDate}
              maxDate={dateKey(new Date())}
              ariaLabel="自定义统计周期"
              onChange={(nextStartDate, nextEndDate) => {
                setDraftStartDate(nextStartDate);
                setDraftEndDate(nextEndDate);
                setDateRange({ startDate: nextStartDate, endDate: nextEndDate });
                setPeriodDays(null);
                setPage(1);
              }}
            />
          </div>
          <button className="usage-refresh" type="button" onClick={() => void loadUsage()} disabled={loading} aria-label="刷新用量">
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          </button>
        </div>
      </header>

      <div className="usage-layout">
        <main className="usage-main">
          <section className="usage-metric-grid" aria-label="用量统计">
            <UsageMetricCard label="图片数量" value={summary.imageCount.toLocaleString()} icon={ImageIcon} tone="green" trend="12.5%" trendTone="up" />
            <UsageMetricCard label="请求数" value={summary.total.toLocaleString()} icon={UploadCloud} tone="mint" trend="8.3%" trendTone="up" />
            <UsageMetricCard label="失败" value={summary.failed.toLocaleString()} icon={XCircle} tone="danger" trend="5.1%" trendTone="down" />
            <UsageMetricCard label="成功" value={summary.success.toLocaleString()} icon={CheckCircle2} tone="green" trend="9.4%" trendTone="up" />
            <UsageMetricCard label="扣费余额" value={formatCNY(Number(summary.chargedCredits || 0))} icon={CircleDollarSign} tone="charge" trend="当前筛选周期" trendTone="neutral" trendNote="" />
            <UsageMetricCard label="剩余余额" value={balance === null ? '--' : formatCNY(balance)} icon={KeyRound} tone="balance" trend="6.7%" trendTone="up" />
          </section>

          <section className="usage-ledger-card" aria-labelledby="usage-ledger-title">
            <header className="usage-ledger-head">
              <div>
                <h2 id="usage-ledger-title">最近请求 / REQUEST LEDGER</h2>
                <p>{rangeText}</p>
              </div>
              <form className="usage-filter-form" onSubmit={applyFilters}>
                <div className="usage-search-box">
                  <Search size={14} aria-hidden="true" />
                  <input
                    aria-label="搜索调用记录"
                    value={draftKeyword}
                    onChange={(event) => setDraftKeyword(event.target.value)}
                    placeholder="搜索请求 ID / 渠道 ID"
                  />
                </div>
                <AppSelect id="usage-status" value={status} options={STATUS_OPTIONS} onValueChange={(nextStatus) => { setStatus(nextStatus); setPage(1); }} />
                <AppSelect id="usage-page-size" value={String(pageSize)} options={PAGE_SIZE_OPTIONS} onValueChange={(nextPageSize) => { setPageSize(Number(nextPageSize)); setPage(1); }} />
                <button className="usage-filter-submit" type="submit" aria-label="查询"><Search size={14} /></button>
                <button className="usage-export" type="button" onClick={() => void exportUsage()} disabled={exporting || loading}>
                  <Download size={14} />
                  <span>{exporting ? '导出中' : '导出 Excel'}</span>
                </button>
              </form>
            </header>

            {error && <div className="usage-notice" role="alert">{error}</div>}

            {loading && logs.length === 0 ? (
              <div className="usage-empty">正在读取调用记录...</div>
            ) : logs.length === 0 ? (
              <div className="usage-empty"><ImageIcon size={22} /><strong>暂无调用记录</strong><span>{keyword || status ? '当前筛选条件没有匹配结果。' : 'API 请求发起后会在这里显示。'}</span></div>
            ) : (
              <DataTable
                embedded
                className="usage-data-table"
                headers={USAGE_TABLE_HEADERS}
                data={logs}
                currentPage={page}
                totalPages={totalPages}
                totalItems={total}
                onPageChange={setPage}
                paginationDisabled={loading}
                tableWrapClassName="usage-table-wrap"
                tableClassName="usage-table"
                mobileListClassName="usage-mobile-list"
                renderRow={(log) => {
                  const meta = statusMeta(log.status);
                  return (
                    <tr
                      key={log.id}
                      tabIndex={0}
                      onClick={() => openDetail(log)}
                      onKeyDown={(event) => handleRowKeyDown(event, log)}
                      title="点击查看调用明细"
                    >
                      <td><code className="usage-endpoint">{log.endpoint || '-'}</code></td>
                      <td><code className="usage-task-id" title={log.taskId || undefined}>{log.taskId || '-'}</code></td>
                      <td><span className="usage-request-type">{requestType(log)}</span></td>
                      <td>
                        <div className="usage-model-cell">
                          <strong>{log.model || '-'}</strong>
                          <small>{log.keyName || '默认渠道'}</small>
                        </div>
                      </td>
                      <td><span className="usage-resolution">{log.size || '-'}</span></td>
                      <td><span className="usage-charge">{chargeLabel(log)}</span></td>
                      <td><time className="usage-time">{formatDate(log.createdAt)}</time></td>
                      <td><span className={`usage-status-text ${meta.className}`}><i />{meta.label}</span></td>
                    </tr>
                  );
                }}
                renderMobileItem={(log) => {
                  const meta = statusMeta(log.status);
                  return (
                    <article key={log.id} className="usage-mobile-item" onClick={() => openDetail(log)}>
                      <div className="usage-mobile-item-head">
                        <div className="usage-mobile-item-copy">
                          <strong>{log.model || '-'}</strong>
                          <code>{log.endpoint || '-'}</code>
                        </div>
                        <span className={`usage-status-text ${meta.className}`}><i />{meta.label}</span>
                      </div>
                      <div className="usage-mobile-item-meta">
                        <span className="usage-mobile-task-id" title={log.taskId || undefined}>任务 {log.taskId || '-'}</span><span>{requestType(log)}</span><span>{log.size || '-'}</span><span className="usage-mobile-charge">扣费 {chargeLabel(log)}</span><time>{formatDate(log.createdAt)}</time>
                      </div>
                    </article>
                  );
                }}
              />
            )}
          </section>
        </main>

      </div>

      {detailLog && (() => {
        const meta = statusMeta(detailLog.status);
        const duration = durationMeta(detailLog);
        const StatusIcon = meta.icon;
        const parameters = detailTab === 'request' ? requestParameters(detailLog) : responseParameters(detailLog);
        return (
          <div
            className="fixed inset-0 z-50 overflow-y-auto bg-black/40 p-2 sm:grid sm:place-items-center sm:p-4"
            role="presentation"
            onMouseDown={(event) => { if (event.target === event.currentTarget) setDetailLog(null); }}
          >
            <section className="mx-auto flex max-h-[calc(100dvh-16px)] w-full max-w-4xl flex-col overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl sm:max-h-[calc(100dvh-32px)]" role="dialog" aria-modal="true" aria-labelledby="usage-detail-title">
              <header className="flex shrink-0 items-start justify-between gap-4 border-b border-[#DCE4DF] px-4 py-3.5 sm:px-5 sm:py-4">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2"><h2 id="usage-detail-title" className="text-sm font-semibold">API 调用明细</h2><span className={`status-pill gap-1 ${meta.className}`}><StatusIcon size={11} className={meta.className === 'processing' ? 'animate-spin' : ''} />{meta.label}</span></div>
                  <p className="mt-1 truncate font-mono text-[10px] text-zinc-500" title={detailLog.endpoint}>{detailLog.endpoint || '-'}</p>
                </div>
                <button type="button" onClick={() => setDetailLog(null)} title="关闭" aria-label="关闭调用明细" className="btn icon shrink-0 border-0"><X size={16} /></button>
              </header>
              <div className="grid shrink-0 grid-cols-2 border-b border-[#EDF0EE] bg-[#FAFBFA] sm:grid-cols-3 lg:grid-cols-6">
                <div className="border-b border-r border-[#EDF0EE] px-3 py-2.5 sm:border-b-0 sm:px-4 sm:py-3"><span className="block text-[10px] text-zinc-400">请求时间</span><strong className="mt-1 block whitespace-nowrap text-[11px] font-medium">{formatDate(detailLog.createdAt)}</strong></div>
                <div className="border-b border-[#EDF0EE] px-3 py-2.5 sm:border-b-0 sm:border-r sm:px-4 sm:py-3"><span className="block text-[10px] text-zinc-400">响应时间</span><span className={`status-pill mono mt-1 min-w-[58px] justify-center ${duration.className}`}>{duration.label}</span></div>
                <div className="border-b border-r border-[#EDF0EE] px-3 py-2.5 sm:border-b-0 sm:px-4 sm:py-3"><span className="block text-[10px] text-zinc-400">模型</span><strong className="mt-1 block truncate text-[11px] font-medium" title={detailLog.model}>{detailLog.model || '-'}</strong></div>
                <div className="border-b border-[#EDF0EE] px-3 py-2.5 sm:border-b-0 sm:border-r sm:px-4 sm:py-3"><span className="block text-[10px] text-zinc-400">API Key</span><strong className="mt-1 block truncate text-[11px] font-medium" title={detailLog.keyName || detailLog.keyPrefix || ''}>{detailLog.keyName || detailLog.keyPrefix || '-'}</strong></div>
                <div className="border-r border-[#EDF0EE] px-3 py-2.5 sm:px-4 sm:py-3"><span className="block text-[10px] text-zinc-400">请求 / 输出</span><strong className="mono mt-1 block text-[11px] font-medium">{Number(detailLog.quantity || 0)} / {Number(detailLog.imageCount || 0)}</strong></div>
                <div className="px-3 py-2.5 sm:px-4 sm:py-3"><span className="block text-[10px] text-zinc-400">扣费金额</span><strong className="mono mt-1 block text-[11px] font-semibold text-[#047857]">{Number(detailLog.chargedCredits || 0).toFixed(4)}</strong></div>
              </div>
              {detailLog.errorMessage && <div className="shrink-0 border-b border-red-100 bg-red-50 px-4 py-2.5 text-[11px] leading-5 text-red-700 sm:px-5"><span className="flex items-start gap-1.5"><XCircle size={13} className="mt-0.5 shrink-0" />{detailLog.errorMessage}</span></div>}
              <div className="shrink-0 border-b border-[#DCE4DF] px-4 pt-3 sm:px-5" role="tablist" aria-label="调用参数类型">
                <button type="button" role="tab" aria-selected={detailTab === 'request'} onClick={() => setDetailTab('request')} className={`mr-5 border-b-2 px-0.5 pb-2.5 text-xs font-semibold ${detailTab === 'request' ? 'border-[#12B76A] text-[#047857]' : 'border-transparent text-zinc-500 hover:text-zinc-800'}`}>请求参数</button>
                <button type="button" role="tab" aria-selected={detailTab === 'response'} onClick={() => setDetailTab('response')} className={`border-b-2 px-0.5 pb-2.5 text-xs font-semibold ${detailTab === 'response' ? 'border-[#12B76A] text-[#047857]' : 'border-transparent text-zinc-500 hover:text-zinc-800'}`}>返回参数</button>
              </div>
              <div className="min-h-0 flex-1 overflow-auto p-3 sm:p-5"><pre className="min-h-[220px] overflow-auto rounded-md border border-[#26332C] bg-[#111814] p-3 font-mono text-[11px] leading-5 text-[#DDF5E6] sm:p-4">{JSON.stringify(parameters, null, 2)}</pre></div>
              <footer className="flex shrink-0 items-center justify-between gap-3 border-t border-[#DCE4DF] bg-[#F6F8F6] px-4 py-3 sm:px-5"><code className="hidden min-w-0 truncate text-[9px] text-zinc-400 sm:block" title={detailLog.id}>{detailLog.id}</code><button type="button" onClick={() => void copyDetailParameters()} className="btn ml-auto border-[#BDE8CC] text-[#047857]"><Clipboard size={14} />复制 JSON</button></footer>
            </section>
          </div>
        );
      })()}
    </div>
  );
}
