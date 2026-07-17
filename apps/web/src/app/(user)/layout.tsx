import type { ReactNode } from 'react';
import { AppShell } from '@/components/common/AppShell';

export const dynamic = 'force-dynamic';
export const revalidate = 0;

export default function UserLayout({ children }: { children: ReactNode }) {
  return <AppShell>{children}</AppShell>;
}
