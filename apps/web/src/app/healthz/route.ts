import { proxyToGo } from '@/lib/go-proxy';

export function GET(request: Request) {
  return proxyToGo(request, '/api/health');
}
