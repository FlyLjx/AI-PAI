import { cookies } from 'next/headers';
import { NextResponse } from 'next/server';
import {
  ADMIN_EMAIL_COOKIE,
  ADMIN_ID_COOKIE,
  ADMIN_TOKEN_COOKIE,
  clearAdminCookies,
  requestGo,
} from '@/lib/admin-proxy';

export async function GET(request: Request) {
  const store = await cookies();
  const token = store.get(ADMIN_TOKEN_COOKIE)?.value;
  if (!token) return NextResponse.json({ message: '请先登录后台' }, { status: 401 });

  let upstream: Response;
  try {
    upstream = await requestGo(request, '/api/admin/session', token);
  } catch {
    return NextResponse.json({ message: 'Go 后端服务暂不可用' }, { status: 502 });
  }

  const payload = await upstream.json().catch(() => null) as { data?: { userId?: string }; message?: string } | null;
  const id = store.get(ADMIN_ID_COOKIE)?.value || '';
  const email = store.get(ADMIN_EMAIL_COOKIE)?.value || '';
  if (!upstream.ok || !payload?.data?.userId || payload.data.userId !== id || !email) {
    const response = NextResponse.json(payload || { message: '后台登录已失效' }, { status: upstream.ok ? 401 : upstream.status });
    clearAdminCookies(response);
    return response;
  }

  return NextResponse.json({ data: { id, email, role: 'admin' } });
}
