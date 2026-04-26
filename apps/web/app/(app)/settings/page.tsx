import { Suspense } from 'react';
import { ConnectionCard } from '@/components/settings/connection-card';
import { RecentActivities } from '@/components/settings/recent-activities';
import { SyncCard } from '@/components/settings/sync-card';
import { GlassCard } from '@/components/ui/glass-card';
import { serverFetch } from '@/lib/api-server';
import type { SyncStatus } from '@/lib/api-client';

export const metadata = {
  title: 'Settings — Cadence',
};

// Server component renders once with the initial status from the Go API,
// then hands off to client components that poll while a sync is in flight.
//
// We fetch on every request (no caching) so a freshly-completed callback
// renders the connected state without a forced reload.
export default async function SettingsPage({
  searchParams,
}: {
  searchParams: Promise<{ strava?: string; reason?: string }>;
}) {
  const params = await searchParams;
  let initial: SyncStatus | null = null;
  let loadError: string | null = null;
  try {
    initial = await serverFetch<SyncStatus>('/v1/me/sync');
  } catch (err) {
    loadError = err instanceof Error ? err.message : 'failed to load status';
  }

  return (
    <section className="mx-auto max-w-[1080px] px-6 py-12 md:py-16">
      <header className="mb-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.22em] text-white/45">
          SETTINGS
        </p>
        <h1 className="mt-3 text-[40px] font-medium leading-[1.05] tracking-[-0.025em] text-white md:text-[48px]">
          Account &amp; data
        </h1>
        <p className="mt-3 max-w-[60ch] text-[15px] leading-[1.55] text-white/55">
          Manage your Strava connection, sync new activities, and review what
          Cadence currently knows about you.
        </p>
      </header>

      <CallbackBanner status={params.strava} reason={params.reason} />

      {loadError ? (
        <GlassCard className="p-7">
          <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-[var(--color-danger)]">
            FAILED TO LOAD STATUS
          </p>
          <p className="mt-3 text-[14px] text-white/65">{loadError}</p>
        </GlassCard>
      ) : initial ? (
        <Suspense>
          <ClientShell initial={initial} />
        </Suspense>
      ) : null}
    </section>
  );
}

function ClientShell({ initial }: { initial: SyncStatus }) {
  return (
    <div className="grid grid-cols-1 gap-5 lg:grid-cols-[1.4fr_1fr]">
      <div className="flex flex-col gap-5">
        <ConnectionCard connection={initial.connection} />
        <SyncCard initial={initial} />
      </div>
      <RecentActivities recent={initial.recent} />
    </div>
  );
}

function CallbackBanner({ status, reason }: { status?: string; reason?: string }) {
  if (!status) return null;
  const ok = status === 'connected';
  return (
    <div
      className={
        ok
          ? 'mb-8 rounded-2xl border border-[var(--color-success)]/30 bg-[var(--color-success)]/[0.08] px-5 py-3.5 text-[13.5px] text-white/85'
          : 'mb-8 rounded-2xl border border-[var(--color-danger)]/30 bg-[var(--color-danger)]/[0.08] px-5 py-3.5 text-[13.5px] text-white/85'
      }
    >
      {ok
        ? 'Strava connected. Run a manual sync below to import your activities.'
        : `Strava connect failed${reason ? ` — ${reason}` : ''}. Try again?`}
    </div>
  );
}
