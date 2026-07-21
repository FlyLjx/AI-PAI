'use client';

import { ADMIN_BASE_PATH } from '../../admin-path';

const API_BASE = `${ADMIN_BASE_PATH}/api/backend`;

export type Subscription = {
  id?: string;
  status?: string;
  tier?: string;
  isPaid?: boolean;
  source?: 'plan' | 'admin_custom';
  planId?: string;
  planName?: string;
  quotaImages?: number;
  quotaLimit?: number;
  quotaUsed?: number;
  quotaRemaining?: number;
  effectiveQuotaRemaining?: number;
  expiresAt?: string;
};

export type PortalUser = {
  id: string;
  email: string;
  role: 'user' | 'admin';
  status: string;
  credits: number;
  emailVerifiedAt?: string | null;
  createdAt?: string;
  subscription?: Subscription | null;
};

export type CreditLog = {
  id: string;
  userId: string;
  type: 'deduct' | 'recharge' | 'manual_adjust' | string;
  amount: number;
  balanceAfter: number;
  remark?: string;
  createdAt: string;
};

export type AdminIdentity = Pick<PortalUser, 'id' | 'email'> & { role: 'admin' };

export type APIKeyBillingMode = 'balance' | 'subscription' | 'auto';

export type DynamicConcurrencyConfig = {
  enabled: boolean;
  windowValue: number;
  windowUnit: 'minute' | 'hour';
  requestStep: number;
  increment: number;
};

export type APIKey = {
  id: string;
  userId: string;
  userEmail?: string;
  name: string;
  keyPrefix: string;
  status: string;
  concurrencyLimit: number;
  baseConcurrencyLimit?: number;
  windowRequestCount?: number;
  hourlyRequestCount?: number;
  dynamicConcurrencyBonus?: number;
  billingMode?: APIKeyBillingMode | null;
  lastUsedAt?: string | null;
  createdAt: string;
  requestCount: number;
  successCount: number;
  failedCount: number;
  imageCount: number;
};

export type UsageLog = {
  id: string;
  userId: string;
  taskId?: string;
  userEmail?: string;
  keyName?: string;
  keyPrefix?: string;
  endpoint: string;
  model: string;
  size: string;
  quality: string;
  quantity: number;
  imageCount: number;
  requestParameters?: Record<string, unknown>;
  responseParameters?: Record<string, unknown>;
  prompt?: string;
  responseFormat?: string;
  status: string;
  errorMessage?: string;
  chargedCredits: number;
  modelCostCredits: number;
  durationSeconds: number;
  createdAt: string;
  finishedAt?: string;
};

export type AdminInviteRecord = {
  id: string;
  inviterId: string;
  inviterEmail?: string;
  inviteeId: string;
  inviteeEmail?: string;
  rewardCredits: number;
  rewardType: string;
  rewardPlanId?: string;
  rewardLabel?: string;
  inviteeRewardCredits: number;
  inviteeRewardType: string;
  inviteeRewardPlanId?: string;
  inviteeRewardLabel?: string;
  status: string;
  riskReason?: string;
  inviteeIp?: string;
  verifiedAt?: string;
  rewardedAt?: string;
  rechargeRebateCount: number;
  rechargeRebateCredits: number;
  createdAt: string;
};

export type AdminOperationsRange = 'today' | '7d' | '15d' | '30d';
export type AdminOperationsMetric = 'requests' | 'images' | 'credits' | 'failures' | 'duration';

export type AdminOperationsTopUser = {
  userId: string;
  userEmail?: string;
  billingMode: string;
  requestCount: number;
  successCount: number;
  failedCount: number;
  imageCount: number;
  creditsSpent: number;
  averageDurationSeconds: number;
  successRate: number;
  lastRequestAt: string;
};

export type AdminOperationsActiveCall = {
  logId: string;
  taskId: string;
  userId: string;
  userEmail?: string;
  apiKeyId: string;
  keyName?: string;
  keyPrefix?: string;
  billingMode: string;
  concurrencyLimit: number;
  activeForKey: number;
  model: string;
  sizeTier: string;
  size?: string;
  quantity: number;
  status: string;
  elapsedSeconds: number;
  createdAt: string;
};

