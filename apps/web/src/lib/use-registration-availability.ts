'use client';

import { useEffect, useState } from 'react';
import { portalApi } from '@/lib/portal-api';

export type RegistrationAvailability = 'loading' | 'open' | 'closed';

export function useRegistrationAvailability(): RegistrationAvailability {
  const [availability, setAvailability] = useState<RegistrationAvailability>('loading');

  useEffect(() => {
    let active = true;
    void portalApi.publicSettings()
      .then(({ data }) => {
        if (active) setAvailability(String(data.registerMode || '').toLowerCase() === 'closed' ? 'closed' : 'open');
      })
      .catch(() => {
        if (active) setAvailability('open');
      });
    return () => { active = false; };
  }, []);

  return availability;
}
