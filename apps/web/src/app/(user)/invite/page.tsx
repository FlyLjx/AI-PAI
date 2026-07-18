'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { CheckCircle2, Clock3, Copy, Gift, Link2, LoaderCircle, RefreshCw, ShieldCheck, UserPlus, WalletCards, XCircle } from 'lucide-react';
import { toast } from 'sonner';
import { PageHeader } from '@/components/common/PageHeader';
import { StatBlock } from '@/components/common/StatBlock';
import { formatCNY, formatDate } from '@/lib/common/utils';
import { getSession, portalApi, type InviteRecord, type InviteSummary } from '@/lib/portal-api';

function inviteStatus(status: string) {
  if (status === 'rewarded') return { label: '已发放', className: 'success', icon: CheckCircle2 };
  if (status === 'blocked') return { label: '已拦截', className: 'failed', icon: XCircle };
  return { label: '待验证', className: 'pending', icon: Clock3 };
}

function rewardText(record: InviteRecord, side: 'inviter' | 'invitee'): string {
  const type = side === 'inviter' ? record.rewardType : record.inviteeRewardType;
  const credits = side === 'inviter' ? record.rewardCredits : record.inviteeRewardCredits;
  const label = side === 'inviter' ? record.rewardLabel : record.inviteeRewardLabel;
  if (type === 'balance') return formatCNY(Number(credits || 0));
  if (type === 'subscription') return label || '订阅权益';
  return '-';
}

export default function InvitePage() {
  const [user] = useState(() => getSession());
  const [summary, setSummary] = useState<InviteSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    if (!user) return;
    setLoading(true);
    setError('');
    try {
      setSummary((await portalApi.inviteSummary(user)).data);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '邀请数据加载失败');
    } finally {
      setLoading(false);
    }
  }, [user]);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  const inviteLink = useMemo(() => {
    if (!summary?.inviteCode || typeof window === 'undefined') return '';
    return `${window.location.origin}/register?invite=${encodeURIComponent(summary.inviteCode)}`;
  }, [summary?.inviteCode]);

  const copy = async (value: string, label: string) => {
    if (!value) return;
    await navigator.clipboard.writeText(value);
    toast.success(`${label}已复制`);
  };

  return (
    <div className="page-stack">
      <PageHeader title="邀请返利" description="好友完成注册并验证邮箱后，双方奖励自动到账。">
        <button className="btn" type="button" onClick={() => void load()} disabled={loading}><RefreshCw size={14} className={loading ? 'animate-spin' : ''} />刷新</button>
      </PageHeader>

      {error && <div className="notice border-red-200 bg-red-50 text-red-700" role="alert">{error}</div>}

      {loading && !summary ? (
        <div className="section-panel grid min-h-[320px] place-items-center text-zinc-400"><LoaderCircle className="animate-spin" size={24} /></div>
      ) : summary ? (
        <>
          <section className="metric-grid" aria-label="邀请奖励统计">
            <StatBlock title="成功邀请" value={Number(summary.inviteCount || 0).toLocaleString()} subtext={`${Number(summary.pendingCount || 0)} 个待验证`} icon={UserPlus} color="green" />
            <StatBlock title="余额奖励" value={formatCNY(Number(summary.totalBalanceRewards || 0))} subtext="邀请人累计到账" icon={WalletCards} color="cyan" />
            <StatBlock title="订阅奖励" value={Number(summary.totalSubscriptionRewards || 0).toLocaleString()} subtext="累计发放次数" icon={Gift} color="amber" />
            <StatBlock title="风控拦截" value={Number(summary.blockedCount || 0).toLocaleString()} subtext="异常邀请不发放奖励" icon={ShieldCheck} color="neutral" />
          </section>

          <section className="section-panel overflow-hidden">
            <header className="section-head"><div><strong>专属邀请链接</strong><small className="ml-2">完成邮箱验证后发奖</small></div><span className={`status-pill ${summary.enabled ? 'success' : 'failed'}`}>{summary.enabled ? '活动进行中' : '活动已暂停'}</span></header>
            <div className="grid gap-4 p-4 lg:grid-cols-[minmax(0,1fr)_320px]">
              <div className="min-w-0">
                <span className="text-[10px] font-semibold text-zinc-400">邀请链接</span>
                <div className="mt-1.5 flex min-w-0 items-center gap-2 rounded-[7px] border border-[#dce4df] bg-[#fafbf9] p-2"><Link2 size={15} className="shrink-0 text-[#087443]" /><code className="mono min-w-0 flex-1 truncate text-[11px]">{inviteLink || '-'}</code><button className="btn h-8 shrink-0" type="button" onClick={() => void copy(inviteLink, '邀请链接')}><Copy size={13} />复制</button></div>
                <div className="mt-3 flex items-center gap-2 text-[11px] text-zinc-500"><span>邀请码</span><code className="mono rounded bg-[#edf9f1] px-2 py-1 font-bold text-[#087443]">{summary.inviteCode || '-'}</code><button type="button" className="text-[#087443]" onClick={() => void copy(summary.inviteCode, '邀请码')}>复制</button></div>
              </div>
              <div className="grid grid-cols-2 divide-x divide-[#dce4df] rounded-[7px] border border-[#dce4df] bg-white">
                <div className="p-3"><span className="text-[10px] text-zinc-400">你可获得</span><strong className="mt-1 block text-[12px] text-[#087443]">{summary.rewardText || '暂未配置'}</strong></div>
                <div className="p-3"><span className="text-[10px] text-zinc-400">好友可获得</span><strong className="mt-1 block text-[12px] text-[#2563eb]">{summary.inviteeRewardText || '暂未配置'}</strong></div>
              </div>
            </div>
          </section>

          <section className="section-panel overflow-hidden">
            <header className="section-head"><div><strong>邀请记录</strong><small className="ml-2">最近 50 条</small></div><Gift size={16} className="text-[#087443]" /></header>
            <div className="overflow-x-auto">
              <table className="data-table min-w-[760px]">
                <thead><tr><th>好友</th><th>你的奖励</th><th>好友奖励</th><th>状态</th><th>时间</th></tr></thead>
                <tbody>
                  {(summary.records || []).map((record) => {
                    const status = inviteStatus(record.status);
                    const Icon = status.icon;
                    return <tr key={record.id}><td><strong className="block text-[12px]">{record.inviteeEmail || record.inviteeId}</strong></td><td>{rewardText(record, 'inviter')}</td><td>{rewardText(record, 'invitee')}</td><td><span className={`status-pill ${status.className}`} title={record.riskReason || ''}><Icon size={12} />{status.label}</span>{record.riskReason && <small className="mt-1 block max-w-[220px] truncate text-red-600">{record.riskReason}</small>}</td><td className="mono text-[10px] text-zinc-500">{formatDate(record.rewardedAt || record.createdAt)}</td></tr>;
                  })}
                  {!summary.records?.length && <tr><td colSpan={5} className="py-12 text-center text-zinc-400">还没有邀请记录</td></tr>}
                </tbody>
              </table>
            </div>
          </section>
        </>
      ) : null}
    </div>
  );
}
