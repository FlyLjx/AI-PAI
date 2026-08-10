'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import {
  BarChart3,
  CheckCircle2,
  Clock3,
  Globe2,
  HeartPulse,
  LoaderCircle,
  RefreshCw,
  Server,
  TrendingUp,
  XCircle,
  type LucideIcon,
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
import {
  APIError,
  portalApi,
  type OpenAIImageStatusSnapshot,
  type StabilityRecentWindow,
  type StabilitySnapshot,
} from '@/lib/portal-api';
import { formatDate } from '@/lib/common/utils';
import styles from './page.module.css';

const REFRESH_INTERVAL_MS = 30_000;
const ERROR_RATE_WARNING_THRESHOLD = 5;
const ERROR_RATE_DANGER_THRESHOLD = 10;
const TREND_COLORS = { success: '#18b969', failed: '#ef6262' } as const;

type MetricTone = 'green' | 'blue' | 'amber';
type StatusTone = 'healthy' | 'warning' | 'danger';

type TaskBreakdown = {
  key: keyof Pick<StabilityRecentWindow, 'success' | 'failed' | 'canceled' | 'rejected' | 'running' | 'other'>;
  label: string;
  color: string;
};

const TASK_BREAKDOWN: readonly TaskBreakdown[] = [
  { key: 'success', label: '成功', color: '#18b969' },
  { key: 'failed', label: '失败', color: '#ef6262' },
  { key: 'canceled', label: '已取消', color: '#9ca3af' },
  { key: 'rejected', label: '已拒绝', color: '#d49a3a' },
  { key: 'running', label: '运行中', color: '#4c8bd8' },
  { key: 'other', label: '其他', color: '#8b83b7' },
];

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : '接口状态加载失败';
}

function percentage(value: number | undefined): string {
  return `${Number(value || 0).toFixed(1)}%`;
}

