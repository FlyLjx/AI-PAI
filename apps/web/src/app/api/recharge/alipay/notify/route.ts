import { proxyToGo } from '@/lib/go-proxy';

export function POST(request: Request) {
  return proxyToGo(request, '/api/recharge/alipay/notify');
}
