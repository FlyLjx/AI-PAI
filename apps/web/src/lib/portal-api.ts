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
  inviteCode?: string;
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

export type RegistrationChallenge = {
  required: boolean;
  token?: string;
  minDelaySeconds?: number;
  expiresInSeconds?: number;
};

export type InviteRewardView = {
  type: 'none' | 'balance' | 'subscription';
  credits: number;
  planId?: string;
  planName?: string;
};

export type InviteRecord = {
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
  inviterIp?: string;
  inviteeIp?: string;
  verifiedAt?: string;
  rewardedAt?: string;
  rechargeRebateCount: number;
  rechargeRebateCredits: number;
  createdAt: string;
};

export type InviteRebateRecord = {
  id: string;
  inviteId: string;
  orderId: string;
  inviterId: string;
  inviteeId: string;
  inviteeEmail?: string;
  orderType: string;
  orderAmount: number;
  rechargeRate: number;
  rebatePercent: number;
  rebateCredits: number;
  outTradeNo?: string;
  createdAt: string;
};

export type InviteSummary = {
  enabled: boolean;
  inviteCode: string;
  inviteCount: number;
  pendingCount: number;
  blockedCount: number;
  totalBalanceRewards: number;
  totalSubscriptionRewards: number;
  rechargeRebateEnabled: boolean;
  rechargeRebatePercent: number;
  rebateIncludeSubscriptions: boolean;
  rechargeRebateCount: number;
  rechargeRebateTotal: number;
  rewardText: string;
  inviteeRewardText: string;
  inviterReward: InviteRewardView;
  inviteeReward: InviteRewardView;
  records: InviteRecord[];
  rebateRecords: InviteRebateRecord[];
  receivedInvite?: InviteRecord | null;
};

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

export type APIKeyBillingMode = 'balance' | 'subscription' | 'auto';
export type SelectableAPIKeyBillingMode = Exclude<APIKeyBillingMode, 'auto'>;

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

export type CompatibleModel = {
  id: string;
  object: string;
  enabled_size_tiers?: Array<'1k' | '2k' | '4k'>;
};

export type PricingModel = {
  id: string;
  displayName: string;
  price1k: number;
  price2k: number;
  price4k: number;
  enabledSizeTiers: Array<'1k' | '2k' | '4k'>;
  sortOrder: number;
  updatedAt: string;
};

export type ImageGenerationInput = {
  model: string;
  prompt: string;
  n: number;
  size_tier: '1k' | '2k' | '4k';
  aspect_ratio: '1:1' | '16:9' | '9:16' | '4:3' | '3:4' | '3:2' | '2:3';
  output_format: 'jpeg' | 'png' | 'webp';
  response_format?: 'url' | 'b64_json';
};

export type ImageGenerationResult = {
  created: number;
  data: Array<{ url?: string; b64_json?: string }>;
};

