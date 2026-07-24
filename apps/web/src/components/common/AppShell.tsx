'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import {
  Activity, BadgeDollarSign, BookOpen, ChevronRight, HeartPulse, KeyRound, LayoutDashboard,
  BellRing, Gift, Images, LoaderCircle, LogOut, MailWarning, Megaphone, Menu, Send, Settings, WalletCards, X,
} from 'lucide-react';
import { toast } from 'sonner';
import { BillingRail } from './BillingRail';
import { WEB_BUILD_COMMIT, WEB_BUILD_VERSION, reloadForBuild } from '@/lib/build-version';
import { formatDate } from '@/lib/common/utils';
import { APIError, clearSession, getSession, portalApi, refreshSession, type Announcement, type OpenAIImageStatusSnapshot, type PortalUser } from '@/lib/portal-api';

type NavItem = { label: string; href: string; icon: React.ComponentType<{ size?: number; className?: string }> };

const USER_NAV_ITEMS: NavItem[] = [
  { label: '控制台', href: '/dashboard', icon: LayoutDashboard },
  { label: '生图台', href: '/playground', icon: Images },
  { label: 'API Key', href: '/api-keys', icon: KeyRound },
  { label: '用量记录', href: '/usage', icon: Activity },
  { label: '模型价目', href: '/prices', icon: BadgeDollarSign },
  { label: '计费中心', href: '/billing', icon: WalletCards },
  { label: '邀请返利', href: '/invite', icon: Gift },
  { label: '接口状态', href: '/status', icon: HeartPulse },
  { label: 'API 文档', href: '/docs', icon: BookOpen },
  { label: '账户设置', href: '/settings', icon: Settings },
];

const OPENAI_STATUS_REFRESH_MS = 60_000;

