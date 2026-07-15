import { ADMIN_BASE_PATH } from '../../admin-path';

const FORWARDED_HEADERS = [
  'accept', 'content-type', 'idempotency-key', 'x-request-id', 'user-agent', 'origin',
];

export const ADMIN_TOKEN_COOKIE = 'aipai_sys_admin_token';
export const ADMIN_ID_COOKIE = 'aipai_sys_admin_id';
export const ADMIN_EMAIL_COOKIE = 'aipai_sys_admin_email';

const LEGACY_ADMIN_COOKIES = ['aipai_admin_token', 'aipai_admin_id', 'aipai_admin_email'];

export const adminCookieOptions = {
  httpOnly: true,
  sameSite: 'strict' as const,
  secure: process.env.ADMIN_COOKIE_SECURE === 'true',
  path: ADMIN_BASE_PATH,
  maxAge: 12 * 60 * 60,
};

export function isSameOriginRequest(request: Request): boolean {
  const origin = request.headers.get('origin');
  if (!origin) return false;
  const configuredOrigin = (process.env.ADMIN_PUBLIC_ORIGIN || process.env.APP_PUBLIC_ORIGIN)?.trim().replace(/\/$/, '');
  if (configuredOrigin) return origin === configuredOrigin;
  if (process.env.NODE_ENV === 'production') return false;

  const requestOrigin = new URL(request.url).origin;
  const forwardedHost = request.headers.get('x-forwarded-host')?.split(',')[0]?.trim();
  const forwardedProto = request.headers.get('x-forwarded-proto')?.split(',')[0]?.trim();
  const forwardedOrigin = forwardedHost ? `${forwardedProto || 'http'}://${forwardedHost}` : '';
  return [requestOrigin, forwardedOrigin, 'http://127.0.0.1:3000', 'http://localhost:3000'].includes(origin);
}

export async function requestGo(request: Request, path: string, token?: string): Promise<Response> {
  const backend = (process.env.GO_BACKEND_URL || 'http://127.0.0.1:3001').replace(/\/$/, '');
  const incoming = new URL(request.url);
  const headers = new Headers();
  for (const name of FORWARDED_HEADERS) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }
  if (token) headers.set('authorization', `Bearer ${token}`);
  headers.set('x-forwarded-host', incoming.host);
  headers.set('x-forwarded-proto', incoming.protocol.replace(':', ''));

  return fetch(`${backend}${path}${incoming.search}`, {
    method: request.method,
    headers,
    body: request.method === 'GET' || request.method === 'HEAD' ? undefined : await request.arrayBuffer(),
    cache: 'no-store',
    redirect: 'manual',
  });
}

export function forwardGoResponse(upstream: Response): Response {
  const headers = new Headers();
  for (const name of ['content-type', 'content-disposition', 'cache-control', 'x-request-id', 'location']) {
    const value = upstream.headers.get(name);
    if (value) headers.set(name, value);
  }
  return new Response(upstream.body, { status: upstream.status, headers });
}

type CookieResponse = { cookies: { set: (name: string, value: string, options: typeof adminCookieOptions) => unknown } };

export function clearLegacyAdminCookies(response: CookieResponse) {
  const expired = { ...adminCookieOptions, path: '/', maxAge: 0 };
  for (const name of LEGACY_ADMIN_COOKIES) response.cookies.set(name, '', expired);
}

export function clearAdminCookies(response: CookieResponse) {
  const expired = { ...adminCookieOptions, maxAge: 0 };
  response.cookies.set(ADMIN_TOKEN_COOKIE, '', expired);
  response.cookies.set(ADMIN_ID_COOKIE, '', expired);
  response.cookies.set(ADMIN_EMAIL_COOKIE, '', expired);
  clearLegacyAdminCookies(response);
}
