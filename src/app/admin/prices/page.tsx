'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Calculator, CircleDollarSign, Loader2, Pencil, Plus, RefreshCw, Trash2, X } from 'lucide-react';
import { toast } from 'sonner';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { DataTable } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { StatusBadge } from '@/components/common/StatusBadge';
import { portalApi } from '@/lib/portal-api';
import { formatDate } from '@/lib/common/utils';

type Provider = {
  id: string;
  name: string;
  type?: string;
  status?: string;
};

type Model = {
  id: string;
  providerId: string;
  providerName?: string;
  providerStatus?: string;
  modelName: string;
  displayName: string;
  capability: 'chat_image';
  cost1k: number;
  cost2k: number;
  cost4k: number;
  markupPercent: number;
  priceChangePercent: number;
  price1k: number;
  price2k: number;
  price4k: number;
  appendSizeToPrompt: boolean;
  enabledSizeTiers: string[];
  sortOrder: number;
  status: 'active' | 'disabled';
  createdAt?: string;
  updatedAt?: string;
};

type ModelDraft = Omit<Model, 'id' | 'providerName' | 'providerStatus' | 'createdAt' | 'updatedAt'>;

const emptyDraft: ModelDraft = {
  providerId: '',
  modelName: '',
  displayName: '',
  capability: 'chat_image',
  cost1k: 0,
  cost2k: 0,
  cost4k: 0,
  markupPercent: 0,
  priceChangePercent: 0,
  price1k: 0,
  price2k: 0,
  price4k: 0,
  appendSizeToPrompt: false,
  enabledSizeTiers: ['1k', '2k', '4k'],
  sortOrder: 100,
  status: 'active',
};

function money(value: number) {
  return `¥${Number(value || 0).toFixed(4)}`;
}

function modelInput(model: Model, overrides: Partial<ModelDraft> = {}): ModelDraft {
  return {
    providerId: model.providerId,
    modelName: model.modelName,
    displayName: model.displayName,
    capability: model.capability || 'chat_image',
    cost1k: Number(model.cost1k || 0),
    cost2k: Number(model.cost2k || 0),
    cost4k: Number(model.cost4k || 0),
    markupPercent: Number(model.markupPercent || 0),
    priceChangePercent: Number(model.priceChangePercent || 0),
    price1k: Number(model.price1k || 0),
    price2k: Number(model.price2k || 0),
    price4k: Number(model.price4k || 0),
    appendSizeToPrompt: Boolean(model.appendSizeToPrompt),
    enabledSizeTiers: model.enabledSizeTiers?.length ? model.enabledSizeTiers : ['1k', '2k', '4k'],
    sortOrder: Number(model.sortOrder || 100),
    status: model.status || 'active',
    ...overrides,
  };
}

