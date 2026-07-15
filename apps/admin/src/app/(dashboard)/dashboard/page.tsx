'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Activity,
  Cable,
  CircleDollarSign,
  Clock3,
  Loader2,
  RefreshCw,
  TriangleAlert,
  Users,
} from 'lucide-react';
import { toast } from 'sonner';
import { PageHeader } from '@/components/common/PageHeader';
import { StatBlock } from '@/components/common/StatBlock';
import { portalApi } from '@/lib/admin-api';
import { formatCNY, formatDate } from '@/lib/common/utils';

type RechargeRow = {
  id: string;
  userEmail?: string;
  userId?: string;
  orderType?: string;
  amount?: number;
  status?: string;
  createdAt?: string;
};

type UsageRow = {
  id: string;
  userEmail?: string;
  userId?: string;
  modelDisplayName?: string;
  modelName?: string;
  modelId?: string;
  quantity?: number;
  status?: string;
  createdAt?: string;
};

type DashboardData = {
  today?: {
    users?: number;
    orders?: number;
    paidAmount?: number;
    tasks?: number;
    runningTasks?: number;
    failedTasks?: number;
  };
  users?: { total?: number; active?: number };
  orders?: { all?: number; paid?: number; pending?: number; failed?: number; closed?: number };
  taskStats?: {
    total?: number;
    queued?: number;
    pending?: number;
    processing?: number;
    success?: number;
    failed?: number;
    canceled?: number;
    totalImages?: number;
  };
  pending?: { pendingOrders?: number; runningTasks?: number; recentFailedTasks?: number };
  system?: {
    api?: string;
    database?: string;
    activeProviders?: number;
    disabledProviders?: number;
    activeModels?: number;
    disabledModels?: number;
    lastTaskAt?: string | null;
  };
  recentOrders?: RechargeRow[];
  recentTasks?: UsageRow[];
};

function statusView(status = '') {
  if (status === 'paid' || status === 'success') return { label: '成功', className: 'border-emerald-200 bg-emerald-50 text-emerald-700' };
  if (status === 'pending' || status === 'queued' || status === 'processing') return { label: '处理中', className: 'border-amber-200 bg-amber-50 text-amber-700' };
  if (status === 'failed' || status === 'canceled') return { label: '失败', className: 'border-red-200 bg-red-50 text-red-700' };
  return { label: status || '未知', className: 'border-zinc-200 bg-zinc-50 text-zinc-600' };
}

