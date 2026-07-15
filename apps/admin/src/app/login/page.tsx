'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowRight, LockKeyhole, Mail, ShieldCheck } from 'lucide-react';
import { toast } from 'sonner';
import { adminAuth } from '@/lib/admin-api';

export default function AdminLoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    void adminAuth.session().then(() => router.replace('/dashboard')).catch(() => undefined);
  }, [router]);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setLoading(true);
    try {
      await adminAuth.login(email, password);
      router.replace('/dashboard');
      router.refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '后台登录失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="min-h-screen grid lg:grid-cols-[minmax(0,1fr)_460px] bg-white">
      <section className="hidden lg:flex flex-col justify-between bg-[#17201b] p-12 text-white">
        <div className="flex items-center gap-3"><span className="brand-mark !bg-white !text-[#17201b]">AI</span><strong>AI-PAI</strong></div>
        <div className="max-w-xl">
          <span className="font-mono text-xs text-emerald-300">INDEPENDENT ADMIN CONSOLE</span>
          <h1 className="mt-5 text-4xl font-bold leading-tight">独立管理后台，<br />专注 API 中转运营。</h1>
          <p className="mt-5 text-sm leading-7 text-white/60">管理客户、上游、模型、计费和系统状态。后台会话与客户前台完全隔离。</p>
        </div>
        <div className="flex gap-7 text-xs text-white/55"><span>HttpOnly 会话</span><span>后台接口白名单</span><span>独立部署</span></div>
      </section>
      <section className="flex items-center justify-center bg-[#f7f8f6] p-6 sm:p-10">
        <form onSubmit={submit} className="section-panel w-full max-w-sm p-6 sm:p-8">
          <div className="mb-8 flex items-center gap-2 lg:hidden"><span className="brand-mark">AI</span><strong>AI-PAI 后台</strong></div>
          <ShieldCheck className="text-[#0f7a4b]" size={26} />
          <h2 className="mt-4 text-xl font-bold">登录管理控制台</h2>
          <p className="mt-1 text-xs text-[#6b756f]">仅限已启用的管理员账号</p>
          <div className="mt-7 space-y-4">
            <label className="field"><span className="flex items-center gap-1.5"><Mail size={13} />管理员邮箱</span><input type="email" required autoComplete="username" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="admin@example.com" /></label>
            <label className="field"><span className="flex items-center gap-1.5"><LockKeyhole size={13} />密码</span><input type="password" required minLength={6} autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder="至少 6 位" /></label>
          </div>
          <button className="btn primary mt-6 w-full" disabled={loading}>{loading ? '登录中...' : <><span>进入后台</span><ArrowRight size={15} /></>}</button>
        </form>
      </section>
    </main>
  );
}
