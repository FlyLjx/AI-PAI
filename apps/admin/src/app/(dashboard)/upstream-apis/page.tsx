'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Activity, Cable, Clock3, FlaskConical, Loader2, Pause, Pencil, Play, Plus, RefreshCw, Trash2, X } from 'lucide-react';
import { toast } from 'sonner';
import { AppSelect } from '@/components/common/AppSelect';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { DataTable } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { StatusBadge } from '@/components/common/StatusBadge';
import { portalApi, type OpenAIImageStatusSnapshot, type UpstreamMaintenanceState } from '@/lib/admin-api';
import { formatDate } from '@/lib/common/utils';

type Provider = {
  id: string;
  name: string;
  type: 'sub2api' | 'newapi' | 'custom';
  capability: 'chat_image';
  baseUrl: string;
  apiKey: string;
  status: 'active' | 'disabled';
  createdAt?: string;
  updatedAt?: string;
};

type ProviderDraft = Omit<Provider, 'id' | 'createdAt' | 'updatedAt'>;

const emptyDraft: ProviderDraft = {
  name: '',
  type: 'custom',
  capability: 'chat_image',
  baseUrl: '',
  apiKey: '',
  status: 'active',
};

function maskKey(value: string) {
  const key = String(value || '');
  if (!key) return '-';
  if (key.length < 12) return '******';
  return `${key.slice(0, 6)}••••${key.slice(-4)}`;
}

function typeLabel(type: Provider['type']) {
  if (type === 'sub2api') return 'Sub2API';
  if (type === 'newapi') return 'New API';
  return '自定义兼容';
}

