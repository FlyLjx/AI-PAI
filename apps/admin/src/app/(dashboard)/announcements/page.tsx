'use client';

import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';
import {
  BellRing, Check, Eye, EyeOff, Loader2, Mail, Megaphone, Pencil, Plus, RefreshCw,
  Search, Trash2, Users, X,
} from 'lucide-react';
import { toast } from 'sonner';
import { AppSelect } from '@/components/common/AppSelect';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { DataTable } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { formatDate } from '@/lib/common/utils';
import { portalApi, type Announcement, type PortalUser } from '@/lib/admin-api';

type AnnouncementDraft = Omit<Announcement, 'id' | 'createdAt' | 'updatedAt'>;

const emptyDraft: AnnouncementDraft = {
  title: '',
  content: '',
  displayMode: 'popup',
  targetType: 'all',
  status: 'active',
  sortOrder: 100,
  userIds: [],
};

const STATUS_OPTIONS = [
  { value: 'all', label: '全部状态' },
  { value: 'active', label: '展示中' },
  { value: 'disabled', label: '已停用' },
];

const MODE_OPTIONS = [
  { value: 'all', label: '全部方式' },
  { value: 'popup', label: '确认弹窗' },
  { value: 'banner', label: '页面横幅' },
];

function toDraft(item: Announcement): AnnouncementDraft {
  return {
    title: item.title,
    content: item.content,
    displayMode: item.displayMode,
    targetType: item.targetType,
    status: item.status,
    sortOrder: Number(item.sortOrder || 0),
    userIds: item.userIds || [],
  };
}

function displayLabel(mode: Announcement['displayMode']) {
  return mode === 'banner' ? '页面横幅' : '确认弹窗';
}

