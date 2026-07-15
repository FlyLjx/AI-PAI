import { NextResponse } from 'next/server';
import { clearAdminCookies, isSameOriginRequest } from '@/lib/admin-proxy';

export function POST(request: Request) {
  if (!isSameOriginRequest(request)) {
    return NextResponse.json({ message: '后台请求来源校验失败' }, { status: 403 });
  }
  const response = NextResponse.json({ data: null });
  clearAdminCookies(response);
  return response;
}
