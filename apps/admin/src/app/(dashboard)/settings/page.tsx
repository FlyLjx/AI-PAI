'use client';

import { useCallback, useEffect, useState } from 'react';
import { CreditCard, Gauge, Gift, Loader2, Mail, RefreshCw, Save, Server, ShieldAlert, ShieldCheck } from 'lucide-react';
import { toast } from 'sonner';
import { AppSelect } from '@/components/common/AppSelect';
import { PageHeader } from '@/components/common/PageHeader';
import { SystemUpdatePanel } from '@/components/settings/SystemUpdatePanel';
import { portalApi, type Plan } from '@/lib/admin-api';

type SettingsForm = {
  siteName: string;
  logoText: string;
  frontendUrl: string;
  backendUrl: string;
  registerMode: 'open' | 'closed';
  registerEmailVerification: boolean;
  taskTimeoutMinutes: number;
  dynamicConcurrencyEnabled: boolean;
  dynamicConcurrencyWindowValue: number;
  dynamicConcurrencyWindowUnit: 'minute' | 'hour';
  dynamicConcurrencyRequestStep: number;
  dynamicConcurrencyIncrement: number;
  rechargeRate: number;
  inviteEnabled: boolean;
  inviteInviterRewardType: 'balance' | 'subscription';
  inviteInviterRewardCredits: number;
  inviteInviterRewardPlanId: string;
  inviteInviteeRewardType: 'balance' | 'subscription';
  inviteInviteeRewardCredits: number;
  inviteInviteeRewardPlanId: string;
  inviteRechargeRebateEnabled: boolean;
  inviteRechargeRebatePercent: number;
  inviteRebateIncludeSubscriptions: boolean;
  inviteRiskEnabled: boolean;
  inviteRiskBlockSameIP: boolean;
  inviteRiskBlockSameDevice: boolean;
  inviteRiskMaxPerIP24h: number;
  inviteRiskMaxPerDevice24h: number;
  inviteRiskMaxPerInviter24h: number;
  registrationRiskEnabled: boolean;
  registrationRiskMaxPerIP24h: number;
  registrationRiskMaxPerDevice24h: number;
  registrationChallengeMinSeconds: number;
  registrationChallengeMaxPerIPHour: number;
  alipayAppId: string;
  alipayGateway: string;
  alipayPublicKey: string;
  emailEnabled: boolean;
  emailHost: string;
  emailPort: number;
  emailSecure: boolean;
  emailUser: string;
  emailFromName: string;
  emailFromAddress: string;
  adminRechargeNotificationEnabled: boolean;
  adminUpstreamNotificationEnabled: boolean;
  adminUpstreamCheckIntervalMinutes: number;
};

const emptySettings: SettingsForm = {
  siteName: 'AI-PAI',
  logoText: 'AI-PAI',
  frontendUrl: '',
  backendUrl: '',
  registerMode: 'open',
  registerEmailVerification: false,
  taskTimeoutMinutes: 3,
  dynamicConcurrencyEnabled: true,
  dynamicConcurrencyWindowValue: 1,
  dynamicConcurrencyWindowUnit: 'hour',
  dynamicConcurrencyRequestStep: 50,
  dynamicConcurrencyIncrement: 5,
  rechargeRate: 10,
  inviteEnabled: false,
  inviteInviterRewardType: 'subscription',
  inviteInviterRewardCredits: 0,
  inviteInviterRewardPlanId: '',
  inviteInviteeRewardType: 'balance',
  inviteInviteeRewardCredits: 0,
  inviteInviteeRewardPlanId: '',
  inviteRechargeRebateEnabled: false,
  inviteRechargeRebatePercent: 5,
  inviteRebateIncludeSubscriptions: false,
  inviteRiskEnabled: true,
  inviteRiskBlockSameIP: true,
  inviteRiskBlockSameDevice: true,
  inviteRiskMaxPerIP24h: 2,
  inviteRiskMaxPerDevice24h: 1,
  inviteRiskMaxPerInviter24h: 10,
  registrationRiskEnabled: true,
  registrationRiskMaxPerIP24h: 5,
  registrationRiskMaxPerDevice24h: 2,
  registrationChallengeMinSeconds: 2,
  registrationChallengeMaxPerIPHour: 30,
  alipayAppId: '',
  alipayGateway: 'https://openapi.alipay.com/gateway.do',
  alipayPublicKey: '',
  emailEnabled: false,
  emailHost: '',
  emailPort: 465,
  emailSecure: true,
  emailUser: '',
  emailFromName: 'AI-PAI',
  emailFromAddress: '',
  adminRechargeNotificationEnabled: true,
  adminUpstreamNotificationEnabled: true,
  adminUpstreamCheckIntervalMinutes: 5,
};

const DYNAMIC_WINDOW_UNIT_OPTIONS = [
  { value: 'minute', label: '分钟' },
  { value: 'hour', label: '小时' },
] as const;

