import { cookies } from 'next/headers';
import { ADMIN_TOKEN_COOKIE, forwardGoResponse, isSameOriginRequest, requestGo } from '@/lib/admin-proxy';

type Context = { params: Promise<{ path: string[] }> };
type Rule = { pattern: RegExp; methods: string[] };

const ADMIN_ROUTES: Rule[] = [
  { pattern: /^\/api\/dashboard$/, methods: ['GET'] },
  { pattern: /^\/api\/upstream\/stability$/, methods: ['GET'] },
  { pattern: /^\/api\/users$/, methods: ['GET', 'POST'] },
  { pattern: /^\/api\/users\/[^/]+$/, methods: ['PATCH', 'DELETE'] },
  { pattern: /^\/api\/users\/[^/]+\/balance$/, methods: ['PATCH'] },
  { pattern: /^\/api\/users\/[^/]+\/credit-logs$/, methods: ['GET'] },
  { pattern: /^\/api\/users\/[^/]+\/verify-email$/, methods: ['POST'] },
  { pattern: /^\/api\/users\/[^/]+\/subscription$/, methods: ['POST'] },
  { pattern: /^\/api\/api-providers$/, methods: ['GET', 'POST'] },
  { pattern: /^\/api\/api-providers\/[^/]+$/, methods: ['PATCH', 'DELETE'] },
  { pattern: /^\/api\/api-providers\/[^/]+\/test$/, methods: ['POST'] },
  { pattern: /^\/api\/api-providers\/[^/]+\/models$/, methods: ['GET'] },
  { pattern: /^\/api\/models$/, methods: ['GET', 'POST'] },
  { pattern: /^\/api\/models\/[^/]+$/, methods: ['PATCH', 'DELETE'] },
  { pattern: /^\/api\/subscriptions\/plans$/, methods: ['GET', 'POST'] },
  { pattern: /^\/api\/subscriptions\/plans\/[^/]+$/, methods: ['PATCH', 'DELETE'] },
  { pattern: /^\/api\/recharge\/orders$/, methods: ['GET'] },
  { pattern: /^\/api\/invites$/, methods: ['GET'] },
  { pattern: /^\/api\/admin\/api-access\/keys$/, methods: ['GET'] },
  { pattern: /^\/api\/admin\/api-access\/keys\/[^/]+$/, methods: ['PATCH', 'DELETE'] },
  { pattern: /^\/api\/admin\/api-access\/logs$/, methods: ['GET'] },
  { pattern: /^\/api\/admin\/api-access\/operations$/, methods: ['GET'] },
  { pattern: /^\/api\/admin\/mail-logs$/, methods: ['GET'] },
  { pattern: /^\/api\/admin\/system-update$/, methods: ['GET', 'POST'] },
  { pattern: /^\/api\/tasks\/[^/]+\/cancel$/, methods: ['POST'] },
  { pattern: /^\/api\/settings$/, methods: ['GET', 'PATCH'] },
  { pattern: /^\/api\/system-logs$/, methods: ['GET'] },
  { pattern: /^\/api\/system-logs\/detail$/, methods: ['GET'] },
  { pattern: /^\/api\/system-logs\/[^/]+$/, methods: ['DELETE'] },
];

function isAllowed(path: string, method: string) {
  return ADMIN_ROUTES.some((rule) => rule.pattern.test(path) && rule.methods.includes(method));
}

async function proxy(request: Request, context: Context) {
  const { path } = await context.params;
  const targetPath = `/${path.map(encodeURIComponent).join('/')}`;
  if (!isAllowed(targetPath, request.method)) {
    return Response.json({ message: '该接口不属于管理后台' }, { status: 404 });
  }
  if (request.method !== 'GET' && !isSameOriginRequest(request)) {
    return Response.json({ message: '后台请求来源校验失败' }, { status: 403 });
  }

  const token = (await cookies()).get(ADMIN_TOKEN_COOKIE)?.value;
  if (!token) return Response.json({ message: '请先登录后台' }, { status: 401 });

  try {
    return forwardGoResponse(await requestGo(request, targetPath, token));
  } catch {
    return Response.json({ message: 'Go 后端服务暂不可用' }, { status: 502 });
  }
}

export const GET = proxy;
export const POST = proxy;
export const PATCH = proxy;
export const DELETE = proxy;
