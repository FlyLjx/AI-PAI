'use client';

import Link from 'next/link';
import { useState } from 'react';
import { ArrowLeft, ExternalLink, KeyRound, Mail } from 'lucide-react';
import { toast } from 'sonner';
import { forgotPassword, type PasswordResetRequest } from '@/lib/portal-api';
import styles from '../auth.module.css';

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
    <main className={`${styles.authPage} ${styles.authStandalone}`}>
      <section className={styles.authFormPanel}>
        <div className={styles.brandLockup}><span className={styles.brandMark}>AIπ</span><span className={styles.brandCopy}><strong>图片中转站</strong><small>开发者控制台</small></span></div>
        <span className={`${styles.authIcon} mt-8`}><KeyRound size={22} /></span>
        <div className={styles.formHeader}><h1>重置账户密码</h1><p>输入注册邮箱，我们会发送密码重置说明。</p></div>

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
                <p className="mt-1 break-all text-[11px] leading-5 text-[#66736c]">{result.resetUrl}</p>
                <a className="btn primary mt-3 w-full" href={result.resetUrl}>
                  设置新密码 <ExternalLink size={14} />
                </a>
              </div>
            )}
            <button className="btn mt-5 w-full" type="button" onClick={() => setResult(null)}>重新输入邮箱</button>
          </div>
        ) : (
          <form className={styles.authFields} onSubmit={submit}>
            <label className={styles.authField}>
              <span><Mail size={14} />邮箱</span>
              <input
                type="email"
                required
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="name@example.com"
              />
            </label>
            <button className={`btn primary ${styles.submitButton}`} disabled={loading}>
              {loading ? '提交中...' : '发送重置说明'}
            </button>
          </form>
        )}

        <Link className={`${styles.authLink} mt-5 flex items-center justify-center gap-1.5`} href="/login">
          <ArrowLeft size={13} /> 返回登录
        </Link>
      </section>
    </main>
  );
}
