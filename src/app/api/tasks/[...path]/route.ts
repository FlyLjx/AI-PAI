import { proxyToGo } from '@/lib/go-proxy';

type Context = { params: Promise<{ path: string[] }> };

export async function GET(request: Request, context: Context) {
  const { path } = await context.params;
  return proxyToGo(request, `/api/tasks/${path.map(encodeURIComponent).join('/')}`);
}
