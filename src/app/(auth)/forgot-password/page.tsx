'use client';

import Link from 'next/link';
import { useState } from 'react';
import { ArrowLeft, ExternalLink, KeyRound, Mail } from 'lucide-react';
import { toast } from 'sonner';
import { forgotPassword, type PasswordResetRequest } from '@/lib/portal-api';

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState('');
  const [result, setResult] = useState<PasswordResetRequest | null>(null);
  const [loading, setLoading] = useState(false);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setLoading(true);
    try {
      setResult(await forgotPassword(email));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '提交失败');
    } finally {
      setLoading(false);
    }
  };

  const showLocalLink = Boolean(
    result?.resetUrl && !result.message?.includes('邮件已发送，请查收'),
  );

  return (
    <main className="min-h-screen grid place-items-center bg-[#f7f8f6] p-5">
      <section className="w-full max-w-md section-panel p-6 sm:p-8">
        <div className="flex items-center gap-2"><span className="brand-mark">AI</span><strong>AI-PAI</strong></div>
        <KeyRound className="mt-8 text-[#0f7a4b]" size={26} />
        <h1 className="mt-4 text-xl font-bold">重置账户密码</h1>
        <p className="mt-1 text-xs leading-5 text-[#6b756f]">输入注册邮箱，我们会生成密码重置链接。</p>

        {result ? (
          <div className="mt-6" aria-live="polite">
            <div className="notice">
              <strong className="block text-xs">请求已受理</strong>
              <p className="mt-1 text-xs leading-5">
                {result.message || '若该邮箱已注册，密码重置说明将发送到对应邮箱。'}
              </p>
            </div>
            {showLocalLink && result.resetUrl && (
              <div className="mt-4 min-w-0 rounded-[7px] border border-[#dce4df] bg-[#fafbf9] p-3">
                <strong className="text-xs text-[#27332c]">本地重置链接</strong>
                <p className="mt-1 break-all text-[10px] leading-5 text-[#66736c]">{result.resetUrl}</p>
                <a className="btn primary mt-3 w-full" href={result.resetUrl}>
                  设置新密码 <ExternalLink size={14} />
                </a>
              </div>
            )}
            <button className="btn mt-5 w-full" type="button" onClick={() => setResult(null)}>重新输入邮箱</button>
          </div>
        ) : (
          <form className="mt-7" onSubmit={submit}>
            <label className="field">
              <span className="flex items-center gap-1.5"><Mail size={13} />邮箱</span>
              <input
                type="email"
                required
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="name@example.com"
              />
            </label>
            <button className="btn primary mt-6 w-full" disabled={loading}>
              {loading ? '提交中...' : '获取重置链接'}
            </button>
          </form>
        )}

        <Link className="mt-5 flex items-center justify-center gap-1.5 text-xs font-semibold text-[#087443]" href="/login">
          <ArrowLeft size={13} /> 返回登录
        </Link>
      </section>
    </main>
  );
}