function durationLabel(value: number | undefined): string {
  const seconds = Math.max(0, Number(value || 0));
  if (seconds < 1) return `${Math.round(seconds * 1000)}ms`;
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes}分${Math.round(seconds % 60)}秒`;
}

function timeLabel(value: string): string {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
}

function MetricCard({
  title,
  value,
  detail,
  icon: Icon,
  tone = 'green',
}: {
  title: string;
  value: string;
  detail: string;
  icon: LucideIcon;
  tone?: MetricTone;
}) {
  return (
    <article className={styles.metricCard}>
      <span className={`${styles.metricIcon} ${styles[tone]}`}><Icon size={27} strokeWidth={1.7} /></span>
      <span className={styles.metricCopy}>
        <span>{title}</span>
        <strong>{value}</strong>
        <small>{detail}</small>
      </span>
    </article>
  );
}

function StatusCard({
  eyebrow,
  title,
  detail,
  icon,
  metaLabel,
  metaValue,
  compactMeta = false,
  tone,
  loading,
}: {
  eyebrow: string;
  title: string;
  detail: string;
  icon: ReactNode;
  metaLabel: string;
  metaValue: string;
  compactMeta?: boolean;
  tone: StatusTone;
  loading?: boolean;
}) {
  const iconTone = tone === 'healthy' ? styles.statusIconHealthy : tone === 'warning' ? styles.statusIconWarning : styles.statusIconDanger;
  const textTone = tone === 'healthy' ? styles.positive : tone === 'warning' ? styles.warning : styles.negative;

  return (
    <article className={styles.statusCard}>
      <h2>{eyebrow}</h2>
      <div className={styles.statusBody}>
        <span className={`${styles.statusIcon} ${iconTone}`}>{icon}</span>
        <div className={styles.statusCopy}>
          <strong className={textTone}>{title}</strong>
          <span>{detail}</span>
        </div>
        <div className={`${styles.statusMeta} ${textTone} ${compactMeta ? styles.statusMetaCompact : ''}`}>
          <span>{metaLabel}</span>
          <strong>{metaValue}</strong>
        </div>
        {loading && <LoaderCircle className={styles.spinner} size={17} aria-label="更新中" />}
      </div>
    </article>
  );
}

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
      if (stabilityResponse.status === 'fulfilled') setSnapshot(stabilityResponse.value.data);
      else throw stabilityResponse.reason;
      if (openAIResponse.status === 'fulfilled') setOpenAIStatus(openAIResponse.value.data);
      else setOpenAIError(errorMessage(openAIResponse.reason));
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
  const hourlyErrorRate = Math.max(0, Number(runtime?.error_rate || 0));
  const overallTone: StatusTone = !reachable
    ? 'danger'
    : hourlyErrorRate >= ERROR_RATE_DANGER_THRESHOLD
      ? 'danger'
      : hourlyErrorRate >= ERROR_RATE_WARNING_THRESHOLD ? 'warning' : 'healthy';
  const overallTitle = !reachable
    ? '接口连接异常'
    : overallTone === 'danger'
      ? '接口一小时错误率较高'
      : overallTone === 'warning' ? '接口运行中，近期存在波动' : '接口运行正常';
  const overallDetail = reachable ? `上游 HTTP ${upstreamCode}` : snapshot?.error || '状态服务暂时不可达';
  const openAITone: StatusTone = openAIError || openAIStatus?.severity === 'critical'
    ? 'danger'
    : openAIStatus?.severity === 'warning' ? 'warning' : 'healthy';

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
  })).filter((item) => item.value > 0 || ['success', 'failed', 'canceled', 'rejected'].includes(item.key));

  return (
    <main className={styles.page}>
      <div className={styles.toolbar}>
        <label className={styles.autoRefresh}>
          <span>自动刷新</span>
          <button
            className={`${styles.switch} ${autoRefresh ? styles.switchOn : ''}`}
            type="button"
            role="switch"
            aria-checked={autoRefresh}
            aria-label="自动刷新"
            onClick={() => setAutoRefresh((enabled) => !enabled)}
          ><span /></button>
        </label>
        <span className={styles.refreshInterval}>30 秒更新</span>
        <span className={styles.lastUpdated}>最后更新： <b>{snapshot?.fetched_at ? formatDate(snapshot.fetched_at) : '-'}</b></span>
        <button className={styles.refreshButton} type="button" onClick={() => void loadStatus()} disabled={loading} aria-label="刷新接口状态">
          <RefreshCw size={18} className={loading ? styles.spinner : ''} />
        </button>
      </div>

      {error && <div className={styles.errorNotice} role="alert"><XCircle size={16} />{error}</div>}

      <section className={styles.statusGrid} aria-label="接口总体状态">
        <StatusCard
          eyebrow="总体接口状态"
          title={loading && !snapshot ? '正在检测接口状态' : overallTitle}
          detail={loading && !snapshot ? '正在连接状态服务...' : overallDetail}
          icon={<HeartPulse size={43} strokeWidth={1.7} />}
          metaLabel={`最近 ${runtime?.window_minutes || 60} 分钟成功率`}
          metaValue={loading && !snapshot ? '--' : percentage(runtime?.success_rate)}
          tone={overallTone}
          loading={loading}
        />
        <StatusCard
          eyebrow="OpenAI Image 状态"
          title={loading && !openAIStatus ? '正在检测图像服务' : openAIError || openAIStatus?.statusLabel || '状态未知'}
          detail={openAIStatus?.summary || '数据来源于 OpenAI 官方状态页'}
          icon={<Globe2 size={43} strokeWidth={1.7} />}
          metaLabel="RSS 更新时间"
          metaValue={openAIStatus?.fetchedAt ? formatDate(openAIStatus.fetchedAt) : '-'}
          compactMeta
          tone={openAITone}
        />
      </section>

      {snapshot?.error && <div className={styles.errorNotice} role="alert"><XCircle size={16} />{snapshot.error}</div>}

      <section className={styles.metricsGrid} aria-label="接口状态关键指标">
        <MetricCard title="上游状态" value={loading && !snapshot ? '--' : reachable ? '在线' : '异常'} detail={upstreamCode ? `HTTP ${upstreamCode}` : '等待状态码'} icon={Server} />
        <MetricCard title="最近任务成功率" value={loading && !snapshot ? '--' : percentage(recent?.success_rate)} detail={`${Number(recent?.success || 0).toLocaleString()} 成功`} icon={TrendingUp} />
        <MetricCard title="成功平均耗时" value={loading && !snapshot ? '--' : durationLabel(recent?.average_success_duration_secs)} detail={`失败平均 ${durationLabel(recent?.average_failure_duration_secs)}`} icon={Clock3} tone="blue" />
        <MetricCard title="近一小时请求" value={loading && !snapshot ? '--' : Number(runtime?.total || 0).toLocaleString()} detail={`${Number(runtime?.totals?.success || 0).toLocaleString()} 成功 · ${Number(runtime?.totals?.failed || 0).toLocaleString()} 失败`} icon={BarChart3} tone="amber" />
      </section>

      <section className={styles.analyticsGrid}>
        <article className={`${styles.panel} ${styles.trendPanel}`} aria-labelledby="stability-trend-title">
          <header className={styles.panelHeader}>
            <h2 id="stability-trend-title">最近 {runtime?.window_minutes || 60} 分钟稳定性</h2>
            <div className={styles.legend}>
              <span><i className={styles.successLine} />成功</span>
              <span><i className={styles.failedLine} />失败</span>
              <strong>成功率：<b>{percentage(runtime?.success_rate)}</b></strong>
            </div>
          </header>
          <div className={styles.chart}>
            {loading && !snapshot ? (
              <div className={styles.empty}><LoaderCircle size={18} className={styles.spinner} />正在读取趋势</div>
            ) : trendData.length === 0 ? (
              <div className={styles.empty}>当前窗口暂无趋势数据</div>
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <LineChart accessibilityLayer data={trendData} margin={{ top: 12, right: 20, left: -12, bottom: 2 }}>
                  <CartesianGrid stroke="#e9eeeb" vertical />
                  <XAxis dataKey="label" tick={{ fill: '#6b746f', fontSize: 10 }} tickLine={false} axisLine={{ stroke: '#dce4df' }} minTickGap={28} />
                  <YAxis allowDecimals={false} tick={{ fill: '#6b746f', fontSize: 10 }} tickLine={false} axisLine={false} width={42} />
                  <Tooltip formatter={(value, name) => [Number(value || 0).toLocaleString(), String(name)]} labelFormatter={(label) => `时间 ${String(label)}`} contentStyle={{ border: '1px solid #dce4df', borderRadius: 6, boxShadow: '0 8px 24px rgba(23,32,27,.08)', fontSize: 11 }} />
                  <Line type="monotone" dataKey="success" name="成功" stroke={TREND_COLORS.success} strokeWidth={2} dot={{ r: 2.2, fill: TREND_COLORS.success, strokeWidth: 0 }} activeDot={{ r: 3.5, fill: '#fff', strokeWidth: 2 }} />
                  <Line type="monotone" dataKey="failed" name="失败" stroke={TREND_COLORS.failed} strokeWidth={1.8} strokeDasharray="5 4" dot={{ r: 2, fill: TREND_COLORS.failed, strokeWidth: 0 }} activeDot={{ r: 3.5, fill: '#fff', strokeWidth: 2 }} />
                </LineChart>
              </ResponsiveContainer>
            )}
          </div>
        </article>

        <div className={styles.sidePanels}>
          <article className={styles.panel} aria-labelledby="task-window-title">
            <header className={styles.panelHeader}>
              <h2 id="task-window-title">最近 {Number(recent?.limit || 60)} 个任务</h2>
              <span className={styles.total}>共 {taskTotal.toLocaleString()}</span>
            </header>
            <div className={styles.taskList}>
              {taskRows.map((item) => {
                const ratio = taskTotal > 0 ? (item.value / taskTotal) * 100 : 0;
                return (
                  <div key={item.key} className={styles.taskRow}>
                    <span>{item.label}</span>
                    <span className={styles.taskTrack}><i style={{ width: `${Math.min(100, ratio)}%`, backgroundColor: item.color }} /></span>
                    <strong>{item.value.toLocaleString()} <small>({ratio.toFixed(1)}%)</small></strong>
                  </div>
                );
              })}
            </div>
            <footer className={styles.taskFooter}>
              <span>合计</span><strong>{taskTotal.toLocaleString()} (100%)</strong>
            </footer>
          </article>

          <article className={`${styles.panel} ${styles.errorPanel}`} aria-labelledby="error-reasons-title">
            <header className={styles.panelHeader}><h2 id="error-reasons-title">错误原因</h2></header>
            {errorReasons.length === 0 ? (
              <div className={styles.errorEmpty}><CheckCircle2 size={31} /><strong>当前无错误</strong><span>继续保持！</span></div>
            ) : (
              <ul className={styles.errorList}>
                {errorReasons.map((reason) => (
                  <li key={reason.label}><span>{reason.label}</span><strong>{Number(reason.value || 0).toLocaleString()}</strong></li>
                ))}
              </ul>
            )}
            <span className={styles.source}>数据源 <code>/health/stability</code></span>
          </article>
        </div>
      </section>
    </main>
  );
}
