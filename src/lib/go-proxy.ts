const FORWARDED_HEADERS = [
  'accept', 'authorization', 'content-type', 'cookie', 'idempotency-key',
  'x-api-key', 'x-request-id', 'user-agent', 'origin',
];

export async function proxyToGo(request: Request, path: string): Promise<Response> {
  const backend = (process.env.GO_BACKEND_URL || 'http://127.0.0.1:3001').replace(/\/$/, '');
  const incoming = new URL(request.url);
  const target = `${backend}${path}${incoming.search}`;
  const headers = new Headers();
  for (const name of FORWARDED_HEADERS) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }
  headers.set('x-forwarded-host', incoming.host);
  headers.set('x-forwarded-proto', incoming.protocol.replace(':', ''));

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
