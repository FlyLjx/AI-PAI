import { NextResponse } from 'next/server';
import {
  ADMIN_EMAIL_COOKIE,
  ADMIN_ID_COOKIE,
  ADMIN_TOKEN_COOKIE,
  adminCookieOptions,
  isSameOriginRequest,
  requestGo,
} from '@/lib/admin-proxy';

type LoginPayload = {
  data?: {
    user?: { id?: string; email?: string; role?: string };
    token?: string;
  };
  message?: string;
};

export async function POST(request: Request) {
  if (!isSameOriginRequest(request)) {
    return NextResponse.json({ message: '后台请求来源校验失败' }, { status: 403 });
  }
  let upstream: Response;
  try {
    upstream = await requestGo(request, '/api/admin/login');
  } catch {
    return NextResponse.json({ message: 'Go 后端服务暂不可用' }, { status: 502 });
  }

  const payload = await upstream.json().catch(() => null) as LoginPayload | null;
  if (!upstream.ok) {
    return NextResponse.json(payload || { message: '后台登录失败' }, { status: upstream.status });
  }

  const user = payload?.data?.user;
  const token = payload?.data?.token;
  if (!user?.id || !user.email || user.role !== 'admin' || !token) {
    return NextResponse.json({ message: '后台登录响应不完整' }, { status: 502 });
  }

  const response = NextResponse.json({ data: { id: user.id, email: user.email, role: 'admin' } });
  response.cookies.set(ADMIN_TOKEN_COOKIE, token, adminCookieOptions);
  response.cookies.set(ADMIN_ID_COOKIE, user.id, adminCookieOptions);
  response.cookies.set(ADMIN_EMAIL_COOKIE, user.email, adminCookieOptions);
  return response;
}