export type AdminOperationsSnapshot = {
  range: AdminOperationsRange;
  metric: AdminOperationsMetric;
  activeUsers: number;
  activeRequests: number;
  queuedRequests: number;
  processingRequests: number;
  slowRequests: number;
  averageElapsedSeconds: number;
  topUsers: AdminOperationsTopUser[];
  activeCalls: AdminOperationsActiveCall[];
  generatedAt: string;
};

export type Plan = {
  id: string;
  name: string;
  description?: string;
  amount: number;
  durationDays: number;
  quotaImages: number;
  discountPercent: number;
  badge?: string;
  sortOrder: number;
  status: string;
};

export type SystemLogFile = {
  name: string;
  size: number;
  updatedAt: string;
  category: string;
};

export type SystemLogDetail = {
  name: string;
  size: number;
  content: string;
  offset: number;
  truncated: boolean;
};

export type MailDeliveryLog = {
  id: string;
  category: string;
  fromAddress: string;
  recipient: string;
  subject: string;
  content: string;
  actionUrl?: string;
  status: 'sending' | 'sent' | 'failed' | string;
  errorMessage?: string;
  createdAt: string;
  sentAt?: string;
};

export type MailDeliverySummary = {
  total: number;
  sent: number;
  failed: number;
  sending: number;
  today: number;
};

export type MailDeliveryLogPage = {
  items: MailDeliveryLog[];
  summary: MailDeliverySummary;
};

export type MailBroadcastInput = {
  subject: string;
  content: string;
  actionText?: string;
  actionUrl?: string;
  targetType: 'all' | 'active' | 'specific';
  userIds: string[];
};

export type MailBroadcastResult = {
  accepted: boolean;
  total: number;
  success: number;
  failed: number;
  failures: Array<{ email: string; message: string }>;
  subject: string;
  message: string;
};

export type Announcement = {
  id: string;
  title: string;
  content: string;
  displayMode: 'popup' | 'banner';
  targetType: 'all' | 'users';
  status: 'active' | 'disabled';
  sortOrder: number;
  userIds: string[];
  createdAt: string;
  updatedAt: string;
};

export type AnnouncementMutationInput = Omit<Announcement, 'id' | 'createdAt' | 'updatedAt'> & {
  sendEmail?: boolean;
};

export type RequestMonitorRange = '1h' | '24h' | '7d' | '30d';

export type RequestMonitorLog = {
  id: string;
  method: string;
  path: string;
  queryParams: unknown;
  bodyParams: unknown;
  sourceIp: string;
  sourceHost: string;
  origin: string;
  referer: string;
  userAgent: string;
  statusCode: number;
  durationMs: number;
  responseBytes: number;
  createdAt: string;
};

export type RequestMonitorFrequency = {
  name: string;
  count: number;
  errors: number;
  averageDurationMs: number;
};

export type RequestMonitorSnapshot = {
  range: RequestMonitorRange;
  summary: {
    total: number;
    successful: number;
    clientErrors: number;
    serverErrors: number;
    errorRate: number;
    averageDurationMs: number;
    uniqueSources: number;
  };
  trend: Array<{ time: string; total: number; successful: number; errors: number }>;
  topEndpoints: RequestMonitorFrequency[];
  topSources: RequestMonitorFrequency[];
  items: RequestMonitorLog[];
};

export type ProviderModel = {
  name: string;
  cost1k: number;
  cost2k: number;
  cost4k: number;
};

export type SystemBuildVersion = {
  version: string;
  runId?: number;
  runNumber?: number;
  commit: string;
  publishedAt?: string;
  url?: string;
};

export type SystemUpdateState = {
  status: 'unconfigured' | 'idle' | 'queued' | 'waiting_idle' | 'checking' | 'pulling' | 'backing_up' | 'updating' | 'rolling_back' | 'success' | 'failed';
  targetVersion?: string;
  targetRunId?: number;
  targetCommit?: string;
  message?: string;
  backupDirectory?: string;
  pendingTaskCount?: number;
  force?: boolean;
  startedAt?: string;
  finishedAt?: string;
};

export type SystemUpdateInfo = {
  configured: boolean;
  current: SystemBuildVersion;
  latest: SystemBuildVersion;
  updateAvailable: boolean;
  canUpdate: boolean;
  checkError?: string;
  pendingTaskCount: number;
  state: SystemUpdateState;
  checkedAt: string;
};

export type StabilitySeriesPoint = {
  time: string;
  label?: string;
  success: number;
  failed: number;
};

export type StabilityErrorReason = {
  label: string;
  value: number;
};

