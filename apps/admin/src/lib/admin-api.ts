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

export type AdminIdentity = Pick<PortalUser, 'id' | 'email'> & { role: 'admin' };

export type APIKeyBillingMode = 'balance' | 'subscription' | 'auto';

export type APIKey = {
  id: string;
  userId: string;
  userEmail?: string;
  name: string;
  keyPrefix: string;
  status: string;
  concurrencyLimit: number;
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
  status: string;
  errorMessage?: string;
  createdAt: string;
  finishedAt?: string;
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
  status: 'unconfigured' | 'idle' | 'queued' | 'checking' | 'pulling' | 'backing_up' | 'updating' | 'rolling_back' | 'success' | 'failed';
  targetVersion?: string;
  targetRunId?: number;
  targetCommit?: string;
  message?: string;
  backupDirectory?: string;
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
  state: SystemUpdateState;
  checkedAt: string;
};

type Envelope<T> = { data: T; pagination?: { total: number; page: number; pageSize: number } };

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
  users: () => api<PortalUser[]>('/api/users'),
  createUser: (input: Record<string, unknown>) => api<PortalUser>('/api/users', { method: 'POST', body: JSON.stringify(input) }),
  updateUser: (id: string, input: Record<string, unknown>) => api<PortalUser>(`/api/users/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(input) }),
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
  adminKeys: () => api<{ items: APIKey[]; stats: Record<string, number> }>('/api/admin/api-access/keys'),
  updateAdminKey: (id: string, input: { status?: string; concurrencyLimit?: number }) => api<APIKey>(`/api/admin/api-access/keys/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(input) }),
  deleteAdminKey: (id: string) => api(`/api/admin/api-access/keys/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  adminUsage: (page = 1) => api<UsageLog[]>(`/api/admin/api-access/logs${query({ page, pageSize: 30 })}`),
  cancelTask: (taskId: string) => api(`/api/tasks/${encodeURIComponent(taskId)}/cancel`, { method: 'POST' }),
  settings: () => api<Record<string, unknown>>('/api/settings'),
  updateSettings: (input: Record<string, unknown>) => api('/api/settings', { method: 'PATCH', body: JSON.stringify(input) }),
  systemUpdate: (refresh = false) => api<SystemUpdateInfo>(`/api/admin/system-update${query({ refresh: refresh ? 1 : undefined })}`),
  startSystemUpdate: () => api<SystemUpdateInfo>('/api/admin/system-update', { method: 'POST' }),
  logs: () => api<SystemLogFile[]>('/api/system-logs'),
  systemLogDetail: (name: string, maxBytes = 300000) => api<SystemLogDetail>(`/api/system-logs/detail${query({ name, maxBytes })}`),
  deleteSystemLog: (name: string) => api(`/api/system-logs/${encodeURIComponent(name)}`, { method: 'DELETE' }),
};
