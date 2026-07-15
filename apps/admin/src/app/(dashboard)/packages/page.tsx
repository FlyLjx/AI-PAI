'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Loader2, Package, Pencil, Plus, RefreshCw, Trash2, X } from 'lucide-react';
import { toast } from 'sonner';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { DataTable } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { StatusBadge } from '@/components/common/StatusBadge';
import { type Plan, portalApi } from '@/lib/admin-api';
import { formatCNY } from '@/lib/common/utils';

type Provider = { id: string; name: string; status?: string };
type Model = { id: string; displayName?: string; modelName?: string; providerId?: string; status?: string };

type AdminPlan = Plan & {
  allowedProviderIds?: string[];
  allowedModelIds?: string[];
  createdAt?: string;
  updatedAt?: string;
};

type PlanDraft = {
  name: string;
  description: string;
  amount: number;
  durationDays: number;
  quotaImages: number;
  discountPercent: number;
  allowedProviderIds: string[];
  allowedModelIds: string[];
  badge: string;
  sortOrder: number;
  status: 'active' | 'disabled';
};

const emptyDraft: PlanDraft = {
  name: '',
  description: '',
  amount: 0,
  durationDays: 30,
  quotaImages: 100,
  discountPercent: 0,
  allowedProviderIds: [],
  allowedModelIds: [],
  badge: '',
  sortOrder: 100,
  status: 'active',
};

function planInput(plan: AdminPlan, overrides: Partial<PlanDraft> = {}): PlanDraft {
  return {
    name: plan.name,
    description: plan.description || '',
    amount: Number(plan.amount || 0),
    durationDays: Number(plan.durationDays || 30),
    quotaImages: Number(plan.quotaImages || 0),
    discountPercent: Number(plan.discountPercent || 0),
    allowedProviderIds: plan.allowedProviderIds || [],
    allowedModelIds: plan.allowedModelIds || [],
    badge: plan.badge || '',
    sortOrder: Number(plan.sortOrder || 100),
    status: plan.status === 'active' ? 'active' : 'disabled',
    ...overrides,
  };
}

