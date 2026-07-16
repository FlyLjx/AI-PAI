'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowRight, LoaderCircle, LockKeyhole, Mail } from 'lucide-react';
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
    <main className="grid min-h-screen place-items-center bg-[#F6F8F7] p-5">
      <section className="w-full max-w-[390px]">
        <div className="mb-7 flex items-center justify-center gap-3">
          <span className="brand-mark !h-11 !w-11 !rounded-lg !text-sm">AI</span>
          <span className="flex flex-col"><strong className="text-lg leading-5 text-[#17201B]">AI-PAI</strong><small className="mt-0.5 text-xs text-zinc-500">管理后台</small></span>
        </div>
        <form onSubmit={submit} className="section-panel w-full p-6 sm:p-7">
          <div className="text-center">
            <h1 className="text-lg font-bold text-[#17201B]">登录管理控制台</h1>
            <p className="mt-1 text-[11px] text-zinc-500">使用管理员账户登录</p>
          </div>
          <div className="mt-6 space-y-4">
            <label className="field"><span className="flex items-center gap-1.5"><Mail size={13} />管理员邮箱</span><input type="email" required autoComplete="username" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="admin@example.com" autoFocus /></label>
            <label className="field"><span className="flex items-center gap-1.5"><LockKeyhole size={13} />密码</span><input type="password" required minLength={6} autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder="请输入密码" /></label>
          </div>
          <button className="btn primary mt-6 w-full" disabled={loading}>{loading ? <><LoaderCircle size={15} className="animate-spin" /><span>登录中</span></> : <><span>进入后台</span><ArrowRight size={15} /></>}</button>
        </form>
      </section>
    </main>
  );
}
