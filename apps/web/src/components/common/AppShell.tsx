'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import {
  Activity, BookOpen, ChevronRight, KeyRound, LayoutDashboard,
  LogOut, Menu, Settings, WalletCards, X,
} from 'lucide-react';
import { BillingRail } from './BillingRail';
import { clearSession, getSession, refreshSession, type PortalUser } from '@/lib/portal-api';

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

  if (!ready || !user) {
    return <div className="min-h-screen grid place-items-center bg-[#f7f8f6]"><div className="loading-ring" /></div>;
  }

  const userNav = [
    { label: '控制台', href: '/dashboard', icon: LayoutDashboard },
    { label: 'API Key', href: '/api-keys', icon: KeyRound },
    { label: '用量记录', href: '/usage', icon: Activity },
    { label: '计费中心', href: '/billing', icon: WalletCards },
    { label: 'API 文档', href: '/docs', icon: BookOpen },
    { label: '账户设置', href: '/settings', icon: Settings },
  ];
  const logout = () => {
    clearSession();
    router.replace('/login');
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

      <main className="app-main">{children}</main>
      <nav className="bottom-nav">
        {userNav.slice(0, 4).map(({ label, href, icon: Icon }) => <Link key={href} href={href} className={pathname === href ? 'is-active' : ''}><Icon size={18} /><span>{label}</span></Link>)}
      </nav>
    </div>
  );
}
