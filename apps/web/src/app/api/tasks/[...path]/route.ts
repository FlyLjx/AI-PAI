import { proxyToGo } from '@/lib/go-proxy';

type Context = { params: Promise<{ path: string[] }> };

export async function GET(request: Request, context: Context) {
  const { path } = await context.params;
  const targetPath = `/api/tasks/${path.map(encodeURIComponent).join('/')}`;
  if (!/^\/api\/tasks\/[^/]+\/(?:images|thumbnails)\/\d+(?:\/download)?$/.test(targetPath)) {
    return Response.json({ message: '该任务资源不对客户前台开放' }, { status: 404 });
  }
  return proxyToGo(request, targetPath);
}