export type StabilityRecentWindow = {
  total: number;
  limit: number;
  availability_total: number;
  success: number;
  failed: number;
  canceled: number;
  rejected: number;
  running: number;
  other: number;
  success_rate: number;
  failure_rate: number;
  average_duration_secs: number;
  average_success_duration_secs: number;
  average_failure_duration_secs: number;
};

export type StabilityRuntimeWindow = {
  window_minutes: number;
  bucket_minutes: number;
  total: number;
  success_rate: number;
  error_rate: number;
  start_time: string;
  end_time: string;
  series: StabilitySeriesPoint[];
  error_reasons: StabilityErrorReason[];
  totals: {
    success: number;
    failed: number;
    canceled: number;
    rejected: number;
    running: number;
    other: number;
  };
};

export type StabilitySnapshot = {
  reachable: boolean;
  status: string;
  upstream_status_code: number;
  stability_percent: number;
  generated_at: string;
  fetched_at: string;
  source: string;
  total: number;
  success: number;
  failed: number;
  error?: string;
  recent_60?: StabilityRecentWindow;
  runtime?: StabilityRuntimeWindow;
  series?: StabilitySeriesPoint[];
};

type Envelope<T> = {
  data: T;
  pagination?: { total: number; page: number; pageSize: number };
  mailDelivery?: MailBroadcastResult;
};

export class APIError extends Error {
  constructor(message: string, public status: number) {
    super(message);
  }
}

async function request<T>(url: string, options: RequestInit = {}): Promise<Envelope<T>> {
  const headers = new Headers(options.headers);
  if (options.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
  const response = await fetch(url, { ...options, headers, cache: 'no-store' });
  if (!response.ok) {
    const payload = await response.json().catch(() => null) as { message?: string; error?: { message?: string } } | null;
    throw new APIError(payload?.message || payload?.error?.message || `请求失败 (${response.status})`, response.status);
  }
  if (response.status === 204) return { data: undefined as T };
  return response.json() as Promise<Envelope<T>>;
}

function query(params: Record<string, unknown>): string {
  const search = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') search.set(key, String(value));
  });
  return search.size ? `?${search.toString()}` : '';
}

function api<T>(path: string, options: RequestInit = {}) {
  return request<T>(`${API_BASE}${path}`, options);
}

export const adminAuth = {
  login: (email: string, password: string) => request<AdminIdentity>(`${ADMIN_BASE_PATH}/api/auth/login`, {
    method: 'POST', body: JSON.stringify({ email, password }),
  }),
  session: () => request<AdminIdentity>(`${ADMIN_BASE_PATH}/api/auth/session`),
  logout: () => request<void>(`${ADMIN_BASE_PATH}/api/auth/logout`, { method: 'POST' }),
};

