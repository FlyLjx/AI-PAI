'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { ArrowRight, KeyRound, LockKeyhole, Mail } from 'lucide-react';
import { toast } from 'sonner';
import { login } from '@/lib/portal-api';

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setLoading(true);
    try {
      const user = await login(email, password);
      router.replace(user.role === 'admin' ? '/admin/dashboard' : '/dashboard');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '登录失败');
    } finally { setLoading(false); }
  };

  return (
    <main className="min-h-screen grid lg:grid-cols-[minmax(0,1fr)_460px] bg-white">
      <section className="hidden lg:flex flex-col justify-between bg-[#17201b] text-white p-12">
        <div className="flex items-center gap-3"><span className="brand-mark !bg-white !text-[#17201b]">AI</span><strong>AI-PAI</strong></div>
        <div className="max-w-xl">
          <span className="text-xs text-emerald-300 font-mono">OPENAI-COMPATIBLE API GATEWAY</span>
          <h1 className="mt-5 text-4xl font-bold leading-tight">稳定连接上游模型，<br />统一管理调用与成本。</h1>
          <p className="mt-5 text-sm leading-7 text-white/60">面向开发者和团队的图像 API 中转站。支持订阅额度与账户余额两种计费方式。</p>
        </div>
        <div className="flex gap-7 text-xs text-white/55"><span>API Key 隔离</span><span>用量可追溯</span><span>失败不扣费</span></div>
      </section>
      <section className="flex items-center justify-center p-6 sm:p-10 bg-[#f7f8f6]">
        <form onSubmit={submit} className="w-full max-w-sm section-panel p-6 sm:p-8">
          <div className="lg:hidden flex items-center gap-2 mb-8"><span className="brand-mark">AI</span><strong>AI-PAI</strong></div>
          <KeyRound className="text-[#0f7a4b]" size={25} />
          <h2 className="mt-4 text-xl font-bold">登录开发者控制台</h2>
          <p className="mt-1 text-xs text-[#6b756f]">使用账户邮箱和密码继续</p>
          <div className="mt-7 space-y-4">
            <label className="field"><span className="flex items-center gap-1.5"><Mail size={13} />邮箱</span><input type="email" required value={email} onChange={(e) => setEmail(e.target.value)} placeholder="name@example.com" /></label>
            <label className="field"><span className="flex items-center gap-1.5"><LockKeyhole size={13} />密码</span><input type="password" required minLength={6} value={password} onChange={(e) => setPassword(e.target.value)} placeholder="至少 6 位" /></label>
          </div>
          <div className="mt-3 flex justify-end">
            <Link className="text-xs font-semibold text-[#087443] hover:text-[#065f37]" href="/forgot-password">忘记密码？</Link>
          </div>
          <button className="btn primary w-full mt-6" disabled={loading}>{loading ? '登录中...' : <><span>登录</span><ArrowRight size={15} /></>}</button>
          <p className="mt-5 text-center text-xs text-[#748078]">还没有账户？ <Link className="text-[#087443] font-bold" href="/register">注册账户</Link></p>
        </form>
      </section>
    </main>
  );
}
