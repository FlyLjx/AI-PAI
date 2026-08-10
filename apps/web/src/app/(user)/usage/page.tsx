'use client';

import { useCallback, useEffect, useState, type KeyboardEvent as ReactKeyboardEvent } from 'react';
import {
  CheckCircle2,
  Clipboard,
  Clock3,
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
import {
  APIError,
  getSession,
  portalApi,
  type UsageAnalytics,
  type UsageLog,
  type UsageModelStat,
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
  { key: 'image', label: '返回图片' },
  { key: 'endpoint', label: '接口地址' },
  { key: 'type', label: '请求类型' },
  { key: 'model', label: '渠道 / 模型' },
  { key: 'resolution', label: '分辨率' },
  { key: 'time', label: '请求时间' },
  { key: 'status', label: '状态' },
];

const MODEL_STATUS_HEADERS = [
  { key: 'model', label: '模型' },
  { key: 'resolution', label: '分辨率' },
  { key: 'success-rate', label: '成功率' },
  { key: 'requests', label: '请求数' },
];

const EMPTY_ANALYTICS: UsageAnalytics = {
  models: [],
  hourly: Array.from({ length: 24 }, (_, hour) => ({ hour, total: 0, success: 0, failed: 0 })),
};

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
  return ['failed', 'canceled', 'cancelled'].includes(status)
    && ![429, 502].includes(Number(log.responseStatusCode || 0));
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

function successRate(success: number, failed: number): number {
  const counted = success + failed;
  return counted > 0 ? (success / counted) * 100 : 0;
}

function fallbackAnalytics(logs: UsageLog[]): UsageAnalytics {
  const modelMap = new Map<string, UsageModelStat>();
  const hourly = Array.from({ length: 24 }, (_, hour) => ({ hour, total: 0, success: 0, failed: 0 }));
  logs.forEach((log) => {
    const model = log.model || '未知模型';
    const size = log.size || '-';
    const key = `${model}\u0000${size}`;
    const current = modelMap.get(key) || { model, size, total: 0, success: 0, failed: 0, successRate: 0 };
    current.total += 1;
    if (['success', 'succeeded'].includes(log.status.toLowerCase())) current.success += 1;
    if (isCountedFailure(log)) current.failed += 1;
    current.successRate = successRate(current.success, current.failed);
    modelMap.set(key, current);
    const hour = new Date(log.createdAt).getHours();
    if (hour >= 0 && hour < 24) {
      hourly[hour].total += 1;
      if (['success', 'succeeded'].includes(log.status.toLowerCase())) hourly[hour].success += 1;
      if (isCountedFailure(log)) hourly[hour].failed += 1;
    }
  });
  return { models: Array.from(modelMap.values()).sort((a, b) => b.total - a.total), hourly };
}

function UsageMetricCard({
  label,
  value,
  icon: Icon,
  tone,
  trend,
  trendTone,
}: {
  label: string;
  value: string;
  icon: LucideIcon;
  tone: 'green' | 'mint' | 'danger' | 'balance';
  trend: string;
  trendTone: 'up' | 'down';
}) {
  return (
    <article className={`usage-metric-card ${tone}`}>
      <div className="usage-metric-card-head">
        <span className="usage-metric-icon"><Icon size={24} strokeWidth={1.7} /></span>
        <div className="usage-metric-copy">
          <strong>{label}</strong>
          <div className="usage-metric-value">{value}</div>
          <div className={`usage-metric-trend ${trendTone}`}>
            <span>{trendTone === 'down' ? '↓' : '↑'} {trend}</span>
            <small>较昨日</small>
          </div>
        </div>
      </div>
    </article>
  );
}

function LogThumbnail({ log }: { log: UsageLog }) {
  const [failed, setFailed] = useState(false);
  const src = log.taskId ? `/api/tasks/${log.taskId}/images/0` : '';
  if (!src || failed) {
    return <span className="usage-thumbnail placeholder"><ImageIcon size={18} /></span>;
  }
  // The image URL is an authenticated local task route, so Next Image cannot optimize it safely here.
  // eslint-disable-next-line @next/next/no-img-element
  return <img className="usage-thumbnail" src={src} alt="返回图片缩略图" onError={() => setFailed(true)} />;
}

export default function UsagePage() {
  const [logs, setLogs] = useState<UsageLog[]>([]);
  const [summary, setSummary] = useState<UsageSummary>({ total: 0, counted: 0, success: 0, failed: 0, imageCount: 0 });
  const [analytics, setAnalytics] = useState<UsageAnalytics>(EMPTY_ANALYTICS);
  const [balance, setBalance] = useState<number | null>(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [periodDays, setPeriodDays] = useState(1);
  const [keyword, setKeyword] = useState('');
  const [draftKeyword, setDraftKeyword] = useState('');
  const [status, setStatus] = useState('');
  const [loading, setLoading] = useState(true);
  const [analyticsLoading, setAnalyticsLoading] = useState(true);
  const [error, setError] = useState('');
  const [detailLog, setDetailLog] = useState<UsageLog | null>(null);
  const [detailTab, setDetailTab] = useState<'request' | 'response'>('request');

  const loadUsage = useCallback(async () => {
    const current = getSession();
    if (!current) {
      setError('登录状态已失效，请重新登录');
      setLoading(false);
      return;
    }
    const range = periodRange(periodDays);
    setBalance(Number(current.credits || 0));
    setLoading(true);
    setAnalyticsLoading(true);
    setError('');
    try {
      const summaryRequest = keyword || status
        ? portalApi.usage(current, 1, 1, '', '', range.startDate, range.endDate)
        : null;
      const [response, summaryResponse, analyticsResponse] = await Promise.all([
        portalApi.usage(current, page, pageSize, keyword, status, range.startDate, range.endDate),
        summaryRequest,
        portalApi.usageAnalytics(current, range.startDate, range.endDate).catch(() => null),
      ]);
      const items = response.data || [];
      const responseTotal = response.pagination?.total || 0;
      const fallbackSummary: UsageSummary = {
        total: responseTotal,
        counted: items.filter((log) => ['success', 'succeeded'].includes(log.status.toLowerCase()) || isCountedFailure(log)).length,
        success: items.filter((log) => ['success', 'succeeded'].includes(log.status.toLowerCase())).length,
        failed: items.filter(isCountedFailure).length,
        imageCount: items.reduce((sum, log) => sum + Number(log.imageCount || 0), 0),
      };
      setLogs(items);
      setTotal(responseTotal);
      setSummary(summaryResponse?.summary || response.summary || fallbackSummary);
      setAnalytics(analyticsResponse?.data || fallbackAnalytics(items));
    } catch (loadError) {
      setError(errorMessage(loadError));
    } finally {
      setLoading(false);
      setAnalyticsLoading(false);
    }
  }, [keyword, page, pageSize, periodDays, status]);

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
  const periodLabel = `${periodDays} 天`;
  const modelStats = analytics.models.slice(0, 5);
  const hourly = analytics.hourly.length === 24 ? analytics.hourly : EMPTY_ANALYTICS.hourly;
  const maxHourly = Math.max(1, ...hourly.map((point) => Number(point.total || 0)));
  const rangeText = `近 ${periodLabel} · 共 ${summary.total.toLocaleString()} 次请求`;

  const applyFilters = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setPage(1);
    setKeyword(draftKeyword.trim());
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
                onClick={() => { setPeriodDays(option.days); setPage(1); }}
              >
                {option.label}
              </button>
            ))}
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
                      <td><LogThumbnail log={log} /></td>
                      <td><code className="usage-endpoint">{log.endpoint || '-'}</code></td>
                      <td><span className="usage-request-type">{requestType(log)}</span></td>
                      <td>
                        <div className="usage-model-cell">
                          <strong>{log.model || '-'}</strong>
                          <small>{log.keyName || '默认渠道'}</small>
                        </div>
                      </td>
                      <td><span className="usage-resolution">{log.size || '-'}</span></td>
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
                        <LogThumbnail log={log} />
                        <div className="usage-mobile-item-copy">
                          <strong>{log.model || '-'}</strong>
                          <code>{log.endpoint || '-'}</code>
                        </div>
                        <span className={`usage-status-text ${meta.className}`}><i />{meta.label}</span>
                      </div>
                      <div className="usage-mobile-item-meta">
                        <span>{requestType(log)}</span><span>{log.size || '-'}</span><time>{formatDate(log.createdAt)}</time>
                      </div>
                    </article>
                  );
                }}
              />
            )}
          </section>
        </main>

        <aside className="usage-side">
          <section className="usage-side-card" aria-labelledby="usage-model-title">
            <header>
              <div><h2 id="usage-model-title">接口状态（本账户）</h2><p>成功率仅统计本账户近 {periodLabel} 调用</p></div>
            </header>
            <DataTable
              embedded
              className="usage-model-data-table"
              headers={MODEL_STATUS_HEADERS}
              data={modelStats}
              loading={analyticsLoading}
              loadingState={<div className="usage-side-empty">正在统计...</div>}
              emptyState={<div className="usage-side-empty">暂无数据</div>}
              tableWrapClassName="usage-model-table-wrap"
              tableClassName="usage-model-table"
              renderRow={(item) => (
                <tr key={`${item.model}-${item.size}`}>
                  <td title={item.model}>{item.model || '-'}</td>
                  <td>{item.size || '-'}</td>
                  <td><span className="usage-rate"><i />{Number(item.successRate || successRate(item.success, item.failed)).toFixed(1)}%</span></td>
                  <td>{Number(item.total || 0).toLocaleString()}</td>
                </tr>
              )}
            />
          </section>

          <section className="usage-side-card usage-distribution-card" aria-labelledby="usage-distribution-title">
            <header>
              <div><h2 id="usage-distribution-title">24 小时分布</h2><p>按小时聚合近 {periodLabel} 请求</p></div>
              <span className="usage-chart-legend"><i />请求数</span>
            </header>
            <div className="usage-hourly-list">
              {hourly.map((point) => (
                <div className="usage-hourly-row" key={point.hour}>
                  <span>{String(point.hour).padStart(2, '0')}:00</span>
                  <div className="usage-hourly-track"><i style={{ width: `${Math.max(3, (Number(point.total || 0) / maxHourly) * 100)}%` }} /></div>
                  <strong>{Number(point.total || 0).toLocaleString()}</strong>
                </div>
              ))}
            </div>
          </section>
        </aside>
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
