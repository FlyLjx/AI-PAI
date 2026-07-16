'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import {
  Activity, BookOpen, ChevronRight, HeartPulse, KeyRound, LayoutDashboard,
  Images, LoaderCircle, LogOut, MailWarning, Menu, Send, Settings, WalletCards, X,
} from 'lucide-react';
import { toast } from 'sonner';
import { BillingRail } from './BillingRail';
import { APIError, clearSession, getSession, portalApi, refreshSession, type PortalUser } from '@/lib/portal-api';

type NavItem = { label: string; href: string; icon: React.ComponentType<{ size?: number; className?: string }> };

function Navigation({ items, pathname, mobile = false, onNavigate }: { items: NavItem[]; pathname: string; mobile?: boolean; onNavigate: () => void }) {
  return (
    <nav className={mobile ? 'grid grid-cols-2 gap-2' : 'space-y-1'}>
      {items.map(({ label, href, icon: Icon }) => {
        const active = pathname === href;
        return (
          <Link key={href} href={href} onClick={onNavigate} className={`nav-item ${active ? 'is-active' : ''}`}>
            <Icon size={16} />
            <span>{label}</span>
            {!mobile && <ChevronRight size={13} className="nav-arrow" />}
          </Link>
        );
      })}
    </nav>
  );
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [user, setUser] = useState<PortalUser | null>(() => getSession());
  const [ready, setReady] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [verificationSending, setVerificationSending] = useState(false);
  const [verificationCooldown, setVerificationCooldown] = useState(0);

  useEffect(() => {
    let active = true;
    const current = getSession();
    if (!current) {
      router.replace('/login');
      return;
    }
    void refreshSession(current).then((fresh) => {
      if (!active) return;
      setUser(fresh);
    }).catch(() => {
      clearSession();
      if (active) router.replace('/login');
    }).finally(() => active && setReady(true));
    return () => { active = false; };
  }, [pathname, router]);

  useEffect(() => {
    if (verificationCooldown < 1) return;
    const timer = window.setInterval(() => {
      setVerificationCooldown((current) => Math.max(0, current - 1));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [verificationCooldown]);

  useEffect(() => {
    const syncSession = (event: Event) => {
      const nextUser = (event as CustomEvent<PortalUser | null>).detail;
      setUser(nextUser || getSession());
    };
    window.addEventListener('aipai:session', syncSession);
    return () => window.removeEventListener('aipai:session', syncSession);
  }, []);

  useEffect(() => {
    let active = true;
    let refreshing = false;
    const syncAccount = async () => {
      if (refreshing || document.visibilityState !== 'visible') return;
      const current = getSession();
      if (!current) return;
      refreshing = true;
      try {
        const fresh = await refreshSession(current);
        if (active) setUser(fresh);
      } catch (error) {
        if (active && error instanceof APIError && (error.status === 401 || error.status === 403)) {
          clearSession();
          router.replace('/login');
        }
      } finally {
        refreshing = false;
      }
    };
    const syncVisibleAccount = () => {
      if (document.visibilityState === 'visible') void syncAccount();
    };
    const timer = window.setInterval(() => void syncAccount(), 10000);
    window.addEventListener('focus', syncVisibleAccount);
    document.addEventListener('visibilitychange', syncVisibleAccount);
    return () => {
      active = false;
      window.clearInterval(timer);
      window.removeEventListener('focus', syncVisibleAccount);
      document.removeEventListener('visibilitychange', syncVisibleAccount);
    };
  }, [router]);

  if (!ready || !user) {
    return <div className="min-h-screen grid place-items-center bg-[#f7f8f6]"><div className="loading-ring" /></div>;
  }

  const userNav: NavItem[] = [
    { label: '控制台', href: '/dashboard', icon: LayoutDashboard },
    { label: '生图台', href: '/playground', icon: Images },
    { label: 'API Key', href: '/api-keys', icon: KeyRound },
    { label: '用量记录', href: '/usage', icon: Activity },
    { label: '计费中心', href: '/billing', icon: WalletCards },
    { label: '接口状态', href: '/status', icon: HeartPulse },
    { label: 'API 文档', href: '/docs', icon: BookOpen },
    { label: '账户设置', href: '/settings', icon: Settings },
  ];
  const mobileNav = userNav.filter((item) => (
    ['/dashboard', '/playground', '/usage', '/billing'].includes(item.href)
  ));
  const logout = () => {
    clearSession();
    router.replace('/login');
  };
  const resendEmailVerification = async () => {
    if (!user || user.emailVerifiedAt || verificationSending || verificationCooldown > 0) return;
    setVerificationSending(true);
    try {
      const response = await portalApi.resendEmailVerification(user);
      setVerificationCooldown(60);
      toast.success(response.data.message || '验证邮件已重新发送，请查收');
    } catch (error) {
      if (error instanceof APIError && error.status === 429) setVerificationCooldown(60);
      toast.error(error instanceof Error ? error.message : '验证邮件发送失败');
    } finally {
      setVerificationSending(false);
    }
  };

  return (
    <div className="app-frame">
      <aside className="app-sidebar">
        <Link href="/dashboard" className="brand-lockup">
          <span className="brand-mark">AI</span>
          <span><strong>AI-PAI</strong><small>API 中转站</small></span>
        </Link>
        <BillingRail user={user} />
        <div className="sidebar-section-label">开发者工作台</div>
        <div className="sidebar-scroll"><Navigation items={userNav} pathname={pathname} onNavigate={() => setMobileOpen(false)} /></div>
        <div className="sidebar-account">
          <span className="account-avatar">{user.email.slice(0, 1).toUpperCase()}</span>
          <span className="account-copy"><strong>{user.email}</strong><small>API 客户</small></span>
          <button type="button" onClick={logout} title="退出登录"><LogOut size={16} /></button>
        </div>
      </aside>

      <header className="mobile-header">
        <Link href="/dashboard" className="brand-lockup compact">
          <span className="brand-mark">AI</span><strong>AI-PAI</strong>
        </Link>
        <button type="button" onClick={() => setMobileOpen(!mobileOpen)} aria-label="打开导航">{mobileOpen ? <X /> : <Menu />}</button>
      </header>
      {mobileOpen && <div className="mobile-drawer"><Navigation items={userNav} pathname={pathname} mobile onNavigate={() => setMobileOpen(false)} /><button className="mobile-logout" onClick={logout}><LogOut size={16} />退出登录</button></div>}

      <main className="app-main">
        {!user.emailVerifiedAt && (
          <section className="mb-4 flex flex-col gap-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-amber-900 sm:flex-row sm:items-center" aria-label="邮箱验证提醒">
            <MailWarning size={18} className="shrink-0 text-amber-700" />
            <div className="min-w-0 flex-1"><strong className="block text-xs">邮箱尚未验证</strong><p className="mt-0.5 break-words text-[11px] text-amber-800">验证邮件将发送至 {user.email}，完成验证后即可创建 API Key。</p></div>
            <button type="button" onClick={() => void resendEmailVerification()} disabled={verificationSending || verificationCooldown > 0} className="btn shrink-0 border-amber-300 bg-white text-amber-900 hover:border-amber-400 disabled:opacity-60">
              {verificationSending ? <LoaderCircle size={14} className="animate-spin" /> : <Send size={14} />}
              {verificationSending ? '发送中' : verificationCooldown > 0 ? `${verificationCooldown} 秒后可重发` : '重新发送验证邮件'}
            </button>
          </section>
        )}
        {children}
      </main>
      <nav className="bottom-nav">
        {mobileNav.map(({ label, href, icon: Icon }) => <Link key={href} href={href} className={pathname === href ? 'is-active' : ''}><Icon size={18} /><span>{label}</span></Link>)}
      </nav>
    </div>
  );
}