export default function AdminDashboardPage() {
  const [data, setData] = useState<DashboardData>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [lastUpdated, setLastUpdated] = useState('');

  const load = useCallback(async (quiet = false) => {
    if (!quiet) setLoading(true);
    setError('');
    try {
      const response = await portalApi.dashboard();
      setData(response.data as DashboardData);
      setLastUpdated(new Date().toISOString());
    } catch (requestError) {
      const message = requestError instanceof Error ? requestError.message : '经营数据加载失败';
      setError(message);
      if (quiet) toast.error(message);
    } finally {
      if (!quiet) setLoading(false);
    }
  }, []);

  useEffect(() => {
    const firstLoad = window.setTimeout(() => void load(), 0);
    const timer = window.setInterval(() => {
      if (!document.hidden) void load(true);
    }, 30_000);
    return () => {
      window.clearTimeout(firstLoad);
      window.clearInterval(timer);
    };
  }, [load]);

  const stats = data.taskStats || {};
  const successful = Number(stats.success || 0);
  const failed = Number(stats.failed || 0) + Number(stats.canceled || 0);
  const completed = successful + failed;
  const successRate = completed ? Math.round((successful / completed) * 100) : 100;
  const pendingCount = Number(data.pending?.runningTasks || 0) + Number(data.pending?.pendingOrders || 0);
  const recentOrders = data.recentOrders || [];
  const recentTasks = data.recentTasks || [];

  const attention = useMemo(() => [
    { label: '待支付充值单', value: Number(data.pending?.pendingOrders || 0), note: '等待支付结果同步', tone: 'amber' },
    { label: '运行中 API 请求', value: Number(data.pending?.runningTasks || 0), note: '排队与上游处理中', tone: 'blue' },
    { label: '24 小时失败', value: Number(data.pending?.recentFailedTasks || 0), note: '建议检查上游和密钥', tone: 'red' },
  ], [data.pending]);

  return (
    <div className="space-y-5">
      <PageHeader title="经营概览" description="API 中转业务、订阅收入和上游运行状态。">
        <button
          type="button"
          onClick={() => void load()}
          disabled={loading}
          className="inline-flex h-8 items-center gap-2 rounded-md border border-[#DCE4DF] bg-white px-3 text-xs font-semibold text-[#17201B] hover:border-[#12B76A] disabled:opacity-50"
        >
          <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
          刷新
        </button>
      </PageHeader>

      {error && !loading && (
        <div className="flex items-center justify-between gap-3 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700">
          <span className="flex items-center gap-2"><TriangleAlert className="h-4 w-4" />{error}</span>
          <button type="button" onClick={() => void load()} className="font-semibold underline">重试</button>
        </div>
      )}

      {loading ? (
        <div className="grid min-h-[320px] place-items-center rounded-md border border-[#DCE4DF] bg-white">
          <Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" />
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <StatBlock title="今日实收" value={formatCNY(Number(data.today?.paidAmount || 0))} subtext={`${Number(data.today?.orders || 0)} 笔充值/订阅订单`} color="green" icon={CircleDollarSign} />
            <StatBlock title="累计 API 请求" value={Number(stats.total || 0).toLocaleString('zh-CN')} subtext={`今日 ${Number(data.today?.tasks || 0).toLocaleString('zh-CN')} 次`} color="cyan" icon={Activity} />
            <StatBlock title="请求成功率" value={`${successRate}%`} subtext={`累计返回 ${Number(stats.totalImages || 0).toLocaleString('zh-CN')} 张图片`} color={successRate >= 95 ? 'green' : 'amber'} icon={Cable} />
            <StatBlock title="API 客户" value={Number(data.users?.total || 0).toLocaleString('zh-CN')} subtext={`启用 ${Number(data.users?.active || 0)}，今日新增 ${Number(data.today?.users || 0)}`} color="neutral" icon={Users} />
          </div>

          <div className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1.4fr)_minmax(300px,.6fr)]">
            <section className="rounded-md border border-[#DCE4DF] bg-white">
              <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[#DCE4DF] px-4 py-3">
                <div>
                  <h2 className="text-sm font-semibold text-[#17201B]">今日运行</h2>
                  <p className="mt-0.5 text-[11px] text-zinc-500">从本地时区 00:00 开始统计</p>
                </div>
                <span className={`rounded border px-2 py-1 text-[10px] font-semibold ${pendingCount ? 'border-amber-200 bg-amber-50 text-amber-700' : 'border-emerald-200 bg-emerald-50 text-emerald-700'}`}>
                  {pendingCount ? `${pendingCount} 项处理中` : '队列平稳'}
                </span>
              </div>
              <div className="grid grid-cols-2 divide-x divide-y divide-[#EDF0EE] sm:grid-cols-4 sm:divide-y-0">
                {[
                  ['API 请求', data.today?.tasks],
                  ['运行中', data.today?.runningTasks],
                  ['失败请求', data.today?.failedTasks],
                  ['新增客户', data.today?.users],
                ].map(([label, value]) => (
                  <div key={String(label)} className="p-4">
                    <span className="text-[10px] font-semibold text-zinc-500">{label}</span>
                    <strong className="mt-2 block text-xl text-[#17201B]">{Number(value || 0).toLocaleString('zh-CN')}</strong>
                  </div>
                ))}
              </div>
              <div className="grid grid-cols-1 gap-3 border-t border-[#DCE4DF] bg-[#FAFBFA] p-4 sm:grid-cols-3">
                {attention.map((item) => (
                  <div key={item.label} className="flex items-center gap-3 rounded-md border border-[#E5E9E6] bg-white px-3 py-2.5">
                    <span className={`h-2 w-2 rounded-full ${item.tone === 'red' ? 'bg-red-500' : item.tone === 'amber' ? 'bg-amber-500' : 'bg-blue-500'}`} />
                    <span className="min-w-0 flex-1">
                      <span className="block text-[10px] text-zinc-500">{item.label}</span>
                      <strong className="block text-base text-[#17201B]">{item.value}</strong>
                      <small className="block truncate text-[10px] text-zinc-400">{item.note}</small>
                    </span>
                  </div>
                ))}
              </div>
            </section>

            <section className="rounded-md border border-[#DCE4DF] bg-white">
              <div className="border-b border-[#DCE4DF] px-4 py-3">
                <h2 className="text-sm font-semibold text-[#17201B]">资源状态</h2>
                <p className="mt-0.5 text-[11px] text-zinc-500">接口与模型可用性</p>
              </div>
              <div className="divide-y divide-[#EDF0EE] px-4">
                {[
                  ['启用上游', data.system?.activeProviders, `${Number(data.system?.disabledProviders || 0)} 个停用`],
                  ['启用模型', data.system?.activeModels, `${Number(data.system?.disabledModels || 0)} 个停用`],
                  ['累计充值单', data.orders?.all, `${Number(data.orders?.paid || 0)} 笔已支付`],
                ].map(([label, value, note]) => (
                  <div key={String(label)} className="flex items-center justify-between gap-4 py-3 text-xs">
                    <span className="text-zinc-600">{label}</span>
                    <span className="text-right"><strong className="block text-[#17201B]">{Number(value || 0)}</strong><small className="text-[10px] text-zinc-400">{note}</small></span>
                  </div>
                ))}
              </div>
              <div className="flex items-center gap-2 border-t border-[#DCE4DF] bg-[#FAFBFA] px-4 py-3 text-[10px] text-zinc-500">
                <Clock3 className="h-3.5 w-3.5" />
                最近请求：{data.system?.lastTaskAt ? formatDate(data.system.lastTaskAt) : '暂无记录'}
              </div>
            </section>
          </div>

          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white">
              <div className="border-b border-[#DCE4DF] px-4 py-3"><h2 className="text-sm font-semibold">最近充值与订阅</h2></div>
              <div className="overflow-x-auto">
                <table className="w-full min-w-[560px] text-left text-xs">
                  <thead className="bg-[#F6F8F6] text-[10px] text-zinc-500"><tr><th className="px-4 py-2">客户</th><th className="px-4 py-2">类型</th><th className="px-4 py-2">金额</th><th className="px-4 py-2">状态</th><th className="px-4 py-2">时间</th></tr></thead>
                  <tbody className="divide-y divide-[#EDF0EE]">
                    {recentOrders.map((row) => { const status = statusView(row.status); return (
                      <tr key={row.id}><td className="max-w-[180px] truncate px-4 py-2.5">{row.userEmail || row.userId || '-'}</td><td className="px-4 py-2.5">{row.orderType === 'subscription' ? '订阅' : '余额'}</td><td className="px-4 py-2.5 font-mono">{formatCNY(Number(row.amount || 0))}</td><td className="px-4 py-2.5"><span className={`rounded border px-1.5 py-0.5 text-[10px] ${status.className}`}>{status.label}</span></td><td className="px-4 py-2.5 text-zinc-500">{formatDate(row.createdAt || '')}</td></tr>
                    ); })}
                    {!recentOrders.length && <tr><td colSpan={5} className="px-4 py-8 text-center text-zinc-400">暂无充值记录</td></tr>}
                  </tbody>
                </table>
              </div>
            </section>

            <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white">
              <div className="border-b border-[#DCE4DF] px-4 py-3"><h2 className="text-sm font-semibold">最近 API 请求</h2></div>
              <div className="overflow-x-auto">
                <table className="w-full min-w-[560px] text-left text-xs">
                  <thead className="bg-[#F6F8F6] text-[10px] text-zinc-500"><tr><th className="px-4 py-2">客户</th><th className="px-4 py-2">模型</th><th className="px-4 py-2">数量</th><th className="px-4 py-2">状态</th><th className="px-4 py-2">时间</th></tr></thead>
                  <tbody className="divide-y divide-[#EDF0EE]">
                    {recentTasks.map((row) => { const status = statusView(row.status); return (
                      <tr key={row.id}><td className="max-w-[160px] truncate px-4 py-2.5">{row.userEmail || row.userId || '-'}</td><td className="max-w-[160px] truncate px-4 py-2.5">{row.modelDisplayName || row.modelName || row.modelId || '-'}</td><td className="px-4 py-2.5 font-mono">{Number(row.quantity || 0)}</td><td className="px-4 py-2.5"><span className={`rounded border px-1.5 py-0.5 text-[10px] ${status.className}`}>{status.label}</span></td><td className="px-4 py-2.5 text-zinc-500">{formatDate(row.createdAt || '')}</td></tr>
                    ); })}
                    {!recentTasks.length && <tr><td colSpan={5} className="px-4 py-8 text-center text-zinc-400">暂无 API 请求</td></tr>}
                  </tbody>
                </table>
              </div>
            </section>
          </div>

          <p className="text-right text-[10px] text-zinc-400">最后同步：{lastUpdated ? formatDate(lastUpdated) : '-'}</p>
        </>
      )}
    </div>
  );
}
