'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  BadgeCheck,
  CalendarDays,
  CheckCircle2,
  CircleDollarSign,
  Clock3,
  Crown,
  LoaderCircle,
  QrCode,
  ReceiptText,
  RefreshCw,
  ShieldCheck,
  WalletCards,
  X,
  XCircle,
} from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import { toast } from 'sonner';
import { DataTable } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { StatBlock } from '@/components/common/StatBlock';
import {
  APIError,
  getSession,
  portalApi,
  refreshSession,
  type Plan,
  type PortalUser,
  type RechargeOrder,
  type Subscription,
} from '@/lib/portal-api';
import { formatCNY, formatDate } from '@/lib/common/utils';

type BillingTab = 'balance' | 'subscription';

const HISTORY_PAGE_SIZE = 10;

type PaymentOrder = {
  id: string;
  outTradeNo: string;
  orderType: string;
  subscriptionPlanId?: string;
  amount: number;
  status: string;
  payUrl?: string;
  qrCode?: string;
  paidAt?: string;
  createdAt?: string;
};

function errorMessage(error: unknown): string {
  return error instanceof APIError || error instanceof Error ? error.message : '支付请求失败，请稍后重试';
}

function textValue(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function paymentFromRecord(value: Record<string, unknown>): PaymentOrder {
  return {
    id: textValue(value.id),
    outTradeNo: textValue(value.outTradeNo),
    orderType: textValue(value.orderType) || 'recharge',
    subscriptionPlanId: textValue(value.subscriptionPlanId) || undefined,
    amount: Number(value.amount || 0),
    status: textValue(value.status) || 'pending',
    payUrl: textValue(value.payUrl) || undefined,
    qrCode: textValue(value.qrCode) || undefined,
    paidAt: textValue(value.paidAt) || undefined,
    createdAt: textValue(value.createdAt) || undefined,
  };
}

function parsePresets(value: unknown): number[] {
  const parsed = String(value || '10,30,50,100')
    .split(',')
    .map((item) => Number(item.trim()))
    .filter((item) => Number.isFinite(item) && item > 0);
  return parsed.length ? Array.from(new Set(parsed)).slice(0, 8) : [10, 30, 50, 100];
}

function paidStatus(status: string): boolean {
  return ['paid', 'success', 'succeeded', 'completed'].includes(status.toLowerCase());
}

function failedStatus(status: string): boolean {
  return ['failed', 'closed', 'cancelled', 'canceled', 'expired'].includes(status.toLowerCase());
}

function orderTypeLabel(orderType: string): string {
  return orderType.toLowerCase() === 'subscription' ? '订阅订单' : '余额充值';
}

function orderStatus(status: string): { label: string; className: string } {
  if (paidStatus(status)) return { label: '已支付', className: 'paid' };
  if (failedStatus(status)) return { label: '已关闭', className: 'failed' };
  return { label: '待支付', className: 'pending' };
}

export default function BillingPage() {
  const [user, setUser] = useState<PortalUser | null>(null);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [subscription, setSubscription] = useState<Subscription | null>(null);
  const [subscriptionEnabled, setSubscriptionEnabled] = useState(false);
  const [settings, setSettings] = useState<Record<string, unknown>>({});
  const [activeTab, setActiveTab] = useState<BillingTab>('balance');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [selectedAmount, setSelectedAmount] = useState(50);
  const [customAmount, setCustomAmount] = useState('');
  const [payingFor, setPayingFor] = useState('');
  const [paymentOrder, setPaymentOrder] = useState<PaymentOrder | null>(null);
  const [paymentTitle, setPaymentTitle] = useState('');
  const [paymentOpen, setPaymentOpen] = useState(false);
  const [paymentError, setPaymentError] = useState('');
  const [syncing, setSyncing] = useState(false);

  const [orderHistory, setOrderHistory] = useState<RechargeOrder[]>([]);
  const [historyPage, setHistoryPage] = useState(1);
  const [historyTotal, setHistoryTotal] = useState(0);
  const [historyLoading, setHistoryLoading] = useState(true);
  const [historyError, setHistoryError] = useState('');
  const [reopeningOrderId, setReopeningOrderId] = useState('');

  const loadBilling = useCallback(async (silent = false) => {
    const current = getSession();
    if (!current) {
      setError('登录状态已失效，请重新登录');
      if (!silent) setLoading(false);
      return;
    }
    if (!silent) setLoading(true);
    setError('');
    setSubscriptionEnabled(false);
    setPlans([]);
    setSubscription(null);
    const results = await Promise.allSettled([
      refreshSession(current),
      portalApi.plans(current),
      portalApi.subscription(current),
      portalApi.publicSettings(),
    ]);
    const [userResult, plansResult, subscriptionResult, settingsResult] = results;
    setUser(userResult.status === 'fulfilled' ? userResult.value : current);
    if (plansResult.status === 'fulfilled') {
      setSubscriptionEnabled(true);
      setPlans((plansResult.value.data || []).filter((plan) => plan.status === 'active'));
    }
    if (subscriptionResult.status === 'fulfilled') {
      setSubscriptionEnabled(true);
      setSubscription(subscriptionResult.value.data);
    }
    if (settingsResult.status === 'fulfilled') setSettings(settingsResult.value.data || {});
    const failure = results.find((result, index) => {
      if (result.status !== 'rejected') return false;
      return (index !== 1 && index !== 2) || !(result.reason instanceof APIError && result.reason.status === 403);
    });
    if (failure?.status === 'rejected') setError(errorMessage(failure.reason));
    if (!silent) setLoading(false);
  }, []);

  const loadOrderHistory = useCallback(async (page = 1, current = getSession(), silent = false) => {
    if (!current) {
      setHistoryError('登录状态已失效，请重新登录');
      if (!silent) setHistoryLoading(false);
      return;
    }
    setHistoryPage(page);
    if (!silent) setHistoryLoading(true);
    setHistoryError('');
    setOrderHistory([]);
    try {
      const response = await portalApi.rechargeHistory(current, page, HISTORY_PAGE_SIZE);
      setOrderHistory(response.data || []);
      setHistoryTotal(response.pagination?.total || 0);
      setHistoryPage(response.pagination?.page || page);
    } catch (historyLoadError) {
      setHistoryError(errorMessage(historyLoadError));
    } finally {
      if (!silent) setHistoryLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadBilling();
      void loadOrderHistory(1);
    }, 0);
    return () => window.clearTimeout(timer);
  }, [loadBilling, loadOrderHistory]);

  const presets = useMemo(() => parsePresets(settings.rechargePresets), [settings.rechargePresets]);
  const rechargeEnabled = settings.rechargeEnabled !== false;
  const minimumAmount = Math.max(1, Number(settings.rechargeMinAmount || 1));
  const configuredRechargeRate = Number(settings.rechargeRate);
  const rechargeRate = Number.isFinite(configuredRechargeRate) && configuredRechargeRate > 0 ? configuredRechargeRate : 10;
  const subscriptionActive = subscriptionEnabled && Boolean(subscription?.isPaid && subscription?.status === 'active');
  const remainingQuota = Number(subscription?.effectiveQuotaRemaining ?? subscription?.quotaRemaining ?? 0);
  const quotaLimit = Number(subscription?.quotaLimit ?? 0);
  const recommendedPlanId = useMemo(() => plans.reduce<Plan | null>((recommended, plan) => {
    if (!recommended || Number(plan.durationDays || 0) > Number(recommended.durationDays || 0)) return plan;
    return recommended;
  }, null)?.id, [plans]);
  const qrValue = paymentOrder?.qrCode || paymentOrder?.payUrl || '';

  const createPayment = async (input: { amount?: number; subscriptionPlanId?: string }, title: string, pendingKey: string) => {
    if (!user) return;
    if (input.subscriptionPlanId && !subscriptionEnabled) {
      toast.error('当前账号暂未开放订阅功能');
      return;
    }
    setPayingFor(pendingKey);
    setPaymentError('');
    try {
      const response = await portalApi.recharge(user, input);
      const order = paymentFromRecord(response.data);
      if (!order.id) throw new Error('支付订单创建失败');
      setPaymentOrder(order);
      setPaymentTitle(title);
      setPaymentOpen(true);
      void loadOrderHistory(1, user);
    } catch (createError) {
      toast.error(errorMessage(createError));
    } finally {
      setPayingFor('');
    }
  };

  const startBalancePayment = () => {
    const amount = customAmount.trim() ? Number(customAmount) : selectedAmount;
    if (!Number.isFinite(amount) || amount < minimumAmount) {
      toast.error(`单次充值金额不得低于 ${formatCNY(minimumAmount)}`);
      return;
    }
    void createPayment({ amount }, `余额充值 ${formatCNY(amount)}`, `balance-${amount}`);
  };

  const startPlanPayment = (plan: Plan) => {
    if (!subscriptionEnabled) return;
    void createPayment({ subscriptionPlanId: plan.id }, `${plan.name}订阅`, `plan-${plan.id}`);
  };

  const reopenPayment = async (order: RechargeOrder) => {
    if (!user || !order.id) return;
    setReopeningOrderId(order.id);
    setPaymentError('');
    try {
      const currentResponse = await portalApi.syncRecharge(user, order.id);
      const currentOrder = paymentFromRecord(currentResponse.data);
      if (!currentOrder.id) throw new Error('支付订单读取失败');
      if (paidStatus(currentOrder.status)) {
        toast.success(currentOrder.orderType === 'subscription' ? '订阅订单已支付' : '充值订单已支付');
        await Promise.all([loadBilling(true), loadOrderHistory(historyPage, user, true)]);
        return;
      }
      if (failedStatus(currentOrder.status)) {
        toast.error('订单已关闭，请重新创建支付订单');
        await loadOrderHistory(historyPage, user, true);
        return;
      }

      let createInput: { amount?: number; subscriptionPlanId?: string };
      if (currentOrder.orderType === 'subscription') {
        if (!currentOrder.subscriptionPlanId) throw new Error('订阅订单缺少套餐信息');
        createInput = { subscriptionPlanId: currentOrder.subscriptionPlanId };
      } else {
        createInput = { amount: currentOrder.amount };
      }

      const createResponse = await portalApi.recharge(user, createInput);
      const nextOrder = paymentFromRecord(createResponse.data);
      if (!nextOrder.id || (!nextOrder.qrCode && !nextOrder.payUrl)) throw new Error('新的支付二维码生成失败');
      setPaymentOrder(nextOrder);
      setPaymentTitle(`${nextOrder.orderType === 'subscription' ? '订阅订单' : '余额充值'} ${formatCNY(nextOrder.amount)}`);
      setPaymentOpen(true);
      toast.success('已生成新的支付二维码');
      void loadOrderHistory(1, user, true);
    } catch (reopenError) {
      toast.error(errorMessage(reopenError));
    } finally {
      setReopeningOrderId('');
    }
  };

  const syncPayment = useCallback(async (showFeedback = false) => {
    if (!user || !paymentOrder?.id || paidStatus(paymentOrder.status) || failedStatus(paymentOrder.status)) return;
    setSyncing(true);
    try {
      const response = await portalApi.syncRecharge(user, paymentOrder.id);
      const nextOrder = paymentFromRecord(response.data);
      setPaymentOrder(nextOrder);
      setPaymentError('');
      if (paidStatus(nextOrder.status)) {
        toast.success(nextOrder.orderType === 'subscription' ? '订阅已生效' : '余额已到账');
        await Promise.all([loadBilling(true), loadOrderHistory(1, user, true)]);
      } else if (showFeedback) {
        if (failedStatus(nextOrder.status)) {
          toast.error('订单已关闭，请重新创建支付订单');
        } else {
          toast.info('订单仍在等待支付');
        }
      }
    } catch (syncError) {
      const message = errorMessage(syncError);
      setPaymentError(message);
      if (showFeedback) toast.error(message);
    } finally {
      setSyncing(false);
    }
  }, [loadBilling, loadOrderHistory, paymentOrder, user]);

  useEffect(() => {
    if (!paymentOrder || paidStatus(paymentOrder.status) || failedStatus(paymentOrder.status)) return;
    const timer = window.setInterval(() => void syncPayment(false), 3000);
    return () => window.clearInterval(timer);
  }, [paymentOrder, syncPayment]);

  useEffect(() => {
    if (!paymentOpen || !paymentOrder || !paidStatus(paymentOrder.status)) return;
    const timer = window.setTimeout(() => setPaymentOpen(false), 1200);
    return () => window.clearTimeout(timer);
  }, [paymentOpen, paymentOrder]);

  const paymentState = paymentOrder
    ? paidStatus(paymentOrder.status) ? 'paid' : failedStatus(paymentOrder.status) ? 'failed' : 'pending'
    : 'pending';
  const historyTotalPages = Math.max(1, Math.ceil(historyTotal / HISTORY_PAGE_SIZE));
  const historyHeaders = [
    { key: 'type', label: '类型 / 订单号' },
    { key: 'amount', label: '金额' },
    { key: 'status', label: '状态' },
    { key: 'createdAt', label: '创建时间' },
    { key: 'paidAt', label: '支付时间' },
    { key: 'actions', label: '操作', className: 'text-right' },
  ];

  return (
    <div className="page-stack billing-page">
      {error && <div className="billing-notice" role="alert">部分计费信息暂未更新：{error}</div>}

      <section className="metric-grid billing-stats" aria-label="计费摘要">
        <StatBlock title="账户余额" value={loading && !user ? '--' : formatCNY(Number(user?.credits || 0))} subtext="按量请求自动扣减" icon={CircleDollarSign} color="green" />
        <StatBlock title="订阅状态" value={loading ? '--' : subscriptionEnabled ? subscriptionActive ? subscription?.planName || '已订阅' : '未订阅' : '未开放'} subtext={subscriptionEnabled ? subscriptionActive ? `有效至 ${subscription?.expiresAt ? formatDate(subscription.expiresAt, false) : '-'}` : '可独立购买套餐' : '当前未启用订阅'} icon={Crown} color="amber" />
        <StatBlock title="订阅剩余" value={loading ? '--' : subscriptionEnabled ? subscriptionActive ? remainingQuota.toLocaleString() : '0' : '--'} subtext={subscriptionEnabled ? subscriptionActive ? `总额度 ${quotaLimit.toLocaleString()}` : '未开通订阅额度' : '订阅功能未启用'} icon={WalletCards} color="cyan" />
        <StatBlock title="充值兑换" value={`1 : ${rechargeRate.toLocaleString()}`} subtext={`最低 ${formatCNY(minimumAmount)}`} icon={ShieldCheck} color="neutral" />
      </section>

      <div className="section-panel billing-workspace overflow-hidden">
        <div className={`billing-tabs ${subscriptionEnabled ? 'has-subscription' : ''}`}>
          <button
            type="button"
            className={`billing-tab ${activeTab === 'balance' ? 'is-active' : ''}`}
            onClick={() => setActiveTab('balance')}
            aria-pressed={activeTab === 'balance'}
          >
            <CircleDollarSign size={14} />余额充值
          </button>
          {subscriptionEnabled && (
            <button
              type="button"
              className={`billing-tab ${activeTab === 'subscription' ? 'is-active is-subscription' : ''}`}
              onClick={() => setActiveTab('subscription')}
              aria-pressed={activeTab === 'subscription'}
            >
              <Crown size={14} />订阅套餐
            </button>
          )}
        </div>

        {activeTab === 'balance' || !subscriptionEnabled ? (
          <div className="section-body billing-balance-body">
            <div className="billing-balance-main">
              <div className="billing-section-heading">
                <div><strong>选择充值金额</strong><p>到账余额用于 API 按量调用</p></div>
                <div className="billing-payment-methods">
                  <span className={`billing-availability ${rechargeEnabled ? 'is-ready' : 'is-disabled'}`}><i />{rechargeEnabled ? '充值可用' : '充值暂停'}</span>
                  <span className="billing-method"><BadgeCheck size={13} />支付宝扫码</span>
                </div>
              </div>
              <div className="billing-amount-grid">
                {presets.map((amount) => (
                  <button
                    key={amount}
                    type="button"
                    className={`billing-amount-option ${!customAmount && selectedAmount === amount ? 'is-selected' : ''}`}
                    onClick={() => { setSelectedAmount(amount); setCustomAmount(''); }}
                    aria-pressed={!customAmount && selectedAmount === amount}
                  >
                    <strong>{formatCNY(amount)}</strong>
                    <small>预计到账 {(amount * rechargeRate).toFixed(2)}</small>
                  </button>
                ))}
              </div>
              <div className="field billing-custom-amount">
                <label htmlFor="custom-recharge">自定义金额</label>
                <div className="billing-custom-row">
                  <input id="custom-recharge" type="number" min={minimumAmount} step="0.01" value={customAmount} onChange={(event) => setCustomAmount(event.target.value)} placeholder={`最低 ${minimumAmount}`} />
                  <span className="billing-currency">元</span>
                  <button className="btn primary billing-pay-button" type="button" onClick={startBalancePayment} disabled={!rechargeEnabled || Boolean(payingFor)}>
                    {payingFor.startsWith('balance-') && <LoaderCircle size={14} className="animate-spin" />}
                    支付充值
                  </button>
                </div>
              </div>
            </div>
            <aside className="billing-balance-aside">
              <div className="billing-aside-card">
                <header><strong>余额账户</strong></header>
                <div className="billing-aside-body">
                  <small>当前可用</small>
                  <strong>{formatCNY(Number(user?.credits || 0))}</strong>
                  <p>充值金额将存入您的余额账户，可用于调用 API 服务抵扣费用。</p>
                </div>
              </div>
            </aside>
          </div>
        ) : (
          <div className="section-body billing-subscription-body">
            <div className="billing-subscription-heading">
              <div className="min-w-0">
                <span className="inline-flex items-center gap-1.5 text-[11px] font-bold text-[#087443]"><Crown size={14} />订阅额度</span>
                <strong className="mt-1.5 block text-base">选择适合调用周期的套餐</strong>
                <p className="mt-1 max-w-xl text-[11px] leading-5 text-zinc-500">订阅有效期内优先使用套餐额度，额度用完后继续使用账户余额。</p>
              </div>
              <dl className="billing-subscription-summary" aria-label="当前计费状态">
                <div className="min-w-0 px-3 py-2.5">
                  <dt className="text-[10px] text-zinc-400">当前方案</dt>
                  <dd className={`mt-1 truncate text-xs font-bold ${subscriptionActive ? 'text-amber-700' : 'text-[#59645d]'}`}>{subscriptionActive ? subscription?.planName || '已订阅' : '暂未订阅'}</dd>
                </div>
                <div className="min-w-0 border-l border-[#e4e9e6] px-3 py-2.5">
                  <dt className="text-[10px] text-zinc-400">订阅可用</dt>
                  <dd className="mono mt-1 truncate text-xs font-bold text-[#17201b]">{subscriptionActive ? remainingQuota.toLocaleString() : '0'} 张</dd>
                </div>
                <div className="min-w-0 border-l border-[#e4e9e6] px-3 py-2.5">
                  <dt className="text-[10px] text-zinc-400">余额兜底</dt>
                  <dd className="mono mt-1 truncate text-xs font-bold text-[#17201b]">{formatCNY(Number(user?.credits || 0))}</dd>
                </div>
              </dl>
            </div>

            {loading && plans.length === 0 ? (
              <div className="empty-row">正在读取订阅套餐...</div>
            ) : plans.length === 0 ? (
              <div className="empty-row">暂无可购买的订阅套餐</div>
            ) : (
              <div className="billing-plan-grid">
                {plans.map((plan) => {
                  const currentPlan = subscriptionActive && subscription?.planId === plan.id;
                  const recommendedPlan = plan.id === recommendedPlanId;
                  const durationDays = Math.max(1, Number(plan.durationDays || 0));
                  const quotaImages = Math.max(0, Number(plan.quotaImages || 0));
                  const dailyAmount = Number(plan.amount || 0) / durationDays;
                  const dailyQuota = Math.floor(quotaImages / durationDays);
                  return (
                    <article
                      key={plan.id}
                      className={`billing-plan-card ${currentPlan ? 'is-current' : recommendedPlan ? 'is-recommended' : ''}`}
                      aria-current={currentPlan ? 'true' : undefined}
                    >
                      {recommendedPlan && !currentPlan && <span className="billing-plan-accent" aria-hidden="true" />}
                      <div className="flex items-start justify-between gap-4">
                        <div className="billing-plan-identity">
                          <span className={`billing-plan-icon ${currentPlan ? 'is-current' : ''}`}><Crown size={17} /></span>
                          <div className="min-w-0">
                            <strong className="block truncate text-[15px]">{plan.name}</strong>
                            <span className="mt-0.5 block text-[10px] text-zinc-400">{durationDays} 天订阅周期</span>
                          </div>
                        </div>
                        {currentPlan ? (
                          <span className="status-pill paid">当前订阅</span>
                        ) : recommendedPlan ? (
                          <span className="status-pill active shrink-0">推荐</span>
                        ) : plan.badge ? (
                          <span className="shrink-0 rounded bg-[#f1f3f2] px-1.5 py-1 text-[10px] font-bold text-[#59625d]">{plan.badge}</span>
                        ) : null}
                      </div>

                      <p className="billing-plan-description">{plan.description || '按周期提供稳定的 API 调用额度。'}</p>

                      <div className="billing-plan-price-row">
                        <div>
                          <span className="block text-[10px] text-zinc-400">订阅价格</span>
                          <div className="billing-plan-price"><strong>{formatCNY(Number(plan.amount || 0))}</strong><small>/ {durationDays} 天</small></div>
                        </div>
                        <div className="billing-plan-daily">
                          <span className="block text-[9px] text-zinc-400">日均价格</span>
                          <strong className="mono mt-0.5 block text-[11px] text-[#526059]">{formatCNY(dailyAmount)}</strong>
                        </div>
                      </div>

                      <div className="billing-plan-quota">
                        <div>
                          <span>套餐总额度</span>
                          <strong>{quotaImages.toLocaleString()} <small>张</small></strong>
                        </div>
                        <div className="billing-plan-meta">
                          <span><CalendarDays size={13} />有效 {durationDays} 天</span>
                          <span><WalletCards size={13} />日均约 {dailyQuota.toLocaleString()} 张</span>
                        </div>
                      </div>

                      <button className={`btn billing-plan-button ${currentPlan ? '' : 'primary'}`} type="button" onClick={() => startPlanPayment(plan)} disabled={Boolean(payingFor)}>
                        {payingFor === `plan-${plan.id}` && <LoaderCircle size={14} className="animate-spin" />}
                        {currentPlan ? '续订当前套餐' : `订阅 ${plan.name}`}
                      </button>
                    </article>
                  );
                })}
              </div>
            )}

            <div className="billing-subscription-notes" aria-label="订阅计费说明">
              <div><BadgeCheck /><span><strong>支付后自动生效</strong><small>支付完成后自动同步套餐状态</small></span></div>
              <div><WalletCards /><span><strong>订阅额度优先</strong><small>有效期内请求优先扣套餐额度</small></span></div>
              <div><ShieldCheck /><span><strong>余额自动衔接</strong><small>套餐不足时继续按量扣除余额</small></span></div>
            </div>
          </div>
        )}
      </div>

      <section className="billing-history-section" aria-labelledby="billing-history-title">
        <div className="billing-history-header">
          <div className="billing-history-title">
            <span className="billing-icon"><ReceiptText size={16} /></span>
            <div className="min-w-0">
              <strong id="billing-history-title" className="block text-sm">{subscriptionEnabled ? '充值与订阅记录' : '充值记录'}</strong>
              <small className="mt-0.5 block text-[11px] text-zinc-500">{subscriptionEnabled ? '余额充值与订阅订单' : '余额充值订单'}，共 {historyTotal.toLocaleString()} 条</small>
            </div>
          </div>
          <button className="btn billing-history-refresh" type="button" onClick={() => void loadOrderHistory(historyPage)} disabled={historyLoading}>
            <RefreshCw size={14} className={historyLoading ? 'animate-spin' : ''} />刷新记录
          </button>
        </div>

        {historyLoading ? (
          <div className="section-panel empty-row" role="status"><LoaderCircle size={14} className="mr-2 inline animate-spin" />正在读取订单记录...</div>
        ) : historyError ? (
          <div className="flex flex-col items-start gap-3 rounded-[7px] border border-red-200 bg-red-50 p-4 text-[11px] text-red-700 sm:flex-row sm:items-center" role="alert">
            <span className="min-w-0 flex-1">{subscriptionEnabled ? '充值与订阅记录暂未更新' : '充值记录暂未更新'}：{historyError}</span>
            <button className="btn shrink-0" type="button" onClick={() => void loadOrderHistory(historyPage)}>重新加载</button>
          </div>
        ) : (
          <div className="billing-history-table">
            <DataTable
            headers={historyHeaders}
            data={orderHistory}
            currentPage={historyPage}
            totalPages={historyTotalPages}
            totalItems={historyTotal}
            onPageChange={(page) => void loadOrderHistory(page)}
            renderRow={(order) => {
              const status = orderStatus(order.status);
              const canContinuePayment = !paidStatus(order.status) && !failedStatus(order.status);
              return (
                <tr key={order.id}>
                  <td className="px-4 py-3">
                    <strong className="block text-xs">{orderTypeLabel(order.orderType)}</strong>
                    <code className="mt-1 block max-w-[220px] truncate text-[10px] text-zinc-400" title={order.outTradeNo}>{order.outTradeNo || order.id}</code>
                  </td>
                  <td className="mono px-4 py-3 font-bold">{formatCNY(Number(order.amount || 0))}</td>
                  <td className="px-4 py-3"><span className={`status-pill ${status.className}`}>{status.label}</span></td>
                  <td className="mono whitespace-nowrap px-4 py-3 text-zinc-500">{formatDate(order.createdAt)}</td>
                  <td className="mono whitespace-nowrap px-4 py-3 text-zinc-500">{order.paidAt ? formatDate(order.paidAt) : '-'}</td>
                  <td className="px-4 py-3 text-right">
                    {canContinuePayment ? (
                      <button
                        type="button"
                        className="inline-flex h-7 items-center gap-1.5 rounded-md border border-[#b7e4c7] bg-[#f0fdf4] px-2.5 text-[11px] font-bold text-[#087443] hover:border-[#86efac] disabled:opacity-50"
                        onClick={() => void reopenPayment(order)}
                        disabled={Boolean(reopeningOrderId)}
                        title="重新打开支付二维码"
                      >
                        {reopeningOrderId === order.id ? <LoaderCircle size={12} className="animate-spin" /> : <QrCode size={12} />}
                        继续支付
                      </button>
                    ) : <span className="text-zinc-300">-</span>}
                  </td>
                </tr>
              );
            }}
            renderMobileItem={(order) => {
              const status = orderStatus(order.status);
              const canContinuePayment = !paidStatus(order.status) && !failedStatus(order.status);
              return (
                <article key={order.id} className="section-panel p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <strong className="block text-sm">{orderTypeLabel(order.orderType)}</strong>
                      <code className="mt-1 block truncate text-[10px] text-zinc-400">{order.outTradeNo || order.id}</code>
                    </div>
                    <span className={`status-pill ${status.className} shrink-0`}>{status.label}</span>
                  </div>
                  <div className="mono mt-3 text-xl font-bold text-[#17201b]">{formatCNY(Number(order.amount || 0))}</div>
                  <dl className="mt-3 grid grid-cols-2 gap-3 border-t border-[#edf0ee] pt-3 text-[11px]">
                    <div><dt className="text-zinc-400">创建时间</dt><dd className="mt-1 text-[#526059]">{formatDate(order.createdAt)}</dd></div>
                    <div><dt className="text-zinc-400">支付时间</dt><dd className="mt-1 text-[#526059]">{order.paidAt ? formatDate(order.paidAt) : '-'}</dd></div>
                  </dl>
                  {canContinuePayment && (
                    <button
                      type="button"
                      className="btn mt-3 w-full border-[#b7e4c7] bg-[#f0fdf4] text-[#087443]"
                      onClick={() => void reopenPayment(order)}
                      disabled={Boolean(reopeningOrderId)}
                    >
                      {reopeningOrderId === order.id ? <LoaderCircle size={13} className="animate-spin" /> : <QrCode size={13} />}
                      继续支付
                    </button>
                  )}
                </article>
              );
            }}
            emptyState={<EmptyState title="暂无订单记录" description={subscriptionEnabled ? '完成余额充值或购买订阅后，订单会显示在这里。' : '完成余额充值后，订单会显示在这里。'} icon={ReceiptText} />}
            />
          </div>
        )}
      </section>

      {paymentOpen && paymentOrder && (
        <div className="modal-backdrop billing-payment-backdrop" role="presentation">
          <section className="modal-panel billing-payment-modal max-w-[430px]" role="dialog" aria-modal="true" aria-labelledby="payment-title">
            <div className="modal-title">
              <strong id="payment-title">{paymentTitle}</strong>
              <button type="button" onClick={() => setPaymentOpen(false)} title="关闭" aria-label="关闭"><X size={17} /></button>
            </div>
            <div className="modal-content text-center">
              {paymentState === 'paid' ? (
                <div className="py-5">
                  <CheckCircle2 size={48} className="mx-auto text-[#087443]" />
                  <strong className="mt-3 block text-base">{paymentOrder.orderType === 'subscription' ? '订阅已生效' : '余额已到账'}</strong>
                  <p className="mt-1 text-xs text-zinc-500">支付金额 {formatCNY(paymentOrder.amount)}</p>
                  <button className="btn primary mt-5" type="button" onClick={() => setPaymentOpen(false)}>完成</button>
                </div>
              ) : paymentState === 'failed' ? (
                <div className="py-5">
                  <XCircle size={48} className="mx-auto text-red-600" />
                  <strong className="mt-3 block text-base">订单已关闭</strong>
                  <p className="mt-1 text-xs text-zinc-500">请关闭窗口后重新创建支付订单。</p>
                  <div className="mt-5 flex justify-center gap-2">
                    <button className="btn" type="button" onClick={() => void syncPayment(true)} disabled={syncing}>
                      {syncing ? <LoaderCircle size={14} className="animate-spin" /> : <RefreshCw size={14} />}
                      重新查询到账
                    </button>
                    <button className="btn" type="button" onClick={() => setPaymentOpen(false)}>关闭</button>
                  </div>
                </div>
              ) : (
                <>
                  <div className="mx-auto grid min-h-[220px] w-[220px] place-items-center rounded-md border border-[#dce4df] bg-white p-3">
                    {qrValue ? <QRCodeSVG value={qrValue} size={190} level="M" includeMargin /> : <QrCode size={48} className="text-zinc-300" />}
                  </div>
                  <strong className="mt-4 block text-base">支付宝扫码支付 {formatCNY(paymentOrder.amount)}</strong>
                  <p className="mt-1 flex items-center justify-center gap-1.5 text-[11px] text-zinc-500"><Clock3 size={12} />订单状态将自动更新</p>
                  <div className="mt-4 rounded-md bg-[#fafbf9] p-3 text-left text-[11px] text-zinc-500">
                    <div className="flex justify-between gap-3"><span>商户订单号</span><code className="truncate text-[#17201b]">{paymentOrder.outTradeNo || paymentOrder.id}</code></div>
                    <div className="mt-2 flex justify-between gap-3"><span>创建时间</span><span>{paymentOrder.createdAt ? formatDate(paymentOrder.createdAt) : '-'}</span></div>
                  </div>
                  {paymentError && <p className="mt-3 rounded-md bg-red-50 p-2 text-[11px] text-red-700">{paymentError}</p>}
                  <div className="mt-4 flex justify-center gap-2">
                    <button className="btn" type="button" onClick={() => setPaymentOpen(false)}>稍后支付</button>
                    <button className="btn primary" type="button" onClick={() => void syncPayment(true)} disabled={syncing}>
                      {syncing ? <LoaderCircle size={14} className="animate-spin" /> : <RefreshCw size={14} />}
                      查询支付状态
                    </button>
                  </div>
                </>
              )}
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
