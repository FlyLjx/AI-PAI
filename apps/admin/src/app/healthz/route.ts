import { forwardGoResponse, requestGo } from '@/lib/admin-proxy';

export async function GET(request: Request) {
  try {
    return forwardGoResponse(await requestGo(request, '/api/health'));
  } catch {
    return Response.json({ message: 'Go 后端服务暂不可用' }, { status: 502 });
  }
}
