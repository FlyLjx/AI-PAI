'use client';

const API_BASE = '/api/backend';
const SESSION_KEY = 'aipai_portal_session';

export type Subscription = {
  id?: string;
  status?: string;
  tier?: string;
  isPaid?: boolean;
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
  token: string;
  emailVerifiedAt?: string | null;
  createdAt?: string;
  subscription?: Subscription | null;
};

export type RegistrationVerification = {
  verificationRequired: true;
  email: string;
  sent: boolean;
  verificationUrl?: string;
  message?: string;
};

export type RegisterResult = PortalUser | RegistrationVerification;

export type VerifiedAccount = Pick<PortalUser, 'id' | 'email' | 'role' | 'status'> &
  Partial<Pick<PortalUser, 'credits' | 'emailVerifiedAt' | 'createdAt' | 'subscription'>>;

export type PasswordResetRequest = {
  sent: boolean;
  resetUrl?: string;
  message?: string;
};

export type EmailChangeRequest = {
  sent: boolean;
  email: string;
  verificationUrl?: string;
  message?: string;
};

export type APIKey = {
  id: string;
  userId: string;
  userEmail?: string;
  name: string;
  keyPrefix: string;
  keyPlain?: string;
  key?: string;
  status: string;
  concurrencyLimit: number;
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

export type UsageSummary = {
  total: number;
  success: number;
  failed: number;
  imageCount: number;
};

export type UsageTrendPoint = {
  date: string;
  total: number;
  success: number;
  failed: number;
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

type Envelope<T> = {
  data: T;
  pagination?: { total: number; page: number; pageSize: number };
  summary?: UsageSummary;
};

export class APIError extends Error {
  constructor(message: string, public status: number) {
    super(message);
  }
}

export function getSession(): PortalUser | null {
  if (typeof window === 'undefined') return null;
  try {
    const value = JSON.parse(localStorage.getItem(SESSION_KEY) || 'null') as PortalUser | null;
    return value?.id && value.token ? value : null;
  } catch {
    return null;
  }
}

export function saveSession(user: PortalUser): PortalUser {
  localStorage.setItem(SESSION_KEY, JSON.stringify(user));
  window.dispatchEvent(new CustomEvent('aipai:session', { detail: user }));
  return user;
}

export function clearSession(): void {
  localStorage.removeItem(SESSION_KEY);
  window.dispatchEvent(new CustomEvent('aipai:session', { detail: null }));
}

function query(params: Record<string, unknown>): string {
  const search = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') search.set(key, String(value));
  });
  return search.size ? `?${search.toString()}` : '';
}

