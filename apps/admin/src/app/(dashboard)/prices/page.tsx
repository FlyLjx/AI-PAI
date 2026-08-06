'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Calculator, CircleDollarSign, EyeOff, Loader2, Pencil, Plus, RefreshCw, Trash2, UserRound, X } from 'lucide-react';
import { toast } from 'sonner';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { DataTable, SortableHeader, sortItems, type SortState, type TableHeader } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { AppSelect } from '@/components/common/AppSelect';
import { PageHeader } from '@/components/common/PageHeader';
import { StatusBadge } from '@/components/common/StatusBadge';
import { portalApi, type PortalUser, type ProviderModel, type UserModelPriceOverride } from '@/lib/admin-api';
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

const overrideHeaders: TableHeader<UserModelPriceOverride>[] = [
  { key: 'user', label: '用户', sortValue: (item) => item.userEmail || item.userId },
  { key: 'model', label: '模型', sortValue: (item) => item.modelDisplayName || item.modelName },
  { key: 'price', label: '专属单价', sortValue: (item) => item.unitPrice },
  { key: 'updated', label: '更新时间', sortValue: (item) => Date.parse(item.updatedAt || item.createdAt) || 0 },
  { key: 'actions', label: '操作', sortable: false, className: 'text-right' },
];

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

