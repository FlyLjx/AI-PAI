'use client';

import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';
import {
  CalendarClock,
  Check,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Clock3,
  ExternalLink,
  Link2,
  Loader2,
  Mail,
  MailPlus,
  RefreshCw,
  Search,
  Send,
  UserCheck,
  Users,
  X,
  XCircle,
} from 'lucide-react';
import { toast } from 'sonner';
import { AppSelect } from '@/components/common/AppSelect';
import { PageHeader } from '@/components/common/PageHeader';
import { formatDate } from '@/lib/common/utils';
import {
  portalApi,
  type MailBroadcastInput,
  type MailDeliveryLog,
  type MailDeliverySummary,
  type PortalUser,
} from '@/lib/admin-api';

const PAGE_SIZE = 30;

const STATUS_OPTIONS = [
  { value: 'all', label: '全部状态' },
  { value: 'sent', label: '发送成功' },
  { value: 'failed', label: '发送失败' },
  { value: 'sending', label: '发送中' },
];

const CATEGORY_OPTIONS = [
  { value: 'all', label: '全部分类' },
  { value: 'recharge_success', label: '充值到账' },
  { value: 'upstream_alert', label: '上游异常' },
  { value: 'upstream_recovery', label: '上游恢复' },
  { value: 'email_verification', label: '邮箱验证' },
  { value: 'email_change', label: '修改邮箱' },
  { value: 'password_reset', label: '密码重置' },
  { value: 'balance_reminder', label: '余额提醒' },
  { value: 'subscription_expiry', label: '订阅到期' },
  { value: 'announcement', label: '公告邮件' },
  { value: 'broadcast', label: '群发邮件' },
  { value: 'smtp_test', label: 'SMTP 测试' },
];

const emptySummary: MailDeliverySummary = { total: 0, sent: 0, failed: 0, sending: 0, today: 0 };

const emptyBroadcast: MailBroadcastInput = {
  subject: '',
  content: '',
  actionText: '',
  actionUrl: '',
  targetType: 'active',
  userIds: [],
};

function statusView(status: string) {
  if (status === 'sent') return { label: '成功', icon: CheckCircle2, className: 'border-emerald-200 bg-emerald-50 text-emerald-700' };
  if (status === 'failed') return { label: '失败', icon: XCircle, className: 'border-red-200 bg-red-50 text-red-700' };
  return { label: '发送中', icon: Clock3, className: 'border-amber-200 bg-amber-50 text-amber-700' };
}

function categoryLabel(category: string) {
  return CATEGORY_OPTIONS.find((item) => item.value === category)?.label || category || '系统邮件';
}