const INVITE_REWARD_TYPE_OPTIONS = [
  { value: 'balance', label: '余额奖励' },
  { value: 'subscription', label: '订阅奖励' },
] as const;

function positiveInteger(value: unknown, fallback: number) {
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function normalizeSettings(data: Record<string, unknown>): SettingsForm {
  const rechargeRate = Number(data.rechargeRate);
  return {
    siteName: String(data.siteName || 'AI-PAI'),
    logoText: String(data.logoText || data.siteName || 'AI-PAI'),
    frontendUrl: String(data.frontendUrl || ''),
    backendUrl: String(data.backendUrl || ''),
    registerMode: data.registerMode === 'closed' ? 'closed' : 'open',
    registerEmailVerification: Boolean(data.registerEmailVerification),
    taskTimeoutMinutes: positiveInteger(data.taskTimeoutMinutes, 3),
    dynamicConcurrencyEnabled: data.dynamicConcurrencyEnabled !== false,
    dynamicConcurrencyWindowValue: positiveInteger(data.dynamicConcurrencyWindowValue, 1),
    dynamicConcurrencyWindowUnit: data.dynamicConcurrencyWindowUnit === 'minute' ? 'minute' : 'hour',
    dynamicConcurrencyRequestStep: positiveInteger(data.dynamicConcurrencyRequestStep, 50),
    dynamicConcurrencyIncrement: positiveInteger(data.dynamicConcurrencyIncrement, 5),
    rechargeRate: Number.isFinite(rechargeRate) && rechargeRate > 0 ? rechargeRate : 10,
    inviteEnabled: data.inviteEnabled !== false,
    inviteInviterRewardType: data.inviteInviterRewardType === 'balance' ? 'balance' : 'subscription',
    inviteInviterRewardCredits: Number(data.inviteInviterRewardCredits || 0),
    inviteInviterRewardPlanId: String(data.inviteInviterRewardPlanId || data.inviteRewardPlanId || ''),
    inviteInviteeRewardType: data.inviteInviteeRewardType === 'subscription' ? 'subscription' : 'balance',
    inviteInviteeRewardCredits: Number(data.inviteInviteeRewardCredits || 0),
    inviteInviteeRewardPlanId: String(data.inviteInviteeRewardPlanId || ''),
    inviteRechargeRebateEnabled: Boolean(data.inviteRechargeRebateEnabled),
    inviteRechargeRebatePercent: Number(data.inviteRechargeRebatePercent || 5),
    inviteRebateIncludeSubscriptions: Boolean(data.inviteRebateIncludeSubscriptions),
    inviteRiskEnabled: data.inviteRiskEnabled !== false,
    inviteRiskBlockSameIP: data.inviteRiskBlockSameIP !== false,
    inviteRiskBlockSameDevice: data.inviteRiskBlockSameDevice !== false,
    inviteRiskMaxPerIP24h: positiveInteger(data.inviteRiskMaxPerIP24h, 2),
    inviteRiskMaxPerDevice24h: positiveInteger(data.inviteRiskMaxPerDevice24h, 1),
    inviteRiskMaxPerInviter24h: positiveInteger(data.inviteRiskMaxPerInviter24h, 10),
    registrationRiskEnabled: data.registrationRiskEnabled !== false,
    registrationRiskMaxPerIP24h: positiveInteger(data.registrationRiskMaxPerIP24h, 5),
    registrationRiskMaxPerDevice24h: positiveInteger(data.registrationRiskMaxPerDevice24h, 2),
    registrationChallengeMinSeconds: positiveInteger(data.registrationChallengeMinSeconds, 2),
    registrationChallengeMaxPerIPHour: positiveInteger(data.registrationChallengeMaxPerIPHour, 30),
    alipayAppId: String(data.alipayAppId || ''),
    alipayGateway: String(data.alipayGateway || 'https://openapi.alipay.com/gateway.do'),
    alipayPublicKey: String(data.alipayPublicKey || ''),
    emailEnabled: Boolean(data.emailEnabled),
    emailHost: String(data.emailHost || ''),
    emailPort: Number(data.emailPort || 465),
    emailSecure: data.emailSecure !== false,
    emailUser: String(data.emailUser || ''),
    emailFromName: String(data.emailFromName || data.siteName || 'AI-PAI'),
    emailFromAddress: String(data.emailFromAddress || ''),
    adminRechargeNotificationEnabled: data.adminRechargeNotificationEnabled !== false,
    adminUpstreamNotificationEnabled: data.adminUpstreamNotificationEnabled !== false,
    adminUpstreamCheckIntervalMinutes: positiveInteger(data.adminUpstreamCheckIntervalMinutes, 5),
  };
}

export default function AdminSettingsPage() {
  const [form, setForm] = useState<SettingsForm>(emptySettings);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [emailPassword, setEmailPassword] = useState('');
  const [alipayPrivateKey, setAlipayPrivateKey] = useState('');
  const [emailPasswordConfigured, setEmailPasswordConfigured] = useState(false);
  const [alipayPrivateKeyConfigured, setAlipayPrivateKeyConfigured] = useState(false);
  const [plans, setPlans] = useState<Plan[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [response, planResponse] = await Promise.all([portalApi.settings(), portalApi.adminPlans()]);
      const values = response.data;
      setPlans((planResponse.data || []).filter((plan) => plan.status === 'active'));
      setForm(normalizeSettings(values));
      setEmailPasswordConfigured(Boolean(values.emailPassword));
      setAlipayPrivateKeyConfigured(Boolean(values.alipayPrivateKey));
      setEmailPassword('');
      setAlipayPrivateKey('');
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : '系统设置加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  const updateField = <K extends keyof SettingsForm>(key: K, value: SettingsForm[K]) => setForm((current) => ({ ...current, [key]: value }));

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!form.siteName.trim() || !form.logoText.trim()) return toast.error('请填写站点名称和 Logo 文字');
    if (form.taskTimeoutMinutes < 1) return toast.error('API 请求超时至少 1 分钟');
    if (!Number.isSafeInteger(form.dynamicConcurrencyWindowValue) || form.dynamicConcurrencyWindowValue < 1) return toast.error('动态并发统计窗口必须是大于 0 的整数');
    if (!Number.isSafeInteger(form.dynamicConcurrencyRequestStep) || form.dynamicConcurrencyRequestStep < 1) return toast.error('每档调用次数必须是大于 0 的整数');
    if (!Number.isSafeInteger(form.dynamicConcurrencyIncrement) || form.dynamicConcurrencyIncrement < 1) return toast.error('每档增加并发必须是大于 0 的整数');
    if (!Number.isFinite(form.rechargeRate) || form.rechargeRate <= 0) return toast.error('充值比例必须大于 0');
    if (form.inviteRechargeRebateEnabled && (!Number.isFinite(form.inviteRechargeRebatePercent) || form.inviteRechargeRebatePercent <= 0 || form.inviteRechargeRebatePercent > 100)) return toast.error('充值返利比例必须大于 0 且不超过 100%');
    if (!Number.isSafeInteger(form.adminUpstreamCheckIntervalMinutes) || form.adminUpstreamCheckIntervalMinutes < 1 || form.adminUpstreamCheckIntervalMinutes > 1440) return toast.error('上游状态检查间隔必须是 1 到 1440 分钟的整数');
    if (form.inviteEnabled) {
      if (form.inviteInviterRewardType === 'balance' && form.inviteInviterRewardCredits <= 0) return toast.error('请设置邀请人的余额奖励');
      if (form.inviteInviterRewardType === 'subscription' && !form.inviteInviterRewardPlanId) return toast.error('请选择邀请人的订阅奖励');
      if (form.inviteInviteeRewardType === 'balance' && form.inviteInviteeRewardCredits <= 0) return toast.error('请设置新用户的余额奖励');
      if (form.inviteInviteeRewardType === 'subscription' && !form.inviteInviteeRewardPlanId) return toast.error('请选择新用户的订阅奖励');
    }
    setSaving(true);
    try {
      const input: Record<string, unknown> = {
        ...form,
        siteName: form.siteName.trim(),
        logoText: form.logoText.trim(),
        frontendUrl: form.frontendUrl.trim(),
        backendUrl: form.backendUrl.trim(),
        taskTimeoutMinutes: Number(form.taskTimeoutMinutes || 3),
        rechargeRate: Number(form.rechargeRate),
        inviteRewardType: form.inviteInviterRewardType,
        inviteRewardPlanId: form.inviteInviterRewardPlanId,
        alipayAppId: form.alipayAppId.trim(),
        alipayGateway: form.alipayGateway.trim(),
        alipayPublicKey: form.alipayPublicKey.trim(),
        emailHost: form.emailHost.trim(),
        emailPort: Number(form.emailPort || 465),
        emailUser: form.emailUser.trim(),
        emailFromName: form.emailFromName.trim(),
        emailFromAddress: form.emailFromAddress.trim(),
      };
      if (emailPassword) input.emailPassword = emailPassword;
      if (alipayPrivateKey) input.alipayPrivateKey = alipayPrivateKey;
      const response = await portalApi.updateSettings(input);
      setForm(normalizeSettings(response.data as Record<string, unknown>));
      setEmailPasswordConfigured(Boolean((response.data as Record<string, unknown>).emailPassword));
      setAlipayPrivateKeyConfigured(Boolean((response.data as Record<string, unknown>).alipayPrivateKey));
      setEmailPassword('');
      setAlipayPrivateKey('');
      toast.success('系统设置已保存');
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '系统设置保存失败');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-5">
      <PageHeader title="系统设置" description="配置 API 中转站的注册、请求处理、支付和通知参数。">
        <button type="button" onClick={() => void load()} disabled={loading} title="重新加载" className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-[#DCE4DF] bg-white hover:border-[#12B76A] disabled:opacity-50"><RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} /></button>
      </PageHeader>

      <SystemUpdatePanel />

      {error && <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void load()} className="font-semibold underline">重试</button></div>}

      {loading ? (
        <div className="grid min-h-[320px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <form onSubmit={save} className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white">
          <div className="grid grid-cols-2 gap-px border-b border-[#DCE4DF] bg-[#E8ECE9] lg:grid-cols-6">
            {[
              ['注册状态', form.registerMode === 'open' ? '开放注册' : '关闭注册', form.registerMode === 'open'],
              ['邮箱验证', form.registerEmailVerification ? '必须验证' : '不强制', form.registerEmailVerification],
              ['邮件服务', form.emailEnabled ? '已启用' : '已停用', form.emailEnabled],
              ['动态并发', form.dynamicConcurrencyEnabled ? '已启用' : '已停用', form.dynamicConcurrencyEnabled],
              ['支付配置', form.alipayAppId && alipayPrivateKeyConfigured && form.alipayPublicKey ? '配置完整' : '待完善', Boolean(form.alipayAppId && alipayPrivateKeyConfigured && form.alipayPublicKey)],
              ['邀请返利', form.inviteEnabled ? '活动开启' : '活动暂停', form.inviteEnabled],
            ].map(([label, value, active]) => <div key={String(label)} className="bg-[#FAFBFA] px-4 py-3"><span className="block text-[10px] font-semibold text-zinc-400">{label}</span><strong className={`mt-1 block text-xs ${active ? 'text-[#047857]' : 'text-zinc-600'}`}>{value}</strong></div>)}
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-2">
            <section className="space-y-4 border-b border-[#DCE4DF] p-5 xl:border-r">
              <div className="flex items-center gap-2 border-b border-[#DCE4DF] pb-2.5"><Server className="h-4 w-4 text-[#047857]" /><div><h2 className="text-xs font-semibold">站点与 API</h2><p className="mt-0.5 text-[10px] text-zinc-400">品牌、入口地址和请求超时</p></div></div>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">站点名称</span><input required value={form.siteName} onChange={(event) => updateField('siteName', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">Logo 文字</span><input required value={form.logoText} onChange={(event) => updateField('logoText', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">前端地址</span><input type="url" value={form.frontendUrl} onChange={(event) => updateField('frontendUrl', event.target.value)} placeholder="https://portal.example.com" className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs outline-none focus:border-[#12B76A]" /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">Go 后端地址</span><input type="url" value={form.backendUrl} onChange={(event) => updateField('backendUrl', event.target.value)} placeholder="https://api.example.com" className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs outline-none focus:border-[#12B76A]" /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">API 请求超时（分钟）</span><input min={1} max={120} type="number" value={form.taskTimeoutMinutes} onChange={(event) => updateField('taskTimeoutMinutes', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
              </div>
            </section>

            <section className="space-y-4 border-b border-[#DCE4DF] p-5">
              <div className="flex items-center gap-2 border-b border-[#DCE4DF] pb-2.5"><ShieldCheck className="h-4 w-4 text-[#0891B2]" /><div><h2 className="text-xs font-semibold">注册与账户</h2><p className="mt-0.5 text-[10px] text-zinc-400">API 客户开户规则</p></div></div>
              <label className="flex items-center justify-between gap-4 rounded-md border border-[#DCE4DF] p-3"><span><strong className="block text-[11px]">开放用户注册</strong><small className="mt-0.5 block text-[10px] text-zinc-400">关闭后仅管理员可创建 API 客户</small></span><input type="checkbox" checked={form.registerMode === 'open'} onChange={(event) => updateField('registerMode', event.target.checked ? 'open' : 'closed')} className="h-4 w-4 accent-[#047857]" /></label>
              <label className="flex items-center justify-between gap-4 rounded-md border border-[#DCE4DF] p-3"><span><strong className="block text-[11px]">注册邮箱验证</strong><small className="mt-0.5 block text-[10px] text-zinc-400">启用后须验证邮箱才能完成开户</small></span><input type="checkbox" checked={form.registerEmailVerification} onChange={(event) => updateField('registerEmailVerification', event.target.checked)} className="h-4 w-4 accent-[#047857]" /></label>
            </section>

            <section className="space-y-4 border-b border-[#DCE4DF] p-5 xl:col-span-2">
              <div className="flex items-center justify-between gap-4 border-b border-[#DCE4DF] pb-2.5">
                <div className="flex items-center gap-2"><Gift className="h-4 w-4 text-[#D97706]" /><div><h2 className="text-xs font-semibold">邀请返利</h2><p className="mt-0.5 text-[10px] text-zinc-400">双方奖励独立配置，邮箱验证后自动发放</p></div></div>
                <label className="flex shrink-0 items-center gap-2 text-[11px] font-semibold text-zinc-500"><input type="checkbox" checked={form.inviteEnabled} onChange={(event) => updateField('inviteEnabled', event.target.checked)} className="h-4 w-4 accent-[#047857]" />启用活动</label>
              </div>
              <div className={`grid grid-cols-1 gap-4 lg:grid-cols-2 ${form.inviteEnabled ? '' : 'opacity-50'}`}>
                <div className="grid gap-3 rounded-md border border-[#DCE4DF] p-4 sm:grid-cols-2">
                  <div className="sm:col-span-2"><strong className="text-[11px]">邀请人奖励</strong><p className="mt-0.5 text-[10px] text-zinc-400">好友验证邮箱后到账</p></div>
                  <div><span className="mb-1 block text-[11px] font-semibold text-zinc-500">奖励类型</span><AppSelect disabled={!form.inviteEnabled} value={form.inviteInviterRewardType} options={INVITE_REWARD_TYPE_OPTIONS} onValueChange={(value) => updateField('inviteInviterRewardType', value as 'balance' | 'subscription')} ariaLabel="邀请人奖励类型" /></div>
                  {form.inviteInviterRewardType === 'balance' ? <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">奖励余额</span><input disabled={!form.inviteEnabled} min={0.0001} max={100000000} step={0.0001} type="number" value={form.inviteInviterRewardCredits} onChange={(event) => updateField('inviteInviterRewardCredits', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs disabled:bg-zinc-50" /></label> : <div><span className="mb-1 block text-[11px] font-semibold text-zinc-500">订阅套餐</span><AppSelect disabled={!form.inviteEnabled} value={form.inviteInviterRewardPlanId} options={plans.map((plan) => ({ value: plan.id, label: `${plan.name} · ${plan.durationDays} 天` }))} onValueChange={(value) => updateField('inviteInviterRewardPlanId', value)} ariaLabel="邀请人订阅奖励" placeholder="选择套餐" /></div>}
                </div>
                <div className="grid gap-3 rounded-md border border-[#DCE4DF] p-4 sm:grid-cols-2">
                  <div className="sm:col-span-2"><strong className="text-[11px]">新用户奖励</strong><p className="mt-0.5 text-[10px] text-zinc-400">被邀请人完成验证后到账</p></div>
                  <div><span className="mb-1 block text-[11px] font-semibold text-zinc-500">奖励类型</span><AppSelect disabled={!form.inviteEnabled} value={form.inviteInviteeRewardType} options={INVITE_REWARD_TYPE_OPTIONS} onValueChange={(value) => updateField('inviteInviteeRewardType', value as 'balance' | 'subscription')} ariaLabel="新用户奖励类型" /></div>
                  {form.inviteInviteeRewardType === 'balance' ? <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">奖励余额</span><input disabled={!form.inviteEnabled} min={0.0001} max={100000000} step={0.0001} type="number" value={form.inviteInviteeRewardCredits} onChange={(event) => updateField('inviteInviteeRewardCredits', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs disabled:bg-zinc-50" /></label> : <div><span className="mb-1 block text-[11px] font-semibold text-zinc-500">订阅套餐</span><AppSelect disabled={!form.inviteEnabled} value={form.inviteInviteeRewardPlanId} options={plans.map((plan) => ({ value: plan.id, label: `${plan.name} · ${plan.durationDays} 天` }))} onValueChange={(value) => updateField('inviteInviteeRewardPlanId', value)} ariaLabel="新用户订阅奖励" placeholder="选择套餐" /></div>}
                </div>
              </div>
              <div className="border-t border-[#EDF0EE] pt-4">
                <div className="mb-3 flex items-center justify-between gap-3">
                  <span><strong className="block text-[11px]">好友充值返利</strong><small className="text-[10px] text-zinc-400">邀请关系已生效后，按好友实际支付金额返还邀请人余额</small></span>
                  <label className="flex shrink-0 items-center gap-2 text-[11px] text-zinc-500"><input type="checkbox" checked={form.inviteRechargeRebateEnabled} onChange={(event) => updateField('inviteRechargeRebateEnabled', event.target.checked)} className="h-4 w-4 accent-[#047857]" />启用</label>
                </div>
                <div className={`grid grid-cols-1 gap-3 sm:grid-cols-[220px_minmax(0,1fr)_220px] ${form.inviteRechargeRebateEnabled ? '' : 'opacity-50'}`}>
                  <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">返利比例</span><div className="flex h-[34px] items-center overflow-hidden rounded-md border border-[#DCE4DF] bg-white"><input disabled={!form.inviteRechargeRebateEnabled} min={0.01} max={100} step={0.01} type="number" value={form.inviteRechargeRebatePercent} onChange={(event) => updateField('inviteRechargeRebatePercent', Number(event.target.value))} className="min-w-0 flex-1 border-0 px-3 font-mono text-xs outline-none disabled:bg-zinc-50" /><span className="border-l border-[#DCE4DF] bg-[#F8FAF8] px-3 text-[11px] font-semibold text-zinc-500">%</span></div></label>
                  <div><span className="mb-1 block text-[10px] font-semibold text-zinc-500">结算示例</span><p className="flex min-h-[34px] items-center rounded-md bg-[#F6F8F6] px-3 text-[11px] text-zinc-600">好友充值 100 元，邀请人获得 {Math.round(100 * form.rechargeRate * form.inviteRechargeRebatePercent) / 100} 余额</p></div>
                  <label className="flex min-h-[34px] items-center gap-2 self-end rounded-md border border-[#DCE4DF] px-3 text-[10px] text-zinc-600"><input disabled={!form.inviteRechargeRebateEnabled} type="checkbox" checked={form.inviteRebateIncludeSubscriptions} onChange={(event) => updateField('inviteRebateIncludeSubscriptions', event.target.checked)} className="h-3.5 w-3.5 accent-[#047857]" />订阅订单也参与返利</label>
                </div>
                <p className="mt-2 text-[10px] text-zinc-400">返利余额 = 实付金额 × 充值比例 × 返利比例；每个支付订单只结算一次。</p>
              </div>
              <div className="border-t border-[#EDF0EE] pt-4">
                <div className="mb-3 flex items-center justify-between gap-3"><span className="flex items-center gap-2"><ShieldAlert className="h-4 w-4 text-[#B42318]" /><span><strong className="block text-[11px]">邀请风控</strong><small className="text-[10px] text-zinc-400">异常邀请保留审计记录但不发奖</small></span></span><label className="flex items-center gap-2 text-[11px] text-zinc-500"><input type="checkbox" checked={form.inviteRiskEnabled} onChange={(event) => updateField('inviteRiskEnabled', event.target.checked)} className="h-4 w-4 accent-[#047857]" />启用</label></div>
                <div className={`grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-5 ${form.inviteRiskEnabled ? '' : 'opacity-50'}`}>
                  <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">单 IP / 24 小时</span><input disabled={!form.inviteRiskEnabled} min={1} type="number" value={form.inviteRiskMaxPerIP24h} onChange={(event) => updateField('inviteRiskMaxPerIP24h', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                  <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">单设备 / 24 小时</span><input disabled={!form.inviteRiskEnabled} min={1} type="number" value={form.inviteRiskMaxPerDevice24h} onChange={(event) => updateField('inviteRiskMaxPerDevice24h', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                  <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">单邀请人 / 24 小时</span><input disabled={!form.inviteRiskEnabled} min={1} type="number" value={form.inviteRiskMaxPerInviter24h} onChange={(event) => updateField('inviteRiskMaxPerInviter24h', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                  <label className="flex items-center gap-2 rounded-md border border-[#DCE4DF] px-3 text-[10px] text-zinc-600"><input disabled={!form.inviteRiskEnabled} type="checkbox" checked={form.inviteRiskBlockSameIP} onChange={(event) => updateField('inviteRiskBlockSameIP', event.target.checked)} className="h-3.5 w-3.5 accent-[#047857]" />拦截同 IP</label>
                  <label className="flex items-center gap-2 rounded-md border border-[#DCE4DF] px-3 text-[10px] text-zinc-600"><input disabled={!form.inviteRiskEnabled} type="checkbox" checked={form.inviteRiskBlockSameDevice} onChange={(event) => updateField('inviteRiskBlockSameDevice', event.target.checked)} className="h-3.5 w-3.5 accent-[#047857]" />拦截同设备</label>
                </div>
              </div>
              <div className="border-t border-[#EDF0EE] pt-4">
                <div className="mb-3 flex items-center justify-between gap-3"><span><strong className="block text-[11px]">注册机防护</strong><small className="text-[10px] text-zinc-400">一次性挑战、提交延迟、蜜罐、IP 与设备注册上限</small></span><label className="flex items-center gap-2 text-[11px] text-zinc-500"><input type="checkbox" checked={form.registrationRiskEnabled} onChange={(event) => updateField('registrationRiskEnabled', event.target.checked)} className="h-4 w-4 accent-[#047857]" />启用</label></div>
                <div className={`grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4 ${form.registrationRiskEnabled ? '' : 'opacity-50'}`}>
                  <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">单 IP 注册 / 24 小时</span><input disabled={!form.registrationRiskEnabled} min={1} type="number" value={form.registrationRiskMaxPerIP24h} onChange={(event) => updateField('registrationRiskMaxPerIP24h', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                  <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">单设备注册 / 24 小时</span><input disabled={!form.registrationRiskEnabled} min={1} type="number" value={form.registrationRiskMaxPerDevice24h} onChange={(event) => updateField('registrationRiskMaxPerDevice24h', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                  <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">最短填写时间（秒）</span><input disabled={!form.registrationRiskEnabled} min={1} type="number" value={form.registrationChallengeMinSeconds} onChange={(event) => updateField('registrationChallengeMinSeconds', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                  <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">挑战请求 / IP / 小时</span><input disabled={!form.registrationRiskEnabled} min={1} type="number" value={form.registrationChallengeMaxPerIPHour} onChange={(event) => updateField('registrationChallengeMaxPerIPHour', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                </div>
              </div>
              <p className="text-[10px] text-zinc-400">邀请注册始终要求完成邮箱验证后才发奖，即使全站注册邮箱验证开关关闭也不例外。</p>
            </section>

            <section className="space-y-4 border-b border-[#DCE4DF] p-5 xl:col-span-2">
              <div className="flex items-center justify-between gap-4 border-b border-[#DCE4DF] pb-2.5">
                <div className="flex items-center gap-2"><Gauge className="h-4 w-4 text-[#2563EB]" /><div><h2 className="text-xs font-semibold">动态并发</h2><p className="mt-0.5 text-[10px] text-zinc-400">按每个 API Key 在滚动时间窗口内的调用量自动扩容</p></div></div>
                <label className="flex shrink-0 items-center gap-2 text-[11px] font-semibold text-zinc-500"><input type="checkbox" checked={form.dynamicConcurrencyEnabled} onChange={(event) => updateField('dynamicConcurrencyEnabled', event.target.checked)} className="h-4 w-4 accent-[#047857]" />启用</label>
              </div>
              <div className={`grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4 ${form.dynamicConcurrencyEnabled ? '' : 'opacity-50'}`}>
                <div><label htmlFor="dynamic-concurrency-window"><span className="mb-1 block text-[11px] font-semibold text-zinc-500">滚动统计窗口</span></label><div className="grid grid-cols-[minmax(0,1fr)_110px] gap-2"><input id="dynamic-concurrency-window" disabled={!form.dynamicConcurrencyEnabled} min={1} max={1000000} step={1} type="number" value={form.dynamicConcurrencyWindowValue} onChange={(event) => updateField('dynamicConcurrencyWindowValue', Number(event.target.value))} className="min-w-0 rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs disabled:bg-zinc-50" /><AppSelect disabled={!form.dynamicConcurrencyEnabled} value={form.dynamicConcurrencyWindowUnit} options={DYNAMIC_WINDOW_UNIT_OPTIONS} onValueChange={(value) => updateField('dynamicConcurrencyWindowUnit', value as 'minute' | 'hour')} ariaLabel="动态并发统计窗口单位" /></div></div>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">每档调用次数</span><input disabled={!form.dynamicConcurrencyEnabled} min={1} max={1000000} step={1} type="number" value={form.dynamicConcurrencyRequestStep} onChange={(event) => updateField('dynamicConcurrencyRequestStep', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs disabled:bg-zinc-50" /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">每档增加并发</span><input disabled={!form.dynamicConcurrencyEnabled} min={1} max={1000000} step={1} type="number" value={form.dynamicConcurrencyIncrement} onChange={(event) => updateField('dynamicConcurrencyIncrement', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs disabled:bg-zinc-50" /></label>
                <div><span className="mb-1 block text-[11px] font-semibold text-zinc-500">当前规则</span><p className="flex min-h-[34px] items-center rounded-md bg-[#F6F8F6] px-3 text-[11px] text-zinc-600">{form.dynamicConcurrencyEnabled ? `${form.dynamicConcurrencyWindowValue} ${form.dynamicConcurrencyWindowUnit === 'minute' ? '分钟' : '小时'}内每调用 ${form.dynamicConcurrencyRequestStep} 次，并发 +${form.dynamicConcurrencyIncrement}` : '仅使用各 Key 的基础并发'}</p></div>
              </div>
              <p className="text-[10px] text-zinc-400">动态并发不设上限。每个 Key 的基础并发仍可在“API 调用”页面单独调整。</p>
            </section>

            <section className="space-y-4 border-b border-[#DCE4DF] p-5 xl:border-b-0 xl:border-r">
              <div className="flex items-center gap-2 border-b border-[#DCE4DF] pb-2.5"><CreditCard className="h-4 w-4 text-[#D97706]" /><div><h2 className="text-xs font-semibold">支付宝当面付</h2><p className="mt-0.5 text-[10px] text-zinc-400">余额充值和订阅购买共用</p></div></div>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <label className="sm:col-span-2"><span className="mb-1 block text-[11px] font-semibold text-zinc-500">充值比例（1 元兑换余额）</span><div className="flex h-[34px] max-w-sm items-center overflow-hidden rounded-md border border-[#DCE4DF] bg-white focus-within:border-[#12B76A]"><span className="border-r border-[#DCE4DF] bg-[#F8FAF8] px-3 text-[11px] font-semibold text-zinc-500">1 :</span><input min={0.01} step={0.01} type="number" value={form.rechargeRate} onChange={(event) => updateField('rechargeRate', Number(event.target.value))} className="min-w-0 flex-1 border-0 px-3 py-2 font-mono text-xs outline-none" /></div><small className="mt-1 block text-[10px] text-zinc-400">默认 1 元兑换 10 余额，新建充值订单按当前比例计算。</small></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">应用 App ID</span><input value={form.alipayAppId} onChange={(event) => updateField('alipayAppId', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">支付宝网关</span><input type="url" value={form.alipayGateway} onChange={(event) => updateField('alipayGateway', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                <label className="sm:col-span-2"><span className="mb-1 block text-[11px] font-semibold text-zinc-500">应用私钥</span><textarea rows={3} value={alipayPrivateKey} onChange={(event) => setAlipayPrivateKey(event.target.value)} placeholder={alipayPrivateKeyConfigured ? '已配置，留空保持不变' : '输入 RSA2 应用私钥'} className="w-full resize-none rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-[11px]" /></label>
                <label className="sm:col-span-2"><span className="mb-1 block text-[11px] font-semibold text-zinc-500">支付宝公钥</span><textarea rows={3} value={form.alipayPublicKey} onChange={(event) => updateField('alipayPublicKey', event.target.value)} className="w-full resize-none rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-[11px]" /></label>
              </div>
            </section>

            <section className="space-y-4 p-5">
              <div className="flex items-center justify-between gap-3 border-b border-[#DCE4DF] pb-2.5"><span className="flex items-center gap-2"><Mail className="h-4 w-4 text-[#047857]" /><span><h2 className="text-xs font-semibold">SMTP 邮件</h2><p className="mt-0.5 text-[10px] text-zinc-400">注册验证和账户通知</p></span></span><label className="flex items-center gap-2 text-[11px] font-semibold text-zinc-500"><input type="checkbox" checked={form.emailEnabled} onChange={(event) => updateField('emailEnabled', event.target.checked)} className="h-4 w-4 accent-[#047857]" />启用</label></div>
              <div className="grid grid-cols-[minmax(0,1fr)_100px] gap-3">
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">SMTP Host</span><input value={form.emailHost} onChange={(event) => updateField('emailHost', event.target.value)} placeholder="smtp.example.com" className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">端口</span><input min={1} max={65535} type="number" value={form.emailPort} onChange={(event) => updateField('emailPort', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
              </div>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">登录账户</span><input value={form.emailUser} onChange={(event) => updateField('emailUser', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">授权密码</span><input type="password" value={emailPassword} onChange={(event) => setEmailPassword(event.target.value)} placeholder={emailPasswordConfigured ? '已配置，留空保持不变' : 'SMTP 授权码'} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">发件人名称</span><input value={form.emailFromName} onChange={(event) => updateField('emailFromName', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs" /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">发件邮箱</span><input type="email" value={form.emailFromAddress} onChange={(event) => updateField('emailFromAddress', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
              </div>
              <label className="flex items-center gap-2 text-[11px] text-zinc-500"><input type="checkbox" checked={form.emailSecure} onChange={(event) => updateField('emailSecure', event.target.checked)} className="h-3.5 w-3.5 accent-[#047857]" />使用 SSL/TLS（465 端口通常开启）</label>
              <div className="space-y-2 border-t border-[#EDF0EE] pt-4">
                <div><strong className="block text-[11px]">管理员邮件通知</strong><small className="text-[10px] text-zinc-400">发送给所有启用中的管理员账号邮箱</small></div>
                <label className="flex items-center justify-between gap-3 rounded-md border border-[#DCE4DF] px-3 py-2.5"><span><strong className="block text-[11px]">充值成功通知</strong><small className="text-[10px] text-zinc-400">余额充值或订阅购买到账时发送</small></span><input type="checkbox" checked={form.adminRechargeNotificationEnabled} onChange={(event) => updateField('adminRechargeNotificationEnabled', event.target.checked)} className="h-4 w-4 accent-[#047857]" /></label>
                <label className="flex items-center justify-between gap-3 rounded-md border border-[#DCE4DF] px-3 py-2.5"><span><strong className="block text-[11px]">上游状态通知</strong><small className="text-[10px] text-zinc-400">异常与恢复时发送，持续异常 6 小时内不重复</small></span><input type="checkbox" checked={form.adminUpstreamNotificationEnabled} onChange={(event) => updateField('adminUpstreamNotificationEnabled', event.target.checked)} className="h-4 w-4 accent-[#047857]" /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">上游检查间隔（分钟）</span><input disabled={!form.adminUpstreamNotificationEnabled} min={1} max={1440} step={1} type="number" value={form.adminUpstreamCheckIntervalMinutes} onChange={(event) => updateField('adminUpstreamCheckIntervalMinutes', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs disabled:bg-zinc-50" /></label>
              </div>
            </section>
          </div>

          <div className="flex items-center justify-between gap-4 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-4"><span className="text-[11px] text-zinc-400">只更新中转站相关字段，未展示的旧业务设置保持原值。</span><button type="submit" disabled={saving} className="inline-flex h-9 items-center gap-2 rounded-md bg-[#047857] px-5 text-xs font-semibold text-white hover:bg-[#036b4f] disabled:opacity-50">{saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}保存设置</button></div>
        </form>
      )}
    </div>
  );
}
