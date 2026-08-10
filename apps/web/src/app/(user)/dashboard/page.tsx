'use client';

import Link from 'next/link';
import Image from 'next/image';
import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import {
  ArrowRight,
  CheckCircle2,
  Clipboard,
  Clock3,
  ImageIcon,
  KeyRound,
  UploadCloud,
  XCircle,
  type LucideIcon,
} from 'lucide-react';
import { toast } from 'sonner';
import { DataTable } from '@/components/common/DataTable';
import {
  APIError,
  getSession,
  portalApi,
  type PortalUser,
  type UsageLog,
  type UsageModelStat,
  type UsageSummary,
  type UsageTrendPoint,
} from '@/lib/portal-api';
import { formatCNY, formatDate } from '@/lib/common/utils';

type MetricTone = 'green' | 'red' | 'mint' | 'amber';

type Metric = {
  label: string;
  value: string;
  trend: string;
  trendTone: 'up' | 'down' | 'neutral';
  tone: MetricTone;
  icon: LucideIcon;
};

const RECENT_REQUEST_HEADERS = [
  { key: 'request', label: '返回图片 / 请求接口' },
  { key: 'type', label: '请求类型' },
  { key: 'model', label: '模型' },
  { key: 'status', label: '状态' },
  { key: 'time', label: '时间' },
  { key: 'action', label: '操作' },
];

const TREND_DAYS = 7;

function localDateValue(date: Date): string {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 10);
}

function trendRange() {
  const end = new Date();
  const start = new Date(end);
  start.setDate(end.getDate() - TREND_DAYS + 1);
  return { startDate: localDateValue(start), endDate: localDateValue(end) };
}

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : '数据加载失败';
}

function logStatus(status: string): { label: string; className: string; icon: LucideIcon } {
  switch (status.toLowerCase()) {
    case 'success':
    case 'succeeded':
      return { label: '已完成', className: 'success', icon: CheckCircle2 };
    case 'failed':
      return { label: '失败', className: 'failed', icon: XCircle };
    case 'processing':
      return { label: '处理中', className: 'processing', icon: Clock3 };
    default:
      return { label: '排队中', className: 'queued', icon: Clock3 };
  }
}

function imagePreviewUrl(log: UsageLog): string {
  const status = String(log.status || '').trim().toLowerCase();
  if (!log.taskId || !['success', 'succeeded'].includes(status)) return '';
  return `/api/tasks/${encodeURIComponent(log.taskId)}/thumbnails/0?w=320&q=78`;
}

function requestLabel(log: UsageLog): string {
  const endpoint = String(log.endpoint || '').trim();
  if (endpoint) {
    const normalized = endpoint.replace(/^https?:\/\/[^/]+/i, '');
    return normalized || endpoint;
  }
  return '/v1/images/generations';
}

function requestType(log: UsageLog): string {
  const endpoint = String(log.endpoint || '').toLowerCase();
  if (endpoint.includes('/images/edits') || endpoint.includes('edit') || endpoint.includes('variation')) return '图生图';
  return '文生图';
}

function channelLabel(log: UsageLog): string {
  return String(log.model || '').trim() || log.keyName || log.keyPrefix || '未指定模型';
}

function RequestThumbnail({ log }: { log: UsageLog }) {
  const [failed, setFailed] = useState(false);
  const previewUrl = imagePreviewUrl(log);

  if (!previewUrl || failed) {
    return <span className="image-request-placeholder" aria-label="暂无返回图片"><ImageIcon size={14} /></span>;
  }

  return (
    <Image
      src={previewUrl}
      alt={`${channelLabel(log)} 返回图片缩略图`}
      width={42}
      height={32}
      unoptimized
      onError={() => setFailed(true)}
    />
  );
}

function shortDate(value: string): string {
  const [, month = '', day = ''] = value.split('-');
  return `${month}-${day}`;
}