const PAGE_SIZE = 15;
const MIN_OVERRIDE_PRICE = 0.001;
const MAX_OVERRIDE_PRICE = 99999999.9999;

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
  const [page, setPage] = useState(1);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<Model | null>(null);
  const [draft, setDraft] = useState<ModelDraft>(emptyDraft);
  const [deleteCandidate, setDeleteCandidate] = useState<Model | null>(null);
  const [actionId, setActionId] = useState('');
  const [providerModels, setProviderModels] = useState<ProviderModel[]>([]);
  const [providerModelsLoading, setProviderModelsLoading] = useState(false);
  const [providerModelsError, setProviderModelsError] = useState('');
  const providerModelsRequest = useRef(0);
  const [users, setUsers] = useState<PortalUser[]>([]);
  const [priceOverrides, setPriceOverrides] = useState<UserModelPriceOverride[]>([]);
  const [overrideLoading, setOverrideLoading] = useState(true);
  const [overrideSaving, setOverrideSaving] = useState(false);
  const [overrideDeletingId, setOverrideDeletingId] = useState('');
  const [overrideUserId, setOverrideUserId] = useState('');
  const [overrideModelId, setOverrideModelId] = useState('');
  const [overrideUnitPrice, setOverrideUnitPrice] = useState('');
  const [overrideDeleteCandidate, setOverrideDeleteCandidate] = useState<UserModelPriceOverride | null>(null);
  const [overrideSort, setOverrideSort] = useState<SortState>({ key: 'updated', direction: 'desc' });

  const load = useCallback(async () => {
    setLoading(true);
    setOverrideLoading(true);
    setError('');
    try {
      const [modelResponse, providerResponse, userResponse, overrideResponse] = await Promise.all([
        portalApi.models(),
        portalApi.providers(),
        portalApi.users(),
        portalApi.userModelPriceOverrides(),
      ]);
      setModels(modelResponse.data as unknown as Model[]);
      setProviders(providerResponse.data as unknown as Provider[]);
      setUsers(userResponse.data);
      setPriceOverrides(overrideResponse.data);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : '模型价格与专属扣费加载失败');
    } finally {
      setLoading(false);
      setOverrideLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  const reloadOverrides = useCallback(async () => {
    setOverrideLoading(true);
    try {
      const response = await portalApi.userModelPriceOverrides();
      setPriceOverrides(response.data);
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '专属扣费规则刷新失败');
    } finally {
      setOverrideLoading(false);
    }
  }, []);

  const overrideUsers = useMemo(
    () => users.filter((user) => user.role !== 'admin').sort((left, right) => left.email.localeCompare(right.email)),
    [users],
  );
  const overrideModels = useMemo(
    () => models.filter((model) => model.capability === 'chat_image'),
    [models],
  );
  const overrideModelOptions = useMemo(
    () => overrideModels.map((model) => ({ value: model.id, label: `${model.displayName} · ${model.modelName}` })),
    [overrideModels],
  );
  const selectedOverrideUserId = overrideUsers.some((user) => user.id === overrideUserId) ? overrideUserId : overrideUsers[0]?.id || '';
  const selectedOverrideModelId = overrideModels.some((model) => model.id === overrideModelId) ? overrideModelId : overrideModels[0]?.id || '';
  const sortedOverrides = useMemo(() => {
    const header = overrideHeaders.find((item) => item.key === overrideSort.key) || overrideHeaders[3];
    return sortItems(priceOverrides, header, overrideSort.direction);
  }, [overrideSort, priceOverrides]);
  const handleOverrideSort = (key: string) => {
    setOverrideSort((current) => ({
      key,
      direction: current.key === key && current.direction === 'asc' ? 'desc' : 'asc',
    }));
  };

  const loadProviderModels = useCallback(async (providerId: string) => {
    const requestId = ++providerModelsRequest.current;
    if (!providerId) {
      setProviderModels([]);
      setProviderModelsError('');
      setProviderModelsLoading(false);
      return;
    }
    setProviderModelsLoading(true);
    setProviderModelsError('');
    setProviderModels([]);
    try {
      const response = await portalApi.providerModels(providerId);
      if (providerModelsRequest.current === requestId) setProviderModels(response.data);
    } catch (requestError) {
      if (providerModelsRequest.current !== requestId) return;
      setProviderModels([]);
      setProviderModelsError(requestError instanceof Error ? requestError.message : '上游模型获取失败');
    } finally {
      if (providerModelsRequest.current === requestId) setProviderModelsLoading(false);
    }
  }, []);

  const filtered = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    return models.filter((model) => {
      const matchesKeyword = !keyword || `${model.displayName} ${model.modelName} ${model.providerName || ''}`.toLowerCase().includes(keyword);
      const matchesProvider = providerFilter === 'all' || model.providerId === providerFilter;
      const matchesStatus = statusFilter === 'all' || model.status === statusFilter;
      return matchesKeyword && matchesProvider && matchesStatus;
    }).sort((a, b) => Number(a.sortOrder || 100) - Number(b.sortOrder || 100));
  }, [models, providerFilter, search, statusFilter]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const currentPage = Math.min(page, totalPages);
  const summary = useMemo(() => ({
    total: filtered.length,
    active: filtered.filter((model) => model.status === 'active').length,
    providers: new Set(filtered.map((model) => model.providerId)).size,
    avgMarkup: filtered.length ? filtered.reduce((sum, model) => sum + Number(model.markupPercent || 0), 0) / filtered.length : 0,
  }), [filtered]);

  const updateDraft = <K extends keyof ModelDraft>(key: K, value: ModelDraft[K]) => setDraft((current) => ({ ...current, [key]: value }));

  const openCreate = () => {
    const providerId = providers.find((provider) => provider.status === 'active')?.id || providers[0]?.id || '';
    setEditing(null);
    setDraft({ ...emptyDraft, providerId });
    setEditorOpen(true);
    void loadProviderModels(providerId);
  };

  const openEdit = (model: Model) => {
    setEditing(model);
    setDraft(modelInput(model));
    setEditorOpen(true);
    void loadProviderModels(model.providerId);
  };

  const changeProvider = (providerId: string) => {
    setDraft((current) => ({
      ...current,
      providerId,
      modelName: '',
      displayName: current.displayName === current.modelName ? '' : current.displayName,
    }));
    void loadProviderModels(providerId);
  };

  const changeProviderModel = (modelName: string) => {
    setDraft((current) => ({
      ...current,
      modelName,
      displayName: current.displayName.trim() ? current.displayName : modelName,
    }));
  };

  const providerModelOptions = useMemo(() => {
    const names = [...new Set(providerModels.map((model) => model.name.trim()).filter(Boolean))];
    const options = names.map((name) => ({ value: name, label: name }));
    if (draft.modelName && !options.some((option) => option.value === draft.modelName)) {
      options.unshift({ value: draft.modelName, label: `${draft.modelName}（当前配置）` });
    }
    return options;
  }, [draft.modelName, providerModels]);

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
      toast.success('模型已隐藏；历史调用记录仍保留');
      setDeleteCandidate(null);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '模型隐藏失败');
    }
  };

  const saveOverride = async (event: React.FormEvent) => {
    event.preventDefault();
    const unitPrice = Number(overrideUnitPrice);
    if (!selectedOverrideUserId) return toast.error('请选择普通用户');
    if (!selectedOverrideModelId) return toast.error('请选择生图模型');
    if (!Number.isFinite(unitPrice) || unitPrice < MIN_OVERRIDE_PRICE || unitPrice > MAX_OVERRIDE_PRICE) {
      return toast.error('单张扣费需要在 0.001 到 99999999.9999 之间');
    }
    setOverrideSaving(true);
    try {
      await portalApi.saveUserModelPriceOverride({
        userId: selectedOverrideUserId,
        modelId: selectedOverrideModelId,
        unitPrice: Math.round(unitPrice * 10_000) / 10_000,
      });
      const user = overrideUsers.find((item) => item.id === selectedOverrideUserId);
      const model = overrideModels.find((item) => item.id === selectedOverrideModelId);
      toast.success(`已为 ${user?.email || selectedOverrideUserId} 配置 ${model?.displayName || '该模型'} 的专属扣费`);
      setOverrideUnitPrice('');
      await reloadOverrides();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '专属扣费保存失败');
    } finally {
      setOverrideSaving(false);
    }
  };

  const deleteOverride = async () => {
    if (!overrideDeleteCandidate) return;
    setOverrideDeletingId(overrideDeleteCandidate.id);
    try {
      await portalApi.deleteUserModelPriceOverride(overrideDeleteCandidate.id);
      toast.success('专属扣费规则已移除，用户将恢复使用全局价格');
      setOverrideDeleteCandidate(null);
      await reloadOverrides();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '专属扣费移除失败');
    } finally {
      setOverrideDeletingId('');
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
      <button type="button" onClick={() => void toggleStatus(model)} disabled={actionId === model.id} className={`rounded px-2 py-1 text-[11px] font-semibold ${model.status === 'active' ? 'text-amber-700 hover:bg-amber-50' : 'text-emerald-700 hover:bg-emerald-50'} disabled:opacity-40`}>{model.status === 'active' ? '停用' : '启用'}</button>
      <button type="button" onClick={() => setDeleteCandidate(model)} title="隐藏模型" className="rounded p-1.5 text-red-600 hover:bg-red-50"><EyeOff className="h-3.5 w-3.5" /></button>
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
        ].map(([label, value, note]) => <div key={String(label)} className="rounded-md border border-[#DCE4DF] bg-white p-3.5"><span className="text-[11px] font-semibold text-zinc-500">{label}</span><strong className="mt-1.5 block text-xl">{value}</strong><small className="mt-1 block text-[11px] text-zinc-400">{note}</small></div>)}
      </div>

      <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white">
        <div className="flex flex-wrap items-start justify-between gap-3 border-b border-[#DCE4DF] px-4 py-3.5">
          <div>
            <div className="flex items-center gap-2"><UserRound className="h-4 w-4 text-[#047857]" /><h2 className="text-sm font-semibold">用户专属扣费</h2></div>
            <p className="mt-1 text-[11px] text-zinc-500">为单个用户指定某个生图模型的余额单张扣费，未配置时继续使用全局售价。</p>
          </div>
          <span className="rounded-full bg-[#F0FDF4] px-2.5 py-1 text-[11px] font-semibold text-[#047857]">{priceOverrides.length} 条规则</span>
        </div>
        <form onSubmit={saveOverride} className="grid grid-cols-1 gap-3 border-b border-[#EDF0EE] bg-[#FAFBFA] p-4 sm:grid-cols-[1fr_1fr_180px_auto] sm:items-end">
          <label>
            <span className="mb-1 block text-[11px] font-semibold text-zinc-500">指定用户</span>
            <AppSelect value={selectedOverrideUserId} onValueChange={setOverrideUserId} disabled={!overrideUsers.length} placeholder="选择普通用户" ariaLabel="指定用户" options={overrideUsers.length ? overrideUsers.map((user) => ({ value: user.id, label: `${user.email}${user.status === 'disabled' ? '（已停用）' : ''}` })) : [{ value: '', label: '暂无普通用户' }]} />
          </label>
          <label>
            <span className="mb-1 block text-[11px] font-semibold text-zinc-500">指定模型</span>
            <AppSelect value={selectedOverrideModelId} onValueChange={setOverrideModelId} disabled={!overrideModels.length} placeholder="选择生图模型" ariaLabel="指定模型" options={overrideModelOptions.length ? overrideModelOptions : [{ value: '', label: '暂无生图模型' }]} />
          </label>
          <label>
            <span className="mb-1 block text-[11px] font-semibold text-zinc-500">单张扣费</span>
            <div className="relative"><span className="pointer-events-none absolute inset-y-0 left-2.5 flex items-center font-mono text-xs text-zinc-400">¥</span><input required min={MIN_OVERRIDE_PRICE} max={MAX_OVERRIDE_PRICE} step="0.0001" type="number" value={overrideUnitPrice} onChange={(event) => setOverrideUnitPrice(event.target.value)} placeholder="例如 0.008" className="w-full rounded-md border border-[#86EFAC] bg-white py-2 pl-6 pr-2 font-mono text-xs text-[#047857] outline-none focus:border-[#047857]" /></div>
          </label>
          <button type="submit" disabled={overrideSaving || !overrideUsers.length || !overrideModels.length} className="inline-flex h-9 items-center justify-center gap-1.5 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white hover:bg-[#036b4f] disabled:opacity-50">{overrideSaving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}保存规则</button>
        </form>
        {overrideLoading ? (
          <div className="grid min-h-[120px] place-items-center"><Loader2 className="h-5 w-5 animate-spin text-[#12B76A]" /></div>
        ) : priceOverrides.length === 0 ? (
          <div className="px-4 py-8 text-center text-xs text-zinc-400">暂无用户专属扣费，保存后会显示在这里。</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-[680px] w-full text-left text-xs">
              <thead className="border-b border-[#EDF0EE] bg-white text-[10px] font-semibold text-zinc-400"><tr>{overrideHeaders.map((header) => <SortableHeader key={header.key} header={header} sortState={overrideSort} onSort={handleOverrideSort} />)}</tr></thead>
              <tbody className="divide-y divide-[#EDF0EE]">
                {sortedOverrides.map((item) => <tr key={item.id} className="hover:bg-[#FAFBFA]"><td className="px-4 py-3"><strong className="block max-w-[220px] truncate font-medium">{item.userEmail || item.userId}</strong><small className="mt-0.5 block max-w-[220px] truncate font-mono text-[10px] text-zinc-400">{item.userId}</small></td><td className="px-4 py-3"><strong className="block max-w-[220px] truncate font-medium">{item.modelDisplayName || item.modelName}</strong><small className="mt-0.5 block max-w-[220px] truncate font-mono text-[10px] text-zinc-400">{item.modelName}</small></td><td className="px-4 py-3 font-mono font-semibold text-[#047857]">{money(item.unitPrice)}<small className="ml-1 font-sans font-normal text-zinc-400">/ 张</small></td><td className="px-4 py-3 text-zinc-500">{formatDate(item.updatedAt || item.createdAt)}</td><td className="px-4 py-3 text-right"><button type="button" onClick={() => setOverrideDeleteCandidate(item)} disabled={overrideDeletingId === item.id} title="移除专属扣费" className="rounded p-1.5 text-red-600 hover:bg-red-50 disabled:opacity-40"><Trash2 className="h-3.5 w-3.5" /></button></td></tr>)}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {error && <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void load()} className="font-semibold underline">重试</button></div>}

      {loading ? (
        <div className="grid min-h-[300px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <DataTable
          headers={[
            { key: 'model', label: '模型映射', sortValue: (item) => item.displayName || item.modelName },
            { key: 'provider', label: '上游', sortValue: (item) => item.providerName || item.providerId },
            { key: '1k', label: '1K 成本 / 售价', sortValue: (item) => Number(item.price1k || 0) },
            { key: '2k', label: '2K 成本 / 售价', sortValue: (item) => Number(item.price2k || 0) },
            { key: '4k', label: '4K 成本 / 售价', sortValue: (item) => Number(item.price4k || 0) },
            { key: 'tiers', label: '清晰度', sortValue: (item) => (item.enabledSizeTiers || []).join(',') },
            { key: 'status', label: '状态', sortValue: (item) => item.status === 'active' ? 1 : 0 },
            { key: 'actions', label: '操作', sortable: false, className: 'text-right' },
          ]}
          data={filtered}
          pageSize={PAGE_SIZE}
          clientSidePagination
          searchPlaceholder="搜索模型、展示名或上游"
          searchValue={search}
          onSearchChange={(value) => { setSearch(value); setPage(1); }}
          filterControls={(
            <>
              <AppSelect compact value={providerFilter} onValueChange={(value) => { setProviderFilter(value); setPage(1); }} ariaLabel="筛选上游接口" className="max-w-[180px]" options={[{ value: 'all', label: '全部上游' }, ...providers.map((provider) => ({ value: provider.id, label: provider.name }))]} />
              <AppSelect compact value={statusFilter} onValueChange={(value) => { setStatusFilter(value); setPage(1); }} ariaLabel="筛选模型状态" options={[{ value: 'all', label: '全部状态' }, { value: 'active', label: '已启用' }, { value: 'disabled', label: '已停用' }]} />
              <span className="text-[11px] text-zinc-400">{filtered.length} 条</span>
            </>
          )}
          currentPage={currentPage}
          totalPages={totalPages}
          onPageChange={setPage}
          emptyState={<EmptyState title="暂无模型价格" description="先添加上游接口，再创建可对外调用的模型。" icon={CircleDollarSign} />}
          renderRow={(model) => (
            <tr key={model.id} className="hover:bg-[#FAFBFA]">
              <td className="px-4 py-3"><strong className="block max-w-[180px] truncate font-medium">{model.displayName}</strong><small className="mt-0.5 block max-w-[180px] truncate font-mono text-[10px] text-zinc-400">{model.modelName}</small></td>
              <td className="max-w-[140px] truncate px-4 py-3">{model.providerName || providers.find((provider) => provider.id === model.providerId)?.name || model.providerId}</td>
              {(['1k', '2k', '4k'] as const).map((tier) => <td key={tier} className="px-4 py-3 font-mono"><small className="block text-[10px] text-zinc-400">{money(model[`cost${tier}`])}</small><strong className="block text-[12px] text-[#047857]">{money(model[`price${tier}`])}</strong></td>)}
              <td className="px-4 py-3"><span className="text-[11px] font-semibold text-zinc-600">{(model.enabledSizeTiers || []).map((tier) => tier.toUpperCase()).join(' / ') || '-'}</span></td>
              <td className="px-4 py-3"><StatusBadge status={model.status === 'active' ? 'active' : 'disabled'} /></td>
              <td className="px-4 py-3">{rowActions(model)}</td>
            </tr>
          )}
          renderMobileItem={(model) => (
            <article key={model.id} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
              <div className="flex items-start justify-between gap-3"><div className="min-w-0"><strong className="block truncate text-sm">{model.displayName}</strong><small className="block truncate font-mono text-[10px] text-zinc-400">{model.modelName}</small></div><StatusBadge status={model.status === 'active' ? 'active' : 'disabled'} /></div>
              <div className="mt-3 grid grid-cols-3 divide-x divide-[#EDF0EE] border-y border-[#EDF0EE] py-2 text-center">{(['1k', '2k', '4k'] as const).map((tier) => <div key={tier}><small className="block text-[10px] text-zinc-400">{tier.toUpperCase()} 售价</small><strong className="font-mono text-[12px] text-[#047857]">{money(model[`price${tier}`])}</strong></div>)}</div>
              <div className="mt-2 flex items-center justify-between"><small className="max-w-[180px] truncate text-[10px] text-zinc-400">{model.providerName || model.providerId} · {formatDate(model.updatedAt || '')}</small>{rowActions(model)}</div>
            </article>
          )}
        />
      )}

      {editorOpen && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/40 p-4 sm:grid sm:place-items-center">
          <form onSubmit={saveModel} className="mx-auto w-full max-w-3xl overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5"><div><h2 className="text-sm font-semibold">{editing ? '编辑模型与价格' : '新增模型'}</h2><p className="mt-0.5 text-[11px] text-zinc-500">售价单位与 Go 后端现有模型价格字段保持一致。</p></div><button type="button" onClick={() => setEditorOpen(false)} className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button></div>
            <div className="space-y-5 p-5">
              <section className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">上游接口</span><AppSelect required value={draft.providerId} onValueChange={changeProvider} placeholder="请选择上游" ariaLabel="上游接口" options={[{ value: '', label: '请选择上游' }, ...providers.map((provider) => ({ value: provider.id, label: `${provider.name} ${provider.status === 'disabled' ? '（停用）' : ''}`.trim() }))]} /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">状态</span><AppSelect value={draft.status} onValueChange={(value) => updateDraft('status', value as Model['status'])} ariaLabel="模型状态" options={[{ value: 'active', label: '启用' }, { value: 'disabled', label: '停用' }]} /></label>
                <div>
                  <div className="mb-1 flex items-center justify-between gap-2">
                    <span className="text-[11px] font-semibold text-zinc-500">上游模型名</span>
                    <button type="button" onClick={() => void loadProviderModels(draft.providerId)} disabled={!draft.providerId || providerModelsLoading} title="重新获取上游模型" aria-label="重新获取上游模型" className="inline-flex h-6 w-6 items-center justify-center rounded border border-[#DCE4DF] bg-white text-zinc-500 hover:border-[#12B76A] hover:text-[#047857] disabled:opacity-40"><RefreshCw className={`h-3.5 w-3.5 ${providerModelsLoading ? 'animate-spin' : ''}`} /></button>
                  </div>
                  <AppSelect required value={draft.modelName} onValueChange={changeProviderModel} disabled={!draft.providerId || providerModelsLoading || !providerModelOptions.length} placeholder={providerModelsLoading ? '正在获取上游模型...' : '请选择上游模型'} ariaLabel="上游模型名" options={[{ value: '', label: providerModelsLoading ? '正在获取上游模型...' : '请选择上游模型' }, ...providerModelOptions]} />
                  {providerModelsError ? <div className="mt-1.5 flex items-center justify-between gap-2 text-[10px] text-red-600"><span className="min-w-0 truncate">{providerModelsError}</span><button type="button" onClick={() => void loadProviderModels(draft.providerId)} className="shrink-0 font-semibold underline">重试</button></div> : !providerModelsLoading && draft.providerId && !providerModelOptions.length ? <p className="mt-1.5 text-[10px] text-zinc-400">该上游暂未返回可选模型</p> : null}
                </div>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">对外展示名</span><input required value={draft.displayName} onChange={(event) => updateDraft('displayName', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
              </section>

              <section className="border-t border-[#DCE4DF] pt-4">
                <div className="mb-3 flex flex-wrap items-end justify-between gap-3"><div><h3 className="text-xs font-semibold">成本与售价</h3><p className="mt-0.5 text-[11px] text-zinc-400">保留四位小数，按单张图片计费。</p></div><div className="flex items-end gap-2"><label><span className="mb-1 block text-[10px] text-zinc-400">加价率 %</span><input type="number" step="0.01" value={draft.markupPercent} onChange={(event) => updateDraft('markupPercent', Number(event.target.value))} className="w-24 rounded-md border border-[#DCE4DF] px-2 py-1.5 font-mono text-xs" /></label><button type="button" onClick={calculatePrices} className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[#86EFAC] bg-[#F0FDF4] px-3 text-[11px] font-semibold text-[#047857]"><Calculator className="h-3.5 w-3.5" />计算售价</button></div></div>
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">{(['1k', '2k', '4k'] as const).map((tier) => <div key={tier} className="grid grid-cols-2 gap-2 rounded-md border border-[#E5E9E6] bg-[#FAFBFA] p-3"><strong className="col-span-2 text-[11px]">{tier.toUpperCase()} 清晰度</strong><label><span className="mb-1 block text-[10px] text-zinc-400">成本</span><input min={0} type="number" step="0.0001" value={draft[`cost${tier}`]} onChange={(event) => updateDraft(`cost${tier}`, Number(event.target.value))} className="w-full rounded border border-[#DCE4DF] px-2 py-1.5 font-mono text-xs" /></label><label><span className="mb-1 block text-[10px] text-zinc-400">售价</span><input min={0} type="number" step="0.0001" value={draft[`price${tier}`]} onChange={(event) => updateDraft(`price${tier}`, Number(event.target.value))} className="w-full rounded border border-[#86EFAC] px-2 py-1.5 font-mono text-xs text-[#047857]" /></label></div>)}</div>
              </section>

              <section className="grid grid-cols-1 gap-4 border-t border-[#DCE4DF] pt-4 sm:grid-cols-3">
                <div><span className="mb-2 block text-[11px] font-semibold text-zinc-500">对外开放清晰度</span><div className="flex gap-2">{['1k', '2k', '4k'].map((tier) => <label key={tier} className={`flex cursor-pointer items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-[11px] font-semibold ${draft.enabledSizeTiers.includes(tier) ? 'border-[#86EFAC] bg-[#F0FDF4] text-[#047857]' : 'border-[#DCE4DF] text-zinc-500'}`}><input type="checkbox" checked={draft.enabledSizeTiers.includes(tier)} onChange={() => toggleTier(tier)} className="h-3 w-3 accent-[#047857]" />{tier.toUpperCase()}</label>)}</div></div>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">排序值</span><input min={0} type="number" value={draft.sortOrder} onChange={(event) => updateDraft('sortOrder', Number(event.target.value))} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs" /></label>
                <label className="flex items-center justify-between gap-3 rounded-md border border-[#DCE4DF] px-3 py-2 text-xs"><span><strong className="block text-[11px]">附加尺寸提示</strong><small className="text-[10px] text-zinc-400">将清晰度传给上游</small></span><input type="checkbox" checked={draft.appendSizeToPrompt} onChange={(event) => updateDraft('appendSizeToPrompt', event.target.checked)} className="h-4 w-4 accent-[#047857]" /></label>
              </section>
            </div>
            <div className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><button type="button" onClick={() => setEditorOpen(false)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button><button type="submit" disabled={saving} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}保存模型</button></div>
          </form>
        </div>
      )}

      <ConfirmDialog isOpen={Boolean(overrideDeleteCandidate)} onClose={() => setOverrideDeleteCandidate(null)} onConfirm={() => void deleteOverride()} title="移除专属扣费" description={`确定移除 ${overrideDeleteCandidate?.userEmail || '该用户'} 的 ${overrideDeleteCandidate?.modelDisplayName || '该模型'} 专属扣费吗？移除后将恢复使用全局售价。`} confirmText="移除" type="danger" />
      <ConfirmDialog isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)} onConfirm={() => void deleteModel()} title="隐藏模型" description={`确定隐藏 ${deleteCandidate?.displayName || '该模型'} 吗？隐藏后不会出现在列表中，但历史调用与统计会保留。`} confirmText="隐藏" type="danger" />
    </div>
  );
}
