'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import {
  Activity, BookOpen, Cable, ChevronRight, CircleDollarSign, CreditCard,
  Database, Gauge, KeyRound, LayoutDashboard, LogOut, Menu, Package,
  ReceiptText, Settings, ShieldCheck, Users, WalletCards, X,
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
      if (pathname.startsWith('/admin') && fresh.role !== 'admin') router.replace('/dashboard');
    }).catch(() => {
      clearSession();
      if (active) router.replace('/login');
    }).finally(() => active && setReady(true));
    return () => { active = false; };
  }, [pathname, router]);

  if (!ready || !user) {
    return <div className="min-h-screen grid place-items-center bg-[#f7f8f6]"><div className="loading-ring" /></div>;
  }

  const isAdmin = pathname.startsWith('/admin');
  const userNav = [
    { label: '控制台', href: '/dashboard', icon: LayoutDashboard },
    { label: 'API Key', href: '/api-keys', icon: KeyRound },
    { label: '用量记录', href: '/usage', icon: Activity },
    { label: '计费中心', href: '/billing', icon: WalletCards },
    { label: 'API 文档', href: '/docs', icon: BookOpen },
    { label: '账户设置', href: '/settings', icon: Settings },
  ];
  const adminNav = [
    { label: '数据概览', href: '/admin/dashboard', icon: Gauge },
    { label: '用户管理', href: '/admin/users', icon: Users },
    { label: 'API 调用', href: '/admin/api-access', icon: Activity },
    { label: '上游接口', href: '/admin/upstream-apis', icon: Cable },
    { label: '模型价格', href: '/admin/prices', icon: CircleDollarSign },
    { label: '订阅套餐', href: '/admin/packages', icon: Package },
    { label: '订阅管理', href: '/admin/subscriptions', icon: CreditCard },
    { label: '充值流水', href: '/admin/recharges', icon: ReceiptText },
    { label: '系统日志', href: '/admin/logs', icon: Database },
    { label: '系统设置', href: '/admin/settings', icon: ShieldCheck },
  ];
  const nav = isAdmin ? adminNav : userNav;

  const logout = () => {
    clearSession();
    router.replace('/login');
  };

  return (
    <div className="app-frame">
      <aside className="app-sidebar">
        <Link href={isAdmin ? '/admin/dashboard' : '/dashboard'} className="brand-lockup">
          <span className="brand-mark">AI</span>
          <span><strong>AI-PAI</strong><small>{isAdmin ? '管理控制台' : 'API 中转站'}</small></span>
        </Link>
        {!isAdmin && <BillingRail user={user} />}
        <div className="sidebar-section-label">{isAdmin ? '运营管理' : '开发者工作台'}</div>
        <div className="sidebar-scroll"><Navigation items={nav} pathname={pathname} onNavigate={() => setMobileOpen(false)} /></div>
        {user.role === 'admin' && (
          <Link className="workspace-switch" href={isAdmin ? '/dashboard' : '/admin/dashboard'}>
            {isAdmin ? '进入用户工作台' : '进入管理后台'}
          </Link>
        )}
        <div className="sidebar-account">
          <span className="account-avatar">{user.email.slice(0, 1).toUpperCase()}</span>
          <span className="account-copy"><strong>{user.email}</strong><small>{user.role === 'admin' ? '系统管理员' : 'API 客户'}</small></span>
          <button type="button" onClick={logout} title="退出登录"><LogOut size={16} /></button>
        </div>
      </aside>

      <header className="mobile-header">
        <Link href={isAdmin ? '/admin/dashboard' : '/dashboard'} className="brand-lockup compact">
          <span className="brand-mark">AI</span><strong>AI-PAI</strong>
        </Link>
        <button type="button" onClick={() => setMobileOpen(!mobileOpen)} aria-label="打开导航">{mobileOpen ? <X /> : <Menu />}</button>
      </header>
      {mobileOpen && <div className="mobile-drawer"><Navigation items={nav} pathname={pathname} mobile onNavigate={() => setMobileOpen(false)} /><button className="mobile-logout" onClick={logout}><LogOut size={16} />退出登录</button></div>}

      <main className="app-main">{children}</main>
      {!isAdmin && (
        <nav className="bottom-nav">
          {userNav.slice(0, 4).map(({ label, href, icon: Icon }) => <Link key={href} href={href} className={pathname === href ? 'is-active' : ''}><Icon size={18} /><span>{label}</span></Link>)}
        </nav>
      )}
    </div>
  );
}