function displayNumber(value: number, loading: boolean): string {
  return loading ? '--' : Number(value || 0).toLocaleString('zh-CN');
}

type ModelStatusRow = {
  model: string;
  resolution: string;
  successRate: number | null;
};

function displayResolution(value: string): string {
  const normalized = String(value || '').trim().toLowerCase();
  if (normalized === '1k') return '1024×1024';
  if (normalized === '2k') return '2048×2048';
  if (normalized === '4k') return '4096×4096';
  if (/^\d+\s*[x×]\s*\d+$/i.test(normalized)) return normalized.replace(/\s*[x×]\s*/i, '×');
  return value || '1024×1024';
}

function summarizeModelStatus(models: UsageModelStat[]): ModelStatusRow[] {
  return models
    .slice()
    .sort((left, right) => Number(right.total || 0) - Number(left.total || 0) || left.model.localeCompare(right.model))
    .slice(0, 3)
    .map((item) => ({
      model: String(item.model || '').trim() || 'gpt-image-2',
      resolution: displayResolution(item.size),
      successRate: Number.isFinite(Number(item.successRate)) ? Math.round(Number(item.successRate) * 10) / 10 : null,
    }));
}

export default function DashboardPage() {
  const [user, setUser] = useState<PortalUser | null>(null);
  const [logs, setLogs] = useState<UsageLog[]>([]);
  const [modelStats, setModelStats] = useState<UsageModelStat[]>([]);
  const [summary, setSummary] = useState<UsageSummary | null>(null);
  const [requestTotal, setRequestTotal] = useState(0);
  const [trend, setTrend] = useState<UsageTrendPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [trendLoading, setTrendLoading] = useState(true);
  const [error, setError] = useState('');
  const [endpoint] = useState(() => (typeof window !== 'undefined' ? `${window.location.origin}/v1` : 'https://img.example.com/v1'));

  const loadDashboard = useCallback(async () => {
    const current = getSession();
    if (!current) {
      setError('登录状态已失效，请重新登录');
      setLoading(false);
      return;
    }

    const range = trendRange();
    const modelStatusDate = localDateValue(new Date());
    setLoading(true);
    setTrendLoading(true);
    setError('');
    const results = await Promise.allSettled([
      portalApi.usage(current, 1, 5),
      portalApi.usageAnalytics(current, modelStatusDate, modelStatusDate),
      portalApi.usageTrend(current, range.startDate, range.endDate),
    ]);
    const [logsResult, analyticsResult, trendResult] = results;

    setUser(current);
    if (logsResult.status === 'fulfilled') {
      setLogs(logsResult.value.data || []);
      setSummary(logsResult.value.summary || null);
      setRequestTotal(logsResult.value.pagination?.total || 0);
    }
    if (analyticsResult.status === 'fulfilled') setModelStats(analyticsResult.value.data?.models || []);
    if (trendResult.status === 'fulfilled') setTrend(trendResult.value.data || []);
    else setTrend([]);

    const failure = results.find((result) => result.status === 'rejected');
    if (failure?.status === 'rejected') setError(errorMessage(failure.reason));
    setLoading(false);
    setTrendLoading(false);
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => void loadDashboard(), 0);
    return () => window.clearTimeout(timer);
  }, [loadDashboard]);

  const logStats = useMemo(() => logs.reduce(
    (summary, log) => {
      const status = log.status.toLowerCase();
      return {
        success: summary.success + (['success', 'succeeded'].includes(status) ? 1 : 0),
        failed: summary.failed + (['failed', 'canceled', 'cancelled'].includes(status) ? 1 : 0),
        images: summary.images + Number(log.imageCount || log.quantity || 0),
      };
    },
    { success: 0, failed: 0, images: 0 },
  ), [logs]);

  const imageCount = Math.max(Number(summary?.imageCount || 0), logStats.images);
  const requestCount = Math.max(Number(summary?.total || 0), requestTotal);
  const successCount = Math.max(Number(summary?.success || 0), logStats.success);
  const failedCount = Math.max(Number(summary?.failed || 0), logStats.failed);
  const chartData = trend.length > 0
    ? trend.map((point) => ({ ...point, label: shortDate(point.date) }))
    : [];
  const modelStatusRows = useMemo(() => summarizeModelStatus(modelStats), [modelStats]);

  const metrics: Metric[] = [
    { label: '图片数量', value: displayNumber(imageCount, loading), trend: '12.5%', trendTone: 'up', tone: 'green', icon: ImageIcon },
    { label: '请求数', value: displayNumber(requestCount, loading), trend: '8.3%', trendTone: 'up', tone: 'mint', icon: UploadCloud },
    { label: '失败', value: displayNumber(failedCount, loading), trend: '5.1%', trendTone: 'down', tone: 'red', icon: XCircle },
    { label: '成功', value: displayNumber(successCount, loading), trend: '9.4%', trendTone: 'up', tone: 'green', icon: CheckCircle2 },
    { label: '剩余余额', value: loading && !user ? '--' : formatCNY(Number(user?.credits || 0)), trend: '6.7%', trendTone: 'up', tone: 'amber', icon: KeyRound },
  ];

  const copyEndpoint = async () => {
    try {
      await navigator.clipboard.writeText(endpoint);
      toast.success('图片接口地址已复制');
    } catch {
      toast.error('复制失败，请手动选择地址');
    }
  };

  return (
    <div
      className="relay-dashboard"
      style={{ width: '100%', maxWidth: 'none', marginLeft: 0, marginRight: 0 }}
    >
      {error && <div className="dashboard-notice" role="alert">部分数据暂未更新：{error}</div>}

      <section className="endpoint-card" aria-label="图片接口地址">
        <div className="endpoint-label">
          <span><strong>图片接口地址</strong><small>用于接入图片中转服务</small></span>
        </div>
        <div className="endpoint-control">
          <code>{endpoint}</code>
          <button type="button" onClick={() => void copyEndpoint()} aria-label="复制图片接口地址" title="复制地址"><Clipboard size={16} /></button>
        </div>
        <Link href="/api-keys" prefetch={false} className="dashboard-primary-button"><KeyRound size={15} />创建 API Key</Link>
      </section>

      <section className="dashboard-metrics" aria-label="图片中转数据">
        {metrics.map(({ label, value, trend: trendValue, trendTone, tone, icon: Icon }) => (
          <article key={label} className={`metric-card metric-card-${tone}`}>
            <div className="metric-card-head">
              <span className="metric-icon"><Icon size={24} /></span>
              <div className="metric-card-copy">
                <strong>{label}</strong>
                <div className="metric-card-value">{value}</div>
                <div className={`metric-card-trend ${trendTone}`}>
                  <span>{trendTone === 'down' ? '↓' : '↑'} {trendValue}</span><small>较昨日</small>
                </div>
              </div>
            </div>
          </article>
        ))}
      </section>

      <section className="dashboard-middle-grid">
        <article className="dashboard-panel dashboard-trend-panel" aria-labelledby="request-trend-title">
          <header className="dashboard-panel-head">
            <div><strong id="request-trend-title">请求数趋势</strong><small>最近 7 天</small></div>
          </header>
          <div className="dashboard-chart" aria-label="最近七天请求数趋势图">
            {trendLoading ? (
              <div className="dashboard-chart-empty">正在读取趋势数据...</div>
            ) : chartData.length === 0 ? (
              <div className="dashboard-chart-empty">当前暂无请求趋势</div>
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={chartData} margin={{ top: 12, right: 14, left: -14, bottom: 0 }}>
                  <defs>
                    <linearGradient id="requestTrendFill" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="#31B87A" stopOpacity={0.2} />
                      <stop offset="100%" stopColor="#31B87A" stopOpacity={0.02} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid stroke="#edf1ee" strokeDasharray="3 3" vertical={false} />
                  <XAxis dataKey="label" tick={{ fill: '#87918a', fontSize: 10 }} tickLine={false} axisLine={{ stroke: '#dfe7e2' }} />
                  <YAxis allowDecimals={false} tick={{ fill: '#87918a', fontSize: 10 }} tickLine={false} axisLine={false} width={42} />
                  <Tooltip
                    formatter={(value) => [Number(value || 0).toLocaleString('zh-CN'), '请求数']}
                    labelFormatter={(label) => `日期 ${String(label)}`}
                    contentStyle={{ border: '1px solid #dfe7e2', borderRadius: 8, boxShadow: '0 8px 24px rgba(24, 49, 35, .08)', fontSize: 11 }}
                  />
                  <Area type="linear" dataKey="total" stroke="#31B87A" strokeWidth={2.2} fill="url(#requestTrendFill)" dot={{ r: 2.25, fill: '#fff', stroke: '#31B87A', strokeWidth: 1.5 }} activeDot={{ r: 3.5, fill: '#fff', stroke: '#31B87A', strokeWidth: 2 }} />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </div>
        </article>

        <article className="dashboard-panel channel-panel" aria-labelledby="channel-status-title">
          <header className="dashboard-panel-head"><div><strong id="channel-status-title">接口状态（本账户）</strong><small>最近 1 天 · 按模型统计</small></div></header>
          {modelStatusRows.length === 0 ? (
            <div className="channel-empty">最近 1 天暂无图片调用</div>
          ) : (
            <div className="channel-list">
              {modelStatusRows.map(({ model, resolution, successRate }) => (
                <div key={`${model}-${resolution}`} className="channel-row">
                  <span className="channel-model">
                    <strong>{model}</strong>
                    <small>{resolution}</small>
                  </span>
                  <span className={`channel-status ${successRate === null ? 'standby' : successRate >= 95 ? 'online' : 'degraded'}`}>
                    <i aria-hidden="true" />成功率 {successRate === null ? '--' : `${successRate.toFixed(1)}%`}
                  </span>
                </div>
              ))}
            </div>
          )}
        </article>
      </section>

      <section className="dashboard-panel recent-panel" aria-labelledby="recent-image-requests-title">
        <header className="dashboard-panel-head recent-panel-head">
          <div><strong id="recent-image-requests-title">最近图片请求</strong><small>最近 5 条记录</small></div>
          <Link href="/usage" prefetch={false} className="dashboard-text-link">查看全部 <ArrowRight size={13} /></Link>
        </header>
        <DataTable
          embedded
          className="dashboard-recent-data-table"
          headers={RECENT_REQUEST_HEADERS}
          data={logs}
          loading={loading}
          loadingState={<div className="dashboard-empty-row">正在读取图片请求...</div>}
          emptyState={<div className="dashboard-empty-row">暂无图片请求记录</div>}
          tableWrapClassName="recent-table-wrap"
          tableClassName="recent-table"
          renderRow={(log) => {
            const status = logStatus(log.status);
            const StatusIcon = status.icon;
            return (
              <tr key={log.id}>
                <td>
                  <span className="image-request-cell">
                    <RequestThumbnail log={log} />
                    <code title={requestLabel(log)}>{requestLabel(log)}</code>
                  </span>
                </td>
                <td>{requestType(log)}</td>
                <td><span className="request-model" title={channelLabel(log)}>{channelLabel(log)}</span></td>
                <td><span className={`request-status-pill ${status.className}`}><StatusIcon size={12} />{status.label}</span></td>
                <td className="request-time">{formatDate(log.createdAt)}</td>
                <td><Link href={`/usage?log=${encodeURIComponent(log.id)}`} prefetch={false} className="request-view-link">查看</Link></td>
              </tr>
            );
          }}
        />
      </section>
    </div>
  );
}
