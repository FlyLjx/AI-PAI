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
import { AppSelect } from '@/components/common/AppSelect';
import { PageHeader } from '@/components/common/PageHeader';
import { formatCNY, formatDate } from '@/lib/common/utils';
import { portalApi, type AdminInviteRecord, type AdminInviteSummary } from '@/lib/admin-api';

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

function ipEvidence(record: AdminInviteRecord, side: 'inviter' | 'invitee') {
  const values = side === 'inviter'
    ? [
        { label: '注册', value: record.inviterIp, tone: 'bg-zinc-100 text-zinc-600' },
        { label: '登录', value: record.inviterLoginIp, tone: 'bg-blue-50 text-blue-700' },
        { label: 'API', value: record.inviterApiIp, tone: 'bg-emerald-50 text-emerald-700' },
      ]
    : [
        { label: '注册', value: record.inviteeIp, tone: 'bg-zinc-100 text-zinc-600' },
        { label: '登录', value: record.inviteeLoginIp, tone: 'bg-blue-50 text-blue-700' },
        { label: 'API', value: record.inviteeApiIp, tone: 'bg-emerald-50 text-emerald-700' },
      ];
  return values.filter((item): item is { label: string; value: string; tone: string } => Boolean(item.value));
}

function IPEvidenceCell({ record, side }: { record: AdminInviteRecord; side: 'inviter' | 'invitee' }) {
  const entries = ipEvidence(record, side);
  const counterpart = new Set(ipEvidence(record, side === 'inviter' ? 'invitee' : 'inviter').map((item) => item.value));
  if (!entries.length) return <span className="text-zinc-300">-</span>;
  return (
    <div className="space-y-1">
      {entries.map((entry) => {
        const matches = counterpart.has(entry.value);
        return (
          <div key={`${entry.label}-${entry.value}`} className={`flex w-fit max-w-[190px] items-center gap-1.5 rounded px-1.5 py-0.5 font-mono text-[9px] ${matches ? 'bg-red-50 text-red-700 ring-1 ring-inset ring-red-200' : 'text-zinc-500'}`} title={matches ? '双方使用过相同 IP' : entry.value}>
            <span className={`rounded px-1 py-0.5 font-sans text-[8px] font-semibold ${matches ? 'bg-red-100 text-red-700' : entry.tone}`}>{entry.label}</span>
            <span className="truncate">{entry.value}</span>
          </div>
        );
      })}
    </div>
  );
}

export default function AdminInvitesPage() {
  const [items, setItems] = useState<AdminInviteRecord[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [summary, setSummary] = useState<AdminInviteSummary>({ total: 0, rewarded: 0, pending: 0, blocked: 0 });

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const response = await portalApi.adminInvites(page, PAGE_SIZE);
      setItems(response.data || []);
      const responseTotal = Number(response.pagination?.total ?? response.summary?.total ?? 0);
      setTotal(responseTotal);
      setSummary(response.summary || {
        total: responseTotal,
        rewarded: response.data.filter((item) => item.status === 'rewarded').length,
        pending: response.data.filter((item) => item.status === 'pending').length,
        blocked: response.data.filter((item) => item.status === 'blocked').length,
      });
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

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const metrics = [
    { label: '全部记录', value: summary.total, icon: Gift, tone: 'bg-zinc-100 text-zinc-600' },
    { label: '已发放', value: summary.rewarded, icon: CheckCircle2, tone: 'bg-emerald-50 text-emerald-700' },
    { label: '待验证', value: summary.pending, icon: Clock3, tone: 'bg-amber-50 text-amber-700' },
    { label: '已拦截', value: summary.blocked, icon: ShieldAlert, tone: 'bg-red-50 text-red-700' },
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
          <table className="w-full min-w-[1320px] text-left text-xs">
            <thead className="bg-[#F7F8F6] text-[10px] text-zinc-500">
              <tr><th className="px-4 py-3">邀请人</th><th className="px-4 py-3">被邀请人</th><th className="px-4 py-3">邀请人奖励</th><th className="px-4 py-3">新用户奖励</th><th className="px-4 py-3">充值返利</th><th className="px-4 py-3">状态</th><th className="px-4 py-3">邀请人IP</th><th className="px-4 py-3">被邀请人IP</th><th className="px-4 py-3">创建时间</th></tr>
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
                    <td className="whitespace-nowrap px-4 py-3"><strong className="block text-[11px] text-amber-700">{formatCNY(Number(item.rechargeRebateCredits || 0))}</strong><small className="text-[9px] text-zinc-400">{Number(item.rechargeRebateCount || 0)} 笔</small></td>
                    <td className="max-w-[230px] px-4 py-3"><span className={`inline-flex items-center gap-1 rounded border px-1.5 py-0.5 text-[10px] font-semibold ${status.className}`}><StatusIcon className="h-3 w-3" />{status.label}</span>{item.sharedIp && <small className="mt-1 block text-[10px] font-semibold text-red-600">历史 IP 存在重合</small>}{item.riskReason && <small className="mt-1 block truncate text-[10px] text-red-600" title={item.riskReason}>{item.riskReason}</small>}</td>
                    <td className="px-4 py-3"><IPEvidenceCell record={item} side="inviter" /></td>
                    <td className="px-4 py-3"><IPEvidenceCell record={item} side="invitee" /></td>
                    <td className="whitespace-nowrap px-4 py-3 text-[10px] text-zinc-500">{formatDate(item.rewardedAt || item.createdAt)}</td>
                  </tr>
                );
              })}
              {!loading && !items.length && <tr><td colSpan={9} className="px-4 py-14 text-center text-zinc-400"><UserPlus className="mx-auto mb-2 h-6 w-6" />暂无邀请记录</td></tr>}
            </tbody>
          </table>
        </div>
        {loading && !items.length && <div className="grid min-h-[260px] place-items-center"><Loader2 className="h-5 w-5 animate-spin text-[#12B76A]" /></div>}
        <footer className="flex flex-col gap-2 border-t border-[#EDF0EE] bg-[#FAFBFA] px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <span className="text-[10px] text-zinc-400">第 {page} / {totalPages} 页，共 {total} 条</span>
          <div className="flex flex-wrap gap-2">
            <AppSelect compact value={String(page)} options={Array.from({ length: totalPages }, (_, index) => ({ value: String(index + 1), label: `第 ${index + 1} 页` }))} onValueChange={(value) => setPage(Number(value))} disabled={loading} ariaLabel="选择邀请记录页码" />
            <button type="button" onClick={() => setPage((value) => Math.max(1, value - 1))} disabled={page <= 1 || loading} title="上一页" aria-label="上一页" className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white disabled:opacity-40"><ChevronLeft className="h-4 w-4" /></button>
            <button type="button" onClick={() => setPage((value) => value + 1)} disabled={page >= totalPages || loading} title="下一页" aria-label="下一页" className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white disabled:opacity-40"><ChevronRight className="h-4 w-4" /></button>
          </div>
        </footer>
      </section>
    </div>
  );
}
