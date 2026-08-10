'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { ArrowRight, LoaderCircle, LockKeyhole, Mail } from 'lucide-react';
import { toast } from 'sonner';
import { login } from '@/lib/portal-api';
import { useRegistrationAvailability } from '@/lib/use-registration-availability';
import styles from '../auth.module.css';

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
    <main className={styles.authPage}>
      <section className={styles.authShell}>
        <form onSubmit={submit} className={styles.authFormPanel}>
          <div className={styles.formBrand}><span className={styles.brandMark}>AIπ</span><span className={styles.brandCopy}><strong>图片中转站</strong><small>开发者控制台</small></span></div>
          <div className={styles.formHeader}><h1>登录开发者控制台</h1><p>使用账户邮箱和密码继续</p></div>
          <div className={styles.authFields}>
            <label className={styles.authField}><span><Mail size={14} />邮箱</span><input type="email" required autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="name@example.com" autoFocus /></label>
            <label className={styles.authField}><span><LockKeyhole size={14} />密码</span><input type="password" required minLength={6} autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="请输入密码" /></label>
          </div>
          <div className={styles.authActions}><Link className={styles.forgotLink} href="/forgot-password">忘记密码？</Link></div>
          <button className={`btn primary ${styles.submitButton}`} disabled={loading}>{loading ? <><LoaderCircle size={15} className="animate-spin" /><span>登录中</span></> : <><span>登录</span><ArrowRight size={15} /></>}</button>
          {registrationAvailability === 'open' && <p className={styles.authFooter}>还没有账户？<Link className={styles.authLink} href="/register">注册账户</Link></p>}
        </form>
      </section>
    </main>
  );
}
