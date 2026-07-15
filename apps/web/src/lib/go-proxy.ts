const FORWARDED_HEADERS = [
  'accept', 'authorization', 'content-type', 'idempotency-key',
  'x-api-key', 'x-request-id', 'user-agent', 'origin',
];

export async function proxyToGo(request: Request, path: string): Promise<Response> {
  const backend = (process.env.GO_BACKEND_URL || 'http://127.0.0.1:3001').replace(/\/$/, '');
  const incoming = new URL(request.url);
  let forwardedOrigin: URL | null = process.env.NODE_ENV === 'production' ? null : incoming;
  const configuredOrigin = process.env.APP_PUBLIC_ORIGIN?.trim();
  if (configuredOrigin) {
    try {
      const parsed = new URL(configuredOrigin);
      if (parsed.protocol === 'http:' || parsed.protocol === 'https:') forwardedOrigin = parsed;
    } catch {
      // Production requests fail closed below when this value is invalid.
    }
  }
  if (!forwardedOrigin) {
    return Response.json({ message: '客户站公开来源地址未配置' }, { status: 500 });
  }
  const target = `${backend}${path}${incoming.search}`;
  const headers = new Headers();
  for (const name of FORWARDED_HEADERS) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }
  headers.set('x-forwarded-host', forwardedOrigin.host);
  headers.set('x-forwarded-proto', forwardedOrigin.protocol.replace(':', ''));

  try {
    const upstream = await fetch(target, {
      method: request.method,
      headers,
      body: request.method === 'GET' || request.method === 'HEAD' ? undefined : await request.arrayBuffer(),
      cache: 'no-store',
      redirect: 'manual',
    });
    const responseHeaders = new Headers();
    for (const name of [
      'content-type', 'content-disposition', 'cache-control', 'x-request-id',
      'location',
      'access-control-allow-origin', 'access-control-allow-credentials',
      'access-control-allow-headers', 'access-control-allow-methods', 'vary',
    ]) {
      const value = upstream.headers.get(name);
      if (value) responseHeaders.set(name, value);
    }
    return new Response(upstream.body, { status: upstream.status, headers: responseHeaders });
  } catch {
    return Response.json({ message: 'Go 后端服务暂不可用' }, { status: 502 });
  }
}