export default function AdminPackagesPage() {
  const [plans, setPlans] = useState<AdminPlan[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<AdminPlan | null>(null);
  const [draft, setDraft] = useState<PlanDraft>(emptyDraft);
  const [deleteCandidate, setDeleteCandidate] = useState<AdminPlan | null>(null);
  const [actionId, setActionId] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [planResponse, providerResponse, modelResponse] = await Promise.all([
        portalApi.adminPlans(),
        portalApi.providers(),
        portalApi.models(),
      ]);
      setPlans(planResponse.data as AdminPlan[]);
      setProviders(providerResponse.data as unknown as Provider[]);
      setModels(modelResponse.data as unknown as Model[]);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : '订阅套餐加载失败');
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
    return plans.filter((plan) => {
      const matchesKeyword = !keyword || `${plan.name} ${plan.description || ''} ${plan.badge || ''}`.toLowerCase().includes(keyword);
      return matchesKeyword && (statusFilter === 'all' || plan.status === statusFilter);
    }).sort((a, b) => Number(a.sortOrder || 100) - Number(b.sortOrder || 100));
  }, [plans, search, statusFilter]);

  const summary = useMemo(() => ({
    total: plans.length,
    active: plans.filter((plan) => plan.status === 'active').length,
    quota: plans.reduce((sum, plan) => sum + Number(plan.quotaImages || 0), 0),
    avgPrice: plans.length ? plans.reduce((sum, plan) => sum + Number(plan.amount || 0), 0) / plans.length : 0,
  }), [plans]);

  const updateDraft = <K extends keyof PlanDraft>(key: K, value: PlanDraft[K]) => setDraft((current) => ({ ...current, [key]: value }));

  const openCreate = () => {
    setEditing(null);
    setDraft(emptyDraft);
    setEditorOpen(true);
  };

  const openEdit = (plan: AdminPlan) => {
    setEditing(plan);
    setDraft(planInput(plan));
    setEditorOpen(true);
  };

  const savePlan = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!draft.name.trim()) return toast.error('请填写套餐名称');
    if (draft.durationDays < 1) return toast.error('有效期至少 1 天');
    if (draft.quotaImages < 1) return toast.error('订阅额度至少 1 张');
    setSaving(true);
    try {
      const input = {
        ...draft,
        name: draft.name.trim(),
        description: draft.description.trim() || undefined,
        badge: draft.badge.trim() || undefined,
        amount: Number(draft.amount || 0),
        durationDays: Number(draft.durationDays || 0),
        quotaImages: Number(draft.quotaImages || 0),
        discountPercent: Number(draft.discountPercent || 0),
        sortOrder: Number(draft.sortOrder || 100),
      };
      if (editing) await portalApi.updatePlan(editing.id, input);
      else await portalApi.createPlan(input);
      toast.success(editing ? '订阅套餐已更新' : '订阅套餐已创建');
      setEditorOpen(false);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '套餐保存失败');
    } finally {
      setSaving(false);
    }
  };

  const toggleStatus = async (plan: AdminPlan) => {
    setActionId(plan.id);
    try {
      const nextStatus: PlanDraft['status'] = plan.status === 'active' ? 'disabled' : 'active';
      await portalApi.updatePlan(plan.id, planInput(plan, { status: nextStatus }));
      toast.success(nextStatus === 'active' ? '套餐已上架' : '套餐已下架');
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '套餐状态更新失败');
    } finally {
      setActionId('');
    }
  };

  const deletePlan = async () => {
    if (!deleteCandidate) return;
    try {
      await portalApi.deletePlan(deleteCandidate.id);
      toast.success('订阅套餐已删除');
      setDeleteCandidate(null);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '套餐删除失败');
    }
  };

  const toggleArrayValue = (key: 'allowedProviderIds' | 'allowedModelIds', id: string) => {
    setDraft((current) => ({
      ...current,
      [key]: current[key].includes(id) ? current[key].filter((value) => value !== id) : [...current[key], id],
    }));
  };

  const rowActions = (plan: AdminPlan) => (
    <div className="flex items-center justify-end gap-1">
      <button type="button" onClick={() => openEdit(plan)} title="编辑套餐" className="rounded p-1.5 text-zinc-600 hover:bg-zinc-100"><Pencil className="h-3.5 w-3.5" /></button>
      <button type="button" onClick={() => void toggleStatus(plan)} disabled={actionId === plan.id} className={`rounded px-2 py-1 text-[10px] font-semibold ${plan.status === 'active' ? 'text-amber-700 hover:bg-amber-50' : 'text-emerald-700 hover:bg-emerald-50'} disabled:opacity-40`}>{plan.status === 'active' ? '下架' : '上架'}</button>
      <button type="button" onClick={() => setDeleteCandidate(plan)} title="删除套餐" className="rounded p-1.5 text-red-600 hover:bg-red-50"><Trash2 className="h-3.5 w-3.5" /></button>
    </div>
  );

  return (
    <div className="space-y-5">
      <PageHeader title="订阅套餐" description="配置订阅售价、有效期、图片额度和可用模型范围。">
        <button type="button" onClick={() => void load()} disabled={loading} title="刷新套餐" className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-[#DCE4DF] bg-white hover:border-[#12B76A] disabled:opacity-50"><RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} /></button>
        <button type="button" onClick={openCreate} className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[#047857] px-3 text-xs font-semibold text-white hover:bg-[#036b4f]"><Plus className="h-4 w-4" />新增套餐</button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          ['套餐总数', summary.total, '全部订阅商品'],
          ['已上架', summary.active, '用户可购买'],
          ['额度合计', summary.quota.toLocaleString('zh-CN'), '各套餐额度之和'],
          ['平均售价', formatCNY(summary.avgPrice), '当前套餐均价'],
        ].map(([label, value, note]) => <div key={String(label)} className="rounded-md border border-[#DCE4DF] bg-white p-3.5"><span className="text-[10px] font-semibold text-zinc-500">{label}</span><strong className="mt-1.5 block text-xl">{value}</strong><small className="mt-1 block text-[10px] text-zinc-400">{note}</small></div>)}
      </div>

      {error && <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void load()} className="font-semibold underline">重试</button></div>}

      {loading ? (
        <div className="grid min-h-[300px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <DataTable
          headers={[
            { key: 'name', label: '套餐' },
            { key: 'price', label: '售价' },
            { key: 'duration', label: '有效期' },
            { key: 'quota', label: '图片额度' },
            { key: 'discount', label: '折扣' },
            { key: 'scope', label: '可用范围' },
            { key: 'status', label: '状态' },
            { key: 'actions', label: '操作', className: 'text-right' },
          ]}
          data={filtered}
          searchPlaceholder="搜索套餐名称、说明或标签"
          searchValue={search}
          onSearchChange={setSearch}
          filterControls={<><select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-2 text-xs"><option value="all">全部状态</option><option value="active">已上架</option><option value="disabled">已下架</option></select><span className="text-[10px] text-zinc-400">{filtered.length} 条</span></>}
          emptyState={<EmptyState title="暂无订阅套餐" description="创建一个套餐后即可向 API 客户发放或销售订阅。" icon={Package} />}
          renderRow={(plan) => (
            <tr key={plan.id} className="hover:bg-[#FAFBFA]">
              <td className="px-4 py-3"><span className="flex items-center gap-2"><strong className="block max-w-[170px] truncate font-medium">{plan.name}</strong>{plan.badge && <span className="rounded border border-amber-200 bg-amber-50 px-1.5 py-0.5 text-[9px] font-semibold text-amber-800">{plan.badge}</span>}</span><small className="mt-0.5 block max-w-[230px] truncate text-[9px] text-zinc-400">{plan.description || '无说明'}</small></td>
              <td className="px-4 py-3 font-mono font-semibold text-[#047857]">{formatCNY(Number(plan.amount || 0))}</td>
              <td className="px-4 py-3">{Number(plan.durationDays || 0)} 天</td>
              <td className="px-4 py-3 font-mono">{Number(plan.quotaImages || 0).toLocaleString('zh-CN')} 张</td>
              <td className="px-4 py-3">{Number(plan.discountPercent || 0)}%</td>
              <td className="px-4 py-3 text-[10px] text-zinc-500">{plan.allowedModelIds?.length ? `${plan.allowedModelIds.length} 个模型` : plan.allowedProviderIds?.length ? `${plan.allowedProviderIds.length} 个上游` : '全部模型'}</td>
              <td className="px-4 py-3"><StatusBadge status={plan.status === 'active' ? 'active' : 'disabled'} /></td>
              <td className="px-4 py-3">{rowActions(plan)}</td>
            </tr>
          )}
          renderMobileItem={(plan) => (
            <article key={plan.id} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
              <div className="flex items-start justify-between gap-3"><div className="min-w-0"><strong className="block truncate text-sm">{plan.name}</strong><small className="line-clamp-1 text-[9px] text-zinc-400">{plan.description || '无说明'}</small></div><StatusBadge status={plan.status === 'active' ? 'active' : 'disabled'} /></div>
              <div className="mt-3 grid grid-cols-3 divide-x divide-[#EDF0EE] border-y border-[#EDF0EE] py-2 text-center"><span><small className="block text-[9px] text-zinc-400">售价</small><strong className="text-[11px] text-[#047857]">{formatCNY(Number(plan.amount || 0))}</strong></span><span><small className="block text-[9px] text-zinc-400">有效期</small><strong className="text-[11px]">{plan.durationDays} 天</strong></span><span><small className="block text-[9px] text-zinc-400">额度</small><strong className="text-[11px]">{plan.quotaImages} 张</strong></span></div>
              <div className="mt-2 flex justify-end">{rowActions(plan)}</div>
            </article>
          )}
        />
      )}

      {editorOpen && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/40 p-4 sm:grid sm:place-items-center">
          <form onSubmit={savePlan} className="mx-auto w-full max-w-3xl overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5"><div><h2 className="text-sm font-semibold">{editing ? '编辑订阅套餐' : '新增订阅套餐'}</h2><p className="mt-0.5 text-[10px] text-zinc-500">套餐用于周期额度计费，与余额按量计费并行。</p></div><button type="button" onClick={() => setEditorOpen(false)} className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button></div>
            <div className="space-y-5 p-5">
              <section className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">套餐名称</span><input required value={draft.name} onChange={(event) => updateDraft('name', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">展示标签</span><input value={draft.badge} onChange={(event) => updateDraft('badge', event.target.value)} placeholder="如：推荐" className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
                <label className="sm:col-span-2"><span className="mb-1 block text-[10px] font-semibold text-zinc-500">套餐说明</span><textarea rows={2} value={draft.description} onChange={(event) => updateDraft('description', event.target.value)} className="w-full resize-none rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
              </section>
              <section className="grid grid-cols-2 gap-3 border-t border-[#DCE4DF] pt-4 sm:grid-cols-5">
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">售价（元）</span><input min={0} step="0.01" type="number" value={draft.amount} onChange={(event) => updateDraft('amount', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-2 py-2 font-mono text-xs" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">有效期（天）</span><input min={1} type="number" value={draft.durationDays} onChange={(event) => updateDraft('durationDays', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-2 py-2 font-mono text-xs" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">图片额度</span><input min={1} type="number" value={draft.quotaImages} onChange={(event) => updateDraft('quotaImages', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-2 py-2 font-mono text-xs" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">折扣 %</span><input min={0} max={100} step="0.01" type="number" value={draft.discountPercent} onChange={(event) => updateDraft('discountPercent', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-2 py-2 font-mono text-xs" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">排序</span><input min={0} type="number" value={draft.sortOrder} onChange={(event) => updateDraft('sortOrder', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-2 py-2 font-mono text-xs" /></label>
              </section>
              <section className="grid grid-cols-1 gap-4 border-t border-[#DCE4DF] pt-4 sm:grid-cols-2">
                <div><div className="mb-2 flex items-center justify-between"><span className="text-[10px] font-semibold text-zinc-500">限定上游</span><button type="button" onClick={() => updateDraft('allowedProviderIds', [])} className="text-[9px] text-[#047857]">清空即全部</button></div><div className="max-h-36 space-y-1 overflow-y-auto rounded-md border border-[#DCE4DF] p-2">{providers.map((provider) => <label key={provider.id} className="flex items-center gap-2 rounded px-2 py-1.5 text-[10px] hover:bg-[#F6F8F6]"><input type="checkbox" checked={draft.allowedProviderIds.includes(provider.id)} onChange={() => toggleArrayValue('allowedProviderIds', provider.id)} className="accent-[#047857]" /><span className="truncate">{provider.name}</span></label>)}{!providers.length && <p className="py-4 text-center text-[10px] text-zinc-400">暂无上游</p>}</div></div>
                <div><div className="mb-2 flex items-center justify-between"><span className="text-[10px] font-semibold text-zinc-500">限定模型</span><button type="button" onClick={() => updateDraft('allowedModelIds', [])} className="text-[9px] text-[#047857]">清空即全部</button></div><div className="max-h-36 space-y-1 overflow-y-auto rounded-md border border-[#DCE4DF] p-2">{models.map((model) => <label key={model.id} className="flex items-center gap-2 rounded px-2 py-1.5 text-[10px] hover:bg-[#F6F8F6]"><input type="checkbox" checked={draft.allowedModelIds.includes(model.id)} onChange={() => toggleArrayValue('allowedModelIds', model.id)} className="accent-[#047857]" /><span className="truncate">{model.displayName || model.modelName || model.id}</span></label>)}{!models.length && <p className="py-4 text-center text-[10px] text-zinc-400">暂无模型</p>}</div></div>
              </section>
              <label className="flex items-center justify-between gap-4 border-t border-[#DCE4DF] pt-4 text-xs"><span><strong className="block text-[10px]">套餐状态</strong><small className="text-[9px] text-zinc-400">下架后不再出现在用户购买列表</small></span><select value={draft.status} onChange={(event) => updateDraft('status', event.target.value as PlanDraft['status'])} className="rounded-md border border-[#DCE4DF] px-3 py-2 text-xs"><option value="active">上架</option><option value="disabled">下架</option></select></label>
            </div>
            <div className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><button type="button" onClick={() => setEditorOpen(false)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button><button type="submit" disabled={saving} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}保存套餐</button></div>
          </form>
        </div>
      )}

      <ConfirmDialog isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)} onConfirm={() => void deletePlan()} title="删除订阅套餐" description={`确定删除 ${deleteCandidate?.name || '该套餐'} 吗？已有订阅记录将按数据库现有约束保留。`} confirmText="删除" type="danger" />
    </div>
  );
}
