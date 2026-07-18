'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import {
  Activity, Cable, ChevronRight, CircleDollarSign, CreditCard, Database, Gift,
  Gauge, LogOut, Menu, Package, Radio, ReceiptText, ShieldCheck, Users, X,
} from 'lucide-react';
import { ADMIN_BASE_PATH } from '../../admin-path';
import { adminAuth, type AdminIdentity } from '@/lib/admin-api';
import { ADMIN_BUILD_COMMIT, ADMIN_BUILD_VERSION, reloadForBuild } from '@/lib/build-version';

const adminNav = [
  { label: '数据概览', href: '/dashboard', icon: Gauge },
  { label: '实时运营', href: '/api-operations', icon: Radio },
  { label: '用户管理', href: '/users', icon: Users },
  { label: 'API 调用', href: '/api-access', icon: Activity },
  { label: '上游接口', href: '/upstream-apis', icon: Cable },
  { label: '模型价格', href: '/prices', icon: CircleDollarSign },
  { label: '订阅套餐', href: '/packages', icon: Package },
  { label: '订阅管理', href: '/subscriptions', icon: CreditCard },
  { label: '充值流水', href: '/recharges', icon: ReceiptText },
  { label: '邀请返利', href: '/invites', icon: Gift },
  { label: '系统日志', href: '/logs', icon: Database },
  { label: '系统设置', href: '/settings', icon: ShieldCheck },
];

function Navigation({ pathname, mobile = false, onNavigate }: { pathname: string; mobile?: boolean; onNavigate: () => void }) {
  return (
    <nav className={mobile ? 'grid grid-cols-2 gap-2' : 'space-y-1'}>
      {adminNav.map(({ label, href, icon: Icon }) => (
        <Link key={href} href={href} onClick={onNavigate} className={`nav-item ${pathname === href ? 'is-active' : ''}`}>
          <Icon size={16} />
          <span>{label}</span>
          {!mobile && <ChevronRight size={13} className="nav-arrow" />}
        </Link>
      ))}
    </nav>
  );
}

export function AdminShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [identity, setIdentity] = useState<AdminIdentity | null>(null);
  const [ready, setReady] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);

  useEffect(() => {
    let active = true;
    void adminAuth.session()
      .then((response) => { if (active) setIdentity(response.data); })
      .catch(() => { if (active) router.replace('/login'); })
      .finally(() => { if (active) setReady(true); });
    return () => { active = false; };
  }, [pathname, router]);

  useEffect(() => {
    let active = true;
    const checkBuild = async () => {
      try {
        const response = await fetch(`${ADMIN_BASE_PATH}/api/build?_=${Date.now()}`, { cache: 'no-store' });
        if (!response.ok || !active) return;
        const build = await response.json() as { version?: string };
        if (build.version && build.version !== 'local' && build.version !== ADMIN_BUILD_VERSION) {
          reloadForBuild(build.version);
        }
      } catch {
        // The admin container may be restarting during an update; the next check retries.
      }
    };
    const firstCheck = window.setTimeout(() => void checkBuild(), 3_000);
    const timer = window.setInterval(() => void checkBuild(), 30_000);
    return () => {
      active = false;
      window.clearTimeout(firstCheck);
      window.clearInterval(timer);
    };
  }, []);

  const logout = async () => {
    await adminAuth.logout().catch(() => undefined);
    router.replace('/login');
    router.refresh();
  };

  if (!ready || !identity) {
    return <div className="min-h-screen grid place-items-center bg-[#f7f8f6]"><div className="loading-ring" /></div>;
  }

  return (
    <div className="app-frame admin-frame">
      <aside className="app-sidebar">
        <Link href="/dashboard" className="brand-lockup">
          <span className="brand-mark">AI</span>
          <span><strong>AI-PAI</strong><small title={`Commit ${ADMIN_BUILD_COMMIT}`}>管理控制台 · {ADMIN_BUILD_VERSION}</small></span>
        </Link>
        <div className="sidebar-section-label">运营管理</div>
        <div className="sidebar-scroll"><Navigation pathname={pathname} onNavigate={() => setMobileOpen(false)} /></div>
        <div className="sidebar-account">
          <span className="account-avatar">{identity.email.slice(0, 1).toUpperCase()}</span>
          <span className="account-copy"><strong>{identity.email}</strong><small>系统管理员</small></span>
          <button type="button" onClick={() => void logout()} title="退出登录"><LogOut size={16} /></button>
        </div>
      </aside>

      <header className="mobile-header">
        <Link href="/dashboard" className="brand-lockup compact"><span className="brand-mark">AI</span><strong>AI-PAI 后台</strong></Link>
        <button type="button" onClick={() => setMobileOpen(!mobileOpen)} aria-label="打开导航">{mobileOpen ? <X /> : <Menu />}</button>
      </header>
      {mobileOpen && <div className="mobile-drawer"><Navigation pathname={pathname} mobile onNavigate={() => setMobileOpen(false)} /><button className="mobile-logout" onClick={() => void logout()}><LogOut size={16} />退出登录</button></div>}
      <main className="app-main">{children}</main>
    </div>
  );
}