export default function AdminMailLogsPage() {
  const [items, setItems] = useState<MailDeliveryLog[]>([]);
  const [summary, setSummary] = useState<MailDeliverySummary>(emptySummary);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [status, setStatus] = useState('all');
  const [category, setCategory] = useState('all');
  const [searchInput, setSearchInput] = useState('');
  const [keyword, setKeyword] = useState('');
  const [selectedId, setSelectedId] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [composerOpen, setComposerOpen] = useState(false);
  const [sending, setSending] = useState(false);
  const [usersLoading, setUsersLoading] = useState(false);
  const [users, setUsers] = useState<PortalUser[]>([]);
  const [broadcast, setBroadcast] = useState<MailBroadcastInput>(emptyBroadcast);
  const [userSearch, setUserSearch] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const response = await portalApi.adminMailLogs({
        page,
        pageSize: PAGE_SIZE,
        keyword: keyword || undefined,
        status: status === 'all' ? undefined : status,
        category: category === 'all' ? undefined : category,
      });
      const nextItems = response.data.items || [];
      setItems(nextItems);
      setSummary(response.data.summary || emptySummary);
      setTotal(Number(response.pagination?.total || response.data.summary?.total || 0));
      setSelectedId((current) => nextItems.some((item) => item.id === current) ? current : nextItems[0]?.id || '');
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '邮件记录加载失败');
    } finally {
      setLoading(false);
    }
  }, [category, keyword, page, status]);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  const selected = useMemo(() => items.find((item) => item.id === selectedId) || null, [items, selectedId]);
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const visibleUsers = useMemo(() => {
    const query = userSearch.trim().toLowerCase();
    return users.filter((user) => !query || `${user.email} ${user.id}`.toLowerCase().includes(query));
  }, [userSearch, users]);
  const recipientCount = broadcast.targetType === 'specific'
    ? broadcast.userIds.length
    : broadcast.targetType === 'active'
      ? users.filter((user) => user.status === 'active').length
      : users.length;

  const submitSearch = (event: FormEvent) => {
    event.preventDefault();
    setPage(1);
    setKeyword(searchInput.trim());
  };

  const openComposer = async () => {
    setBroadcast({ ...emptyBroadcast, userIds: [] });
    setUserSearch('');
    setComposerOpen(true);
    if (users.length > 0) return;
    setUsersLoading(true);
    try {
      const response = await portalApi.users();
      setUsers((response.data || []).filter((user) => user.role === 'user'));
    } catch (loadError) {
      toast.error(loadError instanceof Error ? loadError.message : '用户列表加载失败');
    } finally {
      setUsersLoading(false);
    }
  };

  const updateBroadcast = <K extends keyof MailBroadcastInput>(key: K, value: MailBroadcastInput[K]) => {
    setBroadcast((current) => ({ ...current, [key]: value }));
  };

  const toggleRecipient = (userId: string) => {
    setBroadcast((current) => ({
      ...current,
      userIds: current.userIds.includes(userId)
        ? current.userIds.filter((id) => id !== userId)
        : [...current.userIds, userId],
    }));
  };

  const sendBroadcast = async (event: FormEvent) => {
    event.preventDefault();
    if (broadcast.targetType === 'specific' && broadcast.userIds.length === 0) {
      toast.error('请选择至少一个收件用户');
      return;
    }
    if (Boolean(broadcast.actionText?.trim()) !== Boolean(broadcast.actionUrl?.trim())) {
      toast.error('邮件按钮文字和链接需要同时填写');
      return;
    }
    setSending(true);
    try {
      const response = await portalApi.sendMailBroadcast({
        ...broadcast,
        subject: broadcast.subject.trim(),
        content: broadcast.content.trim(),
        actionText: broadcast.actionText?.trim(),
        actionUrl: broadcast.actionUrl?.trim(),
        userIds: broadcast.targetType === 'specific' ? broadcast.userIds : [],
      });
      const result = response.data;
      if (result.failed > 0) toast.warning(`邮件成功 ${result.success} 封，失败 ${result.failed} 封`);
      else toast.success(`已发送 ${result.success} 封邮件`);
      setComposerOpen(false);
      setPage(1);
      await load();
    } catch (sendError) {
      toast.error(sendError instanceof Error ? sendError.message : '邮件发送失败');
    } finally {
      setSending(false);
    }
  };

  const metrics = [
    { label: '筛选结果', value: summary.total, note: '当前条件', icon: Mail, tone: 'bg-zinc-100 text-zinc-600' },
    { label: '发送成功', value: summary.sent, note: '已投递', icon: CheckCircle2, tone: 'bg-emerald-50 text-emerald-700' },
    { label: '发送失败', value: summary.failed, note: '需排查', icon: XCircle, tone: 'bg-red-50 text-red-700' },
    { label: '今日邮件', value: summary.today, note: summary.sending ? `${summary.sending} 封发送中` : '今日触发', icon: CalendarClock, tone: 'bg-blue-50 text-blue-700' },
  ];

  return (
    <div className="space-y-5">
      <PageHeader title="邮件记录" description="查询系统邮件的收发信息、正文内容和投递结果。">
        <button type="button" onClick={() => void load()} disabled={loading} title="刷新邮件记录" className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white hover:border-[#12B76A] disabled:opacity-50">
          <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
        <button type="button" onClick={() => void openComposer()} className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[#047857] px-3 text-xs font-semibold text-white hover:bg-[#036B4F]"><MailPlus className="h-4 w-4" />发送邮件</button>
      </PageHeader>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {metrics.map((metric) => {
          const Icon = metric.icon;
          return (
            <div key={metric.label} className="flex items-center gap-3 rounded-md border border-[#DCE4DF] bg-white p-4">
              <span className={`grid h-9 w-9 shrink-0 place-items-center rounded-md ${metric.tone}`}><Icon className="h-4 w-4" /></span>
              <span className="min-w-0"><small className="block text-[10px] text-zinc-400">{metric.label}</small><strong className="mt-0.5 block font-mono text-lg">{metric.value.toLocaleString()}</strong><small className="block truncate text-[9px] text-zinc-400">{metric.note}</small></span>
            </div>
          );
        })}
      </div>

      {error && <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void load()} className="font-semibold underline">重试</button></div>}

      <section className="overflow-hidden rounded-md border border-[#DCE4DF] bg-white">
        <div className="flex flex-col gap-3 border-b border-[#DCE4DF] bg-[#FAFBFA] p-3 sm:flex-row sm:items-center sm:justify-between">
          <form onSubmit={submitSearch} className="flex min-w-0 flex-1 gap-2 sm:max-w-md">
            <label className="relative min-w-0 flex-1">
              <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400" />
              <input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索收件人、主题或正文" className="h-8 w-full rounded-md border border-[#DCE4DF] bg-white pl-9 pr-3 text-xs outline-none" />
            </label>
            <button type="submit" title="搜索" className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-[#047857] text-white hover:bg-[#036B4F]"><Search className="h-4 w-4" /></button>
          </form>
          <div className="flex flex-wrap items-center gap-2">
            <AppSelect compact value={category} options={CATEGORY_OPTIONS} onValueChange={(value) => { setCategory(value); setPage(1); }} ariaLabel="筛选邮件分类" />
            <AppSelect compact value={status} options={STATUS_OPTIONS} onValueChange={(value) => { setStatus(value); setPage(1); }} ariaLabel="筛选邮件状态" />
          </div>
        </div>

        {loading && !items.length ? (
          <div className="grid min-h-[460px] place-items-center"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
        ) : !items.length ? (
          <div className="grid min-h-[360px] place-items-center text-center text-zinc-400"><span><Mail className="mx-auto mb-2 h-8 w-8" /><small>暂无匹配的邮件记录</small></span></div>
        ) : (
          <div className="grid min-h-[560px] grid-cols-1 xl:grid-cols-[minmax(0,1.35fr)_minmax(340px,.65fr)]">
            <div className="min-w-0 border-b border-[#DCE4DF] xl:border-b-0 xl:border-r">
              <div className="max-h-[620px] overflow-auto">
                <table className="w-full min-w-[720px] text-left text-xs">
                  <thead className="sticky top-0 z-10 bg-[#F7F8F6] text-[10px] text-zinc-500">
                    <tr><th className="px-4 py-3">主题</th><th className="px-4 py-3">收件人</th><th className="px-4 py-3">分类</th><th className="px-4 py-3">状态</th><th className="px-4 py-3">时间</th></tr>
                  </thead>
                  <tbody className="divide-y divide-[#EDF0EE]">
                    {items.map((item) => {
                      const state = statusView(item.status);
                      const StateIcon = state.icon;
                      const active = item.id === selectedId;
                      return (
                        <tr key={item.id} onClick={() => setSelectedId(item.id)} className={`cursor-pointer ${active ? 'bg-[#F0FDF4]' : 'hover:bg-[#FAFBFA]'}`}>
                          <td className="max-w-[280px] px-4 py-3"><strong className="block truncate text-[11px]">{item.subject}</strong><small className="mt-0.5 block truncate font-mono text-[9px] text-zinc-400">{item.id}</small></td>
                          <td className="max-w-[210px] px-4 py-3"><span className="block truncate text-[10px]" title={item.recipient}>{item.recipient}</span></td>
                          <td className="whitespace-nowrap px-4 py-3 text-[10px] text-zinc-600">{categoryLabel(item.category)}</td>
                          <td className="px-4 py-3"><span className={`inline-flex items-center gap-1 rounded border px-1.5 py-0.5 text-[10px] font-semibold ${state.className}`}><StateIcon className="h-3 w-3" />{state.label}</span></td>
                          <td className="whitespace-nowrap px-4 py-3 text-[10px] text-zinc-500">{formatDate(item.sentAt || item.createdAt)}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
              <footer className="flex items-center justify-between border-t border-[#EDF0EE] bg-[#FAFBFA] px-4 py-3">
                <span className="text-[10px] text-zinc-400">第 {page} / {totalPages} 页，共 {total} 条</span>
                <div className="flex gap-2">
                  <button type="button" onClick={() => setPage((value) => Math.max(1, value - 1))} disabled={page <= 1 || loading} title="上一页" className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white disabled:opacity-40"><ChevronLeft className="h-4 w-4" /></button>
                  <button type="button" onClick={() => setPage((value) => value + 1)} disabled={page >= totalPages || loading} title="下一页" className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white disabled:opacity-40"><ChevronRight className="h-4 w-4" /></button>
                </div>
              </footer>
            </div>

            <aside className="min-w-0 bg-[#FCFDFC]">
              {selected ? (() => {
                const state = statusView(selected.status);
                const StateIcon = state.icon;
                return (
                  <div className="p-5">
                    <div className="flex items-start justify-between gap-3 border-b border-[#DCE4DF] pb-4">
                      <div className="min-w-0"><small className="text-[10px] font-semibold text-[#047857]">{categoryLabel(selected.category)}</small><h2 className="mt-1 text-sm font-bold leading-5">{selected.subject}</h2></div>
                      <span className={`inline-flex shrink-0 items-center gap-1 rounded border px-1.5 py-0.5 text-[10px] font-semibold ${state.className}`}><StateIcon className="h-3 w-3" />{state.label}</span>
                    </div>
                    <dl className="grid gap-2 border-b border-[#DCE4DF] py-4 text-[10px]">
                      <div className="grid grid-cols-[54px_minmax(0,1fr)] gap-2"><dt className="text-zinc-400">发件人</dt><dd className="truncate font-mono" title={selected.fromAddress}>{selected.fromAddress || '-'}</dd></div>
                      <div className="grid grid-cols-[54px_minmax(0,1fr)] gap-2"><dt className="text-zinc-400">收件人</dt><dd className="truncate font-mono" title={selected.recipient}>{selected.recipient}</dd></div>
                      <div className="grid grid-cols-[54px_minmax(0,1fr)] gap-2"><dt className="text-zinc-400">创建</dt><dd>{formatDate(selected.createdAt)}</dd></div>
                      <div className="grid grid-cols-[54px_minmax(0,1fr)] gap-2"><dt className="text-zinc-400">发送</dt><dd>{selected.sentAt ? formatDate(selected.sentAt) : '-'}</dd></div>
                    </dl>
                    {selected.errorMessage && <div className="mt-4 rounded-md border border-red-200 bg-red-50 p-3 text-[11px] leading-5 text-red-700"><strong className="mb-1 block">失败原因</strong>{selected.errorMessage}</div>}
                    <div className="mt-4"><span className="text-[10px] font-semibold text-zinc-400">邮件正文</span><div className="mt-2 max-h-[330px] overflow-auto whitespace-pre-wrap break-words rounded-md border border-[#E6EBE8] bg-white p-4 text-[11px] leading-6 text-zinc-700">{selected.content}</div></div>
                    {selected.actionUrl && <a href={selected.actionUrl} target="_blank" rel="noreferrer" className="mt-4 inline-flex items-center gap-1.5 text-[11px] font-semibold text-[#047857] hover:underline"><ExternalLink className="h-3.5 w-3.5" />打开邮件操作链接</a>}
                  </div>
                );
              })() : <div className="grid min-h-[420px] place-items-center text-zinc-400"><span className="text-center"><Send className="mx-auto mb-2 h-7 w-7" /><small>选择一封邮件查看详情</small></span></div>}
            </aside>
          </div>
        )}
      </section>

      {composerOpen && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/40 p-4 sm:grid sm:place-items-center">
          <form onSubmit={sendBroadcast} className="mx-auto w-full max-w-3xl overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5">
              <div><h2 className="flex items-center gap-2 text-sm font-semibold"><MailPlus className="h-4 w-4 text-[#047857]" />发送邮件</h2><p className="mt-0.5 text-[11px] text-zinc-500">邮件会使用系统设置中的 SMTP 服务。</p></div>
              <button type="button" onClick={() => setComposerOpen(false)} disabled={sending} title="关闭" className="rounded p-1 text-zinc-500 hover:bg-zinc-100 disabled:opacity-40"><X className="h-4 w-4" /></button>
            </div>
            <div className="space-y-4 p-5">
              <section>
                <span className="mb-2 block text-[11px] font-semibold text-zinc-500">收件范围</span>
                <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
                  {([
                    { value: 'active', label: '启用用户', note: '仅账户状态正常', icon: UserCheck },
                    { value: 'all', label: '全部用户', note: '包含停用账户', icon: Users },
                    { value: 'specific', label: '指定用户', note: '从用户列表选择', icon: CheckCircle2 },
                  ] as const).map((option) => {
                    const Icon = option.icon;
                    const active = broadcast.targetType === option.value;
                    return <button key={option.value} type="button" onClick={() => updateBroadcast('targetType', option.value)} className={`flex items-center gap-2.5 rounded-md border px-3 py-2.5 text-left ${active ? 'border-emerald-300 bg-emerald-50 text-emerald-800' : 'border-[#DCE4DF] text-zinc-600 hover:bg-[#FAFBFA]'}`}><span className="grid h-7 w-7 shrink-0 place-items-center rounded bg-white"><Icon className="h-3.5 w-3.5" /></span><span><strong className="block text-[11px]">{option.label}</strong><small className="text-[9px] opacity-70">{option.note}</small></span></button>;
                  })}
                </div>
                <div className="mt-2 text-right text-[10px] text-zinc-400">预计收件人 {recipientCount} 位</div>
              </section>

              {broadcast.targetType === 'specific' && (
                <section className="overflow-hidden rounded-md border border-[#DCE4DF]">
                  <label className="relative block border-b border-[#EDF0EE] bg-[#FAFBFA] p-2"><Search className="absolute left-4 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-zinc-400" /><input value={userSearch} onChange={(event) => setUserSearch(event.target.value)} placeholder="搜索用户邮箱或 ID" className="h-8 w-full rounded-md border border-[#DCE4DF] bg-white pl-8 pr-3 text-[11px] outline-none focus:border-[#12B76A]" /></label>
                  <div className="flex items-center justify-between border-b border-[#EDF0EE] px-3 py-2 text-[10px] text-zinc-400"><span>已选择 {broadcast.userIds.length} 位用户</span>{broadcast.userIds.length > 0 && <button type="button" onClick={() => updateBroadcast('userIds', [])} className="font-semibold text-[#047857]">清空选择</button>}</div>
                  <div className="max-h-44 overflow-y-auto p-2">
                    {usersLoading ? <div className="grid h-24 place-items-center"><Loader2 className="h-5 w-5 animate-spin text-[#12B76A]" /></div> : visibleUsers.map((user) => {
                      const selected = broadcast.userIds.includes(user.id);
                      return <button key={user.id} type="button" onClick={() => toggleRecipient(user.id)} className={`grid w-full grid-cols-[24px_minmax(0,1fr)_auto] items-center gap-2 rounded px-2 py-2 text-left hover:bg-[#F6F8F6] ${selected ? 'bg-emerald-50' : ''}`}><span className={`grid h-5 w-5 place-items-center rounded border ${selected ? 'border-emerald-500 bg-emerald-600 text-white' : 'border-[#CBD5CF] bg-white text-transparent'}`}><Check className="h-3 w-3" /></span><span className="min-w-0"><strong className="block truncate text-[11px] font-medium">{user.email}</strong><small className="block truncate font-mono text-[9px] text-zinc-400">{user.id}</small></span><span className={`rounded border px-1.5 py-0.5 text-[9px] ${user.status === 'active' ? 'border-emerald-200 text-emerald-700' : 'border-zinc-200 text-zinc-500'}`}>{user.status === 'active' ? '启用' : '停用'}</span></button>;
                    })}
                    {!usersLoading && !visibleUsers.length && <p className="py-8 text-center text-[11px] text-zinc-400">没有匹配的用户</p>}
                  </div>
                </section>
              )}

              <label className="block"><span className="mb-1 block text-[11px] font-semibold text-zinc-500">邮件标题</span><input required maxLength={255} value={broadcast.subject} onChange={(event) => updateBroadcast('subject', event.target.value)} placeholder="输入邮件标题" className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 text-xs outline-none focus:border-[#12B76A]" /></label>
              <label className="block"><span className="mb-1 flex items-center justify-between text-[11px] font-semibold text-zinc-500"><span>邮件正文</span><small className="font-normal text-zinc-400">支持换行</small></span><textarea required maxLength={50000} rows={7} value={broadcast.content} onChange={(event) => updateBroadcast('content', event.target.value)} placeholder="输入邮件正文" className="w-full resize-y rounded-md border border-[#DCE4DF] px-3 py-2 text-xs leading-6 outline-none focus:border-[#12B76A]" /></label>
              <section className="grid grid-cols-1 gap-3 border-t border-[#EDF0EE] pt-4 sm:grid-cols-[180px_minmax(0,1fr)]">
                <label><span className="mb-1 flex items-center gap-1.5 text-[11px] font-semibold text-zinc-500"><Link2 className="h-3.5 w-3.5" />按钮文字</span><input value={broadcast.actionText} onChange={(event) => updateBroadcast('actionText', event.target.value)} placeholder="例如：查看详情" className="h-9 w-full rounded-md border border-[#DCE4DF] px-3 text-xs outline-none focus:border-[#12B76A]" /></label>
                <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">按钮链接</span><input type="url" value={broadcast.actionUrl} onChange={(event) => updateBroadcast('actionUrl', event.target.value)} placeholder="https://ai.yccc.me/dashboard" className="h-9 w-full rounded-md border border-[#DCE4DF] px-3 font-mono text-xs outline-none focus:border-[#12B76A]" /></label>
              </section>
            </div>
            <div className="flex items-center justify-between gap-3 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><span className="text-[10px] text-zinc-400">发送结果将写入邮件记录</span><div className="flex gap-2"><button type="button" onClick={() => setComposerOpen(false)} disabled={sending} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold disabled:opacity-50">取消</button><button type="submit" disabled={sending || usersLoading || recipientCount === 0} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{sending ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Send className="h-3.5 w-3.5" />}{sending ? '发送中' : '确认发送'}</button></div></div>
          </form>
        </div>
      )}
    </div>
  );
}
