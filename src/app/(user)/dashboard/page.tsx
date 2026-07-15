'use client';

import Link from 'next/link';
import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Activity,
  ArrowRight,
  CircleDollarSign,
  ImageIcon,
  KeyRound,
  RefreshCw,
  ShieldCheck,
  WalletCards,
} from 'lucide-react';
import { PageHeader } from '@/components/common/PageHeader';
import { StatBlock } from '@/components/common/StatBlock';
import {
  APIError,
  getSession,
  portalApi,
  refreshSession,
  type APIKey,
  type PortalUser,
  type Subscription,
  type UsageLog,
} from '@/lib/portal-api';
import { formatCNY, formatDate } from '@/lib/common/utils';

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : '数据加载失败';
}

function scrubKey(key: APIKey): APIKey {
  return { ...key, key: undefined, keyPlain: undefined };
}

function usageStatus(status: string): { label: string; className: string } {
  switch (status.toLowerCase()) {
    case 'success':
    case 'succeeded':
      return { label: '成功', className: 'success' };
    case 'failed':
      return { label: '失败', className: 'failed' };
    case 'processing':
      return { label: '处理中', className: 'processing' };
    default:
      return { label: '排队中', className: 'queued' };
  }
}

export default function DashboardPage() {
  const [user, setUser] = useState<PortalUser | null>(null);
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [logs, setLogs] = useState<UsageLog[]>([]);
  const [subscription, setSubscription] = useState<Subscription | null>(null);
  const [totalCalls, setTotalCalls] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const loadDashboard = useCallback(async () => {
    const current = getSession();
    if (!current) {
      setError('登录状态已失效，请重新登录');
      setLoading(false);
      return;
    }

    setLoading(true);
    setError('');
    const results = await Promise.allSettled([
      refreshSession(current),
      portalApi.listKeys(current),
      portalApi.usage(current, 1, 8),
      portalApi.subscription(current),
    ]);

    const [userResult, keysResult, usageResult, subscriptionResult] = results;
    if (userResult.status === 'fulfilled') setUser(userResult.value);
    else setUser(current);
    if (keysResult.status === 'fulfilled') setKeys((keysResult.value.data || []).map(scrubKey));
    if (usageResult.status === 'fulfilled') {
      setLogs(usageResult.value.data || []);
      setTotalCalls(usageResult.value.pagination?.total || 0);
    }
    if (subscriptionResult.status === 'fulfilled') setSubscription(subscriptionResult.value.data);

    const failure = results.find((result) => result.status === 'rejected');
    if (failure?.status === 'rejected') setError(errorMessage(failure.reason));
    setLoading(false);
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => void loadDashboard(), 0);
    return () => window.clearTimeout(timer);
  }, [loadDashboard]);

  const keyStats = useMemo(() => keys.reduce(
    (summary, key) => ({
      requests: summary.requests + Number(key.requestCount || 0),
      success: summary.success + Number(key.successCount || 0),
      images: summary.images + Number(key.imageCount || 0),
    }),
    { requests: 0, success: 0, images: 0 },
  ), [keys]);

  const activeKeys = keys.filter((key) => key.status === 'active').length;
  const successRate = keyStats.requests > 0
    ? `${((keyStats.success / keyStats.requests) * 100).toFixed(1)}%`
    : '0.0%';
  const remainingQuota = Number(
    subscription?.effectiveQuotaRemaining ?? subscription?.quotaRemaining ?? 0,
  );
  const subscriptionActive = Boolean(subscription?.isPaid && subscription?.status === 'active');

  return (
    <div className="page-stack">
      <PageHeader title="控制台" description="API 接入、额度与调用状态总览">
        <button className="btn" type="button" onClick={() => void loadDashboard()} disabled={loading}>
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          刷新
        </button>
      </PageHeader>

      {error && (
        <div className="notice" role="alert">
          部分数据暂未更新：{error}
        </div>
      )}

      <section className="metric-grid" aria-label="账户汇总">
        <StatBlock
          title="可用余额"
          value={loading && !user ? '--' : formatCNY(Number(user?.credits || 0))}
          subtext="按量调用可用"
          icon={CircleDollarSign}
          color="green"
        />
        <StatBlock
          title="订阅额度"
          value={loading && !subscription ? '--' : subscriptionActive ? remainingQuota.toLocaleString() : '未订阅'}
          subtext={subscriptionActive ? `${subscription?.planName || '订阅套餐'}剩余额度` : '可在计费中心开通'}
          icon={WalletCards}
          color="amber"
        />
        <StatBlock
          title="可用 API Key"
          value={loading ? '--' : activeKeys}
          subtext={`共 ${keys.length} 个 Key`}
          icon={KeyRound}
          color="cyan"
        />
        <StatBlock
          title="累计调用"
          value={loading ? '--' : totalCalls.toLocaleString()}
          subtext={`${keyStats.images.toLocaleString()} 张图片 · 成功率 ${successRate}`}
          icon={Activity}
          color="neutral"
        />
      </section>

      <section className="section-panel">
        <div className="section-head">
          <div>
            <strong>最近调用</strong>
            <small className="ml-2">最近 8 条 API 请求</small>
          </div>
          <Link className="btn" href="/usage">查看全部 <ArrowRight size={13} /></Link>
        </div>
        <div className="data-table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>状态</th>
                <th>接口</th>
                <th>模型</th>
                <th>规格</th>
                <th>图片</th>
                <th>时间</th>
              </tr>
            </thead>
            <tbody>
              {loading && logs.length === 0 ? (
                <tr><td colSpan={6} className="empty-row">正在读取调用记录...</td></tr>
              ) : logs.length === 0 ? (
                <tr><td colSpan={6} className="empty-row">暂无 API 调用记录</td></tr>
              ) : logs.map((log) => {
                const status = usageStatus(log.status);
                return (
                  <tr key={log.id}>
                    <td><span className={`status-pill ${status.className}`}>{status.label}</span></td>
                    <td className="mono truncate-cell" title={log.endpoint}>{log.endpoint || '-'}</td>
                    <td className="truncate-cell" title={log.model}>{log.model || '-'}</td>
                    <td className="mono">{log.size || log.quality || '-'}</td>
                    <td className="mono">{Number(log.imageCount || 0)}</td>
                    <td className="mono">{formatDate(log.createdAt)}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </section>

      <section className="grid gap-3 md:grid-cols-3" aria-label="快捷入口">
        <Link href="/api-keys" className="section-panel flex items-center gap-3 p-4 no-underline hover:border-[#86efac]">
          <span className="billing-icon"><KeyRound size={16} /></span>
          <span className="min-w-0 flex-1"><strong className="block text-xs">管理 API Key</strong><small className="text-[10px] text-zinc-500">创建、启停与轮换凭证</small></span>
          <ArrowRight size={14} className="text-zinc-400" />
        </Link>
        <Link href="/docs" className="section-panel flex items-center gap-3 p-4 no-underline hover:border-[#86efac]">
          <span className="billing-icon bg-blue-50 text-blue-600"><ShieldCheck size={16} /></span>
          <span className="min-w-0 flex-1"><strong className="block text-xs">查看 API 文档</strong><small className="text-[10px] text-zinc-500">OpenAI 兼容接口与示例</small></span>
          <ArrowRight size={14} className="text-zinc-400" />
        </Link>
        <Link href="/billing" className="section-panel flex items-center gap-3 p-4 no-underline hover:border-[#86efac]">
          <span className="billing-icon is-subscription"><ImageIcon size={16} /></span>
          <span className="min-w-0 flex-1"><strong className="block text-xs">补充调用额度</strong><small className="text-[10px] text-zinc-500">余额充值或订阅套餐</small></span>
          <ArrowRight size={14} className="text-zinc-400" />
        </Link>
      </section>
    </div>
  );
}
