'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  ArrowRight,
  CircleDollarSign,
  CreditCard,
  Crown,
  Gauge,
  Loader2,
  MailCheck,
  MailWarning,
  PackageCheck,
  Pencil,
  Plus,
  RefreshCw,
  ShieldCheck,
  Trash2,
  UserRoundCog,
  Wallet,
  X,
} from 'lucide-react';
import { toast } from 'sonner';
import { AppSelect } from '@/components/common/AppSelect';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { DataTable } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { StatusBadge } from '@/components/common/StatusBadge';
import { type Plan, type PortalUser, portalApi } from '@/lib/admin-api';
import { formatCNY, formatDate } from '@/lib/common/utils';

type UserDraft = {
  email: string;
  password: string;
  role: 'user' | 'admin';
  status: 'active' | 'disabled';
};

type GrantMode = 'plan' | 'custom';

const emptyDraft: UserDraft = { email: '', password: '', role: 'user', status: 'active' };
const pageSize = 12;

function subscriptionActive(user: PortalUser) {
  return user.subscription?.status === 'active';
}

function subscriptionName(user: PortalUser) {
  if (!subscriptionActive(user)) return '按余额计费';
  return user.subscription?.planName || user.subscription?.tier || '订阅套餐';
}

function BillingModeLabel({ user }: { user: PortalUser }) {
  if (subscriptionActive(user)) {
    return (
      <span className="inline-flex flex-col items-start gap-0.5">
        <span className="inline-flex items-center gap-1 rounded border border-amber-200 bg-amber-50 px-2 py-0.5 text-[11px] font-semibold text-amber-800"><Crown className="h-3 w-3" />{subscriptionName(user)}</span>
        <small className="text-[10px] text-zinc-400">至 {formatDate(user.subscription?.expiresAt || '', false)}</small>
      </span>
    );
  }
  return <span className="inline-flex items-center gap-1 rounded border border-blue-200 bg-blue-50 px-2 py-0.5 text-[11px] font-semibold text-blue-700"><Wallet className="h-3 w-3" />按余额计费</span>;
}

function SubscriptionStatusBadge({ user }: { user: PortalUser }) {
  const status = user.subscription?.status;
  if (status === 'active') return <span className="inline-flex rounded border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[11px] font-semibold text-emerald-700">生效中</span>;
  if (status === 'expired') return <span className="inline-flex rounded border border-amber-200 bg-amber-50 px-2 py-0.5 text-[11px] font-semibold text-amber-700">已到期</span>;
  if (status === 'canceled' || status === 'cancelled') return <span className="inline-flex rounded border border-red-200 bg-red-50 px-2 py-0.5 text-[11px] font-semibold text-red-700">已取消</span>;
  return <span className="inline-flex rounded border border-zinc-200 bg-zinc-50 px-2 py-0.5 text-[11px] font-semibold text-zinc-500">未订阅</span>;
}

