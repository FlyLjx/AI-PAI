'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  CheckCircle2,
  Clock3,
  Copy,
  Gift,
  LoaderCircle,
  Percent,
  ReceiptText,
  RefreshCw,
  Search,
  ShieldCheck,
  UserPlus,
  WalletCards,
  XCircle,
  type LucideIcon,
} from 'lucide-react';
import { toast } from 'sonner';
import { AppSelect, type AppSelectOption } from '@/components/common/AppSelect';
import { DataTable } from '@/components/common/DataTable';
import { formatCNY, formatDate } from '@/lib/common/utils';
import {
  getSession,
  portalApi,
  type InviteRecord,
  type InviteSummary,
} from '@/lib/portal-api';

const INVITE_TABLE_HEADERS = [
  { key: 'invitee', label: '好友账号' },
  { key: 'registered', label: '注册时间', className: 'is-centered' },
  { key: 'status', label: '验证状态', className: 'is-centered' },
  { key: 'inviter-reward', label: '我的奖励', className: 'is-centered' },
  { key: 'invitee-reward', label: '好友奖励', className: 'is-centered' },
  { key: 'rebate', label: '充值返利', className: 'is-centered' },
  { key: 'rewarded', label: '发放时间', className: 'is-centered' },
];

const REBATE_TABLE_HEADERS = [
  { key: 'invitee', label: '好友账号' },
  { key: 'order', label: '来源订单' },
  { key: 'amount', label: '实付金额', className: 'is-centered' },
  { key: 'rate', label: '返利比例', className: 'is-centered' },
  { key: 'credits', label: '到账余额', className: 'is-centered' },
  { key: 'time', label: '结算时间', className: 'is-centered' },
];

const INVITE_STATUS_OPTIONS: readonly AppSelectOption[] = [
  { value: '', label: '全部状态' },
  { value: 'rewarded', label: '已发放' },
  { value: 'pending', label: '待验证' },
  { value: 'blocked', label: '已拦截' },
];

const REBATE_TYPE_OPTIONS: readonly AppSelectOption[] = [
  { value: '', label: '全部类型' },
  { value: 'recharge', label: '余额充值' },
  { value: 'subscription', label: '订阅订单' },
];

type RecordTab = 'invites' | 'rebates';
type MetricTone = 'green' | 'amber' | 'danger';

type InviteMetric = {
  label: string;
  value: string;
  detail: string;
  icon: LucideIcon;
  tone: MetricTone;
};

