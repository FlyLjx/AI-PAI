'use client';

import { useCallback, useEffect, useState } from 'react';
import { CreditCard, Gauge, Gift, Loader2, PackageCheck, RefreshCw, X } from 'lucide-react';
import { toast } from 'sonner';
import { DataTable } from '@/components/common/DataTable';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { EmptyState } from '@/components/common/EmptyState';
import { AppSelect } from '@/components/common/AppSelect';
import { PageHeader } from '@/components/common/PageHeader';
import { type Plan, type PortalUser, portalApi } from '@/lib/admin-api';

const pageSize = 15;
type GrantMode = 'plan' | 'custom';

function isActive(user: PortalUser) {
  return user.subscription?.status === 'active' && user.subscription?.isPaid === true;
}

export default function AdminSubscriptionsPage() {
  const [users, setUsers] = useState<PortalUser[]>([]);
  const [pickerUsers, setPickerUsers] = useState<PortalUser[]>([]);
  const [total, setTotal] = useState(0);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [cancelingId, setCancelingId] = useState('');
  const [cancelCandidate, setCancelCandidate] = useState<PortalUser | null>(null);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const [grantOpen, setGrantOpen] = useState(false);
  const [userId, setUserId] = useState('');
  const [planId, setPlanId] = useState('');
  const [grantMode, setGrantMode] = useState<GrantMode>('plan');
  const [customName, setCustomName] = useState('自定义订阅');
  const [customDurationDays, setCustomDurationDays] = useState(30);
  const [customQuotaImages, setCustomQuotaImages] = useState(100);

  const load = useCallback(async (targetPage = page) => {
    setLoading(true);
    setError('');
    try {
      const [userResponse, pickerResponse, planResponse] = await Promise.all([
        portalApi.adminUsers({
          page: targetPage,
          pageSize,
          keyword: search,
          billing: 'subscription',
        }),
        portalApi.userOptions({ limit: 1000 }),
        portalApi.adminPlans(),
      ]);
      // Subscription management only lists active paid subscriptions.
      const loadedUsers = (userResponse.data || []).filter(isActive);
      setUsers(loadedUsers);
      setPickerUsers((pickerResponse.data || []).filter((user) => user.role === 'user'));
      setTotal(loadedUsers.length);
      setPlans(planResponse.data);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : '订阅数据加载失败');
    } finally {
      setLoading(false);
    }
  }, [page, search]);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(page), 0);
    return () => window.clearTimeout(timer);
  }, [load, page]);

  const activePlans = plans.filter((plan) => plan.status === 'active');

  const openGrant = (user?: PortalUser) => {
    setUserId(user?.id || pickerUsers[0]?.id || '');
    const currentPlanId = user?.subscription?.source === 'admin_custom' ? '' : user?.subscription?.planId || '';
    setPlanId(activePlans.some((plan) => plan.id === currentPlanId) ? currentPlanId : activePlans[0]?.id || '');
    setGrantMode(user?.subscription?.source === 'admin_custom' ? 'custom' : 'plan');
    setCustomName(user?.subscription?.source === 'admin_custom' ? user.subscription.planName || '自定义订阅' : '自定义订阅');
    setCustomDurationDays(30);
    setCustomQuotaImages(user?.subscription?.source === 'admin_custom' ? Number(user.subscription.quotaLimit || 100) : 100);
    setGrantOpen(true);
  };

  const grant = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!userId) return toast.error('请选择用户');
    if (grantMode === 'plan' && !planId) return toast.error('请选择订阅套餐');
    if (grantMode === 'custom' && (!customName.trim() || customDurationDays < 1 || customDurationDays > 3650 || customQuotaImages < 1)) {
      return toast.error('请填写有效的自定义订阅参数');
    }
    const user = pickerUsers.find((item) => item.id === userId) || users.find((item) => item.id === userId);
    setSaving(true);
    try {
      await portalApi.grantSubscription(userId, grantMode === 'custom'
        ? { grantType: 'custom', name: customName.trim(), durationDays: customDurationDays, quotaImages: customQuotaImages }
        : { grantType: 'plan', planId });
      toast.success(`已为 ${user?.email || userId} 发放${grantMode === 'custom' ? '自定义' : '套餐'}订阅`);
      setGrantOpen(false);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '订阅发放失败');
    } finally {
      setSaving(false);
    }
  };

  const cancelSubscription = async () => {
    if (!cancelCandidate) return;
    const user = cancelCandidate;
    setCancelingId(user.id);
    try {
      await portalApi.cancelSubscription(user.id);
      toast.success(`已取消 ${user.email} 的当前订阅`);
      setCancelCandidate(null);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '取消订阅失败');
    } finally {
      setCancelingId('');
    }
  };

  return (
    <div className="space-y-5">
      <PageHeader title="订阅管理" description="查看客户订阅状态，并按现有 Go 规则发放、续期或更换套餐。">
        <button type="button" onClick={() => void load()} disabled={loading} title="刷新订阅" className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-[#DCE4DF] bg-white hover:border-[#12B76A] disabled:opacity-50"><RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} /></button>
        <button type="button" onClick={() => openGrant()} disabled={!users.length} className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[#047857] px-3 text-xs font-semibold text-white hover:bg-[#036b4f] disabled:opacity-40"><Gift className="h-4 w-4" />发放订阅</button>
      </PageHeader>

      {error && <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void load()} className="font-semibold underline">重试</button></div>}

      {loading ? (
        <div className="grid min-h-[300px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <DataTable
          headers={[
            { key: 'user', label: 'API 客户', sortValue: (item) => item.email },
            { key: 'quota', label: '订阅数量', sortValue: (item) => Number(item.subscription?.effectiveQuotaRemaining ?? item.subscription?.quotaRemaining ?? 0) },
            { key: 'action', label: '操作', sortable: false, className: 'text-right' },
          ]}
          data={users}
          searchPlaceholder="搜索订阅用户邮箱或用户 ID"
          searchValue={search}
          onSearchChange={(value) => { setSearch(value); setPage(1); }}
          filterControls={<span className="text-[11px] text-zinc-400">共 {total.toLocaleString('zh-CN')} 位订阅用户</span>}
          currentPage={page}
          totalPages={Math.max(1, Math.ceil(total / pageSize))}
          totalItems={total}
          onPageChange={setPage}
          emptyState={<EmptyState title="暂无订阅记录" description="可从全部客户中选择用户并发放套餐或自定义额度。" icon={CreditCard} action={<button type="button" onClick={() => openGrant()} disabled={!users.length} className="h-8 rounded-md bg-[#047857] px-3 text-xs font-semibold text-white disabled:opacity-40">发放订阅</button>} />}
          renderRow={(user) => {
            const limit = Number(user.subscription?.quotaLimit ?? user.subscription?.quotaImages ?? 0);
            const remaining = Number(user.subscription?.effectiveQuotaRemaining ?? user.subscription?.quotaRemaining ?? 0);
            const used = Math.max(0, limit - remaining);
            const percent = limit > 0 ? Math.min(100, Math.round((used / limit) * 100)) : 0;
            const progressTone = percent >= 90 ? 'bg-red-500' : percent >= 70 ? 'bg-amber-500' : 'bg-[#12B76A]';
            return (
              <tr key={user.id} className="hover:bg-[#FAFBFA]">
                <td className="px-4 py-3"><strong className="block max-w-[210px] truncate font-medium">{user.email}</strong><small className="font-mono text-[10px] text-zinc-400">{user.id}</small></td>
                <td className="px-4 py-3"><div className="w-52"><div className="mb-1 flex justify-between font-mono text-[10px] text-zinc-500"><span>剩余 {remaining.toLocaleString('zh-CN')} / {limit.toLocaleString('zh-CN')}</span><span>已用 {percent}%</span></div><div className="h-1.5 overflow-hidden rounded-full bg-zinc-100"><span className={`block h-full transition-[width] duration-300 ${progressTone}`} style={{ width: `${percent}%` }} /></div></div></td>
                <td className="px-4 py-3 text-right"><div className="flex justify-end gap-1.5"><button type="button" onClick={() => setCancelCandidate(user)} disabled={cancelingId === user.id} className="rounded-md border border-red-200 bg-white px-2.5 py-1 text-[11px] font-semibold text-red-700 hover:bg-red-50 disabled:opacity-50">取消订阅</button><button type="button" onClick={() => openGrant(user)} className="rounded-md border border-[#DCE4DF] bg-white px-2.5 py-1 text-[11px] font-semibold text-[#047857] hover:border-[#86EFAC]">续期/更换</button></div></td>
              </tr>
            );
          }}
          renderMobileItem={(user) => {
            const remaining = Number(user.subscription?.effectiveQuotaRemaining ?? user.subscription?.quotaRemaining ?? 0);
            const limit = Number(user.subscription?.quotaLimit ?? user.subscription?.quotaImages ?? 0);
            const percent = limit > 0 ? Math.min(100, Math.round(((limit - remaining) / limit) * 100)) : 0;
            return (
              <article key={user.id} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
                <div className="min-w-0"><strong className="block truncate text-sm">{user.email}</strong><small className="font-mono text-[10px] text-zinc-400">{user.id}</small></div>
                <div className="mt-3 border-y border-[#EDF0EE] py-2 text-xs"><div className="mb-1 flex justify-between font-mono text-[10px] text-zinc-500"><span>剩余 {remaining} / {limit}</span><span>已用 {percent}%</span></div><div className="h-1.5 overflow-hidden rounded-full bg-zinc-100"><span className="block h-full bg-[#12B76A]" style={{ width: `${percent}%` }} /></div></div>
                <div className="mt-2 flex justify-end gap-1"><button type="button" onClick={() => setCancelCandidate(user)} disabled={cancelingId === user.id} className="rounded px-2 py-1 text-[11px] font-semibold text-red-700 hover:bg-red-50 disabled:opacity-50">取消</button><button type="button" onClick={() => openGrant(user)} className="rounded px-2 py-1 text-[11px] font-semibold text-[#047857] hover:bg-emerald-50">续期/更换</button></div>
              </article>
            );
          }}
        />
      )}

      {grantOpen && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4">
          <form onSubmit={grant} className="w-full max-w-lg overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5"><div><h2 className="text-sm font-semibold">发放订阅</h2><p className="mt-0.5 text-[11px] text-zinc-500">套餐发放或直接配置自定义额度。</p></div><button type="button" onClick={() => setGrantOpen(false)} className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button></div>
            <div className="space-y-5 p-5">
               <label className="block"><span className="mb-1 block text-[11px] font-semibold text-zinc-500">API 客户</span><AppSelect required value={userId} onValueChange={setUserId} placeholder="请选择客户" ariaLabel="API 客户" options={[{ value: '', label: '请选择客户' }, ...pickerUsers.map((user) => ({ value: user.id, label: `${user.email}${isActive(user) ? ` · ${user.subscription?.planName || '已订阅'}` : ''}` }))]} /></label>
              <div className="grid grid-cols-2 gap-1 rounded-md border border-[#DCE4DF] bg-[#F6F8F6] p-1">
                <button type="button" onClick={() => setGrantMode('plan')} aria-pressed={grantMode === 'plan'} className={`inline-flex h-9 items-center justify-center gap-1.5 rounded border text-[11px] font-semibold ${grantMode === 'plan' ? 'border-[#86EFAC] bg-white text-[#047857]' : 'border-transparent text-zinc-500'}`}><PackageCheck className="h-3.5 w-3.5" />套餐发放</button>
                <button type="button" onClick={() => setGrantMode('custom')} aria-pressed={grantMode === 'custom'} className={`inline-flex h-9 items-center justify-center gap-1.5 rounded border text-[11px] font-semibold ${grantMode === 'custom' ? 'border-[#86EFAC] bg-white text-[#047857]' : 'border-transparent text-zinc-500'}`}><Gauge className="h-3.5 w-3.5" />自定义额度</button>
              </div>
              {grantMode === 'plan' ? (
                <label className="block"><span className="mb-1 block text-[11px] font-semibold text-zinc-500">订阅套餐</span><AppSelect required value={planId} onValueChange={setPlanId} placeholder="请选择套餐" ariaLabel="订阅套餐" options={[{ value: '', label: '请选择套餐' }, ...activePlans.map((plan) => ({ value: plan.id, label: `${plan.name} · ${plan.durationDays} 天 · ${plan.quotaImages} 张` }))]} /></label>
              ) : (
                <div className="grid grid-cols-2 gap-3">
                  <label className="col-span-2"><span className="mb-1 block text-[11px] font-semibold text-zinc-500">权益名称</span><input required maxLength={80} value={customName} onChange={(event) => setCustomName(event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
                  <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">有效天数</span><input required min={1} max={3650} type="number" value={customDurationDays} onChange={(event) => setCustomDurationDays(Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs outline-none focus:border-[#12B76A]" /></label>
                  <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">图片额度</span><input required min={1} max={100000000} type="number" value={customQuotaImages} onChange={(event) => setCustomQuotaImages(Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs outline-none focus:border-[#12B76A]" /></label>
                </div>
              )}
            </div>
            <div className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><button type="button" onClick={() => setGrantOpen(false)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button><button type="submit" disabled={saving || !userId || (grantMode === 'plan' && !planId)} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}确认发放</button></div>
          </form>
        </div>
      )}

      <ConfirmDialog
        isOpen={Boolean(cancelCandidate)}
        onClose={() => setCancelCandidate(null)}
        onConfirm={() => void cancelSubscription()}
        title="取消当前订阅"
        description={cancelCandidate ? `确定立即取消 ${cancelCandidate.email} 的当前订阅吗？取消后会立即停止订阅额度使用，已产生的调用记录和历史订单会保留。` : ''}
        confirmText="立即取消"
        type="danger"
      />
    </div>
  );
}
