'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Clock3,
  Gift,
  Loader2,
  RefreshCw,
  ShieldAlert,
  UserPlus,
  XCircle,
} from 'lucide-react';
import { PageHeader } from '@/components/common/PageHeader';
import { formatCNY, formatDate } from '@/lib/common/utils';
import { portalApi, type AdminInviteRecord } from '@/lib/admin-api';

const PAGE_SIZE = 30;

function statusView(status: string) {
  if (status === 'rewarded') return { label: '已发放', className: 'border-emerald-200 bg-emerald-50 text-emerald-700', icon: CheckCircle2 };
  if (status === 'blocked') return { label: '已拦截', className: 'border-red-200 bg-red-50 text-red-700', icon: XCircle };
  return { label: '待验证', className: 'border-amber-200 bg-amber-50 text-amber-700', icon: Clock3 };
}

function rewardText(record: AdminInviteRecord, side: 'inviter' | 'invitee') {
  const type = side === 'inviter' ? record.rewardType : record.inviteeRewardType;
  const credits = side === 'inviter' ? record.rewardCredits : record.inviteeRewardCredits;
  const label = side === 'inviter' ? record.rewardLabel : record.inviteeRewardLabel;
  if (type === 'balance') return formatCNY(Number(credits || 0));
  if (type === 'subscription') return label || '订阅权益';
  return '-';
}

export default function AdminInvitesPage() {
  const [items, setItems] = useState<AdminInviteRecord[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const response = await portalApi.adminInvites(page, PAGE_SIZE);
      setItems(response.data || []);
      setTotal(Number(response.pagination?.total || 0));
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '邀请记录加载失败');
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  const rewarded = items.filter((item) => item.status === 'rewarded').length;
  const pending = items.filter((item) => item.status === 'pending').length;
  const blocked = items.filter((item) => item.status === 'blocked').length;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const metrics = [
    { label: '全部记录', value: total, icon: Gift, tone: 'bg-zinc-100 text-zinc-600' },
    { label: '本页已发放', value: rewarded, icon: CheckCircle2, tone: 'bg-emerald-50 text-emerald-700' },
    { label: '本页待验证', value: pending, icon: Clock3, tone: 'bg-amber-50 text-amber-700' },
    { label: '本页已拦截', value: blocked, icon: ShieldAlert, tone: 'bg-red-50 text-red-700' },
  ];

  return (
    <div className="space-y-5">
      <PageHeader title="邀请返利审计" description="查看邀请关系、双方奖励与风控拦截原因。">
        <button type="button" onClick={() => void load()} disabled={loading} title="刷新" className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white hover:border-[#12B76A] disabled:opacity-50">
          <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {metrics.map((metric) => {
          const Icon = metric.icon;
          return (
            <div key={metric.label} className="flex items-center gap-3 rounded-md border border-[#DCE4DF] bg-white p-4">
              <span className={`grid h-9 w-9 shrink-0 place-items-center rounded-md ${metric.tone}`}><Icon className="h-4 w-4" /></span>
              <span><small className="block text-[10px] text-zinc-400">{metric.label}</small><strong className="mt-0.5 block font-mono text-lg">{metric.value.toLocaleString()}</strong></span>
            </div>
          );
        })}
      </div>

      {error && <div className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700">{error}</div>}

      <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[1040px] text-left text-xs">
            <thead className="bg-[#F7F8F6] text-[10px] text-zinc-500">
              <tr><th className="px-4 py-3">邀请人</th><th className="px-4 py-3">被邀请人</th><th className="px-4 py-3">邀请人奖励</th><th className="px-4 py-3">新用户奖励</th><th className="px-4 py-3">状态</th><th className="px-4 py-3">网络地址</th><th className="px-4 py-3">创建时间</th></tr>
            </thead>
            <tbody className="divide-y divide-[#EDF0EE]">
              {items.map((item) => {
                const status = statusView(item.status);
                const StatusIcon = status.icon;
                return (
                  <tr key={item.id} className="hover:bg-[#FAFBFA]">
                    <td className="max-w-[190px] px-4 py-3"><strong className="block truncate">{item.inviterEmail || item.inviterId}</strong><small className="block truncate font-mono text-[9px] text-zinc-400">{item.inviterId}</small></td>
                    <td className="max-w-[190px] px-4 py-3"><strong className="block truncate">{item.inviteeEmail || item.inviteeId}</strong><small className="block truncate font-mono text-[9px] text-zinc-400">{item.inviteeId}</small></td>
                    <td className="px-4 py-3 font-medium">{rewardText(item, 'inviter')}</td>
                    <td className="px-4 py-3 font-medium">{rewardText(item, 'invitee')}</td>
                    <td className="max-w-[230px] px-4 py-3"><span className={`inline-flex items-center gap-1 rounded border px-1.5 py-0.5 text-[10px] font-semibold ${status.className}`}><StatusIcon className="h-3 w-3" />{status.label}</span>{item.riskReason && <small className="mt-1 block truncate text-[10px] text-red-600" title={item.riskReason}>{item.riskReason}</small>}</td>
                    <td className="px-4 py-3 font-mono text-[10px] text-zinc-500">{item.inviteeIp || '-'}</td>
                    <td className="whitespace-nowrap px-4 py-3 text-[10px] text-zinc-500">{formatDate(item.rewardedAt || item.createdAt)}</td>
                  </tr>
                );
              })}
              {!loading && !items.length && <tr><td colSpan={7} className="px-4 py-14 text-center text-zinc-400"><UserPlus className="mx-auto mb-2 h-6 w-6" />暂无邀请记录</td></tr>}
            </tbody>
          </table>
        </div>
        {loading && !items.length && <div className="grid min-h-[260px] place-items-center"><Loader2 className="h-5 w-5 animate-spin text-[#12B76A]" /></div>}
        <footer className="flex items-center justify-between border-t border-[#EDF0EE] bg-[#FAFBFA] px-4 py-3">
          <span className="text-[10px] text-zinc-400">第 {page} / {totalPages} 页，共 {total} 条</span>
          <div className="flex gap-2">
            <button type="button" onClick={() => setPage((value) => Math.max(1, value - 1))} disabled={page <= 1 || loading} className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white disabled:opacity-40"><ChevronLeft className="h-4 w-4" /></button>
            <button type="button" onClick={() => setPage((value) => value + 1)} disabled={page >= totalPages || loading} className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white disabled:opacity-40"><ChevronRight className="h-4 w-4" /></button>
          </div>
        </footer>
      </section>
    </div>
  );
}
