'use client';

import { useState } from 'react';
import { ArrowRight, Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';
import { StravaMark } from '@/components/ui/strava-mark';
import { browserFetch } from '@/lib/api-browser';
import type { StartOAuthResponse } from '@/lib/api-client';

export function ConnectStravaButton() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleClick() {
    setBusy(true);
    setError(null);
    try {
      const res = await browserFetch<StartOAuthResponse>('/v1/connections/strava/start');
      window.location.href = res.authorize_url;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start Strava connect');
      setBusy(false);
    }
  }

  return (
    <>
      <Button
        variant="strava"
        size="lg"
        className="w-full"
        onClick={handleClick}
        disabled={busy}
      >
        {busy ? <Spinner size={14} /> : <StravaMark size={18} />}
        Authorize with Strava
        <ArrowRight />
      </Button>
      {error ? (
        <p className="mt-3 text-center text-[12.5px] text-[var(--color-danger)]">
          {error}
        </p>
      ) : null}
    </>
  );
}
