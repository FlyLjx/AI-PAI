'use client';

import { useCallback, useEffect, useState } from 'react';
import { CreditCard, Loader2, Mail, RefreshCw, Save, Server, ShieldCheck } from 'lucide-react';
import { toast } from 'sonner';
import { PageHeader } from '@/components/common/PageHeader';
import { portalApi } from '@/lib/admin-api';

type SettingsForm = {
  siteName: string;
  logoText: string;
  frontendUrl: string;
  backendUrl: string;
  registerMode: 'open' | 'closed';
  registerEmailVerification: boolean;
  taskTimeoutMinutes: number;
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
};

const emptySettings: SettingsForm = {
  siteName: 'AI-PAI',
  logoText: 'AI-PAI',
  frontendUrl: '',
  backendUrl: '',
  registerMode: 'open',
  registerEmailVerification: false,
  taskTimeoutMinutes: 3,
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
};

function normalizeSettings(data: Record<string, unknown>): SettingsForm {
  return {
    siteName: String(data.siteName || 'AI-PAI'),
    logoText: String(data.logoText || data.siteName || 'AI-PAI'),
    frontendUrl: String(data.frontendUrl || ''),
    backendUrl: String(data.backendUrl || ''),
    registerMode: data.registerMode === 'closed' ? 'closed' : 'open',
    registerEmailVerification: Boolean(data.registerEmailVerification),
    taskTimeoutMinutes: Number(data.taskTimeoutMinutes || 3),
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

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const response = await portalApi.settings();
      const values = response.data;
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
    setSaving(true);
    try {
      const input: Record<string, unknown> = {
        ...form,
        siteName: form.siteName.trim(),
        logoText: form.logoText.trim(),
        frontendUrl: form.frontendUrl.trim(),
        backendUrl: form.backendUrl.trim(),
        taskTimeoutMinutes: Number(form.taskTimeoutMinutes || 3),
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

      {error && <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void load()} className="font-semibold underline">重试</button></div>}

      {loading ? (
        <div className="grid min-h-[320px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <form onSubmit={save} className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white">
          <div className="grid grid-cols-2 gap-px border-b border-[#DCE4DF] bg-[#E8ECE9] lg:grid-cols-4">
            {[
              ['注册状态', form.registerMode === 'open' ? '开放注册' : '关闭注册', form.registerMode === 'open'],
              ['邮箱验证', form.registerEmailVerification ? '必须验证' : '不强制', form.registerEmailVerification],
              ['邮件服务', form.emailEnabled ? '已启用' : '已停用', form.emailEnabled],
              ['支付配置', form.alipayAppId && alipayPrivateKeyConfigured && form.alipayPublicKey ? '配置完整' : '待完善', Boolean(form.alipayAppId && alipayPrivateKeyConfigured && form.alipayPublicKey)],
            ].map(([label, value, active]) => <div key={String(label)} className="bg-[#FAFBFA] px-4 py-3"><span className="block text-[9px] font-semibold text-zinc-400">{label}</span><strong className={`mt-1 block text-xs ${active ? 'text-[#047857]' : 'text-zinc-600'}`}>{value}</strong></div>)}
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-2">
            <section className="space-y-4 border-b border-[#DCE4DF] p-5 xl:border-r">
              <div className="flex items-center gap-2 border-b border-[#DCE4DF] pb-2.5"><Server className="h-4 w-4 text-[#047857]" /><div><h2 className="text-xs font-semibold">站点与 API</h2><p className="mt-0.5 text-[9px] text-zinc-400">品牌、入口地址和请求超时</p></div></div>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">站点名称</span><input required value={form.siteName} onChange={(event) => updateField('siteName', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">Logo 文字</span><input required value={form.logoText} onChange={(event) => updateField('logoText', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">前端地址</span><input type="url" value={form.frontendUrl} onChange={(event) => updateField('frontendUrl', event.target.value)} placeholder="https://portal.example.com" className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs outline-none focus:border-[#12B76A]" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">Go 后端地址</span><input type="url" value={form.backendUrl} onChange={(event) => updateField('backendUrl', event.target.value)} placeholder="https://api.example.com" className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs outline-none focus:border-[#12B76A]" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">API 请求超时（分钟）</span><input min={1} max={120} type="number" value={form.taskTimeoutMinutes} onChange={(event) => updateField('taskTimeoutMinutes', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
              </div>
            </section>

            <section className="space-y-4 border-b border-[#DCE4DF] p-5">
              <div className="flex items-center gap-2 border-b border-[#DCE4DF] pb-2.5"><ShieldCheck className="h-4 w-4 text-[#0891B2]" /><div><h2 className="text-xs font-semibold">注册与账户</h2><p className="mt-0.5 text-[9px] text-zinc-400">API 客户开户规则</p></div></div>
              <label className="flex items-center justify-between gap-4 rounded-md border border-[#DCE4DF] p-3"><span><strong className="block text-[10px]">开放用户注册</strong><small className="mt-0.5 block text-[9px] text-zinc-400">关闭后仅管理员可创建 API 客户</small></span><input type="checkbox" checked={form.registerMode === 'open'} onChange={(event) => updateField('registerMode', event.target.checked ? 'open' : 'closed')} className="h-4 w-4 accent-[#047857]" /></label>
              <label className="flex items-center justify-between gap-4 rounded-md border border-[#DCE4DF] p-3"><span><strong className="block text-[10px]">注册邮箱验证</strong><small className="mt-0.5 block text-[9px] text-zinc-400">启用后须验证邮箱才能完成开户</small></span><input type="checkbox" checked={form.registerEmailVerification} onChange={(event) => updateField('registerEmailVerification', event.target.checked)} className="h-4 w-4 accent-[#047857]" /></label>
            </section>

            <section className="space-y-4 border-b border-[#DCE4DF] p-5 xl:border-b-0 xl:border-r">
              <div className="flex items-center gap-2 border-b border-[#DCE4DF] pb-2.5"><CreditCard className="h-4 w-4 text-[#D97706]" /><div><h2 className="text-xs font-semibold">支付宝当面付</h2><p className="mt-0.5 text-[9px] text-zinc-400">余额充值和订阅购买共用</p></div></div>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">应用 App ID</span><input value={form.alipayAppId} onChange={(event) => updateField('alipayAppId', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">支付宝网关</span><input type="url" value={form.alipayGateway} onChange={(event) => updateField('alipayGateway', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                <label className="sm:col-span-2"><span className="mb-1 block text-[10px] font-semibold text-zinc-500">应用私钥</span><textarea rows={3} value={alipayPrivateKey} onChange={(event) => setAlipayPrivateKey(event.target.value)} placeholder={alipayPrivateKeyConfigured ? '已配置，留空保持不变' : '输入 RSA2 应用私钥'} className="w-full resize-none rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-[10px]" /></label>
                <label className="sm:col-span-2"><span className="mb-1 block text-[10px] font-semibold text-zinc-500">支付宝公钥</span><textarea rows={3} value={form.alipayPublicKey} onChange={(event) => updateField('alipayPublicKey', event.target.value)} className="w-full resize-none rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-[10px]" /></label>
              </div>
            </section>

            <section className="space-y-4 p-5">
              <div className="flex items-center justify-between gap-3 border-b border-[#DCE4DF] pb-2.5"><span className="flex items-center gap-2"><Mail className="h-4 w-4 text-[#047857]" /><span><h2 className="text-xs font-semibold">SMTP 邮件</h2><p className="mt-0.5 text-[9px] text-zinc-400">注册验证和账户通知</p></span></span><label className="flex items-center gap-2 text-[10px] font-semibold text-zinc-500"><input type="checkbox" checked={form.emailEnabled} onChange={(event) => updateField('emailEnabled', event.target.checked)} className="h-4 w-4 accent-[#047857]" />启用</label></div>
              <div className="grid grid-cols-[minmax(0,1fr)_100px] gap-3">
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">SMTP Host</span><input value={form.emailHost} onChange={(event) => updateField('emailHost', event.target.value)} placeholder="smtp.example.com" className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">端口</span><input min={1} max={65535} type="number" value={form.emailPort} onChange={(event) => updateField('emailPort', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
              </div>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">登录账户</span><input value={form.emailUser} onChange={(event) => updateField('emailUser', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">授权密码</span><input type="password" value={emailPassword} onChange={(event) => setEmailPassword(event.target.value)} placeholder={emailPasswordConfigured ? '已配置，留空保持不变' : 'SMTP 授权码'} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">发件人名称</span><input value={form.emailFromName} onChange={(event) => updateField('emailFromName', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">发件邮箱</span><input type="email" value={form.emailFromAddress} onChange={(event) => updateField('emailFromAddress', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
              </div>
              <label className="flex items-center gap-2 text-[10px] text-zinc-500"><input type="checkbox" checked={form.emailSecure} onChange={(event) => updateField('emailSecure', event.target.checked)} className="h-3.5 w-3.5 accent-[#047857]" />使用 SSL/TLS（465 端口通常开启）</label>
            </section>
          </div>

          <div className="flex items-center justify-between gap-4 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-4"><span className="text-[10px] text-zinc-400">只更新中转站相关字段，未展示的旧业务设置保持原值。</span><button type="submit" disabled={saving} className="inline-flex h-9 items-center gap-2 rounded-md bg-[#047857] px-5 text-xs font-semibold text-white hover:bg-[#036b4f] disabled:opacity-50">{saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}保存设置</button></div>
        </form>
      )}
    </div>
  );
}
