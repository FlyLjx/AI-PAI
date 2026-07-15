'use client';

import Link from 'next/link';
import { useEffect, useRef, useState } from 'react';
import { BadgeCheck, CircleAlert, KeyRound, MailCheck } from 'lucide-react';
import { resetPassword, verifyEmail } from '@/lib/portal-api';

function AuthResultShell({ children }: { children: React.ReactNode }) {
  return (
    <main className="min-h-screen grid place-items-center bg-[#f7f8f6] p-5">
      <section className="w-full max-w-md section-panel p-6 sm:p-8">
        <div className="flex items-center gap-2"><span className="brand-mark">AI</span><strong>AI-PAI</strong></div>
        {children}
      </section>
    </main>
  );
}

function VerifyEmailPanel({ token }: { token: string }) {
  const attemptedToken = useRef<string | null>(null);
  const [state, setState] = useState<{ status: 'loading' | 'success' | 'error'; message?: string }>(
    token
      ? { status: 'loading' }
      : { status: 'error', message: '验证链接缺少令牌，请检查链接是否完整。' },
  );

  useEffect(() => {
    if (!token) return;
    if (attemptedToken.current === token) return;
    attemptedToken.current = token;
    verifyEmail(token)
      .then((account) => setState({ status: 'success', message: `${account.email} 已完成验证。` }))
      .catch((error) => setState({ status: 'error', message: error instanceof Error ? error.message : '邮箱验证失败' }));
  }, [token]);

  return (
    <AuthResultShell>
      {state.status === 'loading' ? (
        <div className="grid justify-items-center py-12 text-center" aria-live="polite">
          <div className="loading-ring" />
          <MailCheck className="mt-6 text-[#0f7a4b]" size={25} />
          <h1 className="mt-3 text-xl font-bold">正在验证邮箱</h1>
          <p className="mt-2 text-xs text-[#6b756f]">请稍候，不要重复刷新页面。</p>
        </div>
      ) : state.status === 'success' ? (
        <div className="pt-8 text-center" aria-live="polite">
          <BadgeCheck className="mx-auto text-[#0f7a4b]" size={36} />
          <h1 className="mt-4 text-xl font-bold">邮箱验证成功</h1>
          <p className="mt-2 break-all text-xs leading-6 text-[#66736c]">{state.message}</p>
          <Link className="btn primary mt-6 w-full" href="/login">前往登录</Link>
        </div>
      ) : (
        <div className="pt-8 text-center" aria-live="assertive">
          <CircleAlert className="mx-auto text-[#b42318]" size={34} />
          <h1 className="mt-4 text-xl font-bold">邮箱验证未完成</h1>
          <p className="mt-2 break-all text-xs leading-6 text-[#66736c]">{state.message}</p>
          <Link className="btn mt-6 w-full" href="/login">返回登录</Link>
        </div>
      )}
    </AuthResultShell>
  );
}

function ResetPasswordPanel({ token }: { token: string }) {
  const [form, setForm] = useState({ password: '', confirm: '' });
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [complete, setComplete] = useState(false);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setMessage('');
    if (!token) {
      setMessage('重置链接缺少令牌，请检查链接是否完整。');
      return;
    }
    if (form.password !== form.confirm) {
      setMessage('两次输入的密码不一致。');
      return;
    }
    setLoading(true);
    try {
      await resetPassword(token, form.password);
      setComplete(true);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '密码重置失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <AuthResultShell>
      {complete ? (
        <div className="pt-8 text-center" aria-live="polite">
          <BadgeCheck className="mx-auto text-[#0f7a4b]" size={36} />
          <h1 className="mt-4 text-xl font-bold">密码已更新</h1>
          <p className="mt-2 text-xs leading-6 text-[#66736c]">请使用新密码登录开发者控制台。</p>
          <Link className="btn primary mt-6 w-full" href="/login">前往登录</Link>
        </div>
      ) : (
        <form className="pt-8" onSubmit={submit}>
          <KeyRound className="text-[#0f7a4b]" size={26} />
          <h1 className="mt-4 text-xl font-bold">设置新密码</h1>
          <p className="mt-1 text-xs leading-5 text-[#6b756f]">新密码至少 6 位，保存后原密码立即失效。</p>
          <div className="mt-7 space-y-4">
            <label className="field">
              <span>新密码</span>
              <input
                type="password"
                required
                minLength={6}
                autoComplete="new-password"
                value={form.password}
                onChange={(event) => setForm({ ...form, password: event.target.value })}
              />
            </label>
            <label className="field">
              <span>确认新密码</span>
              <input
                type="password"
                required
                minLength={6}
                autoComplete="new-password"
                value={form.confirm}
                onChange={(event) => setForm({ ...form, confirm: event.target.value })}
              />
            </label>
          </div>
          {!token && <p className="mt-4 rounded-[7px] border border-[#ffd6d6] bg-[#fff7f7] p-3 text-xs leading-5 text-[#b42318]" role="alert">重置链接缺少令牌，请检查链接是否完整。</p>}
          {message && <p className="mt-4 rounded-[7px] border border-[#ffd6d6] bg-[#fff7f7] p-3 text-xs leading-5 text-[#b42318]" role="alert">{message}</p>}
          <button className="btn primary mt-6 w-full" disabled={loading || !token}>
            {loading ? '保存中...' : '保存新密码'}
          </button>
          <Link className="btn mt-3 w-full" href="/login">返回登录</Link>
        </form>
      )}
    </AuthResultShell>
  );
}

export function RootAuthFlow({ kind, token }: { kind: 'verify' | 'reset'; token: string }) {
  return kind === 'verify' ? <VerifyEmailPanel token={token} /> : <ResetPasswordPanel token={token} />;
}