export default function AdminPricesPage() {
  const [models, setModels] = useState<Model[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [providerFilter, setProviderFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState('all');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<Model | null>(null);
  const [draft, setDraft] = useState<ModelDraft>(emptyDraft);
  const [deleteCandidate, setDeleteCandidate] = useState<Model | null>(null);
  const [actionId, setActionId] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [modelResponse, providerResponse] = await Promise.all([
        portalApi.models(),
        portalApi.providers(),
      ]);
      setModels(modelResponse.data as unknown as Model[]);
      setProviders(providerResponse.data as unknown as Provider[]);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : '模型价格加载失败');
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
    return models.filter((model) => {
      const matchesKeyword = !keyword || `${model.displayName} ${model.modelName} ${model.providerName || ''}`.toLowerCase().includes(keyword);
      const matchesProvider = providerFilter === 'all' || model.providerId === providerFilter;
      const matchesStatus = statusFilter === 'all' || model.status === statusFilter;
      return matchesKeyword && matchesProvider && matchesStatus;
    }).sort((a, b) => Number(a.sortOrder || 100) - Number(b.sortOrder || 100));
  }, [models, providerFilter, search, statusFilter]);

  const summary = useMemo(() => ({
    total: models.length,
    active: models.filter((model) => model.status === 'active').length,
    providers: new Set(models.map((model) => model.providerId)).size,
    avgMarkup: models.length ? models.reduce((sum, model) => sum + Number(model.markupPercent || 0), 0) / models.length : 0,
  }), [models]);

  const updateDraft = <K extends keyof ModelDraft>(key: K, value: ModelDraft[K]) => setDraft((current) => ({ ...current, [key]: value }));

  const openCreate = () => {
    setEditing(null);
    setDraft({ ...emptyDraft, providerId: providers.find((provider) => provider.status === 'active')?.id || providers[0]?.id || '' });
    setEditorOpen(true);
  };

  const openEdit = (model: Model) => {
    setEditing(model);
    setDraft(modelInput(model));
    setEditorOpen(true);
  };

  const calculatePrices = () => {
    const multiplier = 1 + Number(draft.markupPercent || 0) / 100;
    const round = (value: number) => Math.round(value * 10_000) / 10_000;
    setDraft((current) => ({
      ...current,
      price1k: round(Number(current.cost1k || 0) * multiplier),
      price2k: round(Number(current.cost2k || 0) * multiplier),
      price4k: round(Number(current.cost4k || 0) * multiplier),
    }));
  };

  const saveModel = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!draft.providerId || !draft.modelName.trim() || !draft.displayName.trim()) return toast.error('请选择上游并填写模型名和展示名称');
    if (!draft.enabledSizeTiers.length) return toast.error('至少启用一个清晰度');
    setSaving(true);
    try {
      const input: ModelDraft = {
        ...draft,
        modelName: draft.modelName.trim(),
        displayName: draft.displayName.trim(),
        cost1k: Number(draft.cost1k || 0),
        cost2k: Number(draft.cost2k || 0),
        cost4k: Number(draft.cost4k || 0),
        markupPercent: Number(draft.markupPercent || 0),
        priceChangePercent: Number(draft.priceChangePercent || 0),
        price1k: Number(draft.price1k || 0),
        price2k: Number(draft.price2k || 0),
        price4k: Number(draft.price4k || 0),
        sortOrder: Number(draft.sortOrder || 100),
      };
      if (editing) await portalApi.updateModel(editing.id, input);
      else await portalApi.createModel(input);
      toast.success(editing ? '模型与价格已更新' : '模型已创建');
      setEditorOpen(false);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '模型保存失败');
    } finally {
      setSaving(false);
    }
  };

  const toggleStatus = async (model: Model) => {
    setActionId(model.id);
    try {
      const nextStatus: Model['status'] = model.status === 'active' ? 'disabled' : 'active';
      await portalApi.updateModel(model.id, modelInput(model, { status: nextStatus }));
      toast.success(nextStatus === 'active' ? '模型已启用' : '模型已停用');
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '模型状态更新失败');
    } finally {
      setActionId('');
    }
  };

  const deleteModel = async () => {
    if (!deleteCandidate) return;
    try {
      await portalApi.deleteModel(deleteCandidate.id);
      toast.success('模型已删除；如存在历史调用，Go 后端会自动停用该模型');
      setDeleteCandidate(null);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '模型删除失败');
    }
  };

  const toggleTier = (tier: string) => {
    setDraft((current) => ({
      ...current,
      enabledSizeTiers: current.enabledSizeTiers.includes(tier)
        ? current.enabledSizeTiers.filter((item) => item !== tier)
        : [...current.enabledSizeTiers, tier],
    }));
  };

  const rowActions = (model: Model) => (
    <div className="flex items-center justify-end gap-1">
      <button type="button" onClick={() => openEdit(model)} title="编辑模型" className="rounded p-1.5 text-zinc-600 hover:bg-zinc-100"><Pencil className="h-3.5 w-3.5" /></button>
      <button type="button" onClick={() => void toggleStatus(model)} disabled={actionId === model.id} className={`rounded px-2 py-1 text-[10px] font-semibold ${model.status === 'active' ? 'text-amber-700 hover:bg-amber-50' : 'text-emerald-700 hover:bg-emerald-50'} disabled:opacity-40`}>{model.status === 'active' ? '停用' : '启用'}</button>
      <button type="button" onClick={() => setDeleteCandidate(model)} title="删除模型" className="rounded p-1.5 text-red-600 hover:bg-red-50"><Trash2 className="h-3.5 w-3.5" /></button>
    </div>
  );

  return (
    <div className="space-y-5">
      <PageHeader title="模型与价格" description="配置上游模型映射、成本价、API 售价和可用清晰度。">
        <button type="button" onClick={() => void load()} disabled={loading} title="刷新模型" className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-[#DCE4DF] bg-white hover:border-[#12B76A] disabled:opacity-50"><RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} /></button>
        <button type="button" onClick={openCreate} disabled={!providers.length} className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[#047857] px-3 text-xs font-semibold text-white hover:bg-[#036b4f] disabled:opacity-40"><Plus className="h-4 w-4" />新增模型</button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          ['模型总数', summary.total, '全部价格配置'],
          ['启用模型', summary.active, '对外可调用'],
          ['已接入上游', summary.providers, '模型来源'],
          ['平均加价率', `${summary.avgMarkup.toFixed(1)}%`, '成本到售价'],
        ].map(([label, value, note]) => <div key={String(label)} className="rounded-md border border-[#DCE4DF] bg-white p-3.5"><span className="text-[10px] font-semibold text-zinc-500">{label}</span><strong className="mt-1.5 block text-xl">{value}</strong><small className="mt-1 block text-[10px] text-zinc-400">{note}</small></div>)}
      </div>

      {error && <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void load()} className="font-semibold underline">重试</button></div>}

      {loading ? (
        <div className="grid min-h-[300px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <DataTable
          headers={[
            { key: 'model', label: '模型映射' },
            { key: 'provider', label: '上游' },
            { key: '1k', label: '1K 成本 / 售价' },
            { key: '2k', label: '2K 成本 / 售价' },
            { key: '4k', label: '4K 成本 / 售价' },
            { key: 'tiers', label: '清晰度' },
            { key: 'status', label: '状态' },
            { key: 'actions', label: '操作', className: 'text-right' },
          ]}
          data={filtered}
          searchPlaceholder="搜索模型、展示名或上游"
          searchValue={search}
          onSearchChange={setSearch}
          filterControls={(
            <>
              <select value={providerFilter} onChange={(event) => setProviderFilter(event.target.value)} className="h-8 max-w-[180px] rounded-md border border-[#DCE4DF] bg-white px-2 text-xs"><option value="all">全部上游</option>{providers.map((provider) => <option key={provider.id} value={provider.id}>{provider.name}</option>)}</select>
              <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-2 text-xs"><option value="all">全部状态</option><option value="active">已启用</option><option value="disabled">已停用</option></select>
              <span className="text-[10px] text-zinc-400">{filtered.length} 条</span>
            </>
          )}
          emptyState={<EmptyState title="暂无模型价格" description="先添加上游接口，再创建可对外调用的模型。" icon={CircleDollarSign} />}
          renderRow={(model) => (
            <tr key={model.id} className="hover:bg-[#FAFBFA]">
              <td className="px-4 py-3"><strong className="block max-w-[180px] truncate font-medium">{model.displayName}</strong><small className="mt-0.5 block max-w-[180px] truncate font-mono text-[9px] text-zinc-400">{model.modelName}</small></td>
              <td className="max-w-[140px] truncate px-4 py-3">{model.providerName || providers.find((provider) => provider.id === model.providerId)?.name || model.providerId}</td>
              {(['1k', '2k', '4k'] as const).map((tier) => <td key={tier} className="px-4 py-3 font-mono"><small className="block text-[9px] text-zinc-400">{money(model[`cost${tier}`])}</small><strong className="block text-[11px] text-[#047857]">{money(model[`price${tier}`])}</strong></td>)}
              <td className="px-4 py-3"><span className="text-[10px] font-semibold text-zinc-600">{(model.enabledSizeTiers || []).map((tier) => tier.toUpperCase()).join(' / ') || '-'}</span></td>
              <td className="px-4 py-3"><StatusBadge status={model.status === 'active' ? 'active' : 'disabled'} /></td>
              <td className="px-4 py-3">{rowActions(model)}</td>
            </tr>
          )}
          renderMobileItem={(model) => (
            <article key={model.id} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
              <div className="flex items-start justify-between gap-3"><div className="min-w-0"><strong className="block truncate text-sm">{model.displayName}</strong><small className="block truncate font-mono text-[9px] text-zinc-400">{model.modelName}</small></div><StatusBadge status={model.status === 'active' ? 'active' : 'disabled'} /></div>
              <div className="mt-3 grid grid-cols-3 divide-x divide-[#EDF0EE] border-y border-[#EDF0EE] py-2 text-center">{(['1k', '2k', '4k'] as const).map((tier) => <div key={tier}><small className="block text-[9px] text-zinc-400">{tier.toUpperCase()} 售价</small><strong className="font-mono text-[11px] text-[#047857]">{money(model[`price${tier}`])}</strong></div>)}</div>
              <div className="mt-2 flex items-center justify-between"><small className="max-w-[180px] truncate text-[9px] text-zinc-400">{model.providerName || model.providerId} · {formatDate(model.updatedAt || '')}</small>{rowActions(model)}</div>
            </article>
          )}
        />
      )}

      {editorOpen && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/40 p-4 sm:grid sm:place-items-center">
          <form onSubmit={saveModel} className="mx-auto w-full max-w-3xl overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5"><div><h2 className="text-sm font-semibold">{editing ? '编辑模型与价格' : '新增模型'}</h2><p className="mt-0.5 text-[10px] text-zinc-500">售价单位与 Go 后端现有模型价格字段保持一致。</p></div><button type="button" onClick={() => setEditorOpen(false)} className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button></div>
            <div className="space-y-5 p-5">
              <section className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">上游接口</span><select required value={draft.providerId} onChange={(event) => updateDraft('providerId', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs"><option value="">请选择上游</option>{providers.map((provider) => <option key={provider.id} value={provider.id}>{provider.name} {provider.status === 'disabled' ? '（停用）' : ''}</option>)}</select></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">状态</span><select value={draft.status} onChange={(event) => updateDraft('status', event.target.value as Model['status'])} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs"><option value="active">启用</option><option value="disabled">停用</option></select></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">上游模型名</span><input required value={draft.modelName} onChange={(event) => updateDraft('modelName', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs outline-none focus:border-[#12B76A]" /></label>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">对外展示名</span><input required value={draft.displayName} onChange={(event) => updateDraft('displayName', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
              </section>

              <section className="border-t border-[#DCE4DF] pt-4">
                <div className="mb-3 flex flex-wrap items-end justify-between gap-3"><div><h3 className="text-xs font-semibold">成本与售价</h3><p className="mt-0.5 text-[10px] text-zinc-400">保留四位小数，按单张图片计费。</p></div><div className="flex items-end gap-2"><label><span className="mb-1 block text-[9px] text-zinc-400">加价率 %</span><input type="number" step="0.01" value={draft.markupPercent} onChange={(event) => updateDraft('markupPercent', Number(event.target.value))} className="w-24 rounded-md border border-[#DCE4DF] px-2 py-1.5 font-mono text-xs" /></label><button type="button" onClick={calculatePrices} className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[#86EFAC] bg-[#F0FDF4] px-3 text-[10px] font-semibold text-[#047857]"><Calculator className="h-3.5 w-3.5" />计算售价</button></div></div>
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">{(['1k', '2k', '4k'] as const).map((tier) => <div key={tier} className="grid grid-cols-2 gap-2 rounded-md border border-[#E5E9E6] bg-[#FAFBFA] p-3"><strong className="col-span-2 text-[10px]">{tier.toUpperCase()} 清晰度</strong><label><span className="mb-1 block text-[9px] text-zinc-400">成本</span><input min={0} type="number" step="0.0001" value={draft[`cost${tier}`]} onChange={(event) => updateDraft(`cost${tier}`, Number(event.target.value))} className="w-full rounded border border-[#DCE4DF] px-2 py-1.5 font-mono text-xs" /></label><label><span className="mb-1 block text-[9px] text-zinc-400">售价</span><input min={0} type="number" step="0.0001" value={draft[`price${tier}`]} onChange={(event) => updateDraft(`price${tier}`, Number(event.target.value))} className="w-full rounded border border-[#86EFAC] px-2 py-1.5 font-mono text-xs text-[#047857]" /></label></div>)}</div>
              </section>

              <section className="grid grid-cols-1 gap-4 border-t border-[#DCE4DF] pt-4 sm:grid-cols-3">
                <div><span className="mb-2 block text-[10px] font-semibold text-zinc-500">对外开放清晰度</span><div className="flex gap-2">{['1k', '2k', '4k'].map((tier) => <label key={tier} className={`flex cursor-pointer items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-[10px] font-semibold ${draft.enabledSizeTiers.includes(tier) ? 'border-[#86EFAC] bg-[#F0FDF4] text-[#047857]' : 'border-[#DCE4DF] text-zinc-500'}`}><input type="checkbox" checked={draft.enabledSizeTiers.includes(tier)} onChange={() => toggleTier(tier)} className="h-3 w-3 accent-[#047857]" />{tier.toUpperCase()}</label>)}</div></div>
                <label><span className="mb-1 block text-[10px] font-semibold text-zinc-500">排序值</span><input min={0} type="number" value={draft.sortOrder} onChange={(event) => updateDraft('sortOrder', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                <label className="flex items-center justify-between gap-3 rounded-md border border-[#DCE4DF] px-3 py-2 text-xs"><span><strong className="block text-[10px]">附加尺寸提示</strong><small className="text-[9px] text-zinc-400">将清晰度传给上游</small></span><input type="checkbox" checked={draft.appendSizeToPrompt} onChange={(event) => updateDraft('appendSizeToPrompt', event.target.checked)} className="h-4 w-4 accent-[#047857]" /></label>
              </section>
            </div>
            <div className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><button type="button" onClick={() => setEditorOpen(false)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button><button type="submit" disabled={saving} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}保存模型</button></div>
          </form>
        </div>
      )}

      <ConfirmDialog isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)} onConfirm={() => void deleteModel()} title="删除模型" description={`确定删除 ${deleteCandidate?.displayName || '该模型'} 吗？存在历史调用时，Go 后端会保留记录并自动停用模型。`} confirmText="删除" type="danger" />
    </div>
  );
}