type OpenAIAlertBanner = {
  severity: 'warning' | 'critical';
  statusLabel: string;
  summary: string;
  incidentTitle?: string;
  publishedAt?: string;
  fetchedAt?: string;
  isCritical: boolean;
  componentCount: number;
};

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
  const [announcements, setAnnouncements] = useState<Announcement[]>([]);
  const [openAIStatus, setOpenAIStatus] = useState<OpenAIImageStatusSnapshot | null>(null);
  const [signingAnnouncementId, setSigningAnnouncementId] = useState('');
  const userId = user?.id;
  const userToken = user?.token;

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
    if (!ready || !userId || !userToken) return;
    let active = true;
    const current = getSession();
    if (!current || current.id !== userId) return;
    void portalApi.announcements(current)
      .then((response) => {
        if (active) setAnnouncements(response.data || []);
      })
      .catch((requestError) => {
        if (requestError instanceof APIError && (requestError.status === 401 || requestError.status === 403)) return;
        if (active) toast.error(requestError instanceof Error ? requestError.message : '公告加载失败');
      });
    return () => { active = false; };
  }, [ready, userId, userToken]);

  useEffect(() => {
    if (!ready) return;
    let active = true;
    let loading = false;
    const loadOpenAIStatus = async () => {
      if (loading) return;
      loading = true;
      try {
        const response = await portalApi.openAIImageStatus();
        if (!active) return;
        const snapshot = response.data || null;
        if (!snapshot) {
          setOpenAIStatus(null);
          return;
        }
        const status = String(snapshot.status || '').toLowerCase();
        const severity = String(snapshot.severity || '').toLowerCase();
        if (snapshot.reachable === false || status === 'operational' || severity === 'ok') {
          setOpenAIStatus(null);
          return;
        }
        setOpenAIStatus(snapshot);
      } catch {
        // 保留最近一次可见异常，避免短暂网络抖动把顶部提醒闪没
      } finally {
        loading = false;
      }
    };
    void loadOpenAIStatus();
    const refreshVisible = () => {
      if (document.visibilityState === 'visible') void loadOpenAIStatus();
    };
    const timer = window.setInterval(() => void loadOpenAIStatus(), OPENAI_STATUS_REFRESH_MS);
    window.addEventListener('focus', refreshVisible);
    document.addEventListener('visibilitychange', refreshVisible);
    return () => {
      active = false;
      window.clearInterval(timer);
      window.removeEventListener('focus', refreshVisible);
      document.removeEventListener('visibilitychange', refreshVisible);
    };
  }, [ready]);

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

  useEffect(() => {
    let active = true;
    const checkBuild = async () => {
      try {
        const response = await fetch(`/api/build?_=${Date.now()}`, { cache: 'no-store' });
        if (!response.ok || !active) return;
        const build = await response.json() as { version?: string };
        if (build.version && build.version !== 'local' && build.version !== WEB_BUILD_VERSION) {
          reloadForBuild(build.version);
        }
      } catch {
        // The web container may be restarting during an update; the next check retries.
      }
    };
    const checkVisibleBuild = () => {
      if (document.visibilityState === 'visible') void checkBuild();
    };
    const firstCheck = window.setTimeout(() => void checkBuild(), 1_000);
    const timer = window.setInterval(() => void checkBuild(), 30_000);
    window.addEventListener('focus', checkVisibleBuild);
    document.addEventListener('visibilitychange', checkVisibleBuild);
    return () => {
      active = false;
      window.clearTimeout(firstCheck);
      window.clearInterval(timer);
      window.removeEventListener('focus', checkVisibleBuild);
      document.removeEventListener('visibilitychange', checkVisibleBuild);
    };
  }, []);

  if (!ready || !user) {
    return <div className="min-h-screen grid place-items-center bg-[#f7f8f6]"><div className="loading-ring" /></div>;
  }

  const mobileNav = USER_NAV_ITEMS.filter((item) => (
    ['/dashboard', '/playground', '/usage', '/prices', '/billing'].includes(item.href)
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
  const bannerAnnouncements = announcements.filter((item) => item.displayMode === 'banner');
  const popupAnnouncements = announcements.filter((item) => item.displayMode === 'popup');
  const activePopup = popupAnnouncements[0] || null;
  const openAIAlert: OpenAIAlertBanner | null = (() => {
    if (!openAIStatus) return null;
    const status = String(openAIStatus.status || '').toLowerCase();
    const severity = String(openAIStatus.severity || '').toLowerCase();
    if (openAIStatus.reachable === false || status === 'operational' || severity === 'ok') return null;
    return {
      severity: severity === 'critical' ? 'critical' : 'warning',
      statusLabel: openAIStatus.statusLabel || 'OpenAI Image 异常',
      summary: openAIStatus.summary || 'OpenAI Image 相关事件正在影响生图能力。',
      incidentTitle: openAIStatus.latestImageIncident?.title || '',
      publishedAt: openAIStatus.latestImageIncident?.publishedAt || '',
      fetchedAt: openAIStatus.fetchedAt || '',
      isCritical: severity === 'critical' || status === 'outage' || status === 'partial_outage',
      componentCount: Array.isArray(openAIStatus.affectedComponents) ? openAIStatus.affectedComponents.length : 0,
    };
  })();
  const signAnnouncement = async () => {
    if (!activePopup || signingAnnouncementId) return;
    setSigningAnnouncementId(activePopup.id);
    try {
      await portalApi.signAnnouncement(user, activePopup.id);
      setAnnouncements((current) => current.filter((item) => item.id !== activePopup.id));
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '公告确认失败');
    } finally {
      setSigningAnnouncementId('');
    }
  };

  return (
    <div className="app-frame">
      <aside className="app-sidebar">
        <Link href="/dashboard" className="brand-lockup">
          <span className="brand-mark">AI</span>
          <span><strong>AI-PAI</strong><small title={`Commit ${WEB_BUILD_COMMIT}`}>API 中转站 · {WEB_BUILD_VERSION}</small></span>
        </Link>
        <BillingRail user={user} />
        <div className="sidebar-section-label">开发者工作台</div>
        <div className="sidebar-scroll"><Navigation items={USER_NAV_ITEMS} pathname={pathname} onNavigate={() => setMobileOpen(false)} /></div>
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
      {mobileOpen && <div className="mobile-drawer"><Navigation items={USER_NAV_ITEMS} pathname={pathname} mobile onNavigate={() => setMobileOpen(false)} /><button className="mobile-logout" onClick={logout}><LogOut size={16} />退出登录</button></div>}

      <main className="app-main">
        {(openAIAlert || bannerAnnouncements.length > 0) && (
          <div className="mb-4 space-y-2" aria-label="站内公告">
            {openAIAlert && (
              <section
                className={`flex flex-col gap-3 rounded-md border px-4 py-3 ${openAIAlert.isCritical ? 'border-red-200 bg-red-50 text-red-950' : 'border-amber-200 bg-amber-50 text-amber-950'}`}
                role="alert"
              >
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div className="flex min-w-0 items-start gap-3">
                    <span className={`mt-0.5 grid h-8 w-8 shrink-0 place-items-center rounded-md ${openAIAlert.isCritical ? 'bg-white text-red-700' : 'bg-white text-amber-700'}`}>
                      <HeartPulse size={16} />
                    </span>
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <strong className="block text-xs">OpenAI 生图异常监控</strong>
                        <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-semibold ${openAIAlert.isCritical ? 'bg-red-100 text-red-700' : 'bg-amber-100 text-amber-700'}`}>
                          {openAIAlert.statusLabel}
                        </span>
                      </div>
                      <p className="mt-1 whitespace-pre-wrap break-words text-[11px] leading-5">
                        {openAIAlert.summary}
                      </p>
                      {openAIAlert.incidentTitle && (
                        <p className={`mt-1 break-words text-[10px] leading-4 ${openAIAlert.isCritical ? 'text-red-700/80' : 'text-amber-800/80'}`}>
                          最新事件：{openAIAlert.incidentTitle}
                        </p>
                      )}
                    </div>
                  </div>
                  <Link
                    href="/status"
                    className={`btn shrink-0 justify-center whitespace-nowrap ${openAIAlert.isCritical ? 'border-red-200 bg-white text-red-700 hover:border-red-300 hover:bg-red-50' : 'border-amber-200 bg-white text-amber-700 hover:border-amber-300 hover:bg-amber-50'}`}
                  >
                    查看状态页
                  </Link>
                </div>
                <div className={`flex flex-wrap items-center gap-x-4 gap-y-1 text-[10px] ${openAIAlert.isCritical ? 'text-red-700/80' : 'text-amber-800/80'}`}>
                  <span>更新：{openAIAlert.fetchedAt ? formatDate(openAIAlert.fetchedAt) : '-'}</span>
                  {openAIAlert.publishedAt && <span>事件时间：{formatDate(openAIAlert.publishedAt)}</span>}
                  <span>来源：OpenAI 官方状态源</span>
                  {openAIAlert.componentCount > 0 && <span>影响组件：{openAIAlert.componentCount}</span>}
                </div>
              </section>
            )}
            {bannerAnnouncements.map((item) => (
              <section key={item.id} className="flex items-start gap-3 rounded-md border border-blue-200 bg-blue-50 px-4 py-3 text-blue-950">
                <span className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-md bg-white text-blue-700"><Megaphone size={15} /></span>
                <div className="min-w-0 flex-1"><strong className="block text-xs">{item.title}</strong><p className="mt-1 whitespace-pre-wrap break-words text-[11px] leading-5 text-blue-800">{item.content}</p></div>
              </section>
            ))}
          </div>
        )}
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
      {activePopup && (
        <div className="modal-backdrop" role="presentation">
          <section className="modal-panel max-w-[460px] overflow-hidden" role="dialog" aria-modal="true" aria-labelledby="announcement-dialog-title" aria-describedby="announcement-dialog-content">
            <div className="border-b border-[#EDF0EE] bg-[#FAFBFA] px-5 py-4">
              <div className="flex items-center gap-3">
                <span className="grid h-9 w-9 shrink-0 place-items-center rounded-md bg-amber-50 text-amber-700"><BellRing size={18} /></span>
                <div className="min-w-0"><small className="block text-[10px] font-semibold text-amber-700">站内公告{popupAnnouncements.length > 1 ? ` · 1/${popupAnnouncements.length}` : ''}</small><strong id="announcement-dialog-title" className="mt-0.5 block break-words text-sm">{activePopup.title}</strong></div>
              </div>
            </div>
            <div className="px-5 py-5">
              <p id="announcement-dialog-content" className="max-h-[46vh] overflow-y-auto whitespace-pre-wrap break-words text-xs leading-6 text-zinc-700">{activePopup.content}</p>
            </div>
            <div className="flex items-center justify-between gap-3 border-t border-[#EDF0EE] bg-[#FAFBFA] px-5 py-3">
              <small className="text-[10px] text-zinc-400">确认后不再重复显示</small>
              <button type="button" onClick={() => void signAnnouncement()} disabled={Boolean(signingAnnouncementId)} className="btn primary min-w-24 justify-center disabled:opacity-60">
                {signingAnnouncementId && <LoaderCircle size={14} className="animate-spin" />}
                {signingAnnouncementId ? '确认中' : '我知道了'}
              </button>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
