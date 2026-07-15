import { cookies } from 'next/headers';
import { redirect } from 'next/navigation';
import { ADMIN_TOKEN_COOKIE } from '@/lib/admin-proxy';

export default async function Home() {
  const store = await cookies();
  redirect(store.has(ADMIN_TOKEN_COOKIE) ? '/dashboard' : '/login');
}
