'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Loader2, ReceiptText, RefreshCw } from 'lucide-react';
import { DataTable } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { portalApi } from '@/lib/portal-api';
import { formatCNY, formatDate } from '@/lib/common/utils';

type RechargeOrder = {
  id: string;
  userId: string;
  userEmail?: string;
  outTradeNo: string;
  tradeNo?: string | null;
  orderType: 'recharge' | 'subscription';
  subscriptionPlanId?: string | null;
  amount: number;
  status: string;
  paidAt?: string | null;
  createdAt: string;
  updatedAt?: string;
};

const pageSize = 30;

function statusView(status: string) {
  if (status === 'paid' || status === 'success') return { label: '已支付', className: 'border-emerald-200 bg-emerald-50 text-emerald-700' };
  if (status === 'pending') return { label: '待支付', className: 'border-amber-200 bg-amber-50 text-amber-700' };
  if (status === 'closed' || status === 'canceled') return { label: '已关闭', className: 'border-zinc-200 bg-zinc-50 text-zinc-500' };
  if (status === 'failed') return { label: '失败', className: 'border-red-200 bg-red-50 text-red-700' };
  return { label: status || '未知', className: 'border-zinc-200 bg-zinc-50 text-zinc-500' };
}

export default function AdminRechargesPage() {
  const [orders, setOrders] = useState<RechargeOrder[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState('all');

  const load = useCallback(async (nextPage: number) => {
    setLoading(true);
    setError('');
    try {
      const response = await portalApi.recharges(nextPage);
      setOrders(response.data as unknown as RechargeOrder[]);
      setTotal(response.pagination?.total || response.data.length);
      setPage(nextPage);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : '充值流水加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(1), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  const filtered = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    return orders.filter((order) => {
      const matchesKeyword = !keyword || `${order.userEmail || ''} ${order.userId} ${order.outTradeNo} ${order.tradeNo || ''}`.toLowerCase().includes(keyword);
      const matchesType = typeFilter === 'all' || order.orderType === typeFilter;
      const matchesStatus = statusFilter === 'all' || order.status === statusFilter;
      return matchesKeyword && matchesType && matchesStatus;
    });
  }, [orders, search, statusFilter, typeFilter]);

  const summary = useMemo(() => ({
    paid: orders.filter((order) => order.status === 'paid').reduce((sum, order) => sum + Number(order.amount || 0), 0),
    paidCount: orders.filter((order) => order.status === 'paid').length,
    pending: orders.filter((order) => order.status === 'pending').length,
    subscriptions: orders.filter((order) => order.orderType === 'subscription').length,
  }), [orders]);

  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <div className="space-y-5">
      <PageHeader title="充值流水" description="查看余额充值和订阅购买订单；支付状态由 Go 后端与支付宝同步。">
        <button type="button" onClick={() => void load(page)} disabled={loading} className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[#DCE4DF] bg-white px-3 text-xs font-semibold hover:border-[#12B76A] disabled:opacity-50"><RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />刷新</button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          ['本页已收金额', formatCNY(summary.paid), `${summary.paidCount} 笔已支付`],
          ['待支付订单', summary.pending, '等待支付结果'],
          ['订阅订单', summary.subscriptions, '本页订阅购买'],
          ['全部流水', total.toLocaleString('zh-CN'), '服务器记录总数'],
        ].map(([label, value, note]) => <div key={String(label)} className="rounded-md border border-[#DCE4DF] bg-white p-3.5"><span className="text-[10px] font-semibold text-zinc-500">{label}</span><strong className="mt-1.5 block text-xl">{value}</strong><small className="mt-1 block text-[10px] text-zinc-400">{note}</small></div>)}
      </div>

      {error && <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void load(page)} className="font-semibold underline">重试</button></div>}

      {loading ? (
        <div className="grid min-h-[300px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <DataTable
          headers={[
            { key: 'order', label: '订单号' },
            { key: 'user', label: 'API 客户' },
            { key: 'type', label: '订单类型' },
            { key: 'amount', label: '金额', className: 'text-right' },
            { key: 'status', label: '支付状态' },
            { key: 'trade', label: '渠道流水号' },
            { key: 'created', label: '创建时间' },
            { key: 'paid', label: '支付时间' },
          ]}
          data={filtered}
          searchPlaceholder="搜索用户、商户订单号或支付流水号"
          searchValue={search}
          onSearchChange={setSearch}
          filterControls={(
            <>
              <select value={typeFilter} onChange={(event) => setTypeFilter(event.target.value)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-2 text-xs"><option value="all">全部类型</option><option value="recharge">余额充值</option><option value="subscription">订阅购买</option></select>
              <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-2 text-xs"><option value="all">全部状态</option><option value="paid">已支付</option><option value="pending">待支付</option><option value="failed">失败</option><option value="closed">已关闭</option></select>
              <span className="text-[10px] text-zinc-400">本页 {filtered.length} 条</span>
            </>
          )}
          currentPage={page}
          totalPages={totalPages}
          onPageChange={(nextPage) => void load(nextPage)}
          emptyState={<EmptyState title="暂无充值流水" description="余额充值或订阅购买后，订单会显示在这里。" icon={ReceiptText} />}
          renderRow={(order) => { const status = statusView(order.status); return (
            <tr key={order.id} className="hover:bg-[#FAFBFA]">
              <td className="px-4 py-3"><strong className="block max-w-[180px] truncate font-mono text-[10px]">{order.outTradeNo}</strong><small className="block max-w-[180px] truncate font-mono text-[9px] text-zinc-400">{order.id}</small></td>
              <td className="px-4 py-3"><span className="block max-w-[190px] truncate">{order.userEmail || order.userId}</span></td>
              <td className="px-4 py-3"><span className={`rounded border px-1.5 py-0.5 text-[10px] font-semibold ${order.orderType === 'subscription' ? 'border-amber-200 bg-amber-50 text-amber-800' : 'border-blue-200 bg-blue-50 text-blue-700'}`}>{order.orderType === 'subscription' ? '订阅购买' : '余额充值'}</span></td>
              <td className="px-4 py-3 text-right font-mono font-semibold text-[#047857]">{formatCNY(Number(order.amount || 0))}</td>
              <td className="px-4 py-3"><span className={`rounded border px-1.5 py-0.5 text-[10px] font-semibold ${status.className}`}>{status.label}</span></td>
              <td className="max-w-[150px] truncate px-4 py-3 font-mono text-[9px] text-zinc-500">{order.tradeNo || '-'}</td>
              <td className="whitespace-nowrap px-4 py-3 text-zinc-500">{formatDate(order.createdAt)}</td>
              <td className="whitespace-nowrap px-4 py-3 text-zinc-500">{order.paidAt ? formatDate(order.paidAt) : '-'}</td>
            </tr>
          ); }}
          renderMobileItem={(order) => { const status = statusView(order.status); return (
            <article key={order.id} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
              <div className="flex items-start justify-between gap-3"><div className="min-w-0"><strong className="block truncate font-mono text-[11px]">{order.outTradeNo}</strong><small className="block truncate text-[9px] text-zinc-400">{order.userEmail || order.userId}</small></div><span className={`shrink-0 rounded border px-1.5 py-0.5 text-[10px] font-semibold ${status.className}`}>{status.label}</span></div>
              <div className="mt-3 flex items-center justify-between border-y border-[#EDF0EE] py-2"><span className="text-[10px] text-zinc-500">{order.orderType === 'subscription' ? '订阅购买' : '余额充值'}</span><strong className="font-mono text-sm text-[#047857]">{formatCNY(Number(order.amount || 0))}</strong></div>
              <div className="mt-2 flex items-center justify-between text-[9px] text-zinc-400"><span>{formatDate(order.createdAt)}</span><span className="max-w-[150px] truncate font-mono">{order.tradeNo || '暂无渠道流水'}</span></div>
            </article>
          ); }}
        />
      )}
    </div>
  );
}