export default function AdminUsersPage() {
  const [users, setUsers] = useState<PortalUser[]>([]);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [billingFilter, setBillingFilter] = useState('all');
  const [page, setPage] = useState(1);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<PortalUser | null>(null);
  const [draft, setDraft] = useState<UserDraft>(emptyDraft);
  const [grantUser, setGrantUser] = useState<PortalUser | null>(null);
  const [balanceUser, setBalanceUser] = useState<PortalUser | null>(null);
  const [balanceValue, setBalanceValue] = useState('');
  const [balanceRemark, setBalanceRemark] = useState('');
  const [grantPlanId, setGrantPlanId] = useState('');
  const [grantMode, setGrantMode] = useState<GrantMode>('plan');
  const [customGrantName, setCustomGrantName] = useState('自定义订阅');
  const [customGrantDays, setCustomGrantDays] = useState(30);
  const [customGrantQuota, setCustomGrantQuota] = useState(100);
  const [deleteCandidate, setDeleteCandidate] = useState<PortalUser | null>(null);
  const [actionId, setActionId] = useState('');
  const [verifyingId, setVerifyingId] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [userResponse, planResponse] = await Promise.all([
        portalApi.users(),
        portalApi.adminPlans(),
      ]);
      setUsers(userResponse.data);
      setPlans(planResponse.data);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : '用户列表加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  const filtered = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    return users.filter((user) => {
      const keywordMatch = !keyword || user.email.toLowerCase().includes(keyword) || user.id.toLowerCase().includes(keyword);
      const statusMatch = statusFilter === 'all' || user.status === statusFilter;
      const isSubscription = subscriptionActive(user);
      const billingMatch = billingFilter === 'all' || (billingFilter === 'subscription' ? isSubscription : !isSubscription);
      return keywordMatch && statusMatch && billingMatch;
    });
  }, [billingFilter, search, statusFilter, users]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const visible = filtered.slice((Math.min(page, totalPages) - 1) * pageSize, Math.min(page, totalPages) * pageSize);
  const activePlans = plans.filter((plan) => plan.status === 'active');
  const currentBalance = Number(balanceUser?.credits || 0);
  const nextBalance = Number(balanceValue);
  const balanceDelta = Number.isFinite(nextBalance) ? nextBalance - currentBalance : 0;

  const summary = useMemo(() => ({
    total: users.length,
    active: users.filter((user) => user.status === 'active').length,
    subscribed: users.filter(subscriptionActive).length,
    balance: users.reduce((sum, user) => sum + Number(user.credits || 0), 0),
  }), [users]);

  const resetPage = () => setPage(1);
  const updateDraft = <K extends keyof UserDraft>(key: K, value: UserDraft[K]) => setDraft((current) => ({ ...current, [key]: value }));

  const openCreate = () => {
    setEditing(null);
    setDraft(emptyDraft);
    setEditorOpen(true);
  };

  const openEdit = (user: PortalUser) => {
    setEditing(user);
    setDraft({
      email: user.email,
      password: '',
      role: user.role,
      status: user.status === 'active' ? 'active' : 'disabled',
    });
    setEditorOpen(true);
  };

  const saveUser = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!draft.email.trim()) return toast.error('请填写邮箱');
    if (!editing && draft.password.length < 6) return toast.error('新用户密码至少 6 位');
    setSaving(true);
    try {
      const input: Record<string, unknown> = {
        email: draft.email.trim(),
        role: draft.role,
        status: draft.status,
      };
      if (draft.password) input.password = draft.password;
      if (editing) {
        await portalApi.updateUser(editing.id, input);
      } else {
        const created = await portalApi.createUser(input);
        // The existing create handler always starts active; apply an optional disabled state afterward.
        if (draft.status === 'disabled') {
          await portalApi.updateUser(created.data.id, {
            email: created.data.email,
            role: created.data.role,
            status: 'disabled',
          });
        }
      }
      toast.success(editing ? '用户资料已更新' : '用户已创建');
      setEditorOpen(false);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '用户保存失败');
    } finally {
      setSaving(false);
    }
  };

  const toggleStatus = async (user: PortalUser) => {
    setActionId(user.id);
    try {
      const nextStatus = user.status === 'active' ? 'disabled' : 'active';
      await portalApi.updateUser(user.id, { email: user.email, role: user.role, status: nextStatus });
      toast.success(nextStatus === 'active' ? '用户已启用' : '用户已停用');
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '用户状态更新失败');
    } finally {
      setActionId('');
    }
  };

  const verifyEmail = async (user: PortalUser) => {
    if (user.emailVerifiedAt) return;
    setVerifyingId(user.id);
    try {
      const response = await portalApi.verifyUserEmail(user.id);
      setUsers((items) => items.map((item) => (item.id === user.id ? response.data : item)));
      toast.success(`已验证 ${user.email}`);
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '邮箱验证失败');
    } finally {
      setVerifyingId('');
    }
  };

  const openBalance = (user: PortalUser) => {
    setBalanceUser(user);
    setBalanceValue(String(Math.round(Number(user.credits || 0) * 10000) / 10000));
    setBalanceRemark('');
  };

  const updateBalance = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!balanceUser) return;
    if (!balanceValue.trim()) return toast.error('请填写调整后的余额');
    const value = Number(balanceValue);
    if (!Number.isFinite(value) || value < 0 || value > 99999999.9999) return toast.error('余额必须在 0 到 99999999.9999 之间');
    if (Math.abs(value - Number(balanceUser.credits || 0)) < 0.00005) return toast.error('调整后余额没有变化');
    if (!balanceRemark.trim() || balanceRemark.trim().length > 120) return toast.error('请填写 1-120 字的调整备注');
    setSaving(true);
    try {
      const response = await portalApi.updateUserBalance(balanceUser.id, { balance: value, remark: balanceRemark.trim() });
      setUsers((items) => items.map((item) => (item.id === balanceUser.id ? { ...item, credits: response.data.credits } : item)));
      toast.success(`已更新 ${balanceUser.email} 的余额`);
      setBalanceUser(null);
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '余额更新失败');
    } finally {
      setSaving(false);
    }
  };

  const openGrant = (user: PortalUser) => {
    setGrantUser(user);
    const currentPlanId = user.subscription?.source === 'admin_custom' ? '' : user.subscription?.planId || '';
    setGrantPlanId(activePlans.some((plan) => plan.id === currentPlanId) ? currentPlanId : activePlans[0]?.id || '');
    setGrantMode(user.subscription?.source === 'admin_custom' ? 'custom' : 'plan');
    setCustomGrantName(user.subscription?.source === 'admin_custom' ? user.subscription.planName || '自定义订阅' : '自定义订阅');
    setCustomGrantDays(30);
    setCustomGrantQuota(user.subscription?.source === 'admin_custom' ? Number(user.subscription.quotaLimit || 100) : 100);
  };

  const grantSubscription = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!grantUser) return;
    if (grantMode === 'plan' && !grantPlanId) return toast.error('请选择订阅套餐');
    if (grantMode === 'custom' && (!customGrantName.trim() || customGrantDays < 1 || customGrantDays > 3650 || customGrantQuota < 1)) {
      return toast.error('请填写有效的自定义订阅参数');
    }
    setSaving(true);
    try {
      await portalApi.grantSubscription(grantUser.id, grantMode === 'custom'
        ? { grantType: 'custom', name: customGrantName.trim(), durationDays: customGrantDays, quotaImages: customGrantQuota }
        : { grantType: 'plan', planId: grantPlanId });
      toast.success(`已为 ${grantUser.email} 发放${grantMode === 'custom' ? '自定义' : '套餐'}订阅`);
      setGrantUser(null);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '订阅发放失败');
    } finally {
      setSaving(false);
    }
  };

  const deleteUser = async () => {
    if (!deleteCandidate) return;
    try {
      await portalApi.deleteUser(deleteCandidate.id);
      toast.success('用户已删除');
      setDeleteCandidate(null);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '用户删除失败');
    }
  };

  const rowActions = (user: PortalUser) => (
    <div className="flex items-center justify-end gap-1">
      {!user.emailVerifiedAt && (
        <button type="button" onClick={() => void verifyEmail(user)} disabled={Boolean(verifyingId)} title="直接验证邮箱" aria-label={`直接验证 ${user.email} 的邮箱`} className="rounded p-1.5 text-emerald-700 hover:bg-emerald-50 disabled:opacity-40">
          {verifyingId === user.id ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <MailCheck className="h-3.5 w-3.5" />}
        </button>
      )}
      <button type="button" onClick={() => openGrant(user)} title="发放订阅" className="rounded p-1.5 text-[#0891B2] hover:bg-cyan-50"><CreditCard className="h-3.5 w-3.5" /></button>
      <button type="button" onClick={() => openEdit(user)} title="编辑用户" className="rounded p-1.5 text-zinc-600 hover:bg-zinc-100"><Pencil className="h-3.5 w-3.5" /></button>
      <button type="button" onClick={() => void toggleStatus(user)} disabled={actionId === user.id} title={user.status === 'active' ? '停用用户' : '启用用户'} className={`rounded p-1.5 ${user.status === 'active' ? 'text-amber-600 hover:bg-amber-50' : 'text-emerald-700 hover:bg-emerald-50'} disabled:opacity-40`}><ShieldCheck className="h-3.5 w-3.5" /></button>
      <button type="button" onClick={() => setDeleteCandidate(user)} title="删除用户" className="rounded p-1.5 text-red-600 hover:bg-red-50"><Trash2 className="h-3.5 w-3.5" /></button>
    </div>
  );

  return (
    <div className="space-y-5">
      <PageHeader title="用户管理" description="管理 API 客户、账户状态、余额与订阅权益。">
        <button type="button" onClick={() => void load()} disabled={loading} title="刷新用户" className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-[#DCE4DF] bg-white hover:border-[#12B76A] disabled:opacity-50"><RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} /></button>
        <button type="button" onClick={openCreate} className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[#047857] px-3 text-xs font-semibold text-white hover:bg-[#036b4f]"><Plus className="h-4 w-4" />新增用户</button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          ['全部用户', summary.total, '注册账户'],
          ['启用账户', summary.active, '可调用 API'],
          ['有效订阅', summary.subscribed, '订阅额度计费'],
          ['账户总余额', formatCNY(summary.balance), '按量计费余额'],
        ].map(([label, value, note]) => (
          <div key={String(label)} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
            <span className="text-[11px] font-semibold text-zinc-500">{label}</span>
            <strong className="mt-1.5 block text-xl text-[#17201B]">{value}</strong>
            <small className="mt-1 block text-[11px] text-zinc-400">{note}</small>
          </div>
        ))}
      </div>

      {error && (
        <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void load()} className="font-semibold underline">重试</button></div>
      )}

      {loading ? (
        <div className="grid min-h-[300px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <DataTable
          headers={[
            { key: 'account', label: '账户' },
            { key: 'emailVerification', label: '邮箱验证' },
            { key: 'role', label: '角色' },
            { key: 'billing', label: '计费方式' },
            { key: 'subscriptionStatus', label: '订阅状态' },
            { key: 'balance', label: '余额', className: 'text-right' },
            { key: 'status', label: '状态' },
            { key: 'created', label: '注册时间' },
            { key: 'actions', label: '操作', className: 'text-right' },
          ]}
          data={visible}
          searchPlaceholder="搜索邮箱或用户 ID"
          searchValue={search}
          onSearchChange={(value) => { setSearch(value); resetPage(); }}
          filterControls={(
            <>
              <AppSelect
                compact
                value={billingFilter}
                onValueChange={(value) => { setBillingFilter(value); resetPage(); }}
                ariaLabel="计费方式筛选"
                options={[
                  { value: 'all', label: '全部计费' },
                  { value: 'payg', label: '余额计费' },
                  { value: 'subscription', label: '订阅计费' },
                ]}
              />
              <AppSelect
                compact
                value={statusFilter}
                onValueChange={(value) => { setStatusFilter(value); resetPage(); }}
                ariaLabel="用户状态筛选"
                options={[
                  { value: 'all', label: '全部状态' },
                  { value: 'active', label: '已启用' },
                  { value: 'disabled', label: '已停用' },
                ]}
              />
              <span className="text-[11px] text-zinc-400">{filtered.length} 条</span>
            </>
          )}
          currentPage={Math.min(page, totalPages)}
          totalPages={totalPages}
          onPageChange={setPage}
          emptyState={<EmptyState title="暂无用户" description="调整筛选条件或创建一个 API 客户。" icon={UserRoundCog} />}
          renderRow={(user) => (
            <tr key={user.id} className="hover:bg-[#FAFBFA]">
              <td className="px-4 py-3"><strong className="block max-w-[220px] truncate font-medium">{user.email}</strong><small className="mt-0.5 block max-w-[220px] truncate font-mono text-[10px] text-zinc-400">{user.id}</small></td>
              <td className="px-4 py-3">
                {user.emailVerifiedAt ? (
                  <span className="inline-flex items-center gap-1 rounded border border-emerald-200 bg-emerald-50 px-1.5 py-0.5 text-[11px] font-medium text-emerald-700" title={`验证时间：${formatDate(user.emailVerifiedAt)}`}><MailCheck className="h-3 w-3" />已验证</span>
                ) : (
                  <span className="inline-flex items-center gap-1 rounded border border-amber-200 bg-amber-50 px-1.5 py-0.5 text-[11px] font-medium text-amber-700"><MailWarning className="h-3 w-3" />未验证</span>
                )}
              </td>
              <td className="px-4 py-3"><span className="rounded border border-zinc-200 bg-zinc-50 px-1.5 py-0.5 text-[11px]">{user.role === 'admin' ? '管理员' : 'API 客户'}</span></td>
              <td className="px-4 py-3"><BillingModeLabel user={user} /></td>
              <td className="px-4 py-3"><SubscriptionStatusBadge user={user} /></td>
              <td className="px-4 py-3 text-right">
                <button type="button" onClick={() => openBalance(user)} className="inline-flex items-center gap-1.5 rounded-md border border-emerald-200 bg-emerald-50 px-2 py-1 font-mono text-[11px] font-semibold text-[#047857] hover:border-emerald-300 hover:bg-emerald-100" title={`修改 ${user.email} 的余额`}>
                  <span>{formatCNY(Number(user.credits || 0))}</span><Pencil className="h-3 w-3" /><span className="font-sans">修改</span>
                </button>
              </td>
              <td className="px-4 py-3"><StatusBadge status={user.status === 'active' ? 'active' : 'disabled'} /></td>
              <td className="whitespace-nowrap px-4 py-3 text-zinc-500">{formatDate(user.createdAt || '')}</td>
              <td className="px-4 py-3">{rowActions(user)}</td>
            </tr>
          )}
          renderMobileItem={(user) => (
            <article key={user.id} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
              <div className="flex items-start justify-between gap-3"><div className="min-w-0"><strong className="block truncate text-sm">{user.email}</strong><small className="font-mono text-[10px] text-zinc-400">{user.id}</small></div><StatusBadge status={user.status === 'active' ? 'active' : 'disabled'} /></div>
              <div className="mt-3 grid grid-cols-2 gap-x-3 gap-y-3 border-y border-[#EDF0EE] py-3 text-xs"><span><small className="mb-0.5 block text-[10px] text-zinc-400">计费方式</small><BillingModeLabel user={user} /></span><span className="text-right"><small className="mb-0.5 block text-[10px] text-zinc-400">订阅状态</small><SubscriptionStatusBadge user={user} /></span><span><small className="block text-[10px] text-zinc-400">邮箱验证</small><span className={user.emailVerifiedAt ? 'text-emerald-700' : 'text-amber-700'}>{user.emailVerifiedAt ? '已验证' : '未验证'}</span></span><span className="text-right"><small className="block text-[10px] text-zinc-400">账户余额</small><button type="button" onClick={() => openBalance(user)} className="mt-0.5 inline-flex items-center gap-1 font-semibold text-[#047857]"><strong>{formatCNY(Number(user.credits || 0))}</strong><Pencil className="h-3 w-3" />修改</button></span></div>
              <div className="mt-2 flex items-center justify-between"><small className="text-[10px] text-zinc-400">{formatDate(user.createdAt || '')}</small>{rowActions(user)}</div>
            </article>
          )}
        />
      )}

      {editorOpen && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4">
          <form onSubmit={saveUser} className="w-full max-w-lg overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5"><div><h2 className="text-sm font-semibold">{editing ? '编辑用户' : '新增用户'}</h2><p className="mt-0.5 text-[11px] text-zinc-500">账户用于登录开发者工作台并调用 API。</p></div><button type="button" onClick={() => setEditorOpen(false)} className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button></div>
            <div className="grid grid-cols-1 gap-4 p-5 sm:grid-cols-2">
              <label className="sm:col-span-2"><span className="mb-1 block text-[11px] font-semibold text-zinc-500">邮箱</span><input required type="email" value={draft.email} onChange={(event) => updateDraft('email', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
              <label className="sm:col-span-2"><span className="mb-1 block text-[11px] font-semibold text-zinc-500">{editing ? '重置密码（留空保持不变）' : '初始密码'}</span><input required={!editing} minLength={editing ? undefined : 6} type="password" value={draft.password} onChange={(event) => updateDraft('password', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
              <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">角色</span><AppSelect value={draft.role} onValueChange={(value) => updateDraft('role', value as UserDraft['role'])} options={[{ value: 'user', label: 'API 客户' }, { value: 'admin', label: '管理员' }]} /></label>
              <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">状态</span><AppSelect value={draft.status} onValueChange={(value) => updateDraft('status', value as UserDraft['status'])} options={[{ value: 'active', label: '启用' }, { value: 'disabled', label: '停用' }]} /></label>
            </div>
            <div className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><button type="button" onClick={() => setEditorOpen(false)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button><button type="submit" disabled={saving} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}保存</button></div>
          </form>
        </div>
      )}

      {grantUser && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4">
          <form onSubmit={grantSubscription} className="w-full max-w-lg overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5"><div><h2 className="text-sm font-semibold">发放订阅</h2><p className="mt-0.5 max-w-[300px] truncate text-[11px] text-zinc-500">{grantUser.email}</p></div><button type="button" onClick={() => setGrantUser(null)} className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button></div>
            <div className="space-y-4 p-5">
              <div className="grid grid-cols-2 gap-1 rounded-md border border-[#DCE4DF] bg-[#F6F8F6] p-1">
                <button type="button" onClick={() => setGrantMode('plan')} aria-pressed={grantMode === 'plan'} className={`inline-flex h-9 items-center justify-center gap-1.5 rounded border text-[11px] font-semibold ${grantMode === 'plan' ? 'border-[#86EFAC] bg-white text-[#047857]' : 'border-transparent text-zinc-500'}`}><PackageCheck className="h-3.5 w-3.5" />套餐发放</button>
                <button type="button" onClick={() => setGrantMode('custom')} aria-pressed={grantMode === 'custom'} className={`inline-flex h-9 items-center justify-center gap-1.5 rounded border text-[11px] font-semibold ${grantMode === 'custom' ? 'border-[#86EFAC] bg-white text-[#047857]' : 'border-transparent text-zinc-500'}`}><Gauge className="h-3.5 w-3.5" />自定义额度</button>
              </div>
              {grantMode === 'plan' ? (
                <label><span className="mb-1.5 block text-[11px] font-semibold text-zinc-500">订阅套餐</span><AppSelect required value={grantPlanId} onValueChange={setGrantPlanId} options={[{ value: '', label: '请选择套餐' }, ...activePlans.map((plan) => ({ value: plan.id, label: `${plan.name} · ${plan.durationDays} 天 · ${plan.quotaImages} 张` }))]} /></label>
              ) : (
                <div className="grid grid-cols-2 gap-3">
                  <label className="col-span-2"><span className="mb-1 block text-[11px] font-semibold text-zinc-500">权益名称</span><input required maxLength={80} value={customGrantName} onChange={(event) => setCustomGrantName(event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
                  <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">有效天数</span><input required min={1} max={3650} type="number" value={customGrantDays} onChange={(event) => setCustomGrantDays(Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs outline-none focus:border-[#12B76A]" /></label>
                  <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">图片额度</span><input required min={1} max={100000000} type="number" value={customGrantQuota} onChange={(event) => setCustomGrantQuota(Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs outline-none focus:border-[#12B76A]" /></label>
                </div>
              )}
            </div>
            <div className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><button type="button" onClick={() => setGrantUser(null)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button><button type="submit" disabled={saving || (grantMode === 'plan' && !grantPlanId)} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}确认发放</button></div>
          </form>
        </div>
      )}

      {balanceUser && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4">
          <form onSubmit={updateBalance} className="w-full max-w-md overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5">
              <div className="flex min-w-0 items-center gap-2.5">
                <span className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-emerald-50 text-[#047857]"><CircleDollarSign className="h-4 w-4" /></span>
                <div className="min-w-0"><h2 className="text-sm font-semibold">修改账户余额</h2><p className="mt-0.5 truncate text-[11px] text-zinc-500">{balanceUser.email}</p></div>
              </div>
              <button type="button" onClick={() => setBalanceUser(null)} title="关闭" className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button>
            </div>
            <div className="space-y-4 p-5">
              <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-3 border-y border-[#EDF0EE] bg-[#FAFBFA] px-4 py-3">
                <span><small className="block text-[10px] font-semibold text-zinc-400">当前余额</small><strong className="mt-1 block font-mono text-sm text-zinc-700">{formatCNY(currentBalance)}</strong></span>
                <ArrowRight className="h-4 w-4 text-zinc-300" />
                <span className="text-right"><small className="block text-[10px] font-semibold text-zinc-400">调整后</small><strong className={`mt-1 block font-mono text-sm ${balanceDelta < 0 ? 'text-red-600' : 'text-[#047857]'}`}>{Number.isFinite(nextBalance) ? formatCNY(nextBalance) : '--'}</strong></span>
              </div>
              <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">调整后余额</span><input required autoFocus min={0} max={99999999.9999} step={0.0001} type="number" value={balanceValue} onChange={(event) => setBalanceValue(event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs outline-none focus:border-[#12B76A]" /></label>
              <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">调整备注</span><textarea required maxLength={120} rows={3} value={balanceRemark} onChange={(event) => setBalanceRemark(event.target.value)} placeholder="例如：活动补发、退款或余额修正" className="w-full resize-none rounded-md border border-[#DCE4DF] px-3 py-2 text-xs leading-5 outline-none focus:border-[#12B76A]" /><small className="mt-1 block text-right font-mono text-[10px] text-zinc-400">{balanceRemark.length}/120</small></label>
              {Number.isFinite(nextBalance) && Math.abs(balanceDelta) >= 0.00005 && (
                <div className={`flex items-center justify-between border-t border-[#EDF0EE] pt-3 text-xs ${balanceDelta < 0 ? 'text-red-600' : 'text-[#047857]'}`}><span>{balanceDelta < 0 ? '本次扣减' : '本次增加'}</span><strong className="font-mono">{formatCNY(Math.abs(balanceDelta))}</strong></div>
              )}
            </div>
            <div className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><button type="button" onClick={() => setBalanceUser(null)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button><button type="submit" disabled={saving || !balanceValue.trim() || !balanceRemark.trim() || !Number.isFinite(nextBalance) || nextBalance < 0 || nextBalance > 99999999.9999 || Math.abs(balanceDelta) < 0.00005} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}确认修改</button></div>
          </form>
        </div>
      )}

      <ConfirmDialog
        isOpen={Boolean(deleteCandidate)}
        onClose={() => setDeleteCandidate(null)}
        onConfirm={() => void deleteUser()}
        title="删除用户"
        description={`确定删除 ${deleteCandidate?.email || '该用户'} 吗？关联历史数据将按数据库现有约束处理。`}
        confirmText="删除"
        type="danger"
      />
    </div>
  );
}