export type UsageLog = {
  id: string;
  userId: string;
  userEmail?: string;
  keyName?: string;
  keyPrefix?: string;
  endpoint: string;
  model: string;
  prompt: string;
  size: string;
  quality: string;
  quantity: number;
  imageCount: number;
  status: string;
  errorMessage?: string;
  chargedCredits: number;
  durationSeconds: number;
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

export type OpenAIAffectedComponent = {
  name: string;
  status: string;
  label: string;
};

export type OpenAIImageIncident = {
  title: string;
  link: string;
  guid: string;
  pubDate: string;
  publishedAt?: string;
  status: string;
  statusLabel: string;
  severity: 'ok' | 'warning' | 'critical' | string;
  summary: string;
  affectedComponents: OpenAIAffectedComponent[];
};

export type OpenAIImageStatusSnapshot = {
  reachable: boolean;
  status: 'operational' | 'monitoring' | 'degraded' | 'partial_outage' | 'outage' | 'unreachable' | string;
  statusLabel: string;
  severity: 'ok' | 'warning' | 'critical' | string;
  summary: string;
  source: string;
  feedTitle?: string;
  feedLink?: string;
  lastBuildDate?: string;
  fetchedAt: string;
  upstream_status_code: number;
  latestImageIncident?: OpenAIImageIncident | null;
  imageIncidents: OpenAIImageIncident[];
  affectedComponents: OpenAIAffectedComponent[];
  totalImageIncidents: number;
  error?: string;
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

export type RechargeOrder = {
  id: string;
  outTradeNo: string;
  tradeNo?: string | null;
  orderType: string;
  subscriptionPlanId?: string | null;
  amount: number;
  status: string;
  payUrl?: string | null;
  qrCode?: string | null;
  paidAt?: string | null;
  createdAt: string;
  updatedAt?: string;
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

export async function registrationChallenge(): Promise<RegistrationChallenge> {
  const { data } = await api<RegistrationChallenge>('/api/users/register/challenge');
  return data;
}

export async function register(email: string, password: string, options: {
  inviteCode?: string;
  deviceId?: string;
  challengeToken?: string;
  website?: string;
} = {}): Promise<RegisterResult> {
  const { data } = await api<RegisterResult>('/api/users/register', {
    method: 'POST', body: JSON.stringify({ email, password, ...options }),
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

async function openAIRequest<T>(path: string, apiKey: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set('Authorization', `Bearer ${apiKey}`);
  if (options.body && !(options.body instanceof FormData) && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');

  const response = await fetch(path, { ...options, headers, cache: 'no-store' });
  const payload = await response.json().catch(() => null) as (T & {
    message?: string;
    error?: { message?: string };
  }) | null;
  if (!response.ok) {
    throw new APIError(payload?.error?.message || payload?.message || `请求失败 (${response.status})`, response.status);
  }
  if (!payload) throw new APIError('服务端返回了无效响应', 502);
  return payload;
}

export const portalApi = {
  publicSettings: () => api<Record<string, unknown>>('/api/settings/public'),
  pricingModels: () => api<PricingModel[]>('/api/models/pricing'),
  announcements: (user: PortalUser) => api<Announcement[]>(`/api/announcements/public${query({ userId: user.id })}`, {}, user.token),
  signAnnouncement: (user: PortalUser, id: string) => api<{ signed: boolean }>(`/api/announcements/${encodeURIComponent(id)}/sign`, { method: 'POST', body: JSON.stringify({ userId: user.id }) }, user.token),
  listKeys: (user: PortalUser) => api<APIKey[]>(`/api/api-access/keys${query({ userId: user.id })}`, {}, user.token),
  createKey: (user: PortalUser, name: string, billingMode: SelectableAPIKeyBillingMode) => api<APIKey>('/api/api-access/keys', { method: 'POST', body: JSON.stringify({ userId: user.id, name, billingMode }) }, user.token),
  revealKey: (user: PortalUser, id: string) => api<{ key: string }>(`/api/api-access/keys/${encodeURIComponent(id)}/reveal`, { method: 'POST' }, user.token),
  updateKey: (user: PortalUser, id: string, status: string) => api<APIKey>(`/api/api-access/keys/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify({ userId: user.id, status }) }, user.token),
  deleteKey: (user: PortalUser, id: string) => api(`/api/api-access/keys/${encodeURIComponent(id)}${query({ userId: user.id })}`, { method: 'DELETE' }, user.token),
  usage: (user: PortalUser, page = 1, pageSize = 20, keyword = '', status = '') => api<UsageLog[]>(`/api/api-access/logs${query({ userId: user.id, page, pageSize, keyword, status })}`, {}, user.token),
  usageTrend: (user: PortalUser, startDate: string, endDate: string) => api<UsageTrendPoint[]>(`/api/api-access/logs/trend${query({ userId: user.id, startDate, endDate })}`, {}, user.token),
  stability: () => api<StabilitySnapshot>('/api/upstream/stability'),
  openAIImageStatus: () => api<OpenAIImageStatusSnapshot>('/api/upstream/openai-status'),
  plans: () => api<Plan[]>('/api/subscriptions/public/plans'),
  subscription: (user: PortalUser) => api<Subscription | null>(`/api/subscriptions/public/current${query({ userId: user.id })}`, {}, user.token),
  recharge: (user: PortalUser, input: { amount?: number; subscriptionPlanId?: string }) => api<Record<string, unknown>>('/api/recharge', { method: 'POST', body: JSON.stringify({ userId: user.id, ...input }) }, user.token),
  rechargeHistory: (user: PortalUser, page = 1, pageSize = 10) => api<RechargeOrder[]>(`/api/recharge/history${query({ userId: user.id, page, pageSize })}`, {}, user.token),
  rechargeOrder: (user: PortalUser, id: string) => api<Record<string, unknown>>(`/api/recharge/${encodeURIComponent(id)}${query({ userId: user.id })}`, {}, user.token),
  syncRecharge: (user: PortalUser, id: string) => api<Record<string, unknown>>(`/api/recharge/${encodeURIComponent(id)}/sync`, { method: 'POST', body: JSON.stringify({ userId: user.id }) }, user.token),
  requestEmailChange: (user: PortalUser, password: string, email: string) => api<EmailChangeRequest>(`/api/users/${encodeURIComponent(user.id)}/email`, { method: 'POST', body: JSON.stringify({ userId: user.id, password, email }) }, user.token),
  resendEmailVerification: (user: PortalUser) => api<RegistrationVerification>(`/api/users/${encodeURIComponent(user.id)}/resend-verification`, { method: 'POST' }, user.token),
  inviteSummary: (user: PortalUser) => api<InviteSummary>(`/api/invites/summary${query({ userId: user.id })}`, {}, user.token),
  changePassword: (user: PortalUser, oldPassword: string, password: string) => api(`/api/users/${encodeURIComponent(user.id)}/password`, { method: 'PATCH', body: JSON.stringify({ userId: user.id, oldPassword, password }) }, user.token),
  compatibleModels: (apiKey: string) => openAIRequest<{ object: string; data: CompatibleModel[] }>('/v1/models', apiKey),
  generateImages: (apiKey: string, input: ImageGenerationInput, referenceImages: File[] = []) => {
    const headers = new Headers();
    headers.set('X-Aipi-Image-Result-Mode', 'b64');
    const requestInput: ImageGenerationInput = { ...input, response_format: 'b64_json' };
    if (referenceImages.length === 0) {
      return openAIRequest<ImageGenerationResult>('/v1/images/generations', apiKey, {
        method: 'POST',
        headers,
        body: JSON.stringify(requestInput),
      });
    }
    const body = new FormData();
    body.set('model', requestInput.model);
    body.set('prompt', requestInput.prompt);
    body.set('n', String(requestInput.n));
    body.set('size_tier', requestInput.size_tier);
    body.set('aspect_ratio', requestInput.aspect_ratio);
    body.set('output_format', requestInput.output_format);
    body.set('response_format', 'b64_json');
    referenceImages.forEach((file) => body.append('image[]', file, file.name));
    return openAIRequest<ImageGenerationResult>('/v1/images/edits', apiKey, {
      method: 'POST',
      headers,
      body,
    });
  },
};
