'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { ArrowRight, Button } from '@/components/ui/button';
import { GlassCard } from '@/components/ui/glass-card';
import { Spinner } from '@/components/ui/spinner';
import { StravaMark } from '@/components/ui/strava-mark';
import { browserFetch } from '@/lib/api-browser';
import type { Connection, StartOAuthResponse } from '@/lib/api-client';

interface Props {
  connection: Connection | null;
}

/**
 * Settings → Strava connection card. Two states:
 *   - not connected → CTA that fetches the authorize URL and navigates
 *   - connected     → athlete name, scopes, connected-since, disconnect
 *
 * After disconnect we call `router.refresh()` so the page's server fetch
 * (`/v1/me/sync`) reruns and the new "not connected" state lands without
 * a full page navigation.
 */
export function ConnectionCard({ connection }: Props) {
  const router = useRouter();
  const [busy, setBusy] = useState<'connect' | 'disconnect' | null>(null);
  const [error, setError] = useState<string | null>(null);

  const isConnected = connection?.connected ?? false;

  async function handleConnect() {
    setBusy('connect');
    setError(null);
    try {
      const res = await browserFetch<StartOAuthResponse>('/v1/connections/strava/start');
      window.location.href = res.authorize_url;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start Strava connect');
      setBusy(null);
    }
  }

  async function handleDisconnect() {
    if (!confirm('Disconnect Strava? Your activity history stays in Cadence; new uploads will stop syncing.')) {
      return;
    }
    setBusy('disconnect');
    setError(null);
    try {
      await browserFetch('/v1/connections/strava', { method: 'DELETE' });
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to disconnect');
    } finally {
      setBusy(null);
    }
  }

  return (
    <GlassCard className="relative overflow-hidden p-7 md:p-8">
      <div
        className="pointer-events-none absolute -right-16 -top-16 h-56 w-56 rounded-full opacity-50"
        style={{
          background:
            'radial-gradient(closest-side, oklch(0.72 0.22 45 / 0.55) 0%, transparent 70%)',
          filter: 'blur(36px)',
        }}
      />
      <div className="relative">
        <div className="flex items-center gap-3">
          <span
            className="flex h-10 w-10 items-center justify-center rounded-xl text-white"
            style={{
              background:
                'linear-gradient(135deg, oklch(0.72 0.22 45) 0%, oklch(0.58 0.22 38) 100%)',
            }}
            aria-hidden
          >
            <StravaMark size={18} />
          </span>
          <div>
            <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
              STRAVA CONNECTION
            </p>
            <h3 className="mt-1 text-[20px] font-medium leading-[1.1] tracking-[-0.02em] text-white">
              {isConnected ? connection?.athlete_name || 'Connected' : 'Not connected'}
            </h3>
          </div>
        </div>

        {isConnected ? (
          <>
            <dl className="mt-6 grid grid-cols-1 gap-4 text-[13px] sm:grid-cols-2">
              <div>
                <dt className="font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/40">
                  CONNECTED SINCE
                </dt>
                <dd className="num mt-1 text-white/85">
                  {formatDate(connection?.connected_at)}
                </dd>
              </div>
              <div>
                <dt className="font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/40">
                  SCOPES
                </dt>
                <dd className="mt-1 flex flex-wrap gap-1.5">
                  {(connection?.scopes ?? []).map((s) => (
                    <span
                      key={s}
                      className="rounded-full border border-white/10 bg-white/[0.04] px-2 py-0.5 font-mono text-[10.5px] tracking-[0.14em] text-white/60"
                    >
                      {s}
                    </span>
                  ))}
                </dd>
              </div>
            </dl>

            <div className="mt-7 flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={handleDisconnect}
                disabled={busy !== null}
                className="inline-flex h-10 items-center gap-2 rounded-full border border-white/10 bg-white/[0.04] px-4 text-[13px] text-white/75 backdrop-blur-md transition-colors hover:border-[var(--color-danger)]/40 hover:bg-[var(--color-danger)]/10 hover:text-white disabled:opacity-50"
              >
                {busy === 'disconnect' ? <Spinner size={12} /> : null}
                Disconnect Strava
              </button>
            </div>
          </>
        ) : (
          <>
            <p className="mt-5 max-w-[52ch] text-[14.5px] leading-[1.55] text-white/60">
              Pair your Strava account to import your activity history. Cadence
              reads activities, splits, GPS, heart rate, and power; it doesn't
              post on your behalf.
            </p>
            <div className="mt-6 flex flex-wrap items-center gap-3">
              <Button
                variant="strava"
                onClick={handleConnect}
                disabled={busy !== null}
              >
                {busy === 'connect' ? <Spinner size={14} /> : <StravaMark size={16} />}
                Connect Strava
                <ArrowRight />
              </Button>
            </div>
          </>
        )}

        {error ? (
          <p className="mt-4 text-[12.5px] text-[var(--color-danger)]">{error}</p>
        ) : null}
      </div>
    </GlassCard>
  );
}

function formatDate(iso?: string): string {
  if (!iso) return '—';
  const d = new Date(iso);
  return d.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}
