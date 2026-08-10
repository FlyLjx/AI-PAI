'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Clock3,
  Gift,
  Loader2,
  MessageSquareText,
  RefreshCw,
  ShieldAlert,
  ShieldCheck,
  UserPlus,
  XCircle,
} from 'lucide-react';
import { AppSelect } from '@/components/common/AppSelect';
import { AdminMetricCard } from '@/components/common/AdminMetricCard';
import { SortableHeader, type SortState } from '@/components/common/DataTable';
import { PageHeader } from '@/components/common/PageHeader';
import { formatCNY, formatDate } from '@/lib/common/utils';
import { portalApi, type AdminInviteRecord, type AdminInviteSummary } from '@/lib/admin-api';
import { toast } from 'sonner';

const PAGE_SIZE = 30;
type ReviewAction = 'approve' | 'reject';

function statusView(status: string) {
  if (status === 'rewarded') return { label: '已发放', className: 'border-emerald-200 bg-emerald-50 text-emerald-700', icon: CheckCircle2 };
  if (status === 'blocked') return { label: '已拦截', className: 'border-red-200 bg-red-50 text-red-700', icon: XCircle };
  if (status === 'review') return { label: '待人工审核', className: 'border-amber-200 bg-amber-50 text-amber-800', icon: ShieldAlert };
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
  const [sort, setSort] = useState<SortState>({ key: 'createdAt', direction: 'desc' });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [reviewingId, setReviewingId] = useState('');
  const [reviewDialog, setReviewDialog] = useState<{ item: AdminInviteRecord; action: ReviewAction } | null>(null);
  const [reviewNote, setReviewNote] = useState('');
  const [summary, setSummary] = useState<AdminInviteSummary>({ total: 0, rewarded: 0, pending: 0, review: 0, blocked: 0 });

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const response = await portalApi.adminInvites(page, PAGE_SIZE, sort.key, sort.direction);
      setItems(response.data || []);
      const responseTotal = Number(response.pagination?.total ?? response.summary?.total ?? 0);
      setTotal(responseTotal);
      setSummary({
        total: Number(response.summary?.total ?? responseTotal),
        rewarded: Number(response.summary?.rewarded ?? response.data.filter((item) => item.status === 'rewarded').length),
        pending: Number(response.summary?.pending ?? response.data.filter((item) => item.status === 'pending').length),
        review: Number(response.summary?.review ?? response.data.filter((item) => item.status === 'review').length),
        blocked: Number(response.summary?.blocked ?? response.data.filter((item) => item.status === 'blocked').length),
      });
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '邀请记录加载失败');
    } finally {
      setLoading(false);
    }
  }, [page, sort.direction, sort.key]);

  const openReviewDialog = (item: AdminInviteRecord, action: ReviewAction) => {
    setReviewDialog({ item, action });
    setReviewNote(action === 'approve' ? '核验通过，允许发放邀请奖励' : '');
  };

  const closeReviewDialog = () => {
    if (reviewingId) return;
    setReviewDialog(null);
    setReviewNote('');
  };

  const review = async () => {
    if (!reviewDialog) return;
    const { item, action } = reviewDialog;
    const note = reviewNote.trim();
    if (action === 'reject' && !note) {
      toast.error('驳回时需要填写原因');
      return;
    }
    setReviewingId(item.id);
    try {
      await portalApi.reviewInvite(item.id, action, note.trim());
      toast.success(action === 'approve' ? '审核通过，奖励已发放' : '已驳回并拦截奖励');
      setReviewDialog(null);
      setReviewNote('');
      await load();
    } catch (reviewError) {
      toast.error(reviewError instanceof Error ? reviewError.message : '审核操作失败');
    } finally {
      setReviewingId('');
    }
  };

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const metrics = [
    { label: '全部记录', value: summary.total, icon: Gift, tone: 'neutral' as const },
    { label: '已发放', value: summary.rewarded, icon: CheckCircle2, tone: 'green' as const },
    { label: '待人工审核', value: summary.review, icon: ShieldAlert, tone: 'amber' as const },
    { label: '待验证', value: summary.pending, icon: Clock3, tone: 'amber' as const },
    { label: '已拦截', value: summary.blocked, icon: ShieldAlert, tone: 'red' as const },
  ];

  return (
    <div className="space-y-5">
      <PageHeader title="邀请返利审计" description="查看邀请关系、双方奖励与风控拦截原因。">
        <button type="button" onClick={() => void load()} disabled={loading} title="刷新" className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white hover:border-[#12B76A] disabled:opacity-50">
          <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
        {metrics.map((metric) => <AdminMetricCard key={metric.label} title={metric.label} value={metric.value.toLocaleString()} icon={metric.icon} tone={metric.tone} />)}
      </div>

      {error && <div className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700">{error}</div>}

      <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[1320px] text-left text-xs">
            <thead className="bg-[#F7F8F6] text-[10px] text-zinc-500">
              <tr>
                {[
                  { key: 'inviter', label: '邀请人' },
                  { key: 'invitee', label: '被邀请人' },
                  { key: 'rewardCredits', label: '邀请人奖励' },
                  { key: 'inviteeRewardCredits', label: '新用户奖励' },
                  { key: 'rechargeRebateCredits', label: '充值返利' },
                  { key: 'status', label: '状态' },
                  { key: 'inviterIp', label: '邀请人IP', sortable: false },
                  { key: 'inviteeIp', label: '被邀请人IP', sortable: false },
                  { key: 'createdAt', label: '创建时间' },
                  { key: 'review', label: '审核', sortable: false },
                ].map((header) => (
                  <SortableHeader
                    key={header.key}
                    header={header}
                    sortState={sort}
                    onSort={(key) => {
                      const direction = sort.key === key && sort.direction === 'asc' ? 'desc' : 'asc';
                      setSort({ key, direction });
                      setPage(1);
                    }}
                  />
                ))}
              </tr>
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
                    <td className="px-4 py-3"><div className="flex min-w-[150px] gap-1.5">{item.status !== 'rewarded' && <><button type="button" disabled={reviewingId === item.id} onClick={() => openReviewDialog(item, 'approve')} className="rounded border border-emerald-200 bg-emerald-50 px-2 py-1 text-[10px] font-semibold text-emerald-700 transition-colors hover:border-emerald-300 hover:bg-emerald-100 disabled:opacity-50">{reviewingId === item.id ? '处理中' : '通过发放'}</button><button type="button" disabled={reviewingId === item.id} onClick={() => openReviewDialog(item, 'reject')} className="rounded border border-red-200 bg-red-50 px-2 py-1 text-[10px] font-semibold text-red-700 transition-colors hover:border-red-300 hover:bg-red-100 disabled:opacity-50">驳回</button></>}</div></td>
                  </tr>
                );
              })}
              {!loading && !items.length && <tr><td colSpan={10} className="px-4 py-14 text-center text-zinc-400"><UserPlus className="mx-auto mb-2 h-6 w-6" />暂无邀请记录</td></tr>}
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

      {reviewDialog && (
        <div
          className="fixed inset-0 z-[70] grid place-items-center bg-black/45 p-4 backdrop-blur-[1px]"
          role="presentation"
          onMouseDown={(event) => { if (event.target === event.currentTarget) closeReviewDialog(); }}
        >
          <section className="w-full max-w-md overflow-hidden rounded-lg border border-[#DCE4DF] bg-white shadow-2xl" role="dialog" aria-modal="true" aria-labelledby="invite-review-title">
            <div className={`border-l-4 px-5 py-4 ${reviewDialog.action === 'approve' ? 'border-l-emerald-500' : 'border-l-red-500'}`}>
              <div className="flex items-start gap-3">
                <span className={`grid h-9 w-9 shrink-0 place-items-center rounded-md ${reviewDialog.action === 'approve' ? 'bg-emerald-50 text-emerald-700' : 'bg-red-50 text-red-700'}`}>
                  {reviewDialog.action === 'approve' ? <ShieldCheck className="h-5 w-5" /> : <ShieldAlert className="h-5 w-5" />}
                </span>
                <div className="min-w-0 flex-1">
                  <h2 id="invite-review-title" className="text-sm font-semibold text-[#17201B]">{reviewDialog.action === 'approve' ? '通过邀请审核' : '驳回邀请审核'}</h2>
                  <p className="mt-1 truncate text-[11px] text-zinc-500">{reviewDialog.item.inviteeEmail || reviewDialog.item.inviteeId}</p>
                </div>
                <button type="button" onClick={closeReviewDialog} disabled={Boolean(reviewingId)} aria-label="关闭" className="grid h-7 w-7 place-items-center rounded-md text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-700 disabled:opacity-40">×</button>
              </div>
              <label className="mt-4 block text-[11px] font-semibold text-zinc-600" htmlFor="invite-review-note">{reviewDialog.action === 'approve' ? '审核备注' : '驳回原因'}{reviewDialog.action === 'approve' && <span className="ml-1 font-normal text-zinc-400">（可直接确认）</span>}</label>
              <div className="relative mt-1.5">
                <MessageSquareText className="pointer-events-none absolute left-3 top-3 h-4 w-4 text-zinc-400" />
                <textarea
                  id="invite-review-note"
                  autoFocus
                  rows={4}
                  value={reviewNote}
                  onChange={(event) => setReviewNote(event.target.value)}
                  placeholder={reviewDialog.action === 'approve' ? '补充本次审核依据（可选）' : '请填写驳回原因'}
                  disabled={Boolean(reviewingId)}
                  className="w-full resize-none rounded-md border border-[#CBD8D0] bg-[#FBFCFB] py-2.5 pl-9 pr-3 text-xs leading-5 text-[#17201B] outline-none transition-colors placeholder:text-zinc-400 focus:border-[#12B76A] focus:bg-white focus:ring-2 focus:ring-emerald-100 disabled:cursor-not-allowed disabled:opacity-60"
                />
              </div>
            </div>
            <div className="flex items-center justify-end gap-2 border-t border-[#EDF0EE] bg-[#F7F9F7] px-5 py-3.5">
              <button type="button" onClick={closeReviewDialog} disabled={Boolean(reviewingId)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-3.5 text-xs font-semibold text-zinc-600 transition-colors hover:bg-zinc-50 disabled:opacity-50">取消</button>
              <button type="button" onClick={() => void review()} disabled={Boolean(reviewingId) || (reviewDialog.action === 'reject' && !reviewNote.trim())} className={`inline-flex h-8 items-center gap-1.5 rounded-md px-3.5 text-xs font-semibold text-white transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${reviewDialog.action === 'approve' ? 'bg-[#047857] hover:bg-[#036749]' : 'bg-[#C62828] hover:bg-[#A61F1F]'}`}>
                {reviewingId ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : reviewDialog.action === 'approve' ? <ShieldCheck className="h-3.5 w-3.5" /> : <ShieldAlert className="h-3.5 w-3.5" />}
                {reviewingId ? '处理中' : reviewDialog.action === 'approve' ? '确认通过并发放' : '确认驳回'}
              </button>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