export const portalApi = {
  dashboard: () => api<Record<string, unknown>>('/api/dashboard?limit=8'),
  stability: () => api<StabilitySnapshot>('/api/upstream/stability'),
  users: () => api<PortalUser[]>('/api/users'),
  createUser: (input: Record<string, unknown>) => api<PortalUser>('/api/users', { method: 'POST', body: JSON.stringify(input) }),
  updateUser: (id: string, input: Record<string, unknown>) => api<PortalUser>(`/api/users/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(input) }),
  updateUserBalance: (id: string, input: { balance: number; remark: string }) => api<PortalUser>(`/api/users/${encodeURIComponent(id)}/balance`, { method: 'PATCH', body: JSON.stringify(input) }),
  userCreditLogs: (id: string, page = 1, pageSize = 10, type = 'all') => api<CreditLog[]>(`/api/users/${encodeURIComponent(id)}/credit-logs${query({ page, pageSize, type: type === 'all' ? undefined : type })}`),
  deleteUser: (id: string) => api(`/api/users/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  verifyUserEmail: (id: string) => api<PortalUser>(`/api/users/${encodeURIComponent(id)}/verify-email`, { method: 'POST' }),
  grantSubscription: (id: string, input: Record<string, unknown>) => api(`/api/users/${encodeURIComponent(id)}/subscription`, { method: 'POST', body: JSON.stringify(input) }),
  providers: () => api<Record<string, unknown>[]>('/api/api-providers'),
  createProvider: (input: Record<string, unknown>) => api('/api/api-providers', { method: 'POST', body: JSON.stringify(input) }),
  updateProvider: (id: string, input: Record<string, unknown>) => api(`/api/api-providers/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(input) }),
  deleteProvider: (id: string) => api(`/api/api-providers/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  testProvider: (id: string) => api(`/api/api-providers/${encodeURIComponent(id)}/test`, { method: 'POST' }),
  providerModels: (id: string) => api<ProviderModel[]>(`/api/api-providers/${encodeURIComponent(id)}/models`),
  models: () => api<Record<string, unknown>[]>('/api/models'),
  createModel: (input: Record<string, unknown>) => api('/api/models', { method: 'POST', body: JSON.stringify(input) }),
  updateModel: (id: string, input: Record<string, unknown>) => api(`/api/models/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(input) }),
  deleteModel: (id: string) => api(`/api/models/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  adminPlans: () => api<Plan[]>('/api/subscriptions/plans'),
  createPlan: (input: Partial<Plan>) => api<Plan>('/api/subscriptions/plans', { method: 'POST', body: JSON.stringify(input) }),
  updatePlan: (id: string, input: Partial<Plan>) => api<Plan>(`/api/subscriptions/plans/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(input) }),
  deletePlan: (id: string) => api(`/api/subscriptions/plans/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  recharges: (page = 1) => api<Record<string, unknown>[]>(`/api/recharge/orders${query({ page, pageSize: 30 })}`),
  adminKeys: () => api<{ items: APIKey[]; stats: Record<string, number>; dynamicConcurrency: DynamicConcurrencyConfig }>('/api/admin/api-access/keys'),
  updateAdminKey: (id: string, input: { status?: string; concurrencyLimit?: number }) => api<APIKey>(`/api/admin/api-access/keys/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(input) }),
  deleteAdminKey: (id: string) => api(`/api/admin/api-access/keys/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  adminUsage: (page = 1) => api<UsageLog[]>(`/api/admin/api-access/logs${query({ page, pageSize: 30 })}`),
  adminOperations: (range: AdminOperationsRange, metric: AdminOperationsMetric, limit = 10) => api<AdminOperationsSnapshot>(`/api/admin/api-access/operations${query({ range, metric, limit })}`),
  sendMailBroadcast: (input: MailBroadcastInput) => api<MailBroadcastResult>('/api/admin/mail-broadcast', { method: 'POST', body: JSON.stringify(input) }),
  adminMailLogs: (input: { page?: number; pageSize?: number; keyword?: string; status?: string; category?: string } = {}) => api<MailDeliveryLogPage>(`/api/admin/mail-logs${query(input)}`),
  announcements: () => api<Announcement[]>('/api/announcements'),
  createAnnouncement: (input: AnnouncementMutationInput) => api<Announcement>('/api/announcements', { method: 'POST', body: JSON.stringify(input) }),
  updateAnnouncement: (id: string, input: AnnouncementMutationInput) => api<Announcement>(`/api/announcements/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(input) }),
  deleteAnnouncement: (id: string) => api(`/api/announcements/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  requestMonitor: (input: { range?: RequestMonitorRange; page?: number; pageSize?: number; keyword?: string; method?: string; status?: string } = {}) => api<RequestMonitorSnapshot>(`/api/admin/request-monitor${query(input)}`),
  adminInvites: (page = 1, pageSize = 30) => api<AdminInviteRecord[]>(`/api/invites${query({ page, pageSize })}`),
  cancelTask: (taskId: string) => api(`/api/tasks/${encodeURIComponent(taskId)}/cancel`, { method: 'POST' }),
  settings: () => api<Record<string, unknown>>('/api/settings'),
  updateSettings: (input: Record<string, unknown>) => api('/api/settings', { method: 'PATCH', body: JSON.stringify(input) }),
  systemUpdate: (refresh = false) => api<SystemUpdateInfo>(`/api/admin/system-update${query({ refresh: refresh ? 1 : undefined })}`),
  startSystemUpdate: (force = false) => api<SystemUpdateInfo>('/api/admin/system-update', { method: 'POST', body: JSON.stringify({ force }) }),
  logs: () => api<SystemLogFile[]>('/api/system-logs'),
  systemLogDetail: (name: string, maxBytes = 300000) => api<SystemLogDetail>(`/api/system-logs/detail${query({ name, maxBytes })}`),
  deleteSystemLog: (name: string) => api(`/api/system-logs/${encodeURIComponent(name)}`, { method: 'DELETE' }),
};