export async function api<T>(path: string, options: RequestInit = {}, token?: string): Promise<Envelope<T>> {
  const headers = new Headers(options.headers);
  if (options.body && !(options.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  if (token) headers.set('Authorization', `Bearer ${token}`);
  const response = await fetch(`${API_BASE}${path}`, { ...options, headers, cache: 'no-store' });
  if (!response.ok) {
    const payload = await response.json().catch(() => null) as { message?: string; error?: { message?: string } } | null;
    throw new APIError(payload?.message || payload?.error?.message || `请求失败 (${response.status})`, response.status);
  }
  if (response.status === 204) return { data: undefined as T };
  return response.json() as Promise<Envelope<T>>;
}

export async function login(email: string, password: string): Promise<PortalUser> {
  const { data } = await api<PortalUser>('/api/users/login', {
    method: 'POST', body: JSON.stringify({ email, password }),
  });
  return saveSession({ ...data, credits: Number(data.credits || 0) });
}

export function isRegistrationVerification(result: RegisterResult): result is RegistrationVerification {
  return 'verificationRequired' in result && result.verificationRequired === true;
}

export async function register(email: string, password: string): Promise<RegisterResult> {
  const { data } = await api<RegisterResult>('/api/users/register', {
    method: 'POST', body: JSON.stringify({ email, password }),
  });
  if (isRegistrationVerification(data)) return data;
  if (!data.id || !data.token) throw new APIError('注册响应缺少登录凭据', 502);
  return saveSession({ ...data, credits: Number(data.credits || 0) });
}

export async function verifyEmail(token: string): Promise<VerifiedAccount> {
  const { data } = await api<VerifiedAccount>('/api/users/verify-email', {
    method: 'POST', body: JSON.stringify({ token }),
  });
  return data;
}

export async function forgotPassword(email: string): Promise<PasswordResetRequest> {
  const { data } = await api<PasswordResetRequest>('/api/users/password/forgot', {
    method: 'POST', body: JSON.stringify({ email }),
  });
  return data;
}

export async function resetPassword(token: string, password: string): Promise<VerifiedAccount> {
  const { data } = await api<VerifiedAccount>('/api/users/password/reset', {
    method: 'POST', body: JSON.stringify({ token, password }),
  });
  return data;
}

export async function verifyEmailChange(token: string): Promise<VerifiedAccount> {
  const { data } = await api<VerifiedAccount>('/api/users/verify-email-change', {
    method: 'POST', body: JSON.stringify({ token }),
  });
  return data;
}

export async function refreshSession(user = getSession()): Promise<PortalUser> {
  if (!user) throw new APIError('请先登录', 401);
  const { data } = await api<PortalUser>(`/api/users/${encodeURIComponent(user.id)}/profile`, {}, user.token);
  return saveSession({ ...user, ...data, token: user.token, credits: Number(data.credits || 0) });
}

export function userToken(): string {
  return getSession()?.token || '';
}

export const portalApi = {
  publicSettings: () => api<Record<string, unknown>>('/api/settings/public'),
  listKeys: (user: PortalUser) => api<APIKey[]>(`/api/api-access/keys${query({ userId: user.id })}`, {}, user.token),
  createKey: (user: PortalUser, name: string) => api<APIKey>('/api/api-access/keys', { method: 'POST', body: JSON.stringify({ userId: user.id, name }) }, user.token),
  updateKey: (user: PortalUser, id: string, status: string) => api<APIKey>(`/api/api-access/keys/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify({ userId: user.id, status }) }, user.token),
  deleteKey: (user: PortalUser, id: string) => api(`/api/api-access/keys/${encodeURIComponent(id)}${query({ userId: user.id })}`, { method: 'DELETE' }, user.token),
  usage: (user: PortalUser, page = 1, pageSize = 20, keyword = '', status = '') => api<UsageLog[]>(`/api/api-access/logs${query({ userId: user.id, page, pageSize, keyword, status })}`, {}, user.token),
  usageTrend: (user: PortalUser, startDate: string, endDate: string) => api<UsageTrendPoint[]>(`/api/api-access/logs/trend${query({ userId: user.id, startDate, endDate })}`, {}, user.token),
  plans: () => api<Plan[]>('/api/subscriptions/public/plans'),
  subscription: (user: PortalUser) => api<Subscription | null>(`/api/subscriptions/public/current${query({ userId: user.id })}`, {}, user.token),
  recharge: (user: PortalUser, input: { amount?: number; subscriptionPlanId?: string }) => api<Record<string, unknown>>('/api/recharge', { method: 'POST', body: JSON.stringify({ userId: user.id, ...input }) }, user.token),
  rechargeOrder: (user: PortalUser, id: string) => api<Record<string, unknown>>(`/api/recharge/${encodeURIComponent(id)}${query({ userId: user.id })}`, {}, user.token),
  syncRecharge: (user: PortalUser, id: string) => api<Record<string, unknown>>(`/api/recharge/${encodeURIComponent(id)}/sync`, { method: 'POST', body: JSON.stringify({ userId: user.id }) }, user.token),
  requestEmailChange: (user: PortalUser, password: string, email: string) => api<EmailChangeRequest>(`/api/users/${encodeURIComponent(user.id)}/email`, { method: 'POST', body: JSON.stringify({ userId: user.id, password, email }) }, user.token),
  changePassword: (user: PortalUser, oldPassword: string, password: string) => api(`/api/users/${encodeURIComponent(user.id)}/password`, { method: 'PATCH', body: JSON.stringify({ userId: user.id, oldPassword, password }) }, user.token),
};