export default function AnnouncementsPage() {
  const [items, setItems] = useState<Announcement[]>([]);
  const [users, setUsers] = useState<PortalUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [actionId, setActionId] = useState('');
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [modeFilter, setModeFilter] = useState('all');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<Announcement | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<Announcement | null>(null);
  const [draft, setDraft] = useState<AnnouncementDraft>(emptyDraft);
  const [userSearch, setUserSearch] = useState('');
  const [sendEmail, setSendEmail] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [announcementResponse, userResponse] = await Promise.all([
        portalApi.announcements(),
        portalApi.users(),
      ]);
      setItems(announcementResponse.data || []);
      setUsers((userResponse.data || []).filter((user) => user.role === 'user'));
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '公告列表加载失败');
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
    return items.filter((item) => {
      if (statusFilter !== 'all' && item.status !== statusFilter) return false;
      if (modeFilter !== 'all' && item.displayMode !== modeFilter) return false;
      return !keyword || `${item.title} ${item.content}`.toLowerCase().includes(keyword);
    });
  }, [items, modeFilter, search, statusFilter]);

  const visibleUsers = useMemo(() => {
    const keyword = userSearch.trim().toLowerCase();
    return users.filter((user) => !keyword || `${user.email} ${user.id}`.toLowerCase().includes(keyword));
  }, [userSearch, users]);

  const summary = useMemo(() => ({
    total: items.length,
    active: items.filter((item) => item.status === 'active').length,
    popup: items.filter((item) => item.displayMode === 'popup').length,
    banner: items.filter((item) => item.displayMode === 'banner').length,
  }), [items]);

  const openCreate = () => {
    setEditing(null);
    setDraft({ ...emptyDraft, userIds: [] });
    setUserSearch('');
    setSendEmail(false);
    setEditorOpen(true);
  };

  const openEdit = (item: Announcement) => {
    setEditing(item);
    setDraft(toDraft(item));
    setUserSearch('');
    setSendEmail(false);
    setEditorOpen(true);
  };

  const updateDraft = <K extends keyof AnnouncementDraft>(key: K, value: AnnouncementDraft[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const toggleUser = (userId: string) => {
    setDraft((current) => ({
      ...current,
      userIds: current.userIds.includes(userId)
        ? current.userIds.filter((id) => id !== userId)
        : [...current.userIds, userId],
    }));
  };

  const save = async (event: FormEvent) => {
    event.preventDefault();
    if (draft.targetType === 'users' && draft.userIds.length === 0) {
      toast.error('请选择至少一个接收用户');
      return;
    }
    setSaving(true);
    try {
      const input = { ...draft, userIds: draft.targetType === 'all' ? [] : draft.userIds, sendEmail };
      const response = editing
        ? await portalApi.updateAnnouncement(editing.id, input)
        : await portalApi.createAnnouncement(input);
      const actionLabel = editing ? '公告已更新' : '公告已创建';
      if (!sendEmail) {
        toast.success(actionLabel);
      } else if (!response.mailDelivery?.accepted) {
        toast.error(`${actionLabel}，邮件发送失败：${response.mailDelivery?.message || '邮件服务不可用'}`);
      } else if (response.mailDelivery.failed > 0) {
        toast.warning(`${actionLabel}，邮件成功 ${response.mailDelivery.success} 封，失败 ${response.mailDelivery.failed} 封`);
      } else {
        toast.success(`${actionLabel}，已同步发送 ${response.mailDelivery.success} 封邮件`);
      }
      setEditorOpen(false);
      await load();
    } catch (saveError) {
      toast.error(saveError instanceof Error ? saveError.message : '公告保存失败');
    } finally {
      setSaving(false);
    }
  };

  const toggleStatus = async (item: Announcement) => {
    setActionId(item.id);
    try {
      const nextStatus: Announcement['status'] = item.status === 'active' ? 'disabled' : 'active';
      await portalApi.updateAnnouncement(item.id, { ...toDraft(item), status: nextStatus });
      toast.success(nextStatus === 'active' ? '公告已启用' : '公告已停用');
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '公告状态更新失败');
    } finally {
      setActionId('');
    }
  };

  const remove = async () => {
    if (!deleteCandidate) return;
    try {
      await portalApi.deleteAnnouncement(deleteCandidate.id);
      toast.success('公告已删除');
      setDeleteCandidate(null);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '公告删除失败');
    }
  };

  const rowActions = (item: Announcement) => (
    <div className="flex items-center justify-end gap-1">
      <button type="button" onClick={() => openEdit(item)} title="编辑公告" className="rounded p-1.5 text-zinc-600 hover:bg-zinc-100"><Pencil className="h-3.5 w-3.5" /></button>
      <button type="button" onClick={() => void toggleStatus(item)} disabled={actionId === item.id} title={item.status === 'active' ? '停用公告' : '启用公告'} className={`rounded p-1.5 ${item.status === 'active' ? 'text-amber-700 hover:bg-amber-50' : 'text-emerald-700 hover:bg-emerald-50'} disabled:opacity-40`}>
        {actionId === item.id ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : item.status === 'active' ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
      </button>
      <button type="button" onClick={() => setDeleteCandidate(item)} title="删除公告" className="rounded p-1.5 text-red-600 hover:bg-red-50"><Trash2 className="h-3.5 w-3.5" /></button>
    </div>
  );

  return (
    <div className="space-y-5">
      <PageHeader title="公告管理" description="发布面向全部或指定用户的站内横幅与确认弹窗。">
        <button type="button" onClick={() => void load()} disabled={loading} title="刷新公告" className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white hover:border-[#12B76A] disabled:opacity-50"><RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} /></button>
        <button type="button" onClick={openCreate} className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[#047857] px-3 text-xs font-semibold text-white hover:bg-[#036B4F]"><Plus className="h-4 w-4" />新增公告</button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          { label: '公告总数', value: summary.total, note: '历史公告', icon: Megaphone, tone: 'bg-zinc-100 text-zinc-600' },
          { label: '展示中', value: summary.active, note: '用户当前可见', icon: Eye, tone: 'bg-emerald-50 text-emerald-700' },
          { label: '确认弹窗', value: summary.popup, note: '确认后不再弹出', icon: BellRing, tone: 'bg-amber-50 text-amber-700' },
          { label: '页面横幅', value: summary.banner, note: '持续展示', icon: Megaphone, tone: 'bg-blue-50 text-blue-700' },
        ].map((metric) => {
          const Icon = metric.icon;
          return <div key={metric.label} className="flex items-center gap-3 rounded-md border border-[#DCE4DF] bg-white p-4"><span className={`grid h-9 w-9 place-items-center rounded-md ${metric.tone}`}><Icon className="h-4 w-4" /></span><span><small className="block text-[10px] text-zinc-400">{metric.label}</small><strong className="mt-0.5 block font-mono text-lg">{metric.value}</strong><small className="block text-[9px] text-zinc-400">{metric.note}</small></span></div>;
        })}
      </div>

      {error && <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void load()} className="font-semibold underline">重试</button></div>}

      {loading ? (
        <div className="grid min-h-[320px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <DataTable
          headers={[
            { key: 'title', label: '公告内容' },
            { key: 'mode', label: '展示方式' },
            { key: 'target', label: '接收范围' },
            { key: 'sort', label: '排序' },
            { key: 'status', label: '状态' },
            { key: 'updated', label: '更新时间' },
            { key: 'actions', label: '操作', className: 'text-right' },
          ]}
          data={filtered}
          searchPlaceholder="搜索公告标题或内容"
          searchValue={search}
          onSearchChange={setSearch}
          filterControls={<><AppSelect compact value={modeFilter} onValueChange={setModeFilter} ariaLabel="筛选展示方式" options={MODE_OPTIONS} /><AppSelect compact value={statusFilter} onValueChange={setStatusFilter} ariaLabel="筛选公告状态" options={STATUS_OPTIONS} /><span className="text-[11px] text-zinc-400">{filtered.length} 条</span></>}
          emptyState={<EmptyState title="暂无公告" description="创建公告后，可通过横幅或弹窗通知用户。" icon={Megaphone} />}
          renderRow={(item) => (
            <tr key={item.id} className="hover:bg-[#FAFBFA]">
              <td className="max-w-[320px] px-4 py-3"><strong className="block truncate font-medium" title={item.title}>{item.title}</strong><small className="mt-0.5 block truncate text-[10px] text-zinc-400" title={item.content}>{item.content}</small></td>
              <td className="px-4 py-3"><span className={`inline-flex rounded border px-1.5 py-0.5 text-[10px] font-semibold ${item.displayMode === 'popup' ? 'border-amber-200 bg-amber-50 text-amber-700' : 'border-blue-200 bg-blue-50 text-blue-700'}`}>{displayLabel(item.displayMode)}</span></td>
              <td className="px-4 py-3 text-[11px] text-zinc-600">{item.targetType === 'all' ? '全部用户' : `${item.userIds.length} 位用户`}</td>
              <td className="px-4 py-3 font-mono text-[11px]">{item.sortOrder}</td>
              <td className="px-4 py-3"><span className={`status-pill ${item.status}`}>{item.status === 'active' ? '展示中' : '已停用'}</span></td>
              <td className="whitespace-nowrap px-4 py-3 text-[10px] text-zinc-400">{formatDate(item.updatedAt)}</td>
              <td className="px-4 py-3">{rowActions(item)}</td>
            </tr>
          )}
          renderMobileItem={(item) => (
            <article key={item.id} className="rounded-md border border-[#DCE4DF] bg-white p-3.5">
              <div className="flex items-start justify-between gap-3"><div className="min-w-0"><strong className="block truncate text-sm">{item.title}</strong><p className="mt-1 line-clamp-2 text-[11px] leading-5 text-zinc-500">{item.content}</p></div><span className={`status-pill ${item.status}`}>{item.status === 'active' ? '展示中' : '已停用'}</span></div>
              <div className="mt-3 flex items-center gap-2 border-y border-[#EDF0EE] py-2 text-[10px] text-zinc-500"><span>{displayLabel(item.displayMode)}</span><span>·</span><span>{item.targetType === 'all' ? '全部用户' : `${item.userIds.length} 位用户`}</span><span>·</span><span>排序 {item.sortOrder}</span></div>
              <div className="mt-2 flex items-center justify-between"><small className="text-[10px] text-zinc-400">{formatDate(item.updatedAt)}</small>{rowActions(item)}</div>
            </article>
          )}
        />
      )}

      {editorOpen && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/40 p-4 sm:grid sm:place-items-center">
          <form onSubmit={save} className="mx-auto w-full max-w-2xl overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5"><div><h2 className="text-sm font-semibold">{editing ? '编辑公告' : '新增公告'}</h2><p className="mt-0.5 text-[11px] text-zinc-500">弹窗需用户确认，横幅会持续显示。</p></div><button type="button" onClick={() => setEditorOpen(false)} title="关闭" className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button></div>
            <div className="space-y-4 p-5">
              <label className="block"><span className="mb-1 block text-[11px] font-semibold text-zinc-500">公告标题</span><input required maxLength={120} value={draft.title} onChange={(event) => updateDraft('title', event.target.value)} placeholder="例如：服务维护通知" className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
              <label className="block"><span className="mb-1 flex items-center justify-between text-[11px] font-semibold text-zinc-500"><span>公告内容</span><small className="font-normal text-zinc-400">支持换行</small></span><textarea required rows={6} value={draft.content} onChange={(event) => updateDraft('content', event.target.value)} placeholder="输入需要通知用户的具体内容" className="w-full resize-y rounded-md border border-[#DCE4DF] px-3 py-2 text-xs leading-6 outline-none focus:border-[#12B76A]" /></label>
              <div className="grid grid-cols-1 gap-4 border-t border-[#EDF0EE] pt-4 sm:grid-cols-3">
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">展示方式</span><AppSelect value={draft.displayMode} onValueChange={(value) => updateDraft('displayMode', value as Announcement['displayMode'])} ariaLabel="公告展示方式" options={MODE_OPTIONS.slice(1)} /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">公告状态</span><AppSelect value={draft.status} onValueChange={(value) => { const nextStatus = value as Announcement['status']; updateDraft('status', nextStatus); if (nextStatus !== 'active') setSendEmail(false); }} ariaLabel="公告状态" options={STATUS_OPTIONS.slice(1)} /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">排序值</span><input type="number" value={draft.sortOrder} onChange={(event) => updateDraft('sortOrder', Number(event.target.value))} className="h-9 w-full rounded-md border border-[#DCE4DF] px-3 font-mono text-xs outline-none focus:border-[#12B76A]" /></label>
              </div>
              <section className="border-t border-[#EDF0EE] pt-4">
                <div className="grid grid-cols-2 gap-2">
                  <button type="button" onClick={() => updateDraft('targetType', 'all')} className={`flex items-center gap-2 rounded-md border px-3 py-2.5 text-left text-xs ${draft.targetType === 'all' ? 'border-emerald-300 bg-emerald-50 text-emerald-800' : 'border-[#DCE4DF] bg-white text-zinc-600'}`}><span className="grid h-6 w-6 place-items-center rounded bg-white"><Megaphone className="h-3.5 w-3.5" /></span><span><strong className="block">全部用户</strong><small className="text-[10px] opacity-70">所有登录用户可见</small></span></button>
                  <button type="button" onClick={() => updateDraft('targetType', 'users')} className={`flex items-center gap-2 rounded-md border px-3 py-2.5 text-left text-xs ${draft.targetType === 'users' ? 'border-emerald-300 bg-emerald-50 text-emerald-800' : 'border-[#DCE4DF] bg-white text-zinc-600'}`}><span className="grid h-6 w-6 place-items-center rounded bg-white"><Users className="h-3.5 w-3.5" /></span><span><strong className="block">指定用户</strong><small className="text-[10px] opacity-70">仅所选用户可见</small></span></button>
                </div>
                {draft.targetType === 'users' && (
                  <div className="mt-3 overflow-hidden rounded-md border border-[#DCE4DF]">
                    <label className="relative block border-b border-[#EDF0EE] bg-[#FAFBFA] p-2"><Search className="absolute left-4 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-zinc-400" /><input value={userSearch} onChange={(event) => setUserSearch(event.target.value)} placeholder="搜索用户邮箱或 ID" className="h-8 w-full rounded-md border border-[#DCE4DF] bg-white pl-8 pr-3 text-[11px] outline-none focus:border-[#12B76A]" /></label>
                    <div className="flex items-center justify-between border-b border-[#EDF0EE] px-3 py-2 text-[10px] text-zinc-400"><span>已选择 {draft.userIds.length} 位用户</span>{draft.userIds.length > 0 && <button type="button" onClick={() => updateDraft('userIds', [])} className="font-semibold text-[#047857]">清空选择</button>}</div>
                    <div className="max-h-48 overflow-y-auto p-2">
                      {visibleUsers.map((user) => {
                        const selected = draft.userIds.includes(user.id);
                        return <button key={user.id} type="button" onClick={() => toggleUser(user.id)} className={`grid w-full grid-cols-[24px_minmax(0,1fr)] items-center gap-2 rounded px-2 py-2 text-left hover:bg-[#F6F8F6] ${selected ? 'bg-emerald-50' : ''}`}><span className={`grid h-5 w-5 place-items-center rounded border ${selected ? 'border-emerald-500 bg-emerald-600 text-white' : 'border-[#CBD5CF] bg-white text-transparent'}`}><Check className="h-3 w-3" /></span><span className="min-w-0"><strong className="block truncate text-[11px] font-medium">{user.email}</strong><small className="block truncate font-mono text-[9px] text-zinc-400">{user.id}</small></span></button>;
                      })}
                      {!visibleUsers.length && <p className="py-8 text-center text-[11px] text-zinc-400">没有匹配的用户</p>}
                    </div>
                  </div>
                )}
              </section>
              <label className={`flex items-start gap-3 rounded-md border px-3.5 py-3 ${draft.status === 'active' ? 'border-[#CFE7D9] bg-[#F5FBF7]' : 'border-[#E3E7E4] bg-[#F8F9F8] opacity-60'}`}>
                <input type="checkbox" checked={sendEmail} disabled={draft.status !== 'active'} onChange={(event) => setSendEmail(event.target.checked)} className="mt-0.5 h-4 w-4 accent-[#047857]" />
                <span className="min-w-0"><span className="flex items-center gap-1.5 text-[11px] font-semibold text-zinc-700"><Mail className="h-3.5 w-3.5 text-[#047857]" />同步发送邮件</span><small className="mt-1 block text-[10px] leading-4 text-zinc-500">本次保存后，按上方接收范围发送同标题、同正文的邮件；以后编辑不会自动重复发送。</small></span>
              </label>
            </div>
            <div className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><button type="button" onClick={() => setEditorOpen(false)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button><button type="submit" disabled={saving} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}{editing ? '保存修改' : '发布公告'}</button></div>
          </form>
        </div>
      )}

      <ConfirmDialog isOpen={Boolean(deleteCandidate)} onClose={() => setDeleteCandidate(null)} onConfirm={() => void remove()} title="删除公告" description={`确定删除「${deleteCandidate?.title || '该公告'}」吗？相关确认记录也会一并删除。`} confirmText="删除" type="danger" />
    </div>
  );
}
