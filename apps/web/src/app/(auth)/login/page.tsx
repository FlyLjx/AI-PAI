'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { ArrowRight, LoaderCircle, LockKeyhole, Mail } from 'lucide-react';
import { toast } from 'sonner';
import { login } from '@/lib/portal-api';
import { useRegistrationAvailability } from '@/lib/use-registration-availability';

export default function LoginPage() {
  const router = useRouter();
  const registrationAvailability = useRegistrationAvailability();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setLoading(true);
    try {
      await login(email, password);
      router.replace('/dashboard');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '登录失败');
    } finally { setLoading(false); }
  };

  return (
    <main className="grid min-h-screen place-items-center bg-[#F6F8F7] p-5">
      <section className="w-full max-w-[390px]">
        <div className="mb-6 flex items-center justify-center gap-2.5">
          <span className="brand-mark">AI</span>
          <span className="flex flex-col"><strong className="text-sm leading-4 text-[#17201B]">AI-PAI</strong><small className="text-[10px] text-zinc-500">开发者控制台</small></span>
        </div>
        <form onSubmit={submit} className="section-panel w-full p-6 sm:p-7">
          <div className="text-center">
            <h1 className="text-lg font-bold text-[#17201B]">登录开发者控制台</h1>
            <p className="mt-1 text-[11px] text-zinc-500">使用账户邮箱和密码继续</p>
          </div>
          <div className="mt-6 space-y-4">
            <label className="field"><span className="flex items-center gap-1.5"><Mail size={13} />邮箱</span><input type="email" required autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="name@example.com" autoFocus /></label>
            <label className="field"><span className="flex items-center gap-1.5"><LockKeyhole size={13} />密码</span><input type="password" required minLength={6} autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="请输入密码" /></label>
          </div>
          <div className="mt-3 flex justify-end">
            <Link className="text-xs font-semibold text-[#087443] hover:text-[#065f37]" href="/forgot-password">忘记密码？</Link>
          </div>
          <button className="btn primary mt-6 w-full" disabled={loading}>{loading ? <><LoaderCircle size={15} className="animate-spin" /><span>登录中</span></> : <><span>登录</span><ArrowRight size={15} /></>}</button>
          {registrationAvailability === 'open' && <p className="mt-5 text-center text-xs text-[#748078]">还没有账户？ <Link className="text-[#087443] font-bold" href="/register">注册账户</Link></p>}
        </form>
      </section>
    </main>
  );
}
