import Link from 'next/link';
import { serverFetch } from '@/lib/api-server';
import type { SyncStatus } from '@/lib/api-client';

/**
 * Top-nav status pill that reflects whether Strava is currently linked.
 * Server component: fetches `/v1/me/sync` once per render. The check is
 * best-effort — if the API is unreachable or the user isn't fully
 * provisioned yet, we fall through to the "NOT CONNECTED" state and the
 * Settings page surfaces the real error.
 */
export async function StravaStatusPill() {
  let connected = false;
  try {
    const status = await serverFetch<SyncStatus>('/v1/me/sync');
    connected = status.connection?.connected ?? false;
  } catch {
    connected = false;
  }

  if (connected) {
    return (
      <Link
        href="/settings"
        className="hidden h-8 items-center gap-1.5 rounded-full border border-[var(--color-success)]/25 bg-[var(--color-success)]/[0.07] px-3.5 text-[12.5px] text-white/80 backdrop-blur-md transition-colors hover:bg-[var(--color-success)]/[0.12] hover:text-white sm:inline-flex"
      >
        <span
          aria-hidden
          className="h-1.5 w-1.5 rounded-full bg-[var(--color-success)]"
        />
        <span className="font-mono tracking-[0.12em]">STRAVA — CONNECTED</span>
      </Link>
    );
  }

  return (
    <Link
      href="/connect/strava"
      className="hidden h-8 items-center gap-1.5 rounded-full border border-white/10 bg-white/[0.04] px-3.5 text-[12.5px] text-white/75 backdrop-blur-md transition-colors hover:bg-white/[0.08] hover:text-white sm:inline-flex"
    >
      <span
        aria-hidden
        className="h-1.5 w-1.5 rounded-full bg-[var(--color-warning)]"
      />
      <span className="font-mono tracking-[0.12em]">STRAVA — NOT CONNECTED</span>
    </Link>
  );
}