function inviteStatus(status: string) {
  const normalized = String(status || '').toLowerCase();
  if (normalized === 'rewarded') return { label: '已发放', className: 'success', icon: CheckCircle2 };
  if (normalized === 'blocked') return { label: '已拦截', className: 'failed', icon: XCircle };
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

function percentage(value: number): string {
  return Number(value || 0).toFixed(2).replace(/\.00$/, '').replace(/(\.\d)0$/, '$1');
}

function inviteRebateText(record: InviteRecord, rebatePercent: number): string {
  const credits = Number(record.rechargeRebateCredits || 0);
  if (credits <= 0) return '-';
  const rate = rebatePercent > 0 ? ` (${percentage(rebatePercent)}%)` : '';
  return `${formatCNY(credits)}${rate}`;
}

function InviteMetricCard({ metric }: { metric: InviteMetric }) {
  const Icon = metric.icon;
  return (
    <article className={`invite-metric-card is-${metric.tone}`}>
      <span className="invite-metric-icon"><Icon size={24} strokeWidth={1.65} /></span>
      <span className="invite-metric-copy">
        <strong>{metric.label}</strong>
        <b>{metric.value}</b>
        <small>{metric.detail}</small>
      </span>
    </article>
  );
}

export default function InvitePage() {
  const [user] = useState(() => getSession());
  const [summary, setSummary] = useState<InviteSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [recordTab, setRecordTab] = useState<RecordTab>('invites');
  const [recordQuery, setRecordQuery] = useState('');
  const [inviteStatusFilter, setInviteStatusFilter] = useState('');
  const [rebateTypeFilter, setRebateTypeFilter] = useState('');

  const load = useCallback(async () => {
    if (!user) {
      setError('登录状态已失效，请重新登录');
      setLoading(false);
      return;
    }
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

  const inviteRecords = useMemo(() => summary?.records || [], [summary?.records]);
  const rebateRecords = useMemo(() => summary?.rebateRecords || [], [summary?.rebateRecords]);
  const normalizedQuery = recordQuery.trim().toLowerCase();
  const filteredInviteRecords = useMemo(() => inviteRecords.filter((record) => {
    const matchesQuery = !normalizedQuery || [record.inviteeEmail, record.inviteeId, record.riskReason]
      .some((value) => String(value || '').toLowerCase().includes(normalizedQuery));
    const matchesStatus = !inviteStatusFilter || String(record.status || '').toLowerCase() === inviteStatusFilter;
    return matchesQuery && matchesStatus;
  }), [inviteRecords, inviteStatusFilter, normalizedQuery]);
  const filteredRebateRecords = useMemo(() => rebateRecords.filter((record) => {
    const matchesQuery = !normalizedQuery || [record.inviteeEmail, record.inviteeId, record.orderId, record.outTradeNo]
      .some((value) => String(value || '').toLowerCase().includes(normalizedQuery));
    const matchesType = !rebateTypeFilter || String(record.orderType || '').toLowerCase() === rebateTypeFilter;
    return matchesQuery && matchesType;
  }), [normalizedQuery, rebateRecords, rebateTypeFilter]);

  const showRebateTab = Boolean(summary?.rechargeRebateEnabled || rebateRecords.length > 0);
  const activeTab: RecordTab = recordTab === 'rebates' && showRebateTab ? 'rebates' : 'invites';

  const metrics: InviteMetric[] = summary ? [
    { label: '成功邀请', value: Number(summary.inviteCount || 0).toLocaleString('zh-CN'), detail: `${Number(summary.pendingCount || 0)} 个待验证`, icon: UserPlus, tone: 'green' },
    { label: '余额奖励', value: formatCNY(Number(summary.totalBalanceRewards || 0)), detail: '邀请人累计到账', icon: WalletCards, tone: 'green' },
    { label: '订阅奖励', value: Number(summary.totalSubscriptionRewards || 0).toLocaleString('zh-CN'), detail: '累计发放次数', icon: Gift, tone: 'green' },
    { label: '充值返利', value: formatCNY(Number(summary.rechargeRebateTotal || 0)), detail: `${Number(summary.rechargeRebateCount || 0)} 笔已结算`, icon: Percent, tone: 'amber' },
    { label: '风控拦截', value: Number(summary.blockedCount || 0).toLocaleString('zh-CN'), detail: '异常邀请', icon: ShieldCheck, tone: 'danger' },
  ] : [];

  const copy = async (value: string, label: string) => {
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      toast.success(`${label}已复制`);
    } catch {
      toast.error('复制失败，请手动选择内容');
    }
  };

  const updateQuery = (value: string) => {
    setRecordQuery(value);
  };

  const selectTab = (tab: RecordTab) => {
    setRecordTab(tab);
    setRecordQuery('');
  };

  return (
    <div className="invite-page">
      {error && <div className="invite-error" role="alert">{error}</div>}

      {loading && !summary ? (
        <section className="invite-loading-panel" aria-live="polite">
          <LoaderCircle size={24} className="animate-spin" />
          <span>正在读取邀请数据...</span>
        </section>
      ) : summary ? (
        <>
          <section className="invite-metrics-grid" aria-label="邀请奖励统计">
            {metrics.map((metric) => <InviteMetricCard key={metric.label} metric={metric} />)}
          </section>

          <section className="invite-link-card" aria-labelledby="invite-link-title">
            <div className="invite-link-main">
              <header className="invite-link-header">
                <strong id="invite-link-title">专属邀请链接</strong>
                <span className={`invite-activity-state ${summary.enabled ? 'is-active' : 'is-paused'}`}><i aria-hidden="true" />{summary.enabled ? '活动进行中' : '活动已暂停'}</span>
                <button className="invite-refresh-button" type="button" onClick={() => void load()} disabled={loading} title="刷新邀请数据" aria-label="刷新邀请数据"><RefreshCw size={14} className={loading ? 'animate-spin' : ''} /></button>
              </header>

              <div className="invite-copy-field">
                <label htmlFor="invite-link-value">邀请链接</label>
                <div className="invite-copy-control is-link">
                  <code id="invite-link-value" title={inviteLink}>{inviteLink || '-'}</code>
                  <button type="button" onClick={() => void copy(inviteLink, '邀请链接')} disabled={!inviteLink}><Copy size={15} />复制链接</button>
                </div>
              </div>

              <div className="invite-copy-field">
                <label htmlFor="invite-code-value">邀请码</label>
                <div className="invite-copy-control is-code">
                  <code id="invite-code-value">{summary.inviteCode || '-'}</code>
                  <button type="button" onClick={() => void copy(summary.inviteCode, '邀请码')} disabled={!summary.inviteCode} title="复制邀请码" aria-label="复制邀请码"><Copy size={16} /></button>
                </div>
              </div>
            </div>

            <div className="invite-reward-rules" aria-label="邀请奖励规则">
              <div className="invite-reward-row">
                <span><strong>我的邀请奖励</strong><small>好友完成注册并通过验证后发放</small></span>
                <b title={summary.rewardText || '暂未配置'}>{summary.rewardText || '暂未配置'}</b>
              </div>
              <div className="invite-reward-row">
                <span><strong>好友注册奖励</strong><small>好友注册成功后获得</small></span>
                <b title={summary.inviteeRewardText || '暂未配置'}>{summary.inviteeRewardText || '暂未配置'}</b>
              </div>
              {summary.rechargeRebateEnabled && (
                <div className="invite-reward-row">
                  <span><strong>好友充值返利</strong><small>{summary.rebateIncludeSubscriptions ? '好友充值与订阅后按比例返利' : '好友每次充值后按比例返利'}</small></span>
                  <b>{percentage(Number(summary.rechargeRebatePercent || 0))}%</b>
                </div>
              )}
            </div>
          </section>

          <section className="invite-records-card" aria-label="邀请与返利记录">
            <header className="invite-records-head">
              <div className="invite-record-tabs" role="tablist" aria-label="记录类型">
                <button type="button" role="tab" aria-selected={activeTab === 'invites'} className={activeTab === 'invites' ? 'is-active' : ''} onClick={() => selectTab('invites')}>邀请记录</button>
                {showRebateTab && <button type="button" role="tab" aria-selected={activeTab === 'rebates'} className={activeTab === 'rebates' ? 'is-active' : ''} onClick={() => selectTab('rebates')}>充值返利记录</button>}
              </div>
              <div className="invite-record-filters">
                <label className="invite-record-search">
                  <Search size={14} aria-hidden="true" />
                  <input value={recordQuery} onChange={(event) => updateQuery(event.target.value)} placeholder={activeTab === 'invites' ? '搜索好友' : '搜索好友 / 订单号'} aria-label={activeTab === 'invites' ? '搜索好友' : '搜索好友或订单号'} />
                </label>
                <AppSelect
                  id="invite-record-filter"
                  className="invite-record-select"
                  value={activeTab === 'invites' ? inviteStatusFilter : rebateTypeFilter}
                  options={activeTab === 'invites' ? INVITE_STATUS_OPTIONS : REBATE_TYPE_OPTIONS}
                  onValueChange={(value) => {
                    if (activeTab === 'invites') {
                      setInviteStatusFilter(value);
                    } else {
                      setRebateTypeFilter(value);
                    }
                  }}
                />
              </div>
            </header>

            {activeTab === 'invites' ? (
              <DataTable
                embedded
                className="invite-data-table"
                headers={INVITE_TABLE_HEADERS}
                data={filteredInviteRecords}
                loading={loading}
                emptyState={<div className="invite-table-empty"><UserPlus size={20} /><strong>{recordQuery || inviteStatusFilter ? '没有匹配的邀请记录' : '还没有邀请记录'}</strong></div>}
                tableWrapClassName="invite-record-table-wrap"
                tableClassName="invite-record-table"
                mobileListClassName="invite-mobile-list"
                renderRow={(record) => {
                  const status = inviteStatus(record.status);
                  return (
                    <tr key={record.id}>
                      <td><strong className="invite-account" title={record.inviteeEmail || record.inviteeId}>{record.inviteeEmail || record.inviteeId}</strong></td>
                      <td className="is-centered"><time>{formatDate(record.createdAt)}</time></td>
                      <td className="is-centered"><span className={`invite-record-status ${status.className}`}>{status.label}</span>{record.riskReason && <small className="invite-risk-reason" title={record.riskReason}>{record.riskReason}</small>}</td>
                      <td className="is-centered invite-reward-value">{rewardText(record, 'inviter')}</td>
                      <td className="is-centered invite-reward-value">{rewardText(record, 'invitee')}</td>
                      <td className="is-centered invite-rebate-value">{inviteRebateText(record, Number(summary.rechargeRebatePercent || 0))}</td>
                      <td className="is-centered"><time>{record.rewardedAt ? formatDate(record.rewardedAt) : '-'}</time></td>
                    </tr>
                  );
                }}
                renderMobileItem={(record) => {
                  const status = inviteStatus(record.status);
                  return (
                    <article className="invite-mobile-record" key={record.id}>
                      <header><strong>{record.inviteeEmail || record.inviteeId}</strong><span className={`invite-record-status ${status.className}`}>{status.label}</span></header>
                      <dl>
                        <div><dt>注册时间</dt><dd>{formatDate(record.createdAt)}</dd></div>
                        <div><dt>发放时间</dt><dd>{record.rewardedAt ? formatDate(record.rewardedAt) : '-'}</dd></div>
                        <div><dt>我的奖励</dt><dd>{rewardText(record, 'inviter')}</dd></div>
                        <div><dt>好友奖励</dt><dd>{rewardText(record, 'invitee')}</dd></div>
                        <div className="is-wide"><dt>充值返利</dt><dd>{inviteRebateText(record, Number(summary.rechargeRebatePercent || 0))}</dd></div>
                      </dl>
                      {record.riskReason && <p>{record.riskReason}</p>}
                    </article>
                  );
                }}
              />
            ) : (
              <DataTable
                embedded
                className="invite-data-table"
                headers={REBATE_TABLE_HEADERS}
                data={filteredRebateRecords}
                loading={loading}
                emptyState={<div className="invite-table-empty"><ReceiptText size={20} /><strong>{recordQuery || rebateTypeFilter ? '没有匹配的返利记录' : '还没有充值返利记录'}</strong></div>}
                tableWrapClassName="invite-record-table-wrap"
                tableClassName="invite-rebate-table"
                mobileListClassName="invite-mobile-list"
                renderRow={(record) => (
                  <tr key={record.id}>
                    <td><strong className="invite-account" title={record.inviteeEmail || record.inviteeId}>{record.inviteeEmail || record.inviteeId}</strong></td>
                    <td><span className="invite-order-cell"><strong>{record.orderType === 'subscription' ? '订阅订单' : '余额充值'}</strong><code title={record.outTradeNo || record.orderId}>{record.outTradeNo || record.orderId}</code></span></td>
                    <td className="is-centered">{formatCNY(Number(record.orderAmount || 0))}</td>
                    <td className="is-centered invite-rate-value">{percentage(Number(record.rebatePercent || 0))}%</td>
                    <td className="is-centered invite-rebate-value">{formatCNY(Number(record.rebateCredits || 0))}</td>
                    <td className="is-centered"><time>{formatDate(record.createdAt)}</time></td>
                  </tr>
                )}
                renderMobileItem={(record) => (
                  <article className="invite-mobile-record" key={record.id}>
                    <header><strong>{record.inviteeEmail || record.inviteeId}</strong><span className="invite-record-status success">已结算</span></header>
                    <dl>
                      <div><dt>订单类型</dt><dd>{record.orderType === 'subscription' ? '订阅订单' : '余额充值'}</dd></div>
                      <div><dt>实付金额</dt><dd>{formatCNY(Number(record.orderAmount || 0))}</dd></div>
                      <div><dt>返利比例</dt><dd>{percentage(Number(record.rebatePercent || 0))}%</dd></div>
                      <div><dt>到账余额</dt><dd>{formatCNY(Number(record.rebateCredits || 0))}</dd></div>
                      <div className="is-wide"><dt>结算时间</dt><dd>{formatDate(record.createdAt)}</dd></div>
                    </dl>
                    <code>{record.outTradeNo || record.orderId}</code>
                  </article>
                )}
              />
            )}
          </section>
        </>
      ) : null}
    </div>
  );
}
