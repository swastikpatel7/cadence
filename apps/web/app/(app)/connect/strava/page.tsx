import { auth } from '@clerk/nextjs/server';
import Link from 'next/link';
import { redirect } from 'next/navigation';
import { ConnectStravaButton } from '@/components/connect/connect-strava-button';
import { Aurora } from '@/components/ui/aurora';
import { GlassCard } from '@/components/ui/glass-card';
import { StravaMark } from '@/components/ui/strava-mark';
import type { SyncStatus } from '@/lib/api-client';
import { serverFetch } from '@/lib/api-server';

export const metadata = {
  title: 'Connect Strava — Cadence',
};

const SCOPES = [
  ['Read activities, splits, GPS streams', 'activity:read_all'],
  ['Backfill 90 days of history', 'one-time'],
  ['Stream new workouts within 60 seconds', 'webhook'],
  ['Revoke any time from settings', 'reversible'],
];

export default async function ConnectStravaPage() {
  const { userId } = await auth();
  if (!userId) redirect('/sign-in');

  // Already connected? Skip the marketing pitch and go straight to /home.
  // Fetch failures fall through to render (the user can still try); the
  // redirect itself must run *outside* the try so Next.js's NEXT_REDIRECT
  // sentinel isn't swallowed by the catch.
  let connected = false;
  try {
    const status = await serverFetch<SyncStatus>('/v1/me/sync');
    connected = status.connection?.connected ?? false;
  } catch {
    // fall through to render
  }
  if (connected) redirect('/home');

  return (
    <section className="relative min-h-[calc(100svh-4rem)] overflow-hidden">
      <Aurora
        variant="strava"
        intensity="normal"
        focus={[0.5, 0.55]}
        wind={[0.010, 0.009]}
        scale={1.15}
      />

      <div className="relative z-20 mx-auto flex min-h-[calc(100svh-4rem)] max-w-[1180px] flex-col items-center justify-center px-6 pb-12 pt-12">
        <Link
          href="/home"
          className="mb-10 inline-flex items-center gap-1.5 self-start rounded-full border border-white/10 bg-black/30 px-3.5 py-1.5 text-[12.5px] text-white/70 backdrop-blur-md transition-colors hover:bg-white/[0.06] hover:text-white"
        >
          <span aria-hidden>←</span> Back to Today
        </Link>

        <GlassCard className="w-full max-w-[560px] px-8 py-9 md:px-10 md:py-11">
          <div className="flex items-center gap-4">
            <span
              className="flex h-12 w-12 items-center justify-center rounded-2xl text-white shadow-[inset_0_1px_0_0_rgb(255_255_255_/_0.18),0_8px_24px_-6px_oklch(0.68_0.21_45_/_0.6)]"
              style={{
                background:
                  'linear-gradient(135deg, oklch(0.72 0.22 45) 0%, oklch(0.58 0.22 38) 100%)',
              }}
              aria-hidden
            >
              <StravaMark size={22} />
            </span>
            <span className="font-mono text-[11px] uppercase tracking-[0.22em] text-white/50">
              OAUTH · READ-ONLY
            </span>
          </div>

          <h1 className="mt-7 text-[44px] font-medium leading-[1.02] tracking-[-0.03em] text-white md:text-[52px]">
            Connect <span className="display text-white/95">Strava</span>
          </h1>

          <p className="mt-5 text-[15.5px] leading-[1.55] text-white/65">
            Cadence reads activities, splits, GPS, heart rate, and power — so it
            can correlate training load with recovery. We never post on your
            behalf, follow anyone, or write to your account.
          </p>

          <ul className="mt-8 flex flex-col gap-3">
            {SCOPES.map(([line, tag]) => (
              <li key={line} className="flex items-start gap-3">
                <span
                  aria-hidden
                  className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[var(--color-success)]"
                  style={{
                    background:
                      'oklch(0.78 0.20 150 / 0.12)',
                    boxShadow:
                      'inset 0 0 0 1px oklch(0.78 0.20 150 / 0.35)',
                  }}
                >
                  <svg
                    width="11"
                    height="11"
                    viewBox="0 0 12 12"
                    aria-hidden="true"
                    role="presentation"
                  >
                    <path
                      d="M2.5 6.5 5 9l4.5-5.5"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="1.6"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                  </svg>
                </span>
                <span className="flex-1 text-[14.5px] leading-[1.45] text-white/85">
                  {line}
                  <span className="ml-2 inline-block translate-y-[-1px] rounded-full border border-white/10 bg-white/[0.04] px-2 py-0.5 align-middle font-mono text-[10px] tracking-[0.14em] text-white/50">
                    {tag}
                  </span>
                </span>
              </li>
            ))}
          </ul>

          <div className="mt-9">
            <ConnectStravaButton />
          </div>

          <p className="mt-6 border-t border-white/10 pt-5 text-center font-mono text-[11px] tracking-[0.16em] text-white/40">
            ENCRYPTED AT REST · NEVER SHARED · DELETE TAKES 24H
          </p>
        </GlassCard>

        <p className="mt-10 text-center text-[12.5px] text-white/40">
          Not ready yet?{' '}
          <Link
            href="/home"
            className="text-white/70 underline-offset-4 hover:text-white"
          >
            Skip for now
          </Link>{' '}
          — you can connect from settings.
        </p>
      </div>
    </section>
  );
}
