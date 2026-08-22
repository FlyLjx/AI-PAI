'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Check,
  ChevronLeft,
  ChevronRight,
  CircleDollarSign,
  CreditCard,
  Eye,
  Gauge,
  Loader2,
  MailCheck,
  PackageCheck,
  Pencil,
  Plus,
  RefreshCw,
  ReceiptText,
  Search,
  Trophy,
  ShieldCheck,
  Trash2,
  UserRoundCog,
  X,
} from 'lucide-react';
import { toast } from 'sonner';
import { AppSelect } from '@/components/common/AppSelect';
import { ConfirmDialog } from '@/components/common/ConfirmDialog';
import { DataTable, SortableHeader, sortItems, type SortState, type TableHeader } from '@/components/common/DataTable';
import { EmptyState } from '@/components/common/EmptyState';
import { PageHeader } from '@/components/common/PageHeader';
import { type ConsumptionRank, type CreditLog, type Plan, type PortalUser, portalApi } from '@/lib/admin-api';
import { formatCNY, formatDate } from '@/lib/common/utils';

type UserDraft = {
  email: string;
  password: string;
  role: 'user' | 'admin';
  status: 'active' | 'disabled';
  syncSize: boolean;
};

type GrantMode = 'plan' | 'custom';
type QuotaAdjustMode = 'remaining' | 'reset';
type CreditLogFilter = 'all' | 'deduct' | 'recharge' | 'manual_adjust' | 'invite_reward' | 'invite_rebate';

const emptyDraft: UserDraft = { email: '', password: '', role: 'user', status: 'active', syncSize: false };
const pageSize = 12;
const creditLogPageSize = 10;

function normalizeUserIDs(value: unknown): string[] {
  const values = Array.isArray(value) ? value : String(value || '').split(/[;,\n\r]/);
  return Array.from(new Set(values.map((item) => String(item).trim()).filter(Boolean)));
}

function sameUserIDs(left: string[], right: string[]) {
  return [...new Set(left)].sort().join(',') === [...new Set(right)].sort().join(',');
}

function SelectionCheckbox({
  checked,
  indeterminate = false,
  disabled = false,
  onChange,
  ariaLabel,
  className = '',
}: {
  checked: boolean;
  indeterminate?: boolean;
  disabled?: boolean;
  onChange: () => void;
  ariaLabel: string;
  className?: string;
}) {
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (inputRef.current) inputRef.current.indeterminate = indeterminate;
  }, [indeterminate]);

  return <input ref={inputRef} type="checkbox" checked={checked} disabled={disabled} onChange={onChange} aria-label={ariaLabel} className={`user-selection-checkbox ${className}`} />;
}

const consumptionHeaders: TableHeader<ConsumptionRank>[] = [
  { key: 'user', label: '客户', sortValue: (item) => item.userEmail || item.userId },
  { key: 'creditsSpent', label: '消费金额', sortValue: (item) => Number(item.creditsSpent || 0) },
  { key: 'deductCount', label: '扣费次数', sortValue: (item) => Number(item.deductCount || 0) },
  { key: 'lastDeductAt', label: '最近扣费', sortValue: (item) => Date.parse(item.lastDeductAt || '') || 0 },
];

function creditLogChange(log: CreditLog) {
  const amount = Number(log.amount || 0);
  return log.type === 'deduct' ? -Math.abs(amount) : amount;
}

function creditLogView(log: CreditLog) {
  const change = creditLogChange(log);
  if (log.type === 'deduct') return { label: 'API 消费', tone: 'border-red-200 bg-red-50 text-red-700', amountTone: 'text-red-600', change };
  if (log.type === 'recharge') return { label: '余额充值', tone: 'border-emerald-200 bg-emerald-50 text-emerald-700', amountTone: 'text-emerald-700', change };
  if (log.type === 'invite_reward') return { label: '邀请奖励', tone: 'border-cyan-200 bg-cyan-50 text-cyan-700', amountTone: 'text-cyan-700', change };
  if (log.type === 'invite_rebate') return { label: '充值返利', tone: 'border-amber-200 bg-amber-50 text-amber-700', amountTone: 'text-amber-700', change };
  if (log.type === 'manual_adjust') return change < 0
    ? { label: '后台扣减', tone: 'border-amber-200 bg-amber-50 text-amber-700', amountTone: 'text-amber-700', change }
    : { label: '后台增加', tone: 'border-blue-200 bg-blue-50 text-blue-700', amountTone: 'text-blue-700', change };
  return { label: log.type || '其他变动', tone: 'border-zinc-200 bg-zinc-50 text-zinc-600', amountTone: change < 0 ? 'text-red-600' : 'text-emerald-700', change };
}

function creditLogRemark(value?: string) {
  const remark = value?.trim() || '-';
  const match = /^管理员\s+([^：:]+)[：:]\s*(.*)$/u.exec(remark);
  if (!match) return remark;
  const actor = match[1].trim();
  if (actor.includes('@')) return remark;
  const detail = match[2].trim();
  return detail ? `系统管理员：${detail}` : '系统管理员';
}

function subscriptionActive(user: PortalUser) {
  return user.subscription?.status === 'active';
}

function subscriptionName(user: PortalUser) {
  if (!subscriptionActive(user)) return '按余额计费';
  return user.subscription?.planName || user.subscription?.tier || '订阅套餐';
}

function userInitials(email?: string) {
  const localPart = String(email || '?').split('@')[0].trim();
  return localPart.slice(0, 2).toUpperCase() || '?';
}

function UserStatusBadges({ user }: { user: PortalUser }) {
  return (
    <div className="mt-2 flex flex-nowrap items-center gap-1.5 overflow-x-auto text-[10px]">
      <span className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 font-semibold ${user.status === 'active' ? 'border-[#BDE8CC] bg-[#F0FBF4] text-[#087443]' : 'border-[#E2E8E4] bg-[#F6F8F6] text-[#6B756F]'}`}>
        <i className={`h-1.5 w-1.5 rounded-full ${user.status === 'active' ? 'bg-[#18B969]' : 'bg-[#9AA49E]'}`} />
        {user.status === 'active' ? '已启用' : '已停用'}
      </span>
      {user.activeLast30Days && (
        <span className="inline-flex items-center gap-1 rounded-full border border-[#C6F0D5] bg-[#F5FCF7] px-2 py-0.5 font-semibold text-[#12814E]">
          <span className="h-1.5 w-1.5 rounded-full bg-[#18B969]" />近30天活跃
        </span>
      )}
      <span className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 ${user.emailVerifiedAt ? 'border-[#E2EDE6] bg-[#FBFDFC] text-[#6B756F]' : 'border-amber-200 bg-amber-50 text-amber-700'}`} title={user.emailVerifiedAt ? `验证时间：${formatDate(user.emailVerifiedAt)}` : undefined}>
        <span className={`h-1.5 w-1.5 rounded-full ${user.emailVerifiedAt ? 'bg-[#9AA49E]' : 'bg-amber-500'}`} />
        {user.emailVerifiedAt ? '邮箱已验证' : '邮箱未验证'}
      </span>
    </div>
  );
}

function UserIdentity({ user }: { user: PortalUser }) {
  return (
    <div className="flex min-w-0 items-center gap-3">
      <span className="grid h-9 w-9 shrink-0 place-items-center rounded-xl border border-[#C9F0D7] bg-[#ECFAF1] text-[11px] font-extrabold tracking-wide text-[#087443]">
        {userInitials(user.email)}
      </span>
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <strong className="truncate text-[13px] font-semibold text-[#17201B]">{user.email}</strong>
          {user.role === 'admin' && <span className="shrink-0 rounded-full border border-zinc-200 bg-zinc-50 px-2 py-0.5 text-[9px] font-semibold text-zinc-500">管理员</span>}
        </div>
        <UserStatusBadges user={user} />
      </div>
    </div>
  );
}