export default function UpstreamAPIsPage() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [typeFilter, setTypeFilter] = useState('all');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<Provider | null>(null);
  const [draft, setDraft] = useState<ProviderDraft>(emptyDraft);
  const [testingId, setTestingId] = useState('');
  const [testResult, setTestResult] = useState<{ provider: Provider; result: Record<string, unknown> } | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<Provider | null>(null);
  const [maintenance, setMaintenance] = useState<UpstreamMaintenanceState | null>(null);
  const [maintenanceLoading, setMaintenanceLoading] = useState(true);
  const [maintenanceSaving, setMaintenanceSaving] = useState(false);
  const [openAIStatus, setOpenAIStatus] = useState<OpenAIImageStatusSnapshot | null>(null);
  const [openAIStatusLoading, setOpenAIStatusLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setMaintenanceLoading(true);
    setOpenAIStatusLoading(true);
    setError('');
    const [providerResponse, maintenanceResponse, openAIResponse] = await Promise.allSettled([
      portalApi.providers(),
      portalApi.upstreamMaintenance(),
      portalApi.openAIImageStatus(),
    ]);
    if (providerResponse.status === 'fulfilled') {
      setProviders(providerResponse.value.data as unknown as Provider[]);
    } else {
      const reason = providerResponse.reason;
      setError(reason instanceof Error ? reason.message : '上游接口加载失败');
    }
    if (maintenanceResponse.status === 'fulfilled') {
      setMaintenance(maintenanceResponse.value.data);
    }
    if (openAIResponse.status === 'fulfilled') {
      setOpenAIStatus(openAIResponse.value.data);
    }
    setLoading(false);
    setMaintenanceLoading(false);
    setOpenAIStatusLoading(false);
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  const filtered = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    return providers.filter((provider) => {
      const matchesKeyword = !keyword || `${provider.name} ${provider.baseUrl} ${provider.type}`.toLowerCase().includes(keyword);
      const matchesType = typeFilter === 'all' || provider.type === typeFilter;
      const matchesStatus = statusFilter === 'all' || provider.status === statusFilter;
      return matchesKeyword && matchesType && matchesStatus;
    });
  }, [providers, search, statusFilter, typeFilter]);

  const summary = useMemo(() => ({
    total: providers.length,
    active: providers.filter((provider) => provider.status === 'active').length,
    disabled: providers.filter((provider) => provider.status === 'disabled').length,
    newapi: providers.filter((provider) => provider.type === 'newapi').length,
  }), [providers]);

  const updateDraft = <K extends keyof ProviderDraft>(key: K, value: ProviderDraft[K]) => setDraft((current) => ({ ...current, [key]: value }));

  const openCreate = () => {
    setEditing(null);
    setDraft(emptyDraft);
    setEditorOpen(true);
  };

  const openEdit = (provider: Provider) => {
    setEditing(provider);
    setDraft({
      name: provider.name,
      type: provider.type,
      capability: 'chat_image',
      baseUrl: provider.baseUrl,
      apiKey: provider.apiKey,
      status: provider.status,
    });
    setEditorOpen(true);
  };

  const saveProvider = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!draft.name.trim() || !draft.baseUrl.trim() || !draft.apiKey.trim()) return toast.error('请填写接口名称、Base URL 和 API Key');
    setSaving(true);
    try {
      const input = {
        ...draft,
        name: draft.name.trim(),
        baseUrl: draft.baseUrl.trim().replace(/\/$/, ''),
        apiKey: draft.apiKey.trim(),
      };
      if (editing) await portalApi.updateProvider(editing.id, input);
      else await portalApi.createProvider(input);
      toast.success(editing ? '上游接口已更新' : '上游接口已创建');
      setEditorOpen(false);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '上游接口保存失败');
    } finally {
      setSaving(false);
    }
  };

  const toggleStatus = async (provider: Provider) => {
    try {
      const nextStatus: Provider['status'] = provider.status === 'active' ? 'disabled' : 'active';
      await portalApi.updateProvider(provider.id, {
        name: provider.name,
        type: provider.type,
        capability: provider.capability || 'chat_image',
        baseUrl: provider.baseUrl,
        apiKey: provider.apiKey,
        status: nextStatus,
      });
      toast.success(nextStatus === 'active' ? '上游接口已启用' : '上游接口已停用');
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '状态更新失败');
    }
  };

  const testProvider = async (provider: Provider) => {
    setTestingId(provider.id);
    try {
      const response = await portalApi.testProvider(provider.id);
      const result = (response.data || {}) as Record<string, unknown>;
      setTestResult({ provider, result });
      if (result.ok === false || result.status === 'failed') toast.error('上游连通测试未通过');
      else toast.success('上游连通测试通过');
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '上游测试失败');
    } finally {
      setTestingId('');
    }
  };

  const toggleMaintenance = async () => {
    const nextEnabled = !maintenance?.enabled;
    setMaintenanceSaving(true);
    try {
      const response = await portalApi.updateUpstreamMaintenance(nextEnabled);
      setMaintenance(response.data);
      toast.success(nextEnabled ? '已开启上游更新保护，新任务会暂留队列' : '已关闭上游更新保护，排队任务将继续处理');
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '上游更新保护切换失败');
    } finally {
      setMaintenanceSaving(false);
    }
  };

  const deleteProvider = async () => {
    if (!deleteCandidate) return;
    try {
      await portalApi.deleteProvider(deleteCandidate.id);
      toast.success('上游接口已删除');
      setDeleteCandidate(null);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '上游接口删除失败');
    }
  };

  const rowActions = (provider: Provider) => (
    <div className="flex items-center justify-end gap-1">
      <button type="button" onClick={() => void testProvider(provider)} disabled={testingId === provider.id} title="测试连接" className="rounded p-1.5 text-[#0891B2] hover:bg-cyan-50 disabled:opacity-40">{testingId === provider.id ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <FlaskConical className="h-3.5 w-3.5" />}</button>
      <button type="button" onClick={() => openEdit(provider)} title="编辑接口" className="rounded p-1.5 text-zinc-600 hover:bg-zinc-100"><Pencil className="h-3.5 w-3.5" /></button>
      <button type="button" onClick={() => void toggleStatus(provider)} className={`rounded px-2 py-1 text-[11px] font-semibold ${provider.status === 'active' ? 'text-amber-700 hover:bg-amber-50' : 'text-emerald-700 hover:bg-emerald-50'}`}>{provider.status === 'active' ? '停用' : '启用'}</button>
      <button type="button" onClick={() => setDeleteCandidate(provider)} title="删除接口" className="rounded p-1.5 text-red-600 hover:bg-red-50"><Trash2 className="h-3.5 w-3.5" /></button>
    </div>
  );

  const maintenanceEnabled = Boolean(maintenance?.enabled);
  const openAISeverity = openAIStatus?.severity || 'ok';
  const openAITone = openAIStatusLoading && !openAIStatus
    ? 'border-zinc-200 bg-zinc-50 text-zinc-500'
    : openAISeverity === 'critical'
      ? 'border-red-200 bg-red-50 text-red-700'
      : openAISeverity === 'warning'
        ? 'border-amber-200 bg-amber-50 text-amber-700'
        : 'border-emerald-200 bg-emerald-50 text-emerald-700';
  const openAIDot = openAISeverity === 'critical' ? 'bg-red-500' : openAISeverity === 'warning' ? 'bg-amber-500' : 'bg-emerald-500';

  return (
    <div className="space-y-5">
      <PageHeader title="上游接口" description="维护兼容 OpenAI 图片 API 的服务地址、密钥和运行状态。">
        <button type="button" onClick={() => void load()} disabled={loading} title="刷新接口" className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-[#DCE4DF] bg-white hover:border-[#12B76A] disabled:opacity-50"><RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} /></button>
        <button type="button" onClick={openCreate} className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[#047857] px-3 text-xs font-semibold text-white hover:bg-[#036b4f]"><Plus className="h-4 w-4" />新增接口</button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          ['接口总数', summary.total, '全部上游配置'],
          ['正常启用', summary.active, '参与请求调度'],
          ['已停用', summary.disabled, '不再接收请求'],
          ['New API', summary.newapi, '兼容服务商'],
        ].map(([label, value, note]) => <div key={String(label)} className="rounded-md border border-[#DCE4DF] bg-white p-3.5"><span className="text-[11px] font-semibold text-zinc-500">{label}</span><strong className="mt-1.5 block text-xl">{value}</strong><small className="mt-1 block text-[11px] text-zinc-400">{note}</small></div>)}
      </div>

      <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
        <section className={`overflow-hidden rounded-md border ${maintenanceEnabled ? 'border-amber-200 bg-amber-50/70' : 'border-[#DCE4DF] bg-white'}`}>
          <div className={`h-1 w-full ${maintenanceEnabled ? 'bg-amber-500' : 'bg-[#3F9274]'}`} />
          <div className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className={`grid h-8 w-8 place-items-center rounded-md ${maintenanceEnabled ? 'bg-amber-100 text-amber-700' : 'bg-emerald-50 text-emerald-700'}`}>
                  {maintenanceEnabled ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
                </span>
                <div>
                  <h2 className="text-sm font-semibold text-[#17201B]">上游更新保护</h2>
                  <p className="mt-0.5 text-[11px] text-zinc-500">
                    {maintenanceEnabled ? '已暂停新任务进入上游处理，任务会保持排队状态。' : '当前任务会按正常队列进入上游处理。'}
                  </p>
                </div>
              </div>
              <div className="mt-3 flex flex-wrap gap-2 text-[11px] text-zinc-500">
                <span className="rounded border border-[#E5E9E6] bg-white px-2 py-1">等待 {Number(maintenance?.waitingTasks || 0).toLocaleString('zh-CN')}</span>
                <span className="rounded border border-[#E5E9E6] bg-white px-2 py-1">处理中 {Number(maintenance?.processingTasks || 0).toLocaleString('zh-CN')}</span>
                {maintenance?.pausedAt && <span className="rounded border border-[#E5E9E6] bg-white px-2 py-1">开启于 {formatDate(maintenance.pausedAt)}</span>}
              </div>
            </div>
            <button
              type="button"
              onClick={() => void toggleMaintenance()}
              disabled={maintenanceLoading || maintenanceSaving}
              className={`inline-flex h-8 shrink-0 items-center justify-center gap-1.5 rounded-md px-3 text-xs font-semibold disabled:opacity-50 ${maintenanceEnabled ? 'border border-emerald-200 bg-white text-emerald-700 hover:bg-emerald-50' : 'border border-amber-200 bg-amber-600 text-white hover:bg-amber-700'}`}
            >
              {maintenanceSaving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : maintenanceEnabled ? <Play className="h-3.5 w-3.5" /> : <Pause className="h-3.5 w-3.5" />}
              {maintenanceEnabled ? '关闭保护并放行' : '开启上游更新'}
            </button>
          </div>
        </section>

        <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white">
          <div className={`h-1 w-full ${openAISeverity === 'critical' ? 'bg-red-500' : openAISeverity === 'warning' ? 'bg-amber-500' : 'bg-[#3F9274]'}`} />
          <div className="flex flex-col gap-3 p-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className="grid h-8 w-8 place-items-center rounded-md bg-blue-50 text-blue-700"><Activity className="h-4 w-4" /></span>
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 className="text-sm font-semibold text-[#17201B]">OpenAI Image 状态订阅</h2>
                    <span className={`inline-flex items-center gap-1.5 rounded border px-2 py-0.5 text-[11px] font-semibold ${openAITone}`}><i className={`h-1.5 w-1.5 rounded-full ${openAIDot}`} />{openAIStatusLoading && !openAIStatus ? '检测中' : openAIStatus?.statusLabel || '未知'}</span>
                  </div>
                  <p className="mt-1 line-clamp-2 text-[11px] text-zinc-500">{openAIStatus?.summary || '订阅 status.openai.com/feed.rss，仅关注 Image / Image Generation 相关事件。'}</p>
                </div>
              </div>
              <div className="mt-3 flex flex-wrap gap-2 text-[11px] text-zinc-500">
                <span className="rounded border border-[#E5E9E6] bg-[#FAFBFA] px-2 py-1">Image 事件 {Number(openAIStatus?.totalImageIncidents || 0).toLocaleString('zh-CN')}</span>
                {openAIStatus?.latestImageIncident?.publishedAt && <span className="rounded border border-[#E5E9E6] bg-[#FAFBFA] px-2 py-1">最新 {formatDate(openAIStatus.latestImageIncident.publishedAt)}</span>}
                {openAIStatus?.latestImageIncident?.title && <span className="max-w-full truncate rounded border border-[#E5E9E6] bg-[#FAFBFA] px-2 py-1">{openAIStatus.latestImageIncident.title}</span>}
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-2 text-[11px] text-zinc-400">
              {openAIStatusLoading && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
              <Clock3 className="h-3.5 w-3.5" />
              {openAIStatus?.fetchedAt ? formatDate(openAIStatus.fetchedAt) : '-'}
            </div>
          </div>
        </section>
      </div>

      {error && <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void load()} className="font-semibold underline">重试</button></div>}

      {loading ? (
        <div className="grid min-h-[300px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <DataTable
          headers={[
            { key: 'name', label: '接口名称' },
            { key: 'type', label: '类型' },
            { key: 'url', label: 'Base URL' },
            { key: 'key', label: 'API Key' },
            { key: 'status', label: '状态' },
            { key: 'updated', label: '更新时间' },
            { key: 'actions', label: '操作', className: 'text-right' },
          ]}
          data={filtered}
          searchPlaceholder="搜索名称、地址或类型"
          searchValue={search}
          onSearchChange={setSearch}
          filterControls={(
            <>
              <AppSelect
                compact
                value={typeFilter}
                onValueChange={setTypeFilter}
                ariaLabel="接口类型筛选"
                options={[
                  { value: 'all', label: '全部类型' },
                  { value: 'sub2api', label: 'Sub2API' },
                  { value: 'newapi', label: 'New API' },
                  { value: 'custom', label: '自定义' },
                ]}
              />
              <AppSelect
                compact
                value={statusFilter}
                onValueChange={setStatusFilter}
                ariaLabel="接口状态筛选"
                options={[
                  { value: 'all', label: '全部状态' },
                  { value: 'active', label: '已启用' },
                  { value: 'disabled', label: '已停用' },
                ]}
              />
              <span className="text-[11px] text-zinc-400">{filtered.length} 条</span>
            </>
          )}
          emptyState={<EmptyState title="暂无上游接口" description="添加第一个兼容 OpenAI 图片接口的上游。" icon={Cable} />}
          renderRow={(provider) => (
            <tr key={provider.id} className="hover:bg-[#FAFBFA]">
              <td className="px-4 py-3"><strong className="block max-w-[160px] truncate font-medium">{provider.name}</strong><small className="font-mono text-[10px] text-zinc-400">{provider.id}</small></td>
              <td className="px-4 py-3"><span className="rounded border border-zinc-200 bg-zinc-50 px-1.5 py-0.5 text-[11px]">{typeLabel(provider.type)}</span></td>
              <td className="max-w-[260px] truncate px-4 py-3 font-mono text-[11px] text-zinc-600" title={provider.baseUrl}>{provider.baseUrl}</td>
              <td className="px-4 py-3 font-mono text-[11px] text-zinc-500">{maskKey(provider.apiKey)}</td>
              <td className="px-4 py-3"><StatusBadge status={provider.status === 'active' ? 'active' : 'disabled'} /></td>
              <td className="whitespace-nowrap px-4 py-3 text-zinc-500">{formatDate(provider.updatedAt || provider.createdAt || '')}</td>
              <td className="px-4 py-3">{rowActions(provider)}</td>
            </tr>
          )}
          renderMobileItem={(provider) => (
            <article key={provider.id} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
              <div className="flex items-start justify-between gap-3"><div className="min-w-0"><strong className="block truncate text-sm">{provider.name}</strong><small className="text-[10px] text-zinc-400">{typeLabel(provider.type)}</small></div><StatusBadge status={provider.status === 'active' ? 'active' : 'disabled'} /></div>
              <p className="mt-3 truncate rounded bg-[#F6F8F6] px-2 py-1.5 font-mono text-[11px] text-zinc-600">{provider.baseUrl}</p>
              <div className="mt-2 flex items-center justify-between"><small className="font-mono text-[10px] text-zinc-400">{maskKey(provider.apiKey)}</small>{rowActions(provider)}</div>
            </article>
          )}
        />
      )}

      {editorOpen && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4">
          <form onSubmit={saveProvider} className="w-full max-w-2xl overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5"><div><h2 className="text-sm font-semibold">{editing ? '编辑上游接口' : '新增上游接口'}</h2><p className="mt-0.5 text-[11px] text-zinc-500">保存后可测试 `/v1/models` 连通性。</p></div><button type="button" onClick={() => setEditorOpen(false)} className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button></div>
            <div className="grid grid-cols-1 gap-4 p-5 sm:grid-cols-2">
              <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">接口名称</span><input required value={draft.name} onChange={(event) => updateDraft('name', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
              <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">接口类型</span><AppSelect value={draft.type} onValueChange={(value) => updateDraft('type', value as Provider['type'])} options={[{ value: 'custom', label: '自定义兼容' }, { value: 'newapi', label: 'New API' }, { value: 'sub2api', label: 'Sub2API' }]} /></label>
              <label className="sm:col-span-2"><span className="mb-1 block text-[11px] font-semibold text-zinc-500">Base URL</span><input required type="url" placeholder="https://api.example.com" value={draft.baseUrl} onChange={(event) => updateDraft('baseUrl', event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs outline-none focus:border-[#12B76A]" /></label>
              <label className="sm:col-span-2"><span className="mb-1 block text-[11px] font-semibold text-zinc-500">API Key</span><textarea required rows={3} value={draft.apiKey} onChange={(event) => updateDraft('apiKey', event.target.value)} className="w-full resize-none rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs outline-none focus:border-[#12B76A]" /></label>
              <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">用途</span><AppSelect disabled value={draft.capability} options={[{ value: 'chat_image', label: '图片 API 中转' }]} /></label>
              <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">状态</span><AppSelect value={draft.status} onValueChange={(value) => updateDraft('status', value as Provider['status'])} options={[{ value: 'active', label: '启用' }, { value: 'disabled', label: '停用' }]} /></label>
            </div>
            <div className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><button type="button" onClick={() => setEditorOpen(false)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button><button type="submit" disabled={saving} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}保存接口</button></div>
          </form>
        </div>
      )}

      {testResult && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4">
          <div className="w-full max-w-xl overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5"><div><h2 className="text-sm font-semibold">连接测试 · {testResult.provider.name}</h2><p className="mt-0.5 text-[11px] text-zinc-500">Go 后端返回的实时探测结果</p></div><button type="button" onClick={() => setTestResult(null)} className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button></div>
            <dl className="grid grid-cols-1 gap-px bg-[#E8ECE9] sm:grid-cols-2">
              {Object.entries(testResult.result).filter(([key]) => key !== 'auth').map(([key, value]) => <div key={key} className="bg-white px-4 py-3"><dt className="text-[10px] font-semibold uppercase text-zinc-400">{key}</dt><dd className="mt-1 break-words font-mono text-xs text-[#17201B]">{typeof value === 'object' ? JSON.stringify(value) : String(value ?? '-')}</dd></div>)}
            </dl>
            <div className="flex justify-end border-t border-[#DCE4DF] px-5 py-3"><button type="button" onClick={() => setTestResult(null)} className="h-8 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white">关闭</button></div>
          </div>
        </div>
      )}

      <ConfirmDialog isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)} onConfirm={() => void deleteProvider()} title="删除上游接口" description={`确定删除 ${deleteCandidate?.name || '该接口'} 吗？Go 后端会按现有规则同步处理其模型配置。`} confirmText="删除" type="danger" />
    </div>
  );
}
