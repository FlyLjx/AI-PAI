import { type NextRequest, NextResponse } from 'next/server';

const DEFAULT_ADMIN_INTERNAL_URL = 'http://127.0.0.1:3002';
const APP_CACHE_CONTROL = 'private, no-cache, no-store, max-age=0, must-revalidate';

function disablePageCache(response: NextResponse) {
  response.headers.set('Cache-Control', APP_CACHE_CONTROL);
  response.headers.set('CDN-Cache-Control', 'no-store');
  return response;
}

export function proxy(request: NextRequest) {
  if (!request.nextUrl.pathname.startsWith('/sys-admins')) {
    return disablePageCache(NextResponse.next());
  }

  const configured = process.env.ADMIN_INTERNAL_URL || DEFAULT_ADMIN_INTERNAL_URL;
  let adminOrigin: URL;
  try {
    adminOrigin = new URL(configured);
    if (!['http:', 'https:'].includes(adminOrigin.protocol)) throw new Error('unsupported protocol');
  } catch {
    return NextResponse.json({ message: '管理后台内部地址配置错误' }, { status: 500 });
  }

  const destination = new URL(`${request.nextUrl.pathname}${request.nextUrl.search}`, adminOrigin);
  return disablePageCache(NextResponse.rewrite(destination));
}

export const config = {
  matcher: [
    '/sys-admins',
    '/sys-admins/:path*',
    '/((?!_next/static|_next/image|favicon.ico|.*\\.(?:svg|png|jpg|jpeg|gif|webp|ico|css|js|woff|woff2)$).*)',
  ],
};
