import { type NextRequest, NextResponse } from 'next/server';

const DEFAULT_ADMIN_INTERNAL_URL = 'http://127.0.0.1:3002';

export function proxy(request: NextRequest) {
  const configured = process.env.ADMIN_INTERNAL_URL || DEFAULT_ADMIN_INTERNAL_URL;
  let adminOrigin: URL;
  try {
    adminOrigin = new URL(configured);
    if (!['http:', 'https:'].includes(adminOrigin.protocol)) throw new Error('unsupported protocol');
  } catch {
    return NextResponse.json({ message: '管理后台内部地址配置错误' }, { status: 500 });
  }

  const destination = new URL(`${request.nextUrl.pathname}${request.nextUrl.search}`, adminOrigin);
  return NextResponse.rewrite(destination);
}

export const config = {
  matcher: ['/sys-admins', '/sys-admins/:path*'],
};
