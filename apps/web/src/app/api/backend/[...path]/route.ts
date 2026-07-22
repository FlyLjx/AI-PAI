import { proxyToGo } from '@/lib/go-proxy';

type Context = { params: Promise<{ path: string[] }> };
type Rule = { pattern: RegExp; methods: string[] };

const CUSTOMER_ROUTES: Rule[] = [
  { pattern: /^\/api\/settings\/public$/, methods: ['GET'] },
  { pattern: /^\/api\/models\/pricing$/, methods: ['GET'] },
  { pattern: /^\/api\/announcements\/public$/, methods: ['GET'] },
  { pattern: /^\/api\/announcements\/[^/]+\/sign$/, methods: ['POST'] },
  { pattern: /^\/api\/users\/(login|register|verify-email|verify-email-change)$/, methods: ['POST'] },
  { pattern: /^\/api\/users\/register\/challenge$/, methods: ['GET'] },
  { pattern: /^\/api\/users\/password\/(forgot|reset)$/, methods: ['POST'] },
  { pattern: /^\/api\/users\/[^/]+\/profile$/, methods: ['GET'] },
  { pattern: /^\/api\/users\/[^/]+\/password$/, methods: ['PATCH'] },
  { pattern: /^\/api\/users\/[^/]+\/email$/, methods: ['POST'] },
  { pattern: /^\/api\/users\/[^/]+\/resend-verification$/, methods: ['POST'] },
  { pattern: /^\/api\/api-access\/keys$/, methods: ['GET', 'POST'] },
  { pattern: /^\/api\/api-access\/keys\/[^/]+$/, methods: ['PATCH', 'DELETE'] },
  { pattern: /^\/api\/api-access\/keys\/[^/]+\/reveal$/, methods: ['POST'] },
  { pattern: /^\/api\/api-access\/logs$/, methods: ['GET'] },
  { pattern: /^\/api\/api-access\/logs\/trend$/, methods: ['GET'] },
  { pattern: /^\/api\/upstream\/stability$/, methods: ['GET'] },
  { pattern: /^\/api\/upstream\/openai-status$/, methods: ['GET'] },
  { pattern: /^\/api\/invites\/summary$/, methods: ['GET'] },
  { pattern: /^\/api\/subscriptions\/public\/(plans|current)$/, methods: ['GET'] },
  { pattern: /^\/api\/recharge\/qr-code$/, methods: ['GET'] },
  { pattern: /^\/api\/recharge\/history$/, methods: ['GET'] },
  { pattern: /^\/api\/recharge$/, methods: ['POST'] },
  { pattern: /^\/api\/recharge\/(?!orders(?:\/|$)|qr-code(?:\/|$)|history(?:\/|$)|alipay(?:\/|$))[^/]+$/, methods: ['GET'] },
  { pattern: /^\/api\/recharge\/(?!orders(?:\/|$)|qr-code(?:\/|$)|alipay(?:\/|$))[^/]+\/sync$/, methods: ['POST'] },
];

function isCustomerRoute(path: string, method: string): boolean {
  return CUSTOMER_ROUTES.some((rule) => rule.pattern.test(path) && rule.methods.includes(method));
}

function exposesAuthActionURLs(): boolean {
  const configured = process.env.AUTH_ACTION_URLS_IN_RESPONSE;
  return configured === 'true' || (configured === undefined && process.env.NODE_ENV !== 'production');
}

function isAuthActionPath(path: string): boolean {
  return ['/api/users/register', '/api/users/password/forgot'].includes(path) || /^\/api\/users\/[^/]+\/(?:email|resend-verification)$/.test(path);
}

async function sanitizeAuthResponse(response: Response, path: string): Promise<Response> {
  if (exposesAuthActionURLs() || !response.ok || !isAuthActionPath(path)) {
    return response;
  }
  const payload = await response.clone().json().catch(() => null) as { data?: Record<string, unknown> } | null;
  if (!payload?.data) return response;

  delete payload.data.resetUrl;
  delete payload.data.verificationUrl;
  if (path === '/api/users/password/forgot') {
    payload.data.message = '若该邮箱已注册，密码重置说明将发送到对应邮箱。';
  }

  const headers = new Headers(response.headers);
  headers.delete('content-length');
  return Response.json(payload, { status: response.status, headers });
}

async function proxy(request: Request, context: Context) {
  const { path } = await context.params;
  const targetPath = `/${path.map(encodeURIComponent).join('/')}`;
  if (!isCustomerRoute(targetPath, request.method)) {
    return Response.json({ message: '该接口不属于客户前台' }, { status: 404 });
  }
  return sanitizeAuthResponse(await proxyToGo(request, targetPath), targetPath);
}

export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const PATCH = proxy;
export const DELETE = proxy;
export const OPTIONS = proxy;
