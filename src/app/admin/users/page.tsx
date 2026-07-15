'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  CreditCard,
  Loader2,
  Pencil,
  Plus,
  RefreshCw,
  ShieldCheck,
  Trash2,
  UserRoundCog,
  X,
} from 'lucide-react';
import { toast } from 'sonner';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { DataTable } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { StatusBadge } from '@/components/common/StatusBadge';
import { type Plan, type PortalUser, portalApi } from '@/lib/portal-api';
import { formatCNY, formatDate } from '@/lib/common/utils';

type UserDraft = {
  email: string;
  password: string;
  role: 'user' | 'admin';
  status: 'active' | 'disabled';
};

const emptyDraft: UserDraft = { email: '', password: '', role: 'user', status: 'active' };
const pageSize = 12;

function subscriptionActive(user: PortalUser) {
  return user.subscription?.status === 'active';
}

function subscriptionName(user: PortalUser) {
  if (!subscriptionActive(user)) return '按余额计费';
  return user.subscription?.planName || user.subscription?.tier || '订阅套餐';
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
  const [grantPlanId, setGrantPlanId] = useState('');
  const [deleteCandidate, setDeleteCandidate] = useState<PortalUser | null>(null);
  const [actionId, setActionId] = useState('');

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

  const openGrant = (user: PortalUser) => {
    setGrantUser(user);
    setGrantPlanId(user.subscription?.planId || activePlans[0]?.id || '');
  };

  const grantSubscription = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!grantUser || !grantPlanId) return toast.error('请选择订阅套餐');
    setSaving(true);
    try {
      await portalApi.grantSubscription(grantUser.id, { planId: grantPlanId });
      toast.success(`已为 ${grantUser.email} 发放订阅`);
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
            <span className="text-[10px] font-semibold text-zinc-500">{label}</span>
            <strong className="mt-1.5 block text-xl text-[#17201B]">{value}</strong>
            <small className="mt-1 block text-[10px] text-zinc-400">{note}</small>
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
            { key: 'role', label: '角色' },
            { key: 'billing', label: '计费方式' },
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
              <select value={billingFilter} onChange={(event) => { setBillingFilter(event.target.value); resetPage(); }} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-2 text-xs outline-none focus:border-[#12B76A]">
                <option value="all">全部计费</option><option value="payg">余额计费</option><option value="subscription">订阅计费</option>
              </select>
              <select value={statusFilter} onChange={(event) => { setStatusFilter(event.target.value); resetPage(); }} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-2 text-xs outline-none focus:border-[#12B76A]">
                <option value="all">全部状态</option><option value="active">已启用</option><option value="disabled">已停用</option>
              </select>
              <span className="text-[10px] text-zinc-400">{filtered.length} 条</span>
            </>
          )}
          currentPage={Math.min(page, totalPages)}
          totalPages={totalPages}
          onPageChange={setPage}
          emptyState={<EmptyState title="暂无用户" description="调整筛选条件或创建一个 API 客户。" icon={UserRoundCog} />}
          renderRow={(user) => (
            <tr key={user.id} className="hover:bg-[#FAFBFA]">
              <td className="px-4 py-3"><strong className="block max-w-[220px] truncate font-medium">{user.email}</strong><small className="mt-0.5 block max-w-[220px] truncate font-mono text-[9px] text-zinc-400">{user.id}</small></td>
              <td className="px-4 py-3"><span className="rounded border border-zinc-200 bg-zinc-50 px-1.5 py-0.5 text-[10px]">{user.role === 'admin' ? '管理员' : 'API 客户'}</span></td>
              <td className="px-4 py-3"><strong className="block text-[11px] font-medium">{subscriptionName(user)}</strong>{subscriptionActive(user) && <small className="mt-0.5 block text-[9px] text-zinc-400">至 {formatDate(user.subscription?.expiresAt || '', false)}</small>}</td>
              <td className="px-4 py-3 text-right font-mono font-semibold text-[#047857]">{formatCNY(Number(user.credits || 0))}</td>
              <td className="px-4 py-3"><StatusBadge status={user.status === 'active' ? 'active' : 'disabled'} /></td>
              <td className="whitespace-nowrap px-4 py-3 text-zinc-500">{formatDate(user.createdAt || '')}</td>
              <td className="px-4 py-3">{rowActions(user)}</td>
            </tr>
          )}
          renderMobileItem={(user) => (
            <article key={user.id} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
              <div className="flex items-start justify-between gap-3"><div className="min-w-0"><strong className="block truncate text-sm">{user.email}</strong><small className="font-mono text-[9px] text-zinc-400">{user.id}</small></div><StatusBadge status={user.status === 'active' ? 'active' : 'disabled'} /></div>
              <div className="mt-3 grid grid-cols-2 gap-2 border-y border-[#EDF0EE] py-3 text-xs"><span><small className="block text-[9px] text-zinc-400">计费方式</small>{subscriptionName(user)}</span><span className="text-right"><small className="block text-[9px] text-zinc-400">账户余额</small><strong className="text-[#047857]">{formatCNY(Number(user.credits || 0))}</strong></span></div>
              <div className="mt-2 flex items-center justify-between"><small className="text-[9px] text-zinc-400">{formatDate(user.createdAt || '')}</small>{rowActions(user)}</div>
            </article>
          )}
        />
      )}

      {editorOpen && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4">
          <form onSubmit={saveUser} className="w-full max-w-lg overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5"><div><h2 className="text-sm font-semibold">{editing ? '编辑用户' : '新增用户'}</h2><p className="mt-0.5 text-[10px] text-zinc-500">账户用于登录开发者工作台并调用 API。</p></div><button type="button" onClick={() => setEditorOpen(false)} className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button></div>
            <div className="grid grid-cols-1 gap-4 p-5 sm:grid-cols-2">
              <label className="sm:col-span-2"><span className="mb-1 block text-[10px] font-semibold text-zinc-500">邮箱</span><input required type="email" value={draft.email} onChange={(event) => updateDraft('email', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
              <label className="sm:col-span-2"><span className="mb-1 block text-[10px] font-semibold text-zinc-500">{editing ? '重置密码（留空保持不变）' : '初始密码'}</span><input required={!editing} minLength={editing ? undefined : 6} type="password" value={draft.password} onChange={(event) => updateDraft('password', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
              <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">角色</span><select value={draft.role} onChange={(event) => updateDraft('role', event.target.value as UserDraft['role'])} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs"><option value="user">API 客户</option><option value="admin">管理员</option></select></label>
              <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">状态</span><select value={draft.status} onChange={(event) => updateDraft('status', event.target.value as UserDraft['status'])} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs"><option value="active">启用</option><option value="disabled">停用</option></select></label>
            </div>
            <div className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><button type="button" onClick={() => setEditorOpen(false)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button><button type="submit" disabled={saving} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}保存</button></div>
          </form>
        </div>
      )}

      {grantUser && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4">
          <form onSubmit={grantSubscription} className="w-full max-w-md overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5"><div><h2 className="text-sm font-semibold">发放订阅</h2><p className="mt-0.5 max-w-[300px] truncate text-[10px] text-zinc-500">{grantUser.email}</p></div><button type="button" onClick={() => setGrantUser(null)} className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button></div>
            <div className="p-5"><label><span className="mb-1.5 block text-[10px] font-semibold text-zinc-500">订阅套餐</span><select required value={grantPlanId} onChange={(event) => setGrantPlanId(event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]"><option value="">请选择套餐</option>{activePlans.map((plan) => <option key={plan.id} value={plan.id}>{plan.name} · {plan.durationDays} 天 · {plan.quotaImages} 张</option>)}</select></label><p className="mt-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-[10px] leading-5 text-amber-800">发放后立即生效；已有订阅将按 Go 后端现有规则续期或替换。</p></div>
            <div className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><button type="button" onClick={() => setGrantUser(null)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button><button type="submit" disabled={saving || !activePlans.length} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}确认发放</button></div>
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
