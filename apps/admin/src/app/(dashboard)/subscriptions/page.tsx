'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { CreditCard, Gift, Loader2, RefreshCw, X } from 'lucide-react';
import { toast } from 'sonner';
import { DataTable } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { type Plan, type PortalUser, portalApi } from '@/lib/admin-api';
import { formatDate } from '@/lib/common/utils';

const pageSize = 15;

function isActive(user: PortalUser) {
  return user.subscription?.status === 'active';
}

function expiryDays(user: PortalUser) {
  const time = Date.parse(user.subscription?.expiresAt || '');
  if (!Number.isFinite(time)) return null;
  return Math.ceil((time - Date.now()) / 86_400_000);
}

export default function AdminSubscriptionsPage() {
  const [users, setUsers] = useState<PortalUser[]>([]);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('active');
  const [page, setPage] = useState(1);
  const [grantOpen, setGrantOpen] = useState(false);
  const [userId, setUserId] = useState('');
  const [planId, setPlanId] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [userResponse, planResponse] = await Promise.all([
        portalApi.users(),
        portalApi.adminPlans(),
      ]);
      setUsers(userResponse.data.filter((user) => user.role !== 'admin'));
      setPlans(planResponse.data);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : '订阅数据加载失败');
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
      const active = isActive(user);
      const matchesKeyword = !keyword || `${user.email} ${user.id} ${user.subscription?.planName || ''}`.toLowerCase().includes(keyword);
      const matchesStatus = statusFilter === 'all' || (statusFilter === 'active' ? active : !active);
      return matchesKeyword && matchesStatus;
    });
  }, [search, statusFilter, users]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const currentPage = Math.min(page, totalPages);
  const visible = filtered.slice((currentPage - 1) * pageSize, currentPage * pageSize);
  const activePlans = plans.filter((plan) => plan.status === 'active');
  const activeUsers = users.filter(isActive);

  const summary = useMemo(() => ({
    active: activeUsers.length,
    expiring: activeUsers.filter((user) => { const days = expiryDays(user); return days !== null && days >= 0 && days <= 7; }).length,
    remaining: activeUsers.reduce((sum, user) => sum + Number(user.subscription?.effectiveQuotaRemaining ?? user.subscription?.quotaRemaining ?? 0), 0),
    plans: activePlans.length,
  }), [activePlans.length, activeUsers]);

  const openGrant = (user?: PortalUser) => {
    setUserId(user?.id || users[0]?.id || '');
    setPlanId(user?.subscription?.planId || activePlans[0]?.id || '');
    setGrantOpen(true);
  };

  const grant = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!userId || !planId) return toast.error('请选择用户和订阅套餐');
    const user = users.find((item) => item.id === userId);
    setSaving(true);
    try {
      await portalApi.grantSubscription(userId, { planId });
      toast.success(`已为 ${user?.email || userId} 发放订阅`);
      setGrantOpen(false);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '订阅发放失败');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-5">
      <PageHeader title="订阅管理" description="查看客户订阅状态，并按现有 Go 规则发放、续期或更换套餐。">
        <button type="button" onClick={() => void load()} disabled={loading} title="刷新订阅" className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-[#DCE4DF] bg-white hover:border-[#12B76A] disabled:opacity-50"><RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} /></button>
        <button type="button" onClick={() => openGrant()} disabled={!users.length || !activePlans.length} className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[#047857] px-3 text-xs font-semibold text-white hover:bg-[#036b4f] disabled:opacity-40"><Gift className="h-4 w-4" />发放订阅</button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          ['有效订阅', summary.active, '当前生效客户'],
          ['7 天内到期', summary.expiring, '需要续期关注'],
          ['剩余额度', summary.remaining.toLocaleString('zh-CN'), '有效订阅合计'],
          ['上架套餐', summary.plans, '当前可发放'],
        ].map(([label, value, note]) => <div key={String(label)} className="rounded-md border border-[#DCE4DF] bg-white p-3.5"><span className="text-[10px] font-semibold text-zinc-500">{label}</span><strong className="mt-1.5 block text-xl">{value}</strong><small className="mt-1 block text-[10px] text-zinc-400">{note}</small></div>)}
      </div>

      {error && <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void load()} className="font-semibold underline">重试</button></div>}

      {loading ? (
        <div className="grid min-h-[300px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <DataTable
          headers={[
            { key: 'user', label: 'API 客户' },
            { key: 'plan', label: '订阅套餐' },
            { key: 'quota', label: '额度使用' },
            { key: 'period', label: '有效期' },
            { key: 'status', label: '状态' },
            { key: 'action', label: '操作', className: 'text-right' },
          ]}
          data={visible}
          searchPlaceholder="搜索邮箱、用户 ID 或套餐"
          searchValue={search}
          onSearchChange={(value) => { setSearch(value); setPage(1); }}
          filterControls={<><select value={statusFilter} onChange={(event) => { setStatusFilter(event.target.value); setPage(1); }} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-2 text-xs"><option value="active">有效订阅</option><option value="none">未订阅</option><option value="all">全部客户</option></select><span className="text-[10px] text-zinc-400">{filtered.length} 条</span></>}
          currentPage={currentPage}
          totalPages={totalPages}
          onPageChange={setPage}
          emptyState={<EmptyState title="暂无订阅记录" description="可从全部客户中选择用户并发放订阅套餐。" icon={CreditCard} action={<button type="button" onClick={() => openGrant()} disabled={!users.length || !activePlans.length} className="h-8 rounded-md bg-[#047857] px-3 text-xs font-semibold text-white disabled:opacity-40">发放订阅</button>} />}
          renderRow={(user) => {
            const active = isActive(user);
            const limit = Number(user.subscription?.quotaLimit ?? user.subscription?.quotaImages ?? 0);
            const used = Number(user.subscription?.quotaUsed || 0);
            const percent = limit > 0 ? Math.min(100, Math.round((used / limit) * 100)) : 0;
            const days = expiryDays(user);
            return (
              <tr key={user.id} className="hover:bg-[#FAFBFA]">
                <td className="px-4 py-3"><strong className="block max-w-[210px] truncate font-medium">{user.email}</strong><small className="font-mono text-[9px] text-zinc-400">{user.id}</small></td>
                <td className="px-4 py-3">{active ? <><strong className="block text-[11px] font-medium">{user.subscription?.planName || user.subscription?.tier || '订阅套餐'}</strong><small className="text-[9px] text-zinc-400">{user.subscription?.isPaid === false ? '后台发放/权益订阅' : '付费订阅'}</small></> : <span className="text-zinc-400">按余额计费</span>}</td>
                <td className="px-4 py-3">{active ? <div className="w-36"><div className="mb-1 flex justify-between font-mono text-[9px] text-zinc-500"><span>{used} / {limit || '不限'}</span><span>{percent}%</span></div><div className="h-1.5 overflow-hidden rounded-full bg-zinc-100"><span className="block h-full bg-[#12B76A]" style={{ width: `${percent}%` }} /></div></div> : '-'}</td>
                <td className="px-4 py-3"><span className="block whitespace-nowrap text-[10px]">{active ? formatDate(user.subscription?.expiresAt || '', false) : '-'}</span>{active && days !== null && <small className={`text-[9px] ${days <= 7 ? 'text-amber-700' : 'text-zinc-400'}`}>{days < 0 ? '已到期' : `剩余 ${days} 天`}</small>}</td>
                <td className="px-4 py-3"><span className={`rounded border px-2 py-0.5 text-[10px] font-semibold ${active ? 'border-emerald-200 bg-emerald-50 text-emerald-700' : 'border-zinc-200 bg-zinc-50 text-zinc-500'}`}>{active ? '生效中' : '未订阅'}</span></td>
                <td className="px-4 py-3 text-right"><button type="button" onClick={() => openGrant(user)} disabled={!activePlans.length} className="rounded-md border border-[#DCE4DF] bg-white px-2.5 py-1 text-[10px] font-semibold text-[#047857] hover:border-[#86EFAC] disabled:opacity-40">{active ? '续期/更换' : '发放'}</button></td>
              </tr>
            );
          }}
          renderMobileItem={(user) => {
            const active = isActive(user);
            const remaining = Number(user.subscription?.effectiveQuotaRemaining ?? user.subscription?.quotaRemaining ?? 0);
            return (
              <article key={user.id} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
                <div className="flex items-start justify-between gap-3"><div className="min-w-0"><strong className="block truncate text-sm">{user.email}</strong><small className="font-mono text-[9px] text-zinc-400">{user.id}</small></div><span className={`rounded border px-2 py-0.5 text-[10px] font-semibold ${active ? 'border-emerald-200 bg-emerald-50 text-emerald-700' : 'border-zinc-200 bg-zinc-50 text-zinc-500'}`}>{active ? '生效中' : '未订阅'}</span></div>
                <div className="mt-3 grid grid-cols-2 gap-2 border-y border-[#EDF0EE] py-2 text-xs"><span><small className="block text-[9px] text-zinc-400">套餐</small>{active ? user.subscription?.planName || '订阅套餐' : '按余额计费'}</span><span className="text-right"><small className="block text-[9px] text-zinc-400">剩余额度</small><strong>{active ? `${remaining} 张` : '-'}</strong></span></div>
                <div className="mt-2 flex items-center justify-between"><small className="text-[9px] text-zinc-400">{active ? `到期 ${formatDate(user.subscription?.expiresAt || '', false)}` : '未开通订阅'}</small><button type="button" onClick={() => openGrant(user)} disabled={!activePlans.length} className="rounded px-2 py-1 text-[10px] font-semibold text-[#047857] hover:bg-emerald-50 disabled:opacity-40">{active ? '续期/更换' : '发放'}</button></div>
              </article>
            );
          }}
        />
      )}

      {grantOpen && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4">
          <form onSubmit={grant} className="w-full max-w-lg overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5"><div><h2 className="text-sm font-semibold">发放订阅</h2><p className="mt-0.5 text-[10px] text-zinc-500">选择 API 客户和当前上架套餐。</p></div><button type="button" onClick={() => setGrantOpen(false)} className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button></div>
            <div className="space-y-4 p-5">
              <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">API 客户</span><select required value={userId} onChange={(event) => setUserId(event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs"><option value="">请选择客户</option>{users.map((user) => <option key={user.id} value={user.id}>{user.email}{isActive(user) ? ` · ${user.subscription?.planName || '已订阅'}` : ''}</option>)}</select></label>
              <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">订阅套餐</span><select required value={planId} onChange={(event) => setPlanId(event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs"><option value="">请选择套餐</option>{activePlans.map((plan) => <option key={plan.id} value={plan.id}>{plan.name} · {plan.durationDays} 天 · {plan.quotaImages} 张</option>)}</select></label>
              <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-[10px] leading-5 text-amber-800">提交后立即生效；已有订阅的续期和额度处理由 Go 后端现有事务规则完成。</p>
            </div>
            <div className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><button type="button" onClick={() => setGrantOpen(false)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button><button type="submit" disabled={saving || !activePlans.length} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}确认发放</button></div>
          </form>
        </div>
      )}
    </div>
  );
}
