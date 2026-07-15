'use client';

import Link from 'next/link';
import { Crown, Wallet } from 'lucide-react';
import type { PortalUser } from '@/lib/portal-api';

export function BillingRail({ user }: { user: PortalUser }) {
  const subscription = user.subscription;
  const paid = Boolean(subscription?.isPaid || subscription?.tier === 'paid');
  const remaining = Number(subscription?.effectiveQuotaRemaining ?? subscription?.quotaRemaining ?? 0);
  const limit = Number(subscription?.quotaLimit ?? 0);
  return (
    <Link href="/billing" className="billing-rail">
      <span className={`billing-icon ${paid ? 'is-subscription' : ''}`}>{paid ? <Crown size={17} /> : <Wallet size={17} />}</span>
      <span className="billing-copy">
        <small>{paid ? subscription?.planName || '订阅套餐' : '可用余额'}</small>
        <strong>{paid ? `${remaining.toLocaleString()} / ${limit.toLocaleString()}` : `¥ ${Number(user.credits || 0).toFixed(2)}`}</strong>
      </span>
    </Link>
  );
}
