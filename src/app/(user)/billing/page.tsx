'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  BadgeCheck,
  CheckCircle2,
  CircleDollarSign,
  Clock3,
  Crown,
  LoaderCircle,
  QrCode,
  RefreshCw,
  ShieldCheck,
  WalletCards,
  X,
  XCircle,
} from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import { toast } from 'sonner';
import { PageHeader } from '@/components/common/PageHeader';
import { StatBlock } from '@/components/common/StatBlock';
import {
  APIError,
  getSession,
  portalApi,
  refreshSession,
  type Plan,
  type PortalUser,
  type Subscription,
} from '@/lib/portal-api';
import { formatCNY, formatDate } from '@/lib/common/utils';

type BillingTab = 'balance' | 'subscription';

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

  useEffect(() => {
    const timer = window.setTimeout(() => void loadBilling(), 0);
    return () => window.clearTimeout(timer);
  }, [loadBilling]);

  const presets = useMemo(() => parsePresets(settings.rechargePresets), [settings.rechargePresets]);
  const rechargeEnabled = settings.rechargeEnabled !== false;
  const minimumAmount = Math.max(1, Number(settings.rechargeMinAmount || 1));
  const rechargeRate = Math.max(0, Number(settings.rechargeRate || 1));
  const subscriptionActive = Boolean(subscription?.isPaid && subscription?.status === 'active');
  const remainingQuota = Number(subscription?.effectiveQuotaRemaining ?? subscription?.quotaRemaining ?? 0);
  const quotaLimit = Number(subscription?.quotaLimit ?? 0);
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
        await loadBilling();
      } else if (showFeedback) {
        toast.info('订单仍在等待支付');
      }
    } catch (syncError) {
      const message = errorMessage(syncError);
      setPaymentError(message);
      if (showFeedback) toast.error(message);
    } finally {
      setSyncing(false);
    }
  }, [loadBilling, paymentOrder, user]);

  useEffect(() => {
    if (!paymentOrder || paidStatus(paymentOrder.status) || failedStatus(paymentOrder.status)) return;
    const timer = window.setInterval(() => void syncPayment(false), 3000);
    return () => window.clearInterval(timer);
  }, [paymentOrder, syncPayment]);

  const paymentState = paymentOrder
    ? paidStatus(paymentOrder.status) ? 'paid' : failedStatus(paymentOrder.status) ? 'failed' : 'pending'
    : 'pending';

  return (
    <div className="page-stack">
      <PageHeader title="计费中心" description="余额按量扣费与订阅额度可同时使用">
        <button className="btn" type="button" onClick={() => void loadBilling()} disabled={loading}>
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />刷新
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
                <div><strong className="text-sm">选择充值金额</strong><p className="mt-1 text-[10px] text-zinc-500">到账余额用于 API 按量调用</p></div>
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
                    <small className="mt-1 block text-[10px] text-zinc-500">预计到账 {(amount * rechargeRate).toFixed(2)}</small>
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
                <small className="text-[10px] text-zinc-500">当前可用</small>
                <div className="mt-1 text-2xl font-bold">{formatCNY(Number(user?.credits || 0))}</div>
                <div className="mt-4 grid gap-2 text-[10px] text-zinc-500">
                  <span className="flex items-center gap-2"><BadgeCheck size={13} className="text-[#087443]" />支付完成后自动到账</span>
                  <span className="flex items-center gap-2"><ShieldCheck size={13} className="text-blue-600" />调用失败按服务端账单规则回退</span>
                </div>
              </div>
            </aside>
          </div>
        ) : (
          <div className="section-body">
            {subscriptionActive && (
              <div className="mb-4 flex flex-col gap-3 rounded-md border border-amber-200 bg-amber-50 p-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex min-w-0 items-center gap-3">
                  <span className="billing-icon is-subscription"><Crown size={16} /></span>
                  <div className="min-w-0"><strong className="block truncate text-sm">{subscription?.planName}</strong><small className="text-[10px] text-amber-800">剩余 {remainingQuota.toLocaleString()} · 有效至 {subscription?.expiresAt ? formatDate(subscription.expiresAt, false) : '-'}</small></div>
                </div>
                <span className="status-pill paid">当前订阅</span>
              </div>
            )}

            {loading && plans.length === 0 ? (
              <div className="empty-row">正在读取订阅套餐...</div>
            ) : plans.length === 0 ? (
              <div className="empty-row">暂无可购买的订阅套餐</div>
            ) : (
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {plans.map((plan) => {
                  const currentPlan = subscriptionActive && subscription?.planId === plan.id;
                  return (
                    <article key={plan.id} className={`relative rounded-md border p-4 ${currentPlan ? 'border-amber-300 bg-amber-50/60' : 'border-[#dce4df] bg-white'}`}>
                      <div className="flex items-start justify-between gap-3">
                        <div><strong className="text-sm">{plan.name}</strong>{plan.badge && <span className="ml-2 rounded bg-[#eaf8ef] px-1.5 py-0.5 text-[9px] font-bold text-[#087443]">{plan.badge}</span>}</div>
                        {currentPlan && <Crown size={16} className="text-amber-600" />}
                      </div>
                      <p className="mt-2 min-h-8 text-[10px] leading-4 text-zinc-500">{plan.description || '按周期提供稳定的 API 调用额度。'}</p>
                      <div className="mt-4 flex items-end gap-1"><strong className="text-2xl">{formatCNY(Number(plan.amount || 0))}</strong><small className="pb-1 text-[10px] text-zinc-400">/ {plan.durationDays} 天</small></div>
                      <dl className="mt-4 grid grid-cols-2 gap-2 border-y border-[#edf0ee] py-3 text-[10px]">
                        <div><dt className="text-zinc-400">套餐额度</dt><dd className="mono mt-0.5 font-bold">{Number(plan.quotaImages || 0).toLocaleString()}</dd></div>
                        <div><dt className="text-zinc-400">计费折扣</dt><dd className="mono mt-0.5 font-bold">{Number(plan.discountPercent || 0)}%</dd></div>
                      </dl>
                      <button className={`btn mt-4 w-full ${currentPlan ? '' : 'primary'}`} type="button" onClick={() => startPlanPayment(plan)} disabled={Boolean(payingFor)}>
                        {payingFor === `plan-${plan.id}` && <LoaderCircle size={14} className="animate-spin" />}
                        {currentPlan ? '续订套餐' : '购买套餐'}
                      </button>
                    </article>
                  );
                })}
              </div>
            )}
          </div>
        )}
      </div>

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
                  <button className="btn mt-5" type="button" onClick={() => setPaymentOpen(false)}>关闭</button>
                </div>
              ) : (
                <>
                  <div className="mx-auto grid min-h-[220px] w-[220px] place-items-center rounded-md border border-[#dce4df] bg-white p-3">
                    {qrValue ? <QRCodeSVG value={qrValue} size={190} level="M" includeMargin /> : <QrCode size={48} className="text-zinc-300" />}
                  </div>
                  <strong className="mt-4 block text-base">支付宝扫码支付 {formatCNY(paymentOrder.amount)}</strong>
                  <p className="mt-1 flex items-center justify-center gap-1.5 text-[10px] text-zinc-500"><Clock3 size={12} />订单状态将自动更新</p>
                  <div className="mt-4 rounded-md bg-[#fafbf9] p-3 text-left text-[10px] text-zinc-500">
                    <div className="flex justify-between gap-3"><span>商户订单号</span><code className="truncate text-[#17201b]">{paymentOrder.outTradeNo || paymentOrder.id}</code></div>
                    <div className="mt-2 flex justify-between gap-3"><span>创建时间</span><span>{paymentOrder.createdAt ? formatDate(paymentOrder.createdAt) : '-'}</span></div>
                  </div>
                  {paymentError && <p className="mt-3 rounded-md bg-red-50 p-2 text-[10px] text-red-700">{paymentError}</p>}
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
