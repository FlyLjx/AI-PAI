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
import { PageHeader } from '@/components/common/PageHeader';
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

  const loadBilling = useCallback(async () => {
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
      portalApi.plans(),
      portalApi.subscription(current),
      portalApi.publicSettings(),
    ]);
    const [userResult, plansResult, subscriptionResult, settingsResult] = results;
    setUser(userResult.status === 'fulfilled' ? userResult.value : current);
    if (plansResult.status === 'fulfilled') setPlans((plansResult.value.data || []).filter((plan) => plan.status === 'active'));
    if (subscriptionResult.status === 'fulfilled') setSubscription(subscriptionResult.value.data);
    if (settingsResult.status === 'fulfilled') setSettings(settingsResult.value.data || {});
    const failure = results.find((result) => result.status === 'rejected');
    if (failure?.status === 'rejected') setError(errorMessage(failure.reason));
    setLoading(false);
  }, []);

  const loadOrderHistory = useCallback(async (page = 1, current = getSession()) => {
    if (!current) {
      setHistoryError('登录状态已失效，请重新登录');
      setHistoryLoading(false);
      return;
    }
    setHistoryPage(page);
    setHistoryLoading(true);
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
      setHistoryLoading(false);
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
  const subscriptionActive = Boolean(subscription?.isPaid && subscription?.status === 'active');
  const remainingQuota = Number(subscription?.effectiveQuotaRemaining ?? subscription?.quotaRemaining ?? 0);
  const quotaLimit = Number(subscription?.quotaLimit ?? 0);
  const recommendedPlanId = useMemo(() => plans.reduce<Plan | null>((recommended, plan) => {
    if (!recommended || Number(plan.durationDays || 0) > Number(recommended.durationDays || 0)) return plan;
    return recommended;
  }, null)?.id, [plans]);
  const qrValue = paymentOrder?.qrCode || paymentOrder?.payUrl || '';

  const createPayment = async (input: { amount?: number; subscriptionPlanId?: string }, title: string, pendingKey: string) => {
    if (!user) return;
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
        await Promise.all([loadBilling(), loadOrderHistory(historyPage, user)]);
        return;
      }
      if (failedStatus(currentOrder.status)) {
        toast.error('订单已关闭，请重新创建支付订单');
        await loadOrderHistory(historyPage, user);
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
      void loadOrderHistory(1, user);
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
        await Promise.all([loadBilling(), loadOrderHistory(1, user)]);
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

  const refreshBilling = () => {
    void loadBilling();
    void loadOrderHistory(historyPage);
  };

  return (
    <div className="page-stack">
      <PageHeader title="计费中心" description="余额按量扣费与订阅额度可同时使用">
        <button className="btn" type="button" onClick={refreshBilling} disabled={loading || historyLoading}>
          <RefreshCw size={14} className={loading || historyLoading ? 'animate-spin' : ''} />刷新
        </button>
      </PageHeader>

      {error && <div className="notice" role="alert">部分计费信息暂未更新：{error}</div>}

      <section className="metric-grid">
        <StatBlock title="账户余额" value={loading && !user ? '--' : formatCNY(Number(user?.credits || 0))} subtext="按量请求自动扣减" icon={CircleDollarSign} color="green" />
        <StatBlock title="订阅状态" value={loading ? '--' : subscriptionActive ? subscription?.planName || '已订阅' : '未订阅'} subtext={subscriptionActive ? `有效至 ${subscription?.expiresAt ? formatDate(subscription.expiresAt, false) : '-'}` : '可独立购买套餐'} icon={Crown} color="amber" />
        <StatBlock title="订阅剩余" value={loading ? '--' : subscriptionActive ? remainingQuota.toLocaleString() : '0'} subtext={subscriptionActive ? `总额度 ${quotaLimit.toLocaleString()}` : '未开通订阅额度'} icon={WalletCards} color="cyan" />
        <StatBlock title="充值兑换" value={`1 : ${rechargeRate.toLocaleString()}`} subtext={`最低 ${formatCNY(minimumAmount)}`} icon={ShieldCheck} color="neutral" />
      </section>

      <div className="section-panel overflow-hidden">
        <div className="grid grid-cols-2 border-b border-[#dce4df] bg-[#fafbf9] p-1.5">
          <button
            type="button"
            className={`min-h-9 rounded-md px-3 text-xs font-bold ${activeTab === 'balance' ? 'bg-white text-[#087443] shadow-sm' : 'text-zinc-500'}`}
            onClick={() => setActiveTab('balance')}
            aria-pressed={activeTab === 'balance'}
          >
            <CircleDollarSign size={14} className="mr-1.5 inline" />余额充值
          </button>
          <button
            type="button"
            className={`min-h-9 rounded-md px-3 text-xs font-bold ${activeTab === 'subscription' ? 'bg-white text-[#92400e] shadow-sm' : 'text-zinc-500'}`}
            onClick={() => setActiveTab('subscription')}
            aria-pressed={activeTab === 'subscription'}
          >
            <Crown size={14} className="mr-1.5 inline" />订阅套餐
          </button>
        </div>

        {activeTab === 'balance' ? (
          <div className="section-body grid gap-5 lg:grid-cols-[minmax(0,1fr)_280px]">
            <div>
              <div className="mb-4 flex items-center justify-between gap-3">
                <div><strong className="text-sm">选择充值金额</strong><p className="mt-1 text-[11px] text-zinc-500">到账余额用于 API 按量调用</p></div>
                <span className={`status-pill ${rechargeEnabled ? 'active' : 'disabled'}`}>{rechargeEnabled ? '充值可用' : '充值暂停'}</span>
              </div>
              <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-4">
                {presets.map((amount) => (
                  <button
                    key={amount}
                    type="button"
                    className={`min-h-[72px] rounded-md border p-3 text-left transition-colors ${!customAmount && selectedAmount === amount ? 'border-[#86efac] bg-[#f0fdf4]' : 'border-[#dce4df] bg-white hover:border-[#86efac]'}`}
                    onClick={() => { setSelectedAmount(amount); setCustomAmount(''); }}
                    aria-pressed={!customAmount && selectedAmount === amount}
                  >
                    <strong className="block text-lg">{formatCNY(amount)}</strong>
                    <small className="mt-1 block text-[11px] text-zinc-500">预计到账 {(amount * rechargeRate).toFixed(2)}</small>
                  </button>
                ))}
              </div>
              <div className="field mt-4 max-w-sm">
                <label htmlFor="custom-recharge">自定义金额</label>
                <div className="flex gap-2">
                  <input id="custom-recharge" type="number" min={minimumAmount} step="0.01" value={customAmount} onChange={(event) => setCustomAmount(event.target.value)} placeholder={`最低 ${minimumAmount}`} />
                  <button className="btn primary shrink-0" type="button" onClick={startBalancePayment} disabled={!rechargeEnabled || Boolean(payingFor)}>
                    {payingFor.startsWith('balance-') && <LoaderCircle size={14} className="animate-spin" />}
                    支付充值
                  </button>
                </div>
              </div>
            </div>
            <aside className="border-t border-[#edf0ee] pt-4 lg:border-l lg:border-t-0 lg:pl-5 lg:pt-0">
              <strong className="text-xs">余额账户</strong>
              <div className="mt-3 rounded-md border border-[#dce4df] bg-[#fafbf9] p-4">
                <small className="text-[11px] text-zinc-500">当前可用</small>
                <div className="mt-1 text-2xl font-bold">{formatCNY(Number(user?.credits || 0))}</div>
                <div className="mt-4 grid gap-2 text-[11px] text-zinc-500">
                  <span className="flex items-center gap-2"><BadgeCheck size={13} className="text-[#087443]" />支付完成后自动到账</span>
                  <span className="flex items-center gap-2"><ShieldCheck size={13} className="text-blue-600" />调用失败按服务端账单规则回退</span>
                </div>
              </div>
            </aside>
          </div>
        ) : (
          <div className="section-body">
            <div className="flex flex-col gap-4 border-b border-[#edf0ee] pb-5 lg:flex-row lg:items-end lg:justify-between">
              <div className="min-w-0">
                <span className="inline-flex items-center gap-1.5 text-[11px] font-bold text-[#087443]"><Crown size={14} />订阅额度</span>
                <strong className="mt-1.5 block text-base">选择适合调用周期的套餐</strong>
                <p className="mt-1 max-w-xl text-[11px] leading-5 text-zinc-500">订阅有效期内优先使用套餐额度，额度用完后继续使用账户余额。</p>
              </div>
              <dl className="grid min-w-0 grid-cols-3 overflow-hidden rounded-md border border-[#dce4df] bg-[#fafbf9] lg:min-w-[520px]" aria-label="当前计费状态">
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
              <div className="mt-5 grid items-stretch gap-4 md:grid-cols-2">
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
                      className={`relative flex h-full min-w-0 flex-col overflow-hidden rounded-[7px] border p-5 transition-colors ${currentPlan ? 'border-amber-300 bg-amber-50/40' : recommendedPlan ? 'border-[#86efac] bg-white' : 'border-[#dce4df] bg-white hover:border-[#a7dabb]'}`}
                      aria-current={currentPlan ? 'true' : undefined}
                    >
                      {recommendedPlan && !currentPlan && <span className="absolute inset-x-0 top-0 h-1 bg-[#3f9274]" aria-hidden="true" />}
                      <div className="flex items-start justify-between gap-4">
                        <div className="flex min-w-0 items-center gap-3">
                          <span className={`grid h-9 w-9 shrink-0 place-items-center rounded-md ${currentPlan ? 'bg-amber-100 text-amber-700' : 'bg-[#eaf8ef] text-[#087443]'}`}><Crown size={17} /></span>
                          <div className="min-w-0">
                            <strong className="block truncate text-[15px]">{plan.name}</strong>
                            <span className="mt-0.5 block text-[10px] text-zinc-400">{durationDays} 天订阅周期</span>
                          </div>
                        </div>
                        {currentPlan ? (
                          <span className="status-pill paid shrink-0">当前订阅</span>
                        ) : recommendedPlan ? (
                          <span className="status-pill active shrink-0">推荐</span>
                        ) : plan.badge ? (
                          <span className="shrink-0 rounded bg-[#f1f3f2] px-1.5 py-1 text-[10px] font-bold text-[#59625d]">{plan.badge}</span>
                        ) : null}
                      </div>

                      <p className="mt-3 min-h-10 text-[11px] leading-5 text-zinc-500">{plan.description || '按周期提供稳定的 API 调用额度。'}</p>

                      <div className="mt-4 flex flex-wrap items-end justify-between gap-3 border-b border-[#edf0ee] pb-4">
                        <div>
                          <span className="block text-[10px] text-zinc-400">订阅价格</span>
                          <div className="mt-1 flex items-end gap-1"><strong className="text-[28px] leading-none">{formatCNY(Number(plan.amount || 0))}</strong><small className="pb-0.5 text-[10px] text-zinc-400">/ {durationDays} 天</small></div>
                        </div>
                        <div className="rounded-md bg-[#f6f8f6] px-3 py-2 text-right">
                          <span className="block text-[9px] text-zinc-400">日均价格</span>
                          <strong className="mono mt-0.5 block text-[11px] text-[#526059]">{formatCNY(dailyAmount)}</strong>
                        </div>
                      </div>

                      <div className="py-4">
                        <div className="flex items-end justify-between gap-3">
                          <span className="text-[11px] text-zinc-500">套餐总额度</span>
                          <strong className="mono text-xl text-[#17201b]">{quotaImages.toLocaleString()} <small className="font-sans text-[10px] font-semibold text-zinc-400">张</small></strong>
                        </div>
                        <div className="mt-3 grid grid-cols-2 gap-2 text-[11px]">
                          <span className="flex items-center gap-2 rounded-md bg-[#fafbf9] px-3 py-2 text-[#59645d]"><CalendarDays size={13} className="shrink-0 text-[#087443]" />有效 {durationDays} 天</span>
                          <span className="flex items-center gap-2 rounded-md bg-[#fafbf9] px-3 py-2 text-[#59645d]"><WalletCards size={13} className="shrink-0 text-blue-600" />日均约 {dailyQuota.toLocaleString()} 张</span>
                        </div>
                      </div>

                      <button className={`btn mt-auto w-full ${currentPlan ? '' : 'primary'}`} type="button" onClick={() => startPlanPayment(plan)} disabled={Boolean(payingFor)}>
                        {payingFor === `plan-${plan.id}` && <LoaderCircle size={14} className="animate-spin" />}
                        {currentPlan ? '续订当前套餐' : `订阅 ${plan.name}`}
                      </button>
                    </article>
                  );
                })}
              </div>
            )}

            <div className="mt-5 grid gap-3 border-t border-[#edf0ee] pt-4 sm:grid-cols-3" aria-label="订阅计费说明">
              <div className="flex min-w-0 items-start gap-2.5"><BadgeCheck size={15} className="mt-0.5 shrink-0 text-[#087443]" /><div><strong className="block text-[11px]">支付后自动生效</strong><small className="mt-0.5 block text-[10px] leading-4 text-zinc-500">支付完成后自动同步套餐状态</small></div></div>
              <div className="flex min-w-0 items-start gap-2.5"><WalletCards size={15} className="mt-0.5 shrink-0 text-blue-600" /><div><strong className="block text-[11px]">订阅额度优先</strong><small className="mt-0.5 block text-[10px] leading-4 text-zinc-500">有效期内请求优先扣套餐额度</small></div></div>
              <div className="flex min-w-0 items-start gap-2.5"><ShieldCheck size={15} className="mt-0.5 shrink-0 text-amber-600" /><div><strong className="block text-[11px]">余额自动衔接</strong><small className="mt-0.5 block text-[10px] leading-4 text-zinc-500">套餐不足时继续按量扣除余额</small></div></div>
            </div>
          </div>
        )}
      </div>

      <section aria-labelledby="billing-history-title">
        <div className="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 items-center gap-2.5">
            <span className="billing-icon"><ReceiptText size={16} /></span>
            <div className="min-w-0">
              <strong id="billing-history-title" className="block text-sm">充值与订阅记录</strong>
              <small className="mt-0.5 block text-[11px] text-zinc-500">余额充值与订阅订单，共 {historyTotal.toLocaleString()} 条</small>
            </div>
          </div>
          <button className="btn self-start sm:self-auto" type="button" onClick={() => void loadOrderHistory(historyPage)} disabled={historyLoading}>
            <RefreshCw size={14} className={historyLoading ? 'animate-spin' : ''} />刷新记录
          </button>
        </div>

        {historyLoading ? (
          <div className="section-panel empty-row" role="status"><LoaderCircle size={14} className="mr-2 inline animate-spin" />正在读取订单记录...</div>
        ) : historyError ? (
          <div className="flex flex-col items-start gap-3 rounded-[7px] border border-red-200 bg-red-50 p-4 text-[11px] text-red-700 sm:flex-row sm:items-center" role="alert">
            <span className="min-w-0 flex-1">充值与订阅记录暂未更新：{historyError}</span>
            <button className="btn shrink-0" type="button" onClick={() => void loadOrderHistory(historyPage)}>重新加载</button>
          </div>
        ) : (
          <DataTable
            headers={historyHeaders}
            data={orderHistory}
            currentPage={historyPage}
            totalPages={historyTotalPages}
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
            emptyState={<EmptyState title="暂无订单记录" description="完成余额充值或购买订阅后，订单会显示在这里。" icon={ReceiptText} />}
          />
        )}
      </section>

      {paymentOpen && paymentOrder && (
        <div className="modal-backdrop" role="presentation">
          <section className="modal-panel max-w-[430px]" role="dialog" aria-modal="true" aria-labelledby="payment-title">
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
