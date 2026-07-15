'use client';

import React from 'react';
import { AppShell } from '@/components/common/AppShell';

export default function UserLayout({ children }: { children: React.ReactNode }) {
  return <AppShell>{children}</AppShell>;
}
