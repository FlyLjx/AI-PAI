'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { CalendarDays, Loader2, ReceiptText, RefreshCw } from 'lucide-react';
import { AppSelect } from '@/components/common/AppSelect';
import { DataTable } from '@/components/common/DataTable';
import { DateRangePicker } from '@/components/common/DateRangePicker';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { portalApi, type RechargeOrder, type RechargeSummary } from '@/lib/admin-api';
import { formatCNY, formatDate } from '@/lib/common/utils';

const pageSize = 30;
const ORDER_TYPE_OPTIONS = [
  { value: 'all', label: '全部类型' },
  { value: 'recharge', label: '余额充值' },
  { value: 'subscription', label: '订阅购买' },
] as const;
const ORDER_STATUS_OPTIONS = [
  { value: 'all', label: '全部状态' },
  { value: 'paid', label: '已支付' },
  { value: 'pending', label: '待支付' },
  { value: 'failed', label: '失败' },
  { value: 'closed', label: '已关闭' },
] as const;
type RevenuePeriod = 'current' | 'previous' | 'custom';

function dateValue(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function revenueRange(period: RevenuePeriod, customStart: string, customEnd: string) {
  const today = new Date();
  if (period === 'custom') return { start: customStart, end: customEnd };
  if (period === 'previous') {
    const previousMonth = new Date(today.getFullYear(), today.getMonth() - 1, 1);
    const previousMonthEnd = new Date(today.getFullYear(), today.getMonth(), 0);
    return { start: dateValue(previousMonth), end: dateValue(previousMonthEnd) };
  }
  return { start: dateValue(new Date(today.getFullYear(), today.getMonth(), 1)), end: dateValue(today) };
}

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
  const [period, setPeriod] = useState<RevenuePeriod>('current');
  const [customStart, setCustomStart] = useState('');
  const [customEnd, setCustomEnd] = useState('');
  const [summary, setSummary] = useState<RechargeSummary>({ total: 0, paidAmount: 0, paidCount: 0, pendingCount: 0, subscriptionCount: 0 });
  const requestSequence = useRef(0);
  const range = revenueRange(period, customStart, customEnd);
  const periodLabel = period === 'current' ? '当月收益' : period === 'previous' ? '上月收益' : '自定义收益';

  const load = useCallback(async (nextPage: number) => {
    const sequence = ++requestSequence.current;
    setLoading(true);
    setError('');
    try {
      const response = await portalApi.recharges({
        page: nextPage,
        pageSize,
        keyword: search.trim() || undefined,
        orderType: typeFilter === 'all' ? undefined : typeFilter,
        status: statusFilter === 'all' ? undefined : statusFilter,
        startDate: range.start || undefined,
        endDate: range.end || undefined,
      });
      if (sequence !== requestSequence.current) return;
      const responseTotal = response.pagination?.total ?? response.data.length;
      setOrders(response.data || []);
      setTotal(responseTotal);
      setSummary(response.summary || {
        total: responseTotal,
        paidAmount: response.data.filter((order) => order.status === 'paid').reduce((sum, order) => sum + Number(order.amount || 0), 0),
        paidCount: response.data.filter((order) => order.status === 'paid').length,
        pendingCount: response.data.filter((order) => order.status === 'pending').length,
        subscriptionCount: response.data.filter((order) => order.orderType === 'subscription').length,
      });
      setPage(nextPage);
    } catch (requestError) {
      if (sequence !== requestSequence.current) return;
      setError(requestError instanceof Error ? requestError.message : '充值流水加载失败');
    } finally {
      if (sequence === requestSequence.current) setLoading(false);
    }
  }, [range.end, range.start, search, statusFilter, typeFilter]);

  useEffect(() => {
    if (period === 'custom' && (!customStart || !customEnd)) return;
    const timer = window.setTimeout(() => void load(1), 300);
    return () => window.clearTimeout(timer);
  }, [customEnd, customStart, load, period]);

  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <div className="space-y-5">
      <PageHeader title="充值流水" description="查看充值与订阅订单，并按周期统计收益；支付状态由 Go 后端与支付宝同步。">
        <button type="button" onClick={() => void load(page)} disabled={loading} className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[#DCE4DF] bg-white px-3 text-xs font-semibold hover:border-[#12B76A] disabled:opacity-50"><RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />刷新</button>
      </PageHeader>

      <section className="flex flex-col gap-3 rounded-md border border-[#DCE4DF] bg-white px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2.5">
          <span className="grid h-8 w-8 place-items-center rounded-md bg-emerald-50 text-[#047857]"><CalendarDays className="h-4 w-4" /></span>
          <div><span className="block text-[10px] font-semibold uppercase tracking-wide text-zinc-400">收益统计周期</span><strong className="mt-0.5 block text-xs text-[#17201B]">{periodLabel}{range.start && range.end ? ` · ${range.start} 至 ${range.end}` : ''}</strong></div>
        </div>
        <span className="text-[10px] leading-5 text-zinc-400">已收金额按周期内已支付订单汇总，列表同时保留待支付记录。</span>
      </section>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          ['已收金额', formatCNY(summary.paidAmount), `${summary.paidCount} 笔已支付`],
          ['待支付订单', summary.pendingCount, '当前筛选范围'],
          ['订阅订单', summary.subscriptionCount, '当前筛选范围'],
          ['订单总数', summary.total.toLocaleString('zh-CN'), '当前筛选范围'],
        ].map(([label, value, note]) => <div key={String(label)} className="rounded-md border border-[#DCE4DF] bg-white p-3.5"><span className="text-[11px] font-semibold text-zinc-500">{label}</span><strong className="mt-1.5 block text-xl">{value}</strong><small className="mt-1 block text-[11px] text-zinc-400">{note}</small></div>)}
      </div>

      {error && <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void load(page)} className="font-semibold underline">重试</button></div>}

      {loading && orders.length === 0 ? (
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
          data={orders}
          searchPlaceholder="搜索用户、商户订单号或支付流水号"
          searchValue={search}
          onSearchChange={(value) => { setSearch(value); setPage(1); }}
          filterControls={(
            <>
              <div className="flex items-center gap-1 rounded-md border border-[#DCE4DF] bg-[#F7F9F7] p-1" aria-label="收益统计周期">
                {([['current', '当月'], ['previous', '上月'], ['custom', '自定义']] as const).map(([value, label]) => <button key={value} type="button" onClick={() => { setPeriod(value); setPage(1); }} className={`h-7 rounded px-2.5 text-[11px] font-semibold transition-colors ${period === value ? 'bg-white text-[#047857] shadow-sm ring-1 ring-inset ring-[#BFD2C6]' : 'text-zinc-500 hover:text-[#047857]'}`}>{label}</button>)}
              </div>
              {period === 'custom' && <DateRangePicker startDate={customStart} endDate={customEnd} maxDate={dateValue(new Date())} onChange={(start, end) => { setCustomStart(start); setCustomEnd(end); setPage(1); }} />}
              <AppSelect value={typeFilter} options={ORDER_TYPE_OPTIONS} onValueChange={(value) => { setTypeFilter(value); setPage(1); }} compact ariaLabel="筛选订单类型" />
              <AppSelect value={statusFilter} options={ORDER_STATUS_OPTIONS} onValueChange={(value) => { setStatusFilter(value); setPage(1); }} compact ariaLabel="筛选支付状态" />
              <span className="text-[11px] text-zinc-400">{loading ? '正在查询...' : `共 ${total} 条 · 本页 ${orders.length} 条`}</span>
            </>
          )}
          currentPage={page}
          totalPages={totalPages}
          onPageChange={(nextPage) => void load(nextPage)}
          emptyState={<EmptyState title="暂无充值流水" description="余额充值或订阅购买后，订单会显示在这里。" icon={ReceiptText} />}
          renderRow={(order) => { const status = statusView(order.status); return (
            <tr key={order.id} className="hover:bg-[#FAFBFA]">
              <td className="px-4 py-3"><strong className="block max-w-[180px] truncate font-mono text-[11px]">{order.outTradeNo}</strong><small className="block max-w-[180px] truncate font-mono text-[10px] text-zinc-400">{order.id}</small></td>
              <td className="px-4 py-3"><span className="block max-w-[190px] truncate">{order.userEmail || order.userId}</span></td>
              <td className="px-4 py-3"><span className={`rounded border px-1.5 py-0.5 text-[11px] font-semibold ${order.orderType === 'subscription' ? 'border-amber-200 bg-amber-50 text-amber-800' : 'border-blue-200 bg-blue-50 text-blue-700'}`}>{order.orderType === 'subscription' ? '订阅购买' : '余额充值'}</span></td>
              <td className="px-4 py-3 text-right font-mono font-semibold text-[#047857]">{formatCNY(Number(order.amount || 0))}</td>
              <td className="px-4 py-3"><span className={`rounded border px-1.5 py-0.5 text-[11px] font-semibold ${status.className}`}>{status.label}</span></td>
              <td className="max-w-[150px] truncate px-4 py-3 font-mono text-[10px] text-zinc-500">{order.tradeNo || '-'}</td>
              <td className="whitespace-nowrap px-4 py-3 text-zinc-500">{formatDate(order.createdAt)}</td>
              <td className="whitespace-nowrap px-4 py-3 text-zinc-500">{order.paidAt ? formatDate(order.paidAt) : '-'}</td>
            </tr>
          ); }}
          renderMobileItem={(order) => { const status = statusView(order.status); return (
            <article key={order.id} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
              <div className="flex items-start justify-between gap-3"><div className="min-w-0"><strong className="block truncate font-mono text-[12px]">{order.outTradeNo}</strong><small className="block truncate text-[10px] text-zinc-400">{order.userEmail || order.userId}</small></div><span className={`shrink-0 rounded border px-1.5 py-0.5 text-[11px] font-semibold ${status.className}`}>{status.label}</span></div>
              <div className="mt-3 flex items-center justify-between border-y border-[#EDF0EE] py-2"><span className="text-[11px] text-zinc-500">{order.orderType === 'subscription' ? '订阅购买' : '余额充值'}</span><strong className="font-mono text-sm text-[#047857]">{formatCNY(Number(order.amount || 0))}</strong></div>
              <div className="mt-2 flex items-center justify-between text-[10px] text-zinc-400"><span>{formatDate(order.createdAt)}</span><span className="max-w-[150px] truncate font-mono">{order.tradeNo || '暂无渠道流水'}</span></div>
            </article>
          ); }}
        />
      )}
    </div>
  );
}
