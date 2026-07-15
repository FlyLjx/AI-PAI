'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { ArrowRight, CircleAlert, ExternalLink, MailCheck, UserPlus } from 'lucide-react';
import { toast } from 'sonner';
import { isRegistrationVerification, register, type RegistrationVerification } from '@/lib/portal-api';
import { useRegistrationAvailability } from '@/lib/use-registration-availability';

export default function RegisterPage() {
  const router = useRouter();
  const registrationAvailability = useRegistrationAvailability();
  const [form, setForm] = useState({ email: '', password: '', confirm: '' });
  const [verification, setVerification] = useState<RegistrationVerification | null>(null);
  const [loading, setLoading] = useState(false);
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (form.password !== form.confirm) return toast.error('两次输入的密码不一致');
    setLoading(true);
    try {
      const result = await register(form.email, form.password);
      if (isRegistrationVerification(result)) {
        setVerification(result);
        return;
      }
      router.replace('/dashboard');
    }
    catch (error) { toast.error(error instanceof Error ? error.message : '注册失败'); }
    finally { setLoading(false); }
  };

  if (registrationAvailability === 'loading') {
    return <main className="min-h-screen grid place-items-center bg-[#f7f8f6]"><div className="loading-ring" /></main>;
  }

  if (registrationAvailability === 'closed') {
    return (
      <main className="min-h-screen grid place-items-center bg-[#f7f8f6] p-5">
        <section className="w-full max-w-md section-panel p-6 text-center sm:p-8">
          <div className="flex items-center justify-center gap-2"><span className="brand-mark">AI</span><strong>AI-PAI</strong></div>
          <CircleAlert className="mx-auto mt-8 text-[#b7791f]" size={30} />
          <h1 className="mt-4 text-xl font-bold">注册暂未开放</h1>
          <p className="mt-2 text-xs leading-6 text-[#6b756f]">当前仅支持已有账户登录，请联系管理员开通账户。</p>
          <Link className="btn primary mt-6 w-full" href="/login">返回登录</Link>
        </section>
      </main>
    );
  }

  if (verification) {
    return (
      <main className="min-h-screen grid place-items-center bg-[#f7f8f6] p-5">
        <section className="w-full max-w-md section-panel p-6 sm:p-8" aria-live="polite">
          <div className="flex items-center gap-2"><span className="brand-mark">AI</span><strong>AI-PAI</strong></div>
          <span className="mt-8 grid size-11 place-items-center rounded-[8px] bg-[#eaf8ef] text-[#0f7a4b]">
            <MailCheck size={23} />
          </span>
          <h1 className="mt-4 text-xl font-bold">完成邮箱验证</h1>
          <p className="mt-2 break-all text-sm font-semibold text-[#27332c]">{verification.email}</p>
          <p className="mt-3 text-xs leading-6 text-[#6b756f]">
            {verification.message || (verification.sent ? '验证邮件已发送，请查收后完成验证。' : '验证链接已生成，请完成验证后登录。')}
          </p>

          {!verification.sent && verification.verificationUrl && (
            <div className="notice mt-5 min-w-0">
              <strong className="block text-xs">本地验证链接</strong>
              <p className="mt-1 break-all text-[10px] leading-5 text-[#4c6256]">{verification.verificationUrl}</p>
              <a className="btn primary mt-3 w-full" href={verification.verificationUrl}>
                打开验证链接 <ExternalLink size={14} />
              </a>
            </div>
          )}

          <Link className="btn mt-6 w-full" href="/login">返回登录</Link>
        </section>
      </main>
    );
  }

  return (
    <main className="min-h-screen grid place-items-center bg-[#f7f8f6] p-5">
      <form onSubmit={submit} className="w-full max-w-md section-panel p-6 sm:p-8">
        <div className="flex items-center gap-2"><span className="brand-mark">AI</span><strong>AI-PAI</strong></div>
        <UserPlus className="mt-8 text-[#0f7a4b]" size={26} />
        <h1 className="mt-4 text-xl font-bold">创建 API 账户</h1>
        <p className="mt-1 text-xs text-[#6b756f]">注册后即可创建 Key、查看用量并选择计费方式</p>
        <div className="mt-7 space-y-4">
          <label className="field"><span>邮箱</span><input type="email" required value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} /></label>
          <label className="field"><span>密码</span><input type="password" required minLength={6} value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} /></label>
          <label className="field"><span>确认密码</span><input type="password" required minLength={6} value={form.confirm} onChange={(e) => setForm({ ...form, confirm: e.target.value })} /></label>
        </div>
        <button className="btn primary w-full mt-6" disabled={loading}>{loading ? '创建中...' : <>创建账户 <ArrowRight size={15} /></>}</button>
        <p className="mt-5 text-center text-xs text-[#748078]">已有账户？ <Link className="text-[#087443] font-bold" href="/login">返回登录</Link></p>
      </form>
    </main>
  );
}