function BillingSummary({ user }: { user: PortalUser }) {
  if (!subscriptionActive(user)) {
    const status = user.subscription?.status;
    const statusLabel = status === 'expired'
      ? '订阅已到期'
      : status === 'canceled' || status === 'cancelled'
        ? '订阅已取消'
        : '未开通订阅';
    return (
      <span className="inline-flex max-w-full min-w-[150px] items-center gap-2 overflow-x-auto whitespace-nowrap rounded-lg border border-[#E2EEE6] bg-[#FBFDFC] px-2.5 py-2 leading-tight">
        <span className="inline-flex shrink-0 items-center gap-1.5 text-[11px] font-bold text-[#33443A]"><CircleDollarSign className="h-3.5 w-3.5 text-[#18B969]" />余额计费</span>
        <small className={`shrink-0 text-[10px] ${status === 'expired' ? 'text-amber-700' : status === 'canceled' || status === 'cancelled' ? 'text-red-600' : 'text-[#7A8A81]'}`}>· {statusLabel}</small>
      </span>
    );
  }
  const remaining = Number(user.subscription?.effectiveQuotaRemaining ?? user.subscription?.quotaRemaining ?? 0);
  const limit = Number(user.subscription?.quotaLimit ?? user.subscription?.quotaImages ?? 0);
  const usedPercent = limit > 0 ? Math.min(100, Math.max(0, ((limit - remaining) / limit) * 100)) : 0;
  const remainingPercent = limit > 0 ? Math.min(100, Math.max(0, (remaining / limit) * 100)) : 0;
  const progressTone = remainingPercent <= 10 ? 'bg-red-500' : remainingPercent <= 30 ? 'bg-amber-500' : 'bg-[#18B969]';
  return (
    <span className="inline-flex max-w-full min-w-[300px] items-center gap-2 overflow-x-auto whitespace-nowrap rounded-lg border border-[#D5F0DD] bg-[#F6FCF8] px-2.5 py-2 leading-tight">
      <span className="inline-flex shrink-0 items-center gap-1.5 text-[11px] font-bold text-[#244B35]">
        <i className="h-1.5 w-1.5 shrink-0 rounded-full bg-[#18B969]" />
        <span className="truncate">{subscriptionName(user)}</span>
      </span>
      <small className="shrink-0 font-mono text-[10px] text-[#71877A]">
        剩余 {remaining.toLocaleString('zh-CN')} / {limit.toLocaleString('zh-CN')} · 至 {formatDate(user.subscription?.expiresAt || '', false)}
      </small>
      {limit > 0 && (
        <span className="flex w-24 shrink-0 items-center gap-2" title={`已使用 ${Math.round(usedPercent)}%`}>
          <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-[#DDEEE3]">
            <span className={`block h-full rounded-full transition-[width] duration-300 ${progressTone}`} style={{ width: `${usedPercent}%` }} />
          </span>
          <small className="w-7 text-right font-mono text-[10px] text-[#71877A]">{Math.round(usedPercent)}%</small>
        </span>
      )}
    </span>
  );
}

function userStatusLabel(status?: string) {
  if (status === 'active') return '启用';
  if (status === 'disabled') return '停用';
  if (status === 'deleted') return '已删除';
  return status || '-';
}

export default function AdminUsersPage() {
  const [users, setUsers] = useState<PortalUser[]>([]);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [billingFilter, setBillingFilter] = useState('all');
  const [activityFilter, setActivityFilter] = useState('all');
  const [selectedUserIds, setSelectedUserIds] = useState<string[]>([]);
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [summaryStats, setSummaryStats] = useState({ total: 0, active: 0, verified: 0, subscribed: 0, activeLast30Days: 0 });
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<PortalUser | null>(null);
  const [draft, setDraft] = useState<UserDraft>(emptyDraft);
  const [grantUser, setGrantUser] = useState<PortalUser | null>(null);
  const [quotaUser, setQuotaUser] = useState<PortalUser | null>(null);
  const [quotaMode, setQuotaMode] = useState<QuotaAdjustMode>('remaining');
  const [quotaValue, setQuotaValue] = useState('');
  const [balanceUser, setBalanceUser] = useState<PortalUser | null>(null);
  const [balanceValue, setBalanceValue] = useState('');
  const [balanceRemark, setBalanceRemark] = useState('');
  const [creditLogUser, setCreditLogUser] = useState<PortalUser | null>(null);
  const [creditLogs, setCreditLogs] = useState<CreditLog[]>([]);
  const [creditLogFilter, setCreditLogFilter] = useState<CreditLogFilter>('all');
  const [creditLogPage, setCreditLogPage] = useState(1);
  const [creditLogTotal, setCreditLogTotal] = useState(0);
  const [creditLogSort, setCreditLogSort] = useState<SortState>({ key: 'createdAt', direction: 'desc' });
  const [creditLogLoading, setCreditLogLoading] = useState(false);
  const [creditLogError, setCreditLogError] = useState('');
  const [creditLogRefresh, setCreditLogRefresh] = useState(0);
  const [consumptionRanking, setConsumptionRanking] = useState<ConsumptionRank[]>([]);
  const [consumptionDays, setConsumptionDays] = useState(30);
  const [consumptionLoading, setConsumptionLoading] = useState(false);
  const [consumptionError, setConsumptionError] = useState('');
  const [consumptionSort, setConsumptionSort] = useState<SortState>({ key: 'creditsSpent', direction: 'desc' });
  const [rankingOpen, setRankingOpen] = useState(false);
  const rankingTriggerRef = useRef<HTMLButtonElement>(null);
  const [subscriptionVisibilityOpen, setSubscriptionVisibilityOpen] = useState(false);
  const [subscriptionVisibilityUsers, setSubscriptionVisibilityUsers] = useState<PortalUser[]>([]);
  const [subscriptionAccessUserIds, setSubscriptionAccessUserIds] = useState<string[]>([]);
  const [savedSubscriptionAccessUserIds, setSavedSubscriptionAccessUserIds] = useState<string[]>([]);
  const [subscriptionVisibilitySearch, setSubscriptionVisibilitySearch] = useState('');
  const [subscriptionVisibilityLoading, setSubscriptionVisibilityLoading] = useState(false);
  const [subscriptionVisibilitySaving, setSubscriptionVisibilitySaving] = useState(false);
  const [grantPlanId, setGrantPlanId] = useState('');
  const [grantMode, setGrantMode] = useState<GrantMode>('plan');
  const [customGrantName, setCustomGrantName] = useState('自定义订阅');
  const [customGrantDays, setCustomGrantDays] = useState(30);
  const [customGrantQuota, setCustomGrantQuota] = useState(100);
  const [deleteCandidate, setDeleteCandidate] = useState<PortalUser | null>(null);
  const [actionId, setActionId] = useState('');
  const [verifyingId, setVerifyingId] = useState('');
  const hasLoadedUsersRef = useRef(false);
  const usersRequestRef = useRef<AbortController | null>(null);
  const searching = searchInput.trim() !== search;
  const closeRanking = useCallback(() => {
    setRankingOpen(false);
    window.setTimeout(() => rankingTriggerRef.current?.focus(), 0);
  }, []);

  const openSubscriptionVisibility = useCallback(async () => {
    setSubscriptionVisibilityOpen(true);
    setSubscriptionVisibilityLoading(true);
    setSubscriptionVisibilitySearch('');
    try {
      const [settingsResponse, usersResponse] = await Promise.all([
        portalApi.settings(),
        portalApi.userOptions({ limit: 1000 }),
      ]);
      const allUsers = (usersResponse.data || []).filter((user) => user.role === 'user');
      const configuredUserIds = normalizeUserIDs(settingsResponse.data.subscriptionAccessUserIds || settingsResponse.data.subscriptionAccessUserId);
      const subscribedUserIds = allUsers.filter((user) => user.subscription?.isPaid === true).map((user) => user.id);
      const initialized = Boolean(settingsResponse.data.subscriptionAccessInitialized);
      const nextAccessUserIds = initialized ? configuredUserIds : Array.from(new Set([...configuredUserIds, ...subscribedUserIds]));
      setSubscriptionVisibilityUsers(allUsers);
      setSubscriptionAccessUserIds(nextAccessUserIds);
      setSavedSubscriptionAccessUserIds(nextAccessUserIds);
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '订阅可见账号加载失败');
      setSubscriptionVisibilityOpen(false);
    } finally {
      setSubscriptionVisibilityLoading(false);
    }
  }, []);

  const closeSubscriptionVisibility = useCallback(() => {
    if (subscriptionVisibilitySaving) return;
    setSubscriptionVisibilityOpen(false);
  }, [subscriptionVisibilitySaving]);

  const toggleSubscriptionVisibilityUser = (userId: string) => {
    setSubscriptionAccessUserIds((current) => current.includes(userId)
      ? current.filter((id) => id !== userId)
      : [...current, userId]);
  };

  const saveSubscriptionVisibility = async () => {
    setSubscriptionVisibilitySaving(true);
    try {
      await portalApi.updateSettings({
        subscriptionAccessUserIds: subscriptionAccessUserIds.join(','),
        subscriptionAccessUserId: subscriptionAccessUserIds[0] || '',
        subscriptionAccessInitialized: true,
      });
      setSavedSubscriptionAccessUserIds(subscriptionAccessUserIds);
      toast.success(subscriptionAccessUserIds.length ? `已开放给 ${subscriptionAccessUserIds.length} 个账号` : '订阅入口已关闭');
      setSubscriptionVisibilityOpen(false);
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '订阅可见性保存失败');
    } finally {
      setSubscriptionVisibilitySaving(false);
    }
  };

  const load = useCallback(async () => {
    usersRequestRef.current?.abort();
    const controller = new AbortController();
    usersRequestRef.current = controller;
    const isInitialLoad = !hasLoadedUsersRef.current;
    if (isInitialLoad) setLoading(true);
    else setRefreshing(true);
    setError('');
    try {
      const [userResponse, planResponse] = await Promise.all([
        portalApi.adminUsers({
          page,
          pageSize,
          keyword: search.trim() || undefined,
          status: statusFilter === 'all' ? undefined : statusFilter,
          billing: billingFilter === 'all' ? undefined : billingFilter,
          activity: activityFilter === 'all' ? undefined : activityFilter,
        }, controller.signal),
        portalApi.adminPlans(),
      ]);
      if (controller.signal.aborted) return;
      setUsers(userResponse.data);
      setTotal(Number(userResponse.pagination?.total || 0));
      setSummaryStats(userResponse.summary || { total: 0, active: 0, verified: 0, subscribed: 0, activeLast30Days: 0 });
      setPlans(planResponse.data);
    } catch (requestError) {
      if (controller.signal.aborted) return;
      setError(requestError instanceof Error ? requestError.message : '用户列表加载失败');
    } finally {
      if (controller.signal.aborted || usersRequestRef.current !== controller) return;
      setLoading(false);
      setRefreshing(false);
      hasLoadedUsersRef.current = true;
    }
  }, [activityFilter, billingFilter, page, search, statusFilter]);

  const loadConsumptionRanking = useCallback(async () => {
    setConsumptionLoading(true);
    setConsumptionError('');
    try {
      const response = await portalApi.userConsumptionRanking(consumptionDays, 8);
      setConsumptionRanking(response.data);
    } catch (requestError) {
      setConsumptionRanking([]);
      setConsumptionError(requestError instanceof Error ? requestError.message : '消费排行榜加载失败');
    } finally {
      setConsumptionLoading(false);
    }
  }, [consumptionDays]);

  useEffect(() => {
    const initialSearch = new URLSearchParams(window.location.search).get('search')?.trim();
    const timer = window.setTimeout(() => {
      if (initialSearch) setSearchInput(initialSearch);
    }, 0);
    return () => window.clearTimeout(timer);
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setSearch((current) => {
        const next = searchInput.trim();
        if (current !== next) {
          setPage(1);
          setSelectedUserIds([]);
        }
        return next;
      });
    }, 350);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  useEffect(() => () => usersRequestRef.current?.abort(), []);

  useEffect(() => {
    if (!rankingOpen) return;
    const timer = window.setTimeout(() => void loadConsumptionRanking(), 0);
    return () => window.clearTimeout(timer);
  }, [loadConsumptionRanking, rankingOpen]);

  useEffect(() => {
    if (!rankingOpen) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      closeRanking();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [closeRanking, rankingOpen]);

  useEffect(() => {
    if (!creditLogUser) return;
    let active = true;
    const timer = window.setTimeout(() => {
      setCreditLogLoading(true);
      setCreditLogError('');
      void portalApi.userCreditLogs(creditLogUser.id, creditLogPage, creditLogPageSize, creditLogFilter, creditLogSort.key, creditLogSort.direction)
        .then((response) => {
          if (!active) return;
          setCreditLogs(response.data);
          setCreditLogTotal(Number(response.pagination?.total || 0));
        })
        .catch((requestError) => {
          if (!active) return;
          setCreditLogs([]);
          setCreditLogTotal(0);
          setCreditLogError(requestError instanceof Error ? requestError.message : '积分明细加载失败');
        })
        .finally(() => { if (active) setCreditLogLoading(false); });
    }, 0);
    return () => {
      active = false;
      window.clearTimeout(timer);
    };
  }, [creditLogFilter, creditLogPage, creditLogRefresh, creditLogSort.direction, creditLogSort.key, creditLogUser]);

  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const creditLogTotalPages = Math.max(1, Math.ceil(creditLogTotal / creditLogPageSize));
  const activePlans = plans.filter((plan) => plan.status === 'active');
  const currentBalance = Number(balanceUser?.credits || 0);
  const adjustmentAmount = Number(balanceValue);
  const nextBalance = Number.isFinite(adjustmentAmount)
    ? Math.round((currentBalance + adjustmentAmount) * 10000) / 10000
    : Number.NaN;
  const currentQuotaLimit = Number(quotaUser?.subscription?.quotaLimit ?? quotaUser?.subscription?.quotaImages ?? 0);
  const currentQuotaUsed = Number(quotaUser?.subscription?.quotaUsed ?? 0);
  const currentQuotaRemaining = Number(quotaUser?.subscription?.effectiveQuotaRemaining ?? quotaUser?.subscription?.quotaRemaining ?? 0);
  const nextQuotaRemaining = Number(quotaValue);

  const summary = summaryStats;
  const subscriptionAccessDirty = !sameUserIDs(subscriptionAccessUserIds, savedSubscriptionAccessUserIds);
  const selectedSubscriptionUserCount = subscriptionAccessUserIds.length;
  const filteredSubscriptionVisibilityUsers = useMemo(() => {
    const keyword = subscriptionVisibilitySearch.trim().toLocaleLowerCase();
    return subscriptionVisibilityUsers
      .filter((user) => user.status === 'active' || subscriptionAccessUserIds.includes(user.id))
      .filter((user) => !keyword || `${user.email} ${user.id}`.toLocaleLowerCase().includes(keyword))
      .sort((left, right) => left.email.localeCompare(right.email));
  }, [subscriptionAccessUserIds, subscriptionVisibilitySearch, subscriptionVisibilityUsers]);
  const consumptionWindowLabel = consumptionDays === 0 ? '全部时间' : `近 ${consumptionDays} 天`;
  const sortedConsumptionRanking = useMemo(() => {
    const header = consumptionHeaders.find((item) => item.key === consumptionSort.key) || consumptionHeaders[1];
    return sortItems(consumptionRanking, header, consumptionSort.direction);
  }, [consumptionRanking, consumptionSort.direction, consumptionSort.key]);

  const resetPage = () => {
    setPage(1);
    setSelectedUserIds([]);
  };
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
      syncSize: user.syncSize === true,
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
        syncSize: draft.syncSize,
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
    setBalanceValue('');
    setBalanceRemark('');
  };

  const openQuota = (user: PortalUser) => {
    if (!subscriptionActive(user)) return;
    setQuotaUser(user);
    setQuotaMode('remaining');
    setQuotaValue(String(Number(user.subscription?.effectiveQuotaRemaining ?? user.subscription?.quotaRemaining ?? 0)));
  };

  const changeQuotaMode = (mode: QuotaAdjustMode) => {
    setQuotaMode(mode);
    setQuotaValue(String(mode === 'reset' ? currentQuotaLimit : currentQuotaRemaining));
  };

  const updateSubscriptionQuota = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!quotaUser) return;
    if (!quotaValue.trim()) return toast.error('请填写调整后的订阅额度');
    const value = Number(quotaValue);
    if (!Number.isInteger(value) || value < 0 || value > 100000000) return toast.error('订阅额度必须是 0-100000000 之间的整数');
    if (quotaMode === 'remaining' && value === currentQuotaRemaining) return toast.error('调整后剩余额度没有变化');
    if (quotaMode === 'reset' && currentQuotaUsed === 0 && value === currentQuotaLimit) return toast.error('当前订阅额度已经是重置状态');
    setSaving(true);
    try {
      const response = await portalApi.updateSubscriptionQuota(quotaUser.id, {
        quotaRemaining: value,
        resetUsage: quotaMode === 'reset',
      });
      setUsers((items) => items.map((item) => (item.id === quotaUser.id ? response.data : item)));
      toast.success(quotaMode === 'reset' ? `已重置 ${quotaUser.email} 的订阅额度` : `已修改 ${quotaUser.email} 的剩余额度`);
      setQuotaUser(null);
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '订阅额度调整失败');
    } finally {
      setSaving(false);
    }
  };

  const openCreditLogs = (user: PortalUser) => {
    setCreditLogs([]);
    setCreditLogTotal(0);
    setCreditLogError('');
    setCreditLogFilter('all');
    setCreditLogPage(1);
    setCreditLogSort({ key: 'createdAt', direction: 'desc' });
    setCreditLogUser(user);
  };

  const updateBalance = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!balanceUser) return;
    if (!balanceValue.trim()) return toast.error('请填写调整金额');
    if (!Number.isFinite(adjustmentAmount)) return toast.error('请输入有效的调整金额');
    if (!Number.isFinite(nextBalance) || nextBalance < 0 || nextBalance > 99999999.9999) return toast.error('调整后余额必须在 0 到 99999999.9999 之间');
    if (Math.abs(adjustmentAmount) < 0.00005) return toast.error('调整金额不能为 0');
    if (balanceRemark.trim().length > 120) return toast.error('备注不能超过 120 个字');
    setSaving(true);
    try {
      const response = await portalApi.updateUserBalance(balanceUser.id, { amount: adjustmentAmount, remark: balanceRemark.trim() || undefined });
      setUsers((items) => items.map((item) => (item.id === balanceUser.id ? { ...item, credits: response.data.credits } : item)));
      toast.success(`已为 ${balanceUser.email}${adjustmentAmount > 0 ? '增加' : '扣减'} ${formatCNY(Math.abs(adjustmentAmount))}`);
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

  const selectableUsers = users.filter((user) => user.role !== 'admin');
  const allCurrentUsersSelected = selectableUsers.length > 0 && selectableUsers.every((user) => selectedUserIds.includes(user.id));
  const someCurrentUsersSelected = selectableUsers.some((user) => selectedUserIds.includes(user.id)) && !allCurrentUsersSelected;
  const toggleUserSelection = (userId: string) => {
    setSelectedUserIds((current) => current.includes(userId) ? current.filter((id) => id !== userId) : [...current, userId]);
  };
  const toggleCurrentPageSelection = () => {
    const currentIds = selectableUsers.map((user) => user.id);
    setSelectedUserIds((current) => allCurrentUsersSelected
      ? current.filter((id) => !currentIds.includes(id))
      : Array.from(new Set([...current, ...currentIds])));
  };
  const bulkDeleteUsers = async () => {
    if (selectedUserIds.length === 0) return;
    setSaving(true);
    try {
      const response = await portalApi.deleteUsers(selectedUserIds);
      toast.success(`已删除 ${Number(response.data?.deleted || 0)} 个用户`);
      setSelectedUserIds([]);
      await load();
    } catch (requestError) {
      toast.error(requestError instanceof Error ? requestError.message : '批量删除失败');
    } finally {
      setSaving(false);
    }
  };

  const rowActions = (user: PortalUser) => (
    <div className="flex min-w-0 flex-wrap items-center justify-end gap-1 md:min-w-[250px] md:flex-nowrap">
      <button type="button" onClick={() => openCreditLogs(user)} title="查看积分明细" aria-label={`查看 ${user.email} 的积分明细`} className="inline-flex h-7 items-center gap-1 rounded-md border border-blue-200 bg-blue-50 px-2 text-[10px] font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-100"><ReceiptText className="h-3 w-3" />积分明细</button>
      {!user.emailVerifiedAt && (
        <button type="button" onClick={() => void verifyEmail(user)} disabled={Boolean(verifyingId)} title="直接验证邮箱" aria-label={`直接验证 ${user.email} 的邮箱`} className="grid h-7 w-7 place-items-center rounded-md border border-emerald-200 bg-emerald-50 text-emerald-700 hover:border-emerald-300 hover:bg-emerald-100 disabled:opacity-40">
          {verifyingId === user.id ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <MailCheck className="h-3.5 w-3.5" />}
        </button>
      )}
      <button type="button" onClick={() => openGrant(user)} title="发放订阅" aria-label={`为 ${user.email} 发放订阅`} className="grid h-7 w-7 place-items-center rounded-md border border-cyan-200 bg-cyan-50 text-cyan-700 hover:border-cyan-300 hover:bg-cyan-100"><CreditCard className="h-3.5 w-3.5" /></button>
      {subscriptionActive(user) && <button type="button" onClick={() => openQuota(user)} title="调整订阅额度" aria-label={`调整 ${user.email} 的订阅额度`} className="grid h-7 w-7 place-items-center rounded-md border border-amber-200 bg-amber-50 text-amber-700 hover:border-amber-300 hover:bg-amber-100"><Gauge className="h-3.5 w-3.5" /></button>}
      <button type="button" onClick={() => openEdit(user)} title="编辑用户" aria-label={`编辑 ${user.email}`} className="grid h-7 w-7 place-items-center rounded-md border border-zinc-200 bg-zinc-50 text-zinc-600 hover:border-zinc-300 hover:bg-zinc-100"><Pencil className="h-3.5 w-3.5" /></button>
      <button type="button" onClick={() => void toggleStatus(user)} disabled={actionId === user.id} title={user.status === 'active' ? '停用用户' : '启用用户'} aria-label={`${user.status === 'active' ? '停用' : '启用'} ${user.email}`} className={`grid h-7 w-7 place-items-center rounded-md border ${user.status === 'active' ? 'border-amber-200 bg-amber-50 text-amber-700 hover:border-amber-300 hover:bg-amber-100' : 'border-emerald-200 bg-emerald-50 text-emerald-700 hover:border-emerald-300 hover:bg-emerald-100'} disabled:opacity-40`}><ShieldCheck className="h-3.5 w-3.5" /></button>
      <button type="button" onClick={() => setDeleteCandidate(user)} title="删除用户" aria-label={`删除 ${user.email}`} className="grid h-7 w-7 place-items-center rounded-md border border-red-200 bg-red-50 text-red-600 hover:border-red-300 hover:bg-red-100"><Trash2 className="h-3.5 w-3.5" /></button>
    </div>
  );

  const trapDialogFocus = (event: React.KeyboardEvent<HTMLElement>) => {
    if (event.key !== 'Tab') return;
    const focusable = Array.from(event.currentTarget.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
    )).filter((element) => element.getClientRects().length > 0);
    const first = focusable[0];
    const last = focusable.at(-1);
    if (!first || !last) return;
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  };

  return (
    <div className="space-y-5">
      <PageHeader
        title="用户管理"
        description={loading
          ? '正在加载用户数据...'
          : (
            <span className="inline-flex flex-wrap items-center gap-x-3 gap-y-1 text-[13px]">
              <span className="text-zinc-500">共 <strong className="font-semibold text-zinc-800">{summary.total.toLocaleString('zh-CN')}</strong> 位</span>
              <span className="inline-flex items-center gap-1.5 text-emerald-700"><i className="h-1.5 w-1.5 rounded-full bg-emerald-500" />启用 <strong className="font-semibold">{summary.active.toLocaleString('zh-CN')}</strong> 位</span>
              <span className="inline-flex items-center gap-1.5 text-sky-700"><i className="h-1.5 w-1.5 rounded-full bg-sky-500" />邮箱已验证 <strong className="font-semibold">{summary.verified.toLocaleString('zh-CN')}</strong> 位</span>
              <span className="inline-flex items-center gap-1.5 text-amber-700"><i className="h-1.5 w-1.5 rounded-full bg-amber-500" />有效订阅 <strong className="font-semibold">{summary.subscribed.toLocaleString('zh-CN')}</strong> 位</span>
              <span className="inline-flex items-center gap-1.5 text-violet-700"><i className="h-1.5 w-1.5 rounded-full bg-violet-500" />近30天活跃 <strong className="font-semibold">{summary.activeLast30Days.toLocaleString('zh-CN')}</strong> 位</span>
            </span>
          )}
      >
        <button type="button" onClick={() => void openSubscriptionVisibility()} className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[#DCE4DF] bg-white px-3 text-xs font-semibold text-zinc-700 hover:border-[#86EFAC] hover:text-[#047857]"><Eye className="h-3.5 w-3.5" />可见订阅</button>
        <button ref={rankingTriggerRef} type="button" onClick={() => { setConsumptionLoading(true); setRankingOpen(true); }} className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[#DCE4DF] bg-white px-3 text-xs font-semibold text-zinc-700 hover:border-[#86EFAC] hover:text-[#047857]"><Trophy className="h-3.5 w-3.5" />消费排行</button>
        <button type="button" onClick={() => void load()} disabled={loading || refreshing} title="刷新数据" aria-label="刷新用户数据" className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-[#DCE4DF] bg-white hover:border-[#12B76A] disabled:opacity-50"><RefreshCw className={`h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} /></button>
        <button type="button" onClick={openCreate} className="inline-flex h-8 items-center gap-1.5 rounded-md bg-[#047857] px-3 text-xs font-semibold text-white hover:bg-[#036b4f]"><Plus className="h-4 w-4" />新增用户</button>
      </PageHeader>

      {subscriptionVisibilityOpen && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4" onMouseDown={(event) => { if (event.target === event.currentTarget) closeSubscriptionVisibility(); }}>
          <section className="flex max-h-[calc(100vh-2rem)] w-full max-w-xl flex-col overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl" role="dialog" aria-modal="true" aria-labelledby="subscription-visibility-title" onKeyDown={(event) => { trapDialogFocus(event); if (event.key === 'Escape') closeSubscriptionVisibility(); }}>
            <header className="flex items-center justify-between gap-3 border-b border-[#DCE4DF] px-5 py-3.5">
              <div className="flex min-w-0 items-center gap-2.5">
                <span className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-emerald-50 text-[#047857]"><Eye className="h-4 w-4" /></span>
                <div className="min-w-0"><h2 id="subscription-visibility-title" className="text-sm font-semibold">可见订阅</h2><p className="mt-0.5 truncate text-[11px] text-zinc-500">选择可以看到套餐、订阅入口和订阅订单的用户</p></div>
              </div>
              <button type="button" onClick={closeSubscriptionVisibility} title="关闭" aria-label="关闭可见订阅" className="grid h-8 w-8 shrink-0 place-items-center rounded-md text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button>
            </header>

            <div className="min-h-0 flex-1 overflow-y-auto p-5">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="text-[11px] text-zinc-500">已选择 <strong className="font-mono text-[#047857]">{selectedSubscriptionUserCount}</strong> 个账号</span>
                <div className="flex items-center gap-2 text-[10px] font-semibold"><button type="button" onClick={() => setSubscriptionAccessUserIds(subscriptionVisibilityUsers.filter((user) => user.status === 'active').map((user) => user.id))} className="text-[#047857] hover:underline">全选启用账号</button><span className="text-zinc-300">/</span><button type="button" onClick={() => setSubscriptionAccessUserIds([])} className="text-zinc-500 hover:text-[#047857]">关闭订阅入口</button></div>
              </div>

              <label className="relative mt-3 block">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400" />
                <input value={subscriptionVisibilitySearch} onChange={(event) => setSubscriptionVisibilitySearch(event.target.value)} placeholder="搜索邮箱或用户 ID" className="h-9 w-full rounded-md border border-[#DCE4DF] bg-white pl-9 pr-3 text-xs outline-none focus:border-[#12B76A]" />
              </label>

              <div className="mt-3 overflow-hidden rounded-md border border-[#DCE4DF] bg-[#FAFBFA]">
                {subscriptionVisibilityLoading ? (
                  <div className="grid min-h-44 place-items-center"><Loader2 className="h-5 w-5 animate-spin text-[#12B76A]" /></div>
                ) : (
                  <div className="max-h-72 overflow-y-auto p-1">
                    <button type="button" onClick={() => setSubscriptionAccessUserIds([])} className={`flex w-full items-center gap-2 rounded px-2.5 py-2 text-left text-[11px] hover:bg-[#F0FDF4] ${selectedSubscriptionUserCount === 0 ? 'bg-emerald-50 text-emerald-800' : 'text-zinc-600'}`}>
                      <span className={`grid h-4 w-4 shrink-0 place-items-center rounded border ${selectedSubscriptionUserCount === 0 ? 'border-emerald-500 bg-emerald-600 text-white' : 'border-[#CBD5CF] bg-white'}`}>{selectedSubscriptionUserCount === 0 && <Check className="h-3 w-3" />}</span>
                      暂不开放（默认）
                    </button>
                    {filteredSubscriptionVisibilityUsers.map((user) => {
                      const selected = subscriptionAccessUserIds.includes(user.id);
                      return <button key={user.id} type="button" role="option" aria-selected={selected} onClick={() => toggleSubscriptionVisibilityUser(user.id)} className={`flex w-full items-center gap-2 rounded px-2.5 py-2 text-left hover:bg-[#F0FDF4] ${selected ? 'bg-emerald-50' : ''}`}><span className={`grid h-4 w-4 shrink-0 place-items-center rounded border ${selected ? 'border-emerald-500 bg-emerald-600 text-white' : 'border-[#CBD5CF] bg-white'}`}>{selected && <Check className="h-3 w-3" />}</span><span className="min-w-0"><strong className="block truncate text-[11px] font-medium">{user.email}</strong><small className="block truncate font-mono text-[9px] text-zinc-400">{user.status === 'active' ? '启用账号' : '已停用 · 已开放'}</small></span></button>;
                    })}
                    {!filteredSubscriptionVisibilityUsers.length && <p className="py-8 text-center text-[11px] text-zinc-400">没有匹配的用户</p>}
                  </div>
                )}
              </div>
              <p className="mt-2 text-[10px] leading-5 text-zinc-400">未选择账号时，用户端不显示订阅相关入口；已有有效订阅的账号会在首次初始化时自动保留。</p>
            </div>

            <footer className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3">
              <button type="button" onClick={closeSubscriptionVisibility} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button>
              <button type="button" onClick={() => void saveSubscriptionVisibility()} disabled={subscriptionVisibilityLoading || subscriptionVisibilitySaving || !subscriptionAccessDirty} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{subscriptionVisibilitySaving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}保存设置</button>
            </footer>
          </section>
        </div>
      )}

      {rankingOpen && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4" onMouseDown={(event) => { if (event.target === event.currentTarget) closeRanking(); }}>
          <section className="flex max-h-[calc(100vh-2rem)] w-full max-w-5xl flex-col overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl" role="dialog" aria-modal="true" aria-labelledby="consumption-ranking-title" onKeyDown={trapDialogFocus}>
            <header className="flex flex-col gap-3 border-b border-[#EDF0EE] px-5 py-3.5 sm:flex-row sm:items-center sm:justify-between">
              <div className="min-w-0">
                <h2 id="consumption-ranking-title" className="text-sm font-semibold text-[#17201B]">消费排行</h2>
                <p className="mt-0.5 text-[11px] text-zinc-500">{consumptionWindowLabel}余额扣费最高的客户</p>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <div className="inline-flex rounded-md border border-[#DCE4DF] bg-[#F7F8F6] p-0.5" role="group" aria-label="消费排行榜时间范围">
                  {([ [7, '7天'], [15, '15天'], [30, '30天'], [0, '全部'] ] as const).map(([value, label]) => (
                    <button key={label} type="button" onClick={() => setConsumptionDays(value)} aria-pressed={consumptionDays === value} className={`h-7 min-w-11 rounded px-2 text-[11px] font-semibold ${consumptionDays === value ? 'bg-white text-[#047857] shadow-sm' : 'text-zinc-500 hover:text-zinc-800'}`}>{label}</button>
                  ))}
                </div>
                <button type="button" onClick={() => void loadConsumptionRanking()} disabled={consumptionLoading} title="刷新排行" className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white text-zinc-600 hover:border-[#86EFAC] disabled:opacity-50"><RefreshCw className={`h-3.5 w-3.5 ${consumptionLoading ? 'animate-spin' : ''}`} /></button>
                <button type="button" autoFocus onClick={closeRanking} title="关闭" aria-label="关闭消费排行" className="grid h-8 w-8 place-items-center rounded-md text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button>
              </div>
            </header>
            <div className="min-h-0 flex-1 overflow-auto">
        {consumptionLoading && !consumptionRanking.length ? (
          <div className="grid min-h-[240px] place-items-center"><Loader2 className="h-5 w-5 animate-spin text-[#12B76A]" /></div>
        ) : consumptionError && !consumptionRanking.length ? (
          <div className="grid min-h-[240px] place-items-center px-4 text-center">
            <div>
              <CircleDollarSign className="mx-auto h-8 w-8 text-red-300" />
              <p className="mt-3 text-xs font-semibold text-red-700">消费排行榜加载失败</p>
              <p className="mt-1 text-[11px] text-zinc-400">{consumptionError}</p>
              <button type="button" onClick={() => void loadConsumptionRanking()} className="mt-3 rounded-md border border-red-200 bg-white px-3 py-1.5 text-[11px] font-semibold text-red-700 hover:bg-red-50">重试</button>
            </div>
          </div>
        ) : consumptionRanking.length ? (
          <>
            <div className="hidden md:block overflow-x-auto">
              <table className="w-full min-w-[680px] text-left text-[11px]">
                <thead className="bg-[#F7F8F6] text-[10px] text-zinc-500"><tr><th className="w-11 px-3 py-2 text-center">#</th>{consumptionHeaders.map((header) => <SortableHeader key={header.key} header={{ ...header, className: header.key === 'creditsSpent' || header.key === 'deductCount' || header.key === 'lastDeductAt' ? 'text-right' : undefined }} sortState={consumptionSort} onSort={(key) => { const direction = consumptionSort.key === key && consumptionSort.direction === 'asc' ? 'desc' : 'asc'; setConsumptionSort({ key, direction }); }} />)}</tr></thead>
                <tbody className="divide-y divide-[#EDF0EE]">
                  {sortedConsumptionRanking.map((item, index) => {
                    const statusLabel = userStatusLabel(item.userStatus);
                    const rankTone = index === 0 ? 'bg-amber-50 text-amber-700' : index === 1 ? 'bg-zinc-100 text-zinc-600' : index === 2 ? 'bg-orange-50 text-orange-700' : 'bg-zinc-100 text-zinc-500';
                    return (
                      <tr key={item.userId} className="hover:bg-[#FAFBFA]">
                        <td className="px-3 py-2.5 text-center"><span className={`inline-grid h-5 w-5 place-items-center rounded font-mono text-[10px] font-bold ${rankTone}`}>{index + 1}</span></td>
                        <td className="max-w-[220px] px-3 py-2.5"><strong className="block truncate font-semibold text-[#17201B]">{item.userEmail || item.userId}</strong><small className="mt-0.5 block truncate text-[9px] text-zinc-400">{item.userId} · {statusLabel}</small></td>
                        <td className="px-3 py-2.5 text-right font-mono font-semibold text-[#17201B]">{formatCNY(item.creditsSpent)}</td>
                        <td className="px-3 py-2.5 text-right font-mono">{item.deductCount.toLocaleString('zh-CN')}</td>
                        <td className="px-3 py-2.5 text-right text-zinc-500">{item.lastDeductAt ? formatDate(item.lastDeductAt) : '-'}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>

            <div className="divide-y divide-[#EDF0EE] md:hidden">
              {sortedConsumptionRanking.map((item, index) => {
                const statusLabel = userStatusLabel(item.userStatus);
                const rankTone = index === 0 ? 'bg-amber-50 text-amber-700' : index === 1 ? 'bg-zinc-100 text-zinc-600' : index === 2 ? 'bg-orange-50 text-orange-700' : 'bg-zinc-100 text-zinc-500';
                return (
                  <article key={item.userId} className="px-4 py-3.5">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <span className={`grid h-5 w-5 shrink-0 place-items-center rounded font-mono text-[10px] font-bold ${rankTone}`}>{index + 1}</span>
                          <strong className="truncate text-sm text-[#17201B]">{item.userEmail || item.userId}</strong>
                        </div>
                        <small className="mt-1 block truncate text-[10px] text-zinc-400">{item.userId} · {statusLabel}</small>
                      </div>
                      <div className="text-right">
                        <span className="block font-mono text-sm font-semibold text-[#17201B]">{formatCNY(item.creditsSpent)}</span>
                        <small className="block text-[10px] text-zinc-400">累计消费</small>
                      </div>
                    </div>
                    <div className="mt-3 flex items-center justify-between text-[10px] text-zinc-500">
                      <span>扣费 {item.deductCount} 次</span>
                      <span>{item.lastDeductAt ? formatDate(item.lastDeductAt) : '-'}</span>
                    </div>
                  </article>
                );
              })}
            </div>
          </>
        ) : (
          <div className="grid min-h-[240px] place-items-center px-4 text-center">
            <div>
              <Trophy className="mx-auto h-8 w-8 text-zinc-300" />
              <p className="mt-3 text-xs font-semibold text-zinc-600">暂无消费记录</p>
              <p className="mt-1 text-[11px] text-zinc-400">用户发起 API 调用并产生扣费后会自动进入排行榜。</p>
            </div>
          </div>
        )}

            </div>
            <footer className="flex flex-wrap items-center justify-between gap-2 border-t border-[#EDF0EE] bg-[#FAFBFA] px-5 py-2.5 text-[10px] text-zinc-400">
              <span>按扣费金额排序，展示前 8 名</span>
              <span>{consumptionRanking.length} 位客户</span>
            </footer>
          </section>
        </div>
      )}

      {error && (
        <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700"><span>{error}</span><button type="button" onClick={() => void load()} className="font-semibold underline">重试</button></div>
      )}

      {loading ? (
        <div className="grid min-h-[300px] place-items-center rounded-md border border-[#DCE4DF] bg-white"><Loader2 className="h-6 w-6 animate-spin text-[#12B76A]" /></div>
      ) : (
        <DataTable
          headers={[
            { key: 'select', label: '选择', sortable: false, className: 'w-12' },
            { key: 'account', label: '用户', sortValue: (item) => item.email },
            { key: 'billing', label: '计费与订阅', className: 'min-w-[180px]', sortValue: (item) => subscriptionActive(item) ? item.subscription?.planName || item.subscription?.tier || '订阅' : '余额计费' },
            { key: 'balance', label: '余额', className: 'min-w-[150px] text-right', sortValue: (item) => Number(item.credits || 0) },
            { key: 'loginIp', label: '登录 IP', className: 'min-w-[140px]', sortValue: (item) => item.lastLoginIp || '' },
            { key: 'apiIp', label: 'Key 调用 IP', className: 'min-w-[140px]', sortValue: (item) => item.lastApiIp || '' },
            { key: 'created', label: '注册时间', sortValue: (item) => Date.parse(item.createdAt || '') || 0 },
            { key: 'actions', label: '操作', sortable: false, className: 'text-right' },
          ]}
          data={users}
          pageSize={pageSize}
          searchPlaceholder="搜索邮箱或用户 ID"
          searchValue={searchInput}
          onSearchChange={(value) => setSearchInput(value)}
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
              <AppSelect
                compact
                value={activityFilter}
                onValueChange={(value) => { setActivityFilter(value); resetPage(); }}
                ariaLabel="活跃状态筛选"
                options={[
                  { value: 'all', label: '全部活跃状态' },
                  { value: 'active', label: '近30天活跃' },
                  { value: 'inactive', label: '近30天非活跃' },
                ]}
              />
              <label className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[#DCE4DF] bg-white px-2.5 text-[11px] font-semibold text-zinc-600">
                <SelectionCheckbox checked={allCurrentUsersSelected} indeterminate={someCurrentUsersSelected} onChange={toggleCurrentPageSelection} ariaLabel="全选当前页用户" className="user-selection-checkbox--small" />
                全选本页
              </label>
              {selectedUserIds.length > 0 && <button type="button" onClick={() => setBulkDeleteOpen(true)} disabled={saving} className="inline-flex h-8 items-center gap-1.5 rounded-md border border-red-200 bg-red-50 px-2.5 text-[11px] font-semibold text-red-700 hover:border-red-300 hover:bg-red-100 disabled:opacity-50"><Trash2 className="h-3.5 w-3.5" />删除选中 {selectedUserIds.length}</button>}
              {searching || refreshing ? <span className="inline-flex items-center gap-1 text-[11px] text-zinc-400"><Loader2 className="h-3 w-3 animate-spin" />{searching ? '正在搜索' : '正在更新'}</span> : <span className="text-[11px] text-zinc-400">共 {total.toLocaleString('zh-CN')} 条</span>}
            </>
          )}
          currentPage={Math.min(page, totalPages)}
          totalPages={totalPages}
          totalItems={total}
          onPageChange={(nextPage) => { setSelectedUserIds([]); setPage(nextPage); }}
          paginationLoading={refreshing}
          emptyState={<EmptyState title="暂无用户" description="调整筛选条件或创建一个 API 客户。" icon={UserRoundCog} />}
          renderRow={(user) => (
            <tr key={user.id} className={`group whitespace-nowrap transition-colors ${selectedUserIds.includes(user.id) ? 'bg-[#F2FBF5]' : 'hover:bg-[#FBFEFC]'}`}>
              <td className="w-12 px-4 py-4 align-middle"><SelectionCheckbox checked={selectedUserIds.includes(user.id)} disabled={user.role === 'admin'} onChange={() => toggleUserSelection(user.id)} ariaLabel={`选择 ${user.email}`} /></td>
              <td className="min-w-[280px] px-4 py-4 align-middle"><UserIdentity user={user} /></td>
              <td className="px-4 py-4 align-middle"><BillingSummary user={user} /></td>
              <td className="px-4 py-4 text-right align-middle">
                  <button type="button" onClick={() => openBalance(user)} className="group/balance inline-flex items-center gap-2 whitespace-nowrap rounded-lg border border-[#DDEEE3] bg-[#FBFDFC] px-2.5 py-2 text-right transition-colors hover:border-[#8AD9A8] hover:bg-[#F1FBF5]" title={`修改 ${user.email} 的余额`} aria-label={`修改 ${user.email} 的余额`}>
                  <span className="text-[10px] font-semibold text-[#829087]">账户余额</span>
                  <span className="inline-flex items-center gap-1.5 font-mono text-[12px] font-bold text-[#087443]"><strong>{formatCNY(Number(user.credits || 0))}</strong><Pencil className="h-3 w-3 opacity-60 transition-opacity group-hover/balance:opacity-100" /></span>
                </button>
              </td>
              <td className="px-4 py-4 align-middle"><code className="inline-flex max-w-[140px] truncate rounded-md border border-[#E5ECE7] bg-[#FAFCFB] px-2 py-1 font-mono text-[10px] text-[#617169]" title={user.lastLoginIp || '暂无登录记录'}>{user.lastLoginIp || '-'}</code></td>
              <td className="px-4 py-4 align-middle"><code className="inline-flex max-w-[140px] truncate rounded-md border border-[#E5ECE7] bg-[#FAFCFB] px-2 py-1 font-mono text-[10px] text-[#617169]" title={user.lastApiIp || '暂无 Key 调用记录'}>{user.lastApiIp || '-'}</code></td>
              <td className="whitespace-nowrap px-4 py-4 align-middle text-[11px] text-[#829087]">{formatDate(user.createdAt || '')}</td>
              <td className="px-4 py-4 align-middle">{rowActions(user)}</td>
            </tr>
          )}
          renderMobileItem={(user) => (
            <article key={user.id} className={`rounded-xl border p-3.5 shadow-[0_2px_12px_rgba(28,72,46,0.04)] transition-colors ${selectedUserIds.includes(user.id) ? 'border-[#8AD9A8] bg-[#F4FCF6]' : 'border-[#DCE8DF] bg-white'}`}>
              <div className="flex items-start justify-between gap-3">
                <div className="flex min-w-0 items-center gap-2"><SelectionCheckbox checked={selectedUserIds.includes(user.id)} disabled={user.role === 'admin'} onChange={() => toggleUserSelection(user.id)} ariaLabel={`选择 ${user.email}`} /><UserIdentity user={user} /></div>
              </div>
              <div className="mt-3 grid grid-cols-[minmax(0,1fr)_auto] items-start gap-3 border-y border-[#EDF0EE] py-3">
                <BillingSummary user={user} />
                <button type="button" onClick={() => openBalance(user)} title={`修改 ${user.email} 的余额`} aria-label={`修改 ${user.email} 的余额`} className="inline-flex items-center gap-2 whitespace-nowrap rounded-lg border border-[#DDEEE3] bg-[#FBFDFC] px-2.5 py-2 text-right"><span className="text-[10px] font-semibold text-[#829087]">账户余额</span><span className="inline-flex items-center gap-1.5 font-mono text-[12px] font-bold text-[#087443]"><strong>{formatCNY(Number(user.credits || 0))}</strong><Pencil className="h-3 w-3 opacity-60" /></span></button>
              </div>
              <div className="mt-3 grid grid-cols-2 gap-3 border-b border-[#EDF0EE] pb-3">
                <span className="min-w-0"><small className="block text-[10px] font-semibold text-[#829087]">登录 IP</small><code className="mt-1 block truncate rounded-md border border-[#E5ECE7] bg-[#FAFCFB] px-2 py-1 font-mono text-[10px] text-[#617169]" title={user.lastLoginIp || '暂无登录记录'}>{user.lastLoginIp || '-'}</code></span>
                <span className="min-w-0"><small className="block text-[10px] font-semibold text-[#829087]">Key 调用 IP</small><code className="mt-1 block truncate rounded-md border border-[#E5ECE7] bg-[#FAFCFB] px-2 py-1 font-mono text-[10px] text-[#617169]" title={user.lastApiIp || '暂无 Key 调用记录'}>{user.lastApiIp || '-'}</code></span>
              </div>
              <div className="mt-2 flex items-center justify-between gap-3"><small className={`text-[10px] ${user.emailVerifiedAt ? 'text-zinc-400' : 'text-amber-700'}`}>{user.emailVerifiedAt ? '邮箱已验证' : '邮箱未验证'}</small><small className="text-[10px] text-zinc-400">{formatDate(user.createdAt || '')}</small></div>
              <div className="mt-2 border-t border-[#EDF0EE] pt-2">{rowActions(user)}</div>
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
              <label className="sm:col-span-2 flex items-center justify-between rounded-md border border-[#DCE4DF] bg-[#F8FAF8] px-3 py-2.5"><span><strong className="block text-[11px] font-semibold text-zinc-600">同步尺寸</strong><small className="mt-0.5 block text-[10px] text-zinc-400">向上游生图请求附加 sync_size 布尔参数</small></span><button type="button" role="switch" aria-checked={draft.syncSize} onClick={() => updateDraft('syncSize', !draft.syncSize)} className={`relative h-5 w-9 rounded-full transition-colors ${draft.syncSize ? 'bg-[#12B76A]' : 'bg-zinc-300'}`}><span className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform ${draft.syncSize ? 'translate-x-4' : 'translate-x-0.5'}`} /></button></label>
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

      {quotaUser && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4">
          <form onSubmit={updateSubscriptionQuota} className="w-full max-w-md overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl" role="dialog" aria-modal="true" aria-labelledby="subscription-quota-title">
            <div className="flex items-center justify-between border-b border-[#DCE4DF] px-5 py-3.5">
              <div className="flex min-w-0 items-center gap-2.5">
                <span className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-amber-50 text-amber-700"><Gauge className="h-4 w-4" /></span>
                <div className="min-w-0"><h2 id="subscription-quota-title" className="text-sm font-semibold">调整订阅额度</h2><p className="mt-0.5 truncate text-[11px] text-zinc-500">{quotaUser.email} · {quotaUser.subscription?.planName || '订阅套餐'}</p></div>
              </div>
              <button type="button" onClick={() => setQuotaUser(null)} title="关闭" className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button>
            </div>
            <div className="space-y-4 p-5">
              <div className="grid grid-cols-3 divide-x divide-[#E5E9E6] border-y border-[#EDF0EE] bg-[#FAFBFA] py-3 text-center">
                <span><small className="block text-[10px] font-semibold text-zinc-400">总额度</small><strong className="mt-1 block font-mono text-sm text-[#17201B]">{currentQuotaLimit.toLocaleString('zh-CN')}</strong></span>
                <span><small className="block text-[10px] font-semibold text-zinc-400">已使用</small><strong className="mt-1 block font-mono text-sm text-amber-700">{currentQuotaUsed.toLocaleString('zh-CN')}</strong></span>
                <span><small className="block text-[10px] font-semibold text-zinc-400">剩余额度</small><strong className="mt-1 block font-mono text-sm text-[#047857]">{currentQuotaRemaining.toLocaleString('zh-CN')}</strong></span>
              </div>

              <div className="grid grid-cols-2 gap-1 rounded-md border border-[#DCE4DF] bg-[#F6F8F6] p-1" role="group" aria-label="订阅额度调整方式">
                <button type="button" onClick={() => changeQuotaMode('remaining')} aria-pressed={quotaMode === 'remaining'} className={`inline-flex h-9 items-center justify-center gap-1.5 rounded border text-[11px] font-semibold ${quotaMode === 'remaining' ? 'border-[#86EFAC] bg-white text-[#047857]' : 'border-transparent text-zinc-500'}`}><Pencil className="h-3.5 w-3.5" />设置剩余额度</button>
                <button type="button" onClick={() => changeQuotaMode('reset')} aria-pressed={quotaMode === 'reset'} className={`inline-flex h-9 items-center justify-center gap-1.5 rounded border text-[11px] font-semibold ${quotaMode === 'reset' ? 'border-amber-200 bg-white text-amber-700' : 'border-transparent text-zinc-500'}`}><RefreshCw className="h-3.5 w-3.5" />重置周期额度</button>
              </div>

              <label>
                <span className="mb-1 block text-[11px] font-semibold text-zinc-500">{quotaMode === 'reset' ? '重置后可用额度' : '调整后剩余额度'}</span>
                <input required autoFocus min={0} max={100000000} step={1} type="number" value={quotaValue} onChange={(event) => setQuotaValue(event.target.value)} className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs outline-none focus:border-[#12B76A]" />
                <small className="mt-1.5 block text-[10px] leading-4 text-zinc-400">{quotaMode === 'reset' ? '已用额度会清零，并从当前时间重新统计；订阅到期时间保持不变。' : '只修改当前可用额度，已用量、到期时间和套餐权限保持不变。'}</small>
              </label>

              <div className="flex items-center justify-between border-t border-[#EDF0EE] pt-3 text-xs"><span className="text-zinc-500">调整后可用</span><strong className="font-mono text-base text-[#047857]">{Number.isInteger(nextQuotaRemaining) && nextQuotaRemaining >= 0 ? nextQuotaRemaining.toLocaleString('zh-CN') : '--'}</strong></div>
            </div>
            <div className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><button type="button" onClick={() => setQuotaUser(null)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button><button type="submit" disabled={saving || !quotaValue.trim() || !Number.isInteger(nextQuotaRemaining) || nextQuotaRemaining < 0 || nextQuotaRemaining > 100000000} className="inline-flex h-8 items-center gap-2 rounded-md bg-[#047857] px-4 text-xs font-semibold text-white disabled:opacity-50">{saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}{quotaMode === 'reset' ? '确认重置' : '保存额度'}</button></div>
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
              <div className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-2 border-y border-[#EDF0EE] bg-[#FAFBFA] px-4 py-3">
                <span><small className="block text-[10px] font-semibold text-zinc-400">原本余额</small><strong className="mt-1 block font-mono text-sm text-zinc-700">{formatCNY(currentBalance)}</strong></span>
                <span className="text-sm font-semibold text-zinc-300">+</span>
                <span><small className="block text-[10px] font-semibold text-zinc-400">本次修改</small><strong className={`mt-1 block font-mono text-sm ${adjustmentAmount < 0 ? 'text-red-600' : 'text-[#047857]'}`}>{Number.isFinite(adjustmentAmount) && adjustmentAmount !== 0 ? `${adjustmentAmount > 0 ? '+' : '-'}${formatCNY(Math.abs(adjustmentAmount))}` : '--'}</strong></span>
                <span className="text-sm font-semibold text-zinc-300">=</span>
                <span className="text-right"><small className="block text-[10px] font-semibold text-zinc-400">总余额</small><strong className={`mt-1 block font-mono text-sm ${adjustmentAmount < 0 ? 'text-red-600' : 'text-[#047857]'}`}>{Number.isFinite(nextBalance) ? formatCNY(nextBalance) : '--'}</strong></span>
              </div>
              <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">调整金额</span><input required autoFocus step={0.0001} type="number" value={balanceValue} onChange={(event) => setBalanceValue(event.target.value)} placeholder="例如：10 或 -10" className="w-full rounded-md border border-[#DCE4DF] px-3 py-2 font-mono text-xs outline-none focus:border-[#12B76A]" /><small className="mt-1 block text-[10px] text-zinc-400">正数增加余额，负数扣减余额</small></label>
              <label><span className="mb-1 block text-[11px] font-semibold text-zinc-500">调整备注 <span className="font-normal text-zinc-400">（可选）</span></span><textarea maxLength={120} rows={3} value={balanceRemark} onChange={(event) => setBalanceRemark(event.target.value)} placeholder="例如：活动补发、退款或余额修正" className="w-full resize-none rounded-md border border-[#DCE4DF] px-3 py-2 text-xs leading-5 outline-none focus:border-[#12B76A]" /><small className="mt-1 block text-right font-mono text-[10px] text-zinc-400">{balanceRemark.length}/120</small></label>
              {Number.isFinite(nextBalance) && Math.abs(adjustmentAmount) >= 0.00005 && (
                <div className={`flex items-center justify-between border-t border-[#EDF0EE] pt-3 text-xs ${adjustmentAmount < 0 ? 'text-red-600' : 'text-[#047857]'}`}><span>{adjustmentAmount < 0 ? '本次扣减' : '本次增加'}</span><strong className="font-mono">{formatCNY(Math.abs(adjustmentAmount))}</strong></div>
              )}
            </div>
            <div className="flex justify-end gap-2 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3"><button type="button" onClick={() => setBalanceUser(null)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">取消</button><button type="submit" disabled={saving || !balanceValue.trim() || !Number.isFinite(nextBalance) || nextBalance < 0 || nextBalance > 99999999.9999 || Math.abs(adjustmentAmount) < 0.00005} className="inline-flex items-center gap-2 rounded-md bg-[#047857] px-4 py-2 text-xs font-semibold text-white disabled:opacity-50">{saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}确认调整</button></div>
          </form>
        </div>
      )}

      {creditLogUser && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4">
          <section className="flex max-h-[calc(100vh-2rem)] w-full max-w-3xl flex-col overflow-hidden rounded-md border border-[#DCE4DF] bg-white shadow-xl" role="dialog" aria-modal="true" aria-labelledby="credit-log-title">
            <div className="flex items-center justify-between gap-3 border-b border-[#DCE4DF] px-5 py-3.5">
              <div className="flex min-w-0 items-center gap-2.5">
                <span className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-blue-50 text-blue-700"><ReceiptText className="h-4 w-4" /></span>
                <div className="min-w-0"><h2 id="credit-log-title" className="text-sm font-semibold">积分消费明细</h2><p className="mt-0.5 truncate text-[11px] text-zinc-500">{creditLogUser.email}</p></div>
              </div>
              <button type="button" onClick={() => setCreditLogUser(null)} title="关闭" className="rounded p-1 text-zinc-500 hover:bg-zinc-100"><X className="h-4 w-4" /></button>
            </div>

            <div className="flex flex-wrap items-end gap-4 border-b border-[#EDF0EE] bg-[#FAFBFA] px-5 py-3">
              <div className="min-w-28"><small className="block text-[10px] font-semibold text-zinc-400">当前余额</small><strong className="mt-1 block font-mono text-base text-[#047857]">{formatCNY(Number(creditLogUser.credits || 0))}</strong></div>
              <div className="min-w-24"><small className="block text-[10px] font-semibold text-zinc-400">流水记录</small><strong className="mt-1 block font-mono text-base text-[#17201B]">{creditLogTotal.toLocaleString('zh-CN')} 条</strong></div>
              <div className="ml-auto w-full sm:w-40"><span className="mb-1 block text-[10px] font-semibold text-zinc-400">变动类型</span><AppSelect compact value={creditLogFilter} onValueChange={(value) => { setCreditLogFilter(value as CreditLogFilter); setCreditLogPage(1); }} ariaLabel="积分明细类型" options={[{ value: 'all', label: '全部明细' }, { value: 'deduct', label: 'API 消费' }, { value: 'recharge', label: '余额充值' }, { value: 'invite_reward', label: '邀请奖励' }, { value: 'invite_rebate', label: '充值返利' }, { value: 'manual_adjust', label: '后台调整' }]} /></div>
            </div>

            <div className="min-h-72 flex-1 overflow-auto">
              {creditLogLoading ? (
                <div className="grid min-h-72 place-items-center"><Loader2 className="h-5 w-5 animate-spin text-[#12B76A]" /></div>
              ) : creditLogError ? (
                <div className="grid min-h-72 place-items-center px-5 text-center"><div><p className="text-xs text-red-600">{creditLogError}</p><button type="button" onClick={() => setCreditLogRefresh((value) => value + 1)} className="mt-3 text-xs font-semibold text-[#047857] underline">重新加载</button></div></div>
              ) : creditLogs.length === 0 ? (
                <div className="grid min-h-72 place-items-center px-5 text-center"><div><ReceiptText className="mx-auto h-8 w-8 text-zinc-300" /><p className="mt-3 text-xs font-semibold text-zinc-600">暂无积分变动记录</p><p className="mt-1 text-[11px] text-zinc-400">充值、API 消费或后台调整后会显示在这里。</p></div></div>
              ) : (
                <>
                  <table className="hidden w-full border-collapse text-left text-[11px] sm:table">
                    <thead className="sticky top-0 z-10 bg-white text-zinc-400"><tr className="border-b border-[#EDF0EE]"><SortableHeader header={{ key: 'type', label: '类型' }} sortState={creditLogSort} onSort={(key) => { const direction = creditLogSort.key === key && creditLogSort.direction === 'asc' ? 'desc' : 'asc'; setCreditLogSort({ key, direction }); setCreditLogPage(1); }} className="px-5 py-2.5" /><SortableHeader header={{ key: 'amount', label: '变动', className: 'text-right' }} sortState={creditLogSort} onSort={(key) => { const direction = creditLogSort.key === key && creditLogSort.direction === 'asc' ? 'desc' : 'asc'; setCreditLogSort({ key, direction }); setCreditLogPage(1); }} /><SortableHeader header={{ key: 'balanceAfter', label: '变动后余额', className: 'text-right' }} sortState={creditLogSort} onSort={(key) => { const direction = creditLogSort.key === key && creditLogSort.direction === 'asc' ? 'desc' : 'asc'; setCreditLogSort({ key, direction }); setCreditLogPage(1); }} /><th className="px-4 py-2.5 font-semibold">说明</th><SortableHeader header={{ key: 'createdAt', label: '时间' }} sortState={creditLogSort} onSort={(key) => { const direction = creditLogSort.key === key && creditLogSort.direction === 'asc' ? 'desc' : 'asc'; setCreditLogSort({ key, direction }); setCreditLogPage(1); }} className="px-5 py-2.5" /></tr></thead>
                    <tbody>{creditLogs.map((log) => { const view = creditLogView(log); const remark = creditLogRemark(log.remark); return <tr key={log.id} className="border-b border-[#F0F2F0] last:border-0 hover:bg-[#FAFBFA]"><td className="px-5 py-3"><span className={`inline-flex rounded border px-1.5 py-0.5 font-semibold ${view.tone}`}>{view.label}</span></td><td className={`px-4 py-3 text-right font-mono font-semibold ${view.amountTone}`}>{view.change > 0 ? '+' : '-'}{formatCNY(Math.abs(view.change))}</td><td className="px-4 py-3 text-right font-mono font-semibold text-zinc-700">{formatCNY(Number(log.balanceAfter || 0))}</td><td className="max-w-[210px] px-4 py-3 text-zinc-600"><span className="block truncate" title={remark}>{remark}</span></td><td className="whitespace-nowrap px-5 py-3 text-zinc-400">{formatDate(log.createdAt)}</td></tr>; })}</tbody>
                  </table>
                  <div className="divide-y divide-[#EDF0EE] sm:hidden">{creditLogs.map((log) => { const view = creditLogView(log); const remark = creditLogRemark(log.remark); return <div key={log.id} className="px-4 py-3"><div className="flex items-center justify-between gap-3"><span className={`inline-flex rounded border px-1.5 py-0.5 text-[10px] font-semibold ${view.tone}`}>{view.label}</span><strong className={`font-mono text-xs ${view.amountTone}`}>{view.change > 0 ? '+' : '-'}{formatCNY(Math.abs(view.change))}</strong></div><p className="mt-2 truncate text-[11px] text-zinc-600" title={remark}>{remark}</p><div className="mt-2 flex items-center justify-between text-[10px] text-zinc-400"><span>余额 {formatCNY(Number(log.balanceAfter || 0))}</span><span>{formatDate(log.createdAt)}</span></div></div>; })}</div>
                </>
              )}
            </div>

            <div className="flex flex-col gap-3 border-t border-[#DCE4DF] bg-[#F8FAF8] px-5 py-3 sm:flex-row sm:items-center sm:justify-between">
              <span className="text-[10px] text-zinc-400">第 {creditLogPage} / {creditLogTotalPages} 页</span>
              <div className="flex flex-wrap items-center gap-2"><AppSelect compact value={String(creditLogPage)} options={Array.from({ length: creditLogTotalPages }, (_, index) => ({ value: String(index + 1), label: `第 ${index + 1} 页` }))} onValueChange={(value) => setCreditLogPage(Number(value))} disabled={creditLogLoading} ariaLabel="选择积分明细页码" /><button type="button" onClick={() => setCreditLogPage((value) => Math.max(1, value - 1))} disabled={creditLogPage <= 1 || creditLogLoading} title="上一页" className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white text-zinc-600 disabled:opacity-40"><ChevronLeft className="h-4 w-4" /></button><button type="button" onClick={() => setCreditLogPage((value) => value + 1)} disabled={creditLogPage >= creditLogTotalPages || creditLogLoading} title="下一页" className="grid h-8 w-8 place-items-center rounded-md border border-[#DCE4DF] bg-white text-zinc-600 disabled:opacity-40"><ChevronRight className="h-4 w-4" /></button><button type="button" onClick={() => setCreditLogUser(null)} className="h-8 rounded-md border border-[#DCE4DF] bg-white px-4 text-xs font-semibold">关闭</button></div>
            </div>
          </section>
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
      <ConfirmDialog
        isOpen={bulkDeleteOpen}
        onClose={() => setBulkDeleteOpen(false)}
        onConfirm={() => void bulkDeleteUsers()}
        title="批量删除用户"
        description={`确定删除选中的 ${selectedUserIds.length} 个用户吗？删除后将无法登录，关联历史数据将按数据库现有约束处理。`}
        confirmText="批量删除"
        type="danger"
      />
    </div>
  );
}
