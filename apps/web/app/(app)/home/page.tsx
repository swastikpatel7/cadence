import { currentUser } from '@clerk/nextjs/server';
import Link from 'next/link';
import { redirect } from 'next/navigation';
import { GoalCard } from '@/components/dashboard/goal-card';
import { Heatmap } from '@/components/dashboard/heatmap';
import { ShimmerPoll } from '@/components/dashboard/shimmer-poll';
import { TodaysSession } from '@/components/dashboard/todays-session';
import { Aurora } from '@/components/ui/aurora';
import { ArrowRight, Button } from '@/components/ui/button';
import { GlassCard } from '@/components/ui/glass-card';
import { StravaMark } from '@/components/ui/strava-mark';
import {
  ApiError,
  type Baseline,
  type HeatmapResponse,
  type HeatmapWeek,
  type SyncStatus,
  type UserGoal,
} from '@/lib/api-client';
import { serverFetch } from '@/lib/api-server';

export const metadata = {
  title: 'Today — Cadence',
};

/**
 * /home — the dashboard. After onboarding lands, this is the canonical
 * surface. Per insights.md §6 + §12 we run four parallel server fetches
 * and route the user appropriately:
 *   - /v1/me/sync     → connection state, recent activities (existing).
 *   - /v1/me/goal     → 404 → redirect('/onboarding').
 *   - /v1/me/baseline → 404 → render shimmer (plan still generating).
 *   - /v1/me/plan/heatmap → 404 same fallback.
 *
 * Hero is preserved (the previous redesign anchors on it). The
 * `PlaceholderGrid` is replaced with the real plan widgets.
 */
export default async function HomePage() {
  const user = await currentUser();
  const greetName = user?.firstName ?? user?.username ?? 'athlete';

  // 1) Sync status — also tells us if Strava is connected at all.
  let sync: SyncStatus | null = null;
  try {
    sync = await serverFetch<SyncStatus>('/v1/me/sync');
  } catch {
    // ignore — UI falls back to "not connected" layout below.
  }
  const connected = sync?.connection?.connected ?? false;

  // 2) Goal — 404 means we should redirect into onboarding. Other errors
  //    fall through (we'd rather show a partial dashboard than block).
  let goal: UserGoal | null = null;
  let goalNotFound = false;
  try {
    const res = await serverFetch<{ goal: UserGoal }>('/v1/me/goal');
    goal = res.goal;
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      goalNotFound = true;
    }
  }
  if (goalNotFound && connected) {
    // Connected but no goal → kick into onboarding.
    redirect('/onboarding');
  }

  // 3) Baseline + heatmap — both can 404 if the plan is still generating.
  let baseline: Baseline | null = null;
  try {
    if (goal) {
      const res = await serverFetch<{ baseline: Baseline }>('/v1/me/baseline');
      baseline = res.baseline;
    }
  } catch {
    // 404 is the expected mid-onboarding state.
  }

  let heatmap: HeatmapWeek[] | null = null;
  let heatmapShimmer = false;
  try {
    if (goal) {
      const res = await serverFetch<HeatmapResponse>(
        '/v1/me/plan/heatmap?weeks_back=2&weeks_forward=6',
      );
      heatmap = res.weeks;
      // Heuristic — if every cell is rest, the plan is the skeleton.
      heatmapShimmer = res.weeks.every((w) =>
        w.every((c) => c.prescribed_load === 'rest'),
      );
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      heatmapShimmer = true;
    }
  }

  return (
    <>
      <Hero name={greetName} />
      {goal ? (
        <DashboardBody
          goal={goal}
          baseline={baseline}
          weeks={heatmap}
          shimmer={heatmapShimmer || !heatmap}
        />
      ) : (
        <PlaceholderGrid connected={connected} />
      )}
    </>
  );
}

function Hero({ name }: { name: string }) {
  const today = new Date().toLocaleDateString('en-US', {
    weekday: 'long',
    month: 'long',
    day: 'numeric',
  });

  return (
    <section className="relative overflow-hidden border-b border-white/[0.06]">
      <div className="pointer-events-none absolute inset-0">
        <Aurora
          variant="violet"
          intensity="normal"
          focus={[0.80, 0.48]}
          wind={[-0.012, 0.004]}
          scale={0.85}
        />
        <div
          className="absolute inset-0"
          style={{
            background:
              'linear-gradient(90deg, oklch(0.07 0.02 270 / 0.90) 0%, oklch(0.07 0.02 270 / 0.55) 35%, oklch(0.07 0.02 270 / 0) 70%)',
          }}
        />
        <div
          className="absolute inset-x-0 bottom-0 h-[35%]"
          style={{
            background:
              'linear-gradient(to bottom, transparent 0%, var(--color-bg-deep) 100%)',
          }}
        />
      </div>

      <div className="relative z-10 mx-auto max-w-[1280px] px-6 py-16 md:py-20">
        <p className="font-mono text-[11px] uppercase tracking-[0.22em] text-white/45">
          {today.toUpperCase()}
        </p>
        <h1 className="mt-4 max-w-[20ch] text-[44px] font-semibold leading-[0.98] tracking-[-0.035em] text-white md:text-[68px]">
          Welcome back,{' '}
          <span className="display font-normal text-white/95">{name}.</span>
        </h1>
      </div>
    </section>
  );
}

function DashboardBody({
  goal,
  baseline,
  weeks,
  shimmer,
}: {
  goal: UserGoal;
  baseline: Baseline | null;
  weeks: HeatmapWeek[] | null;
  shimmer: boolean;
}) {
  // Build a shimmer-week scaffold so the grid still has shape.
  const displayWeeks = weeks && weeks.length > 0 ? weeks : buildShimmerWeeks();
  const thisWeek = displayWeeks.find((w) => w.some((c) => c.is_today)) ?? null;

  return (
    <section className="mx-auto max-w-[1280px] px-6 py-10">
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-[1.4fr_1fr]">
        <TodaysSession weeks={displayWeeks} />
        <GoalCard goal={goal} baseline={baseline} thisWeek={thisWeek} />
      </div>
      <div className="mt-5">
        <Heatmap weeks={displayWeeks} shimmering={shimmer} />
        {shimmer ? <ShimmerPoll /> : null}
      </div>
      <div className="mt-5">
        <CoachStub />
      </div>
    </section>
  );
}

function PlaceholderGrid({ connected }: { connected: boolean }) {
  return (
    <section className="mx-auto max-w-[1280px] px-6 py-12">
      {connected ? (
        <div className="grid grid-cols-1 gap-5">
          <CoachStub />
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-5 lg:grid-cols-[1.4fr_1fr]">
          <ConnectCard />
          <CoachStub />
        </div>
      )}
      <p className="mt-12 text-center font-mono text-[11px] tracking-[0.18em] text-white/30">
        v0.1 · YOUR DATA, YOUR PACE
      </p>
    </section>
  );
}

function ConnectCard() {
  return (
    <GlassCard className="relative overflow-hidden p-7 md:p-8">
      <div
        className="pointer-events-none absolute -right-20 -top-20 h-72 w-72 rounded-full opacity-50"
        style={{
          background:
            'radial-gradient(closest-side, oklch(0.72 0.22 45 / 0.55) 0%, transparent 70%)',
          filter: 'blur(40px)',
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
              STEP 01 · CONNECT
            </p>
            <h3 className="mt-1 text-[22px] font-medium leading-[1.1] tracking-[-0.02em] text-white">
              Pair Strava in one click.
            </h3>
          </div>
        </div>

        <p className="mt-5 max-w-[52ch] text-[14.5px] leading-[1.55] text-white/60">
          Read-only OAuth. We pull your last 90 days of activities, splits, GPS
          streams, heart rate, and power, then keep streaming new sessions as
          you upload them.
        </p>

        <div className="mt-6 flex flex-wrap items-center gap-3">
          <Button href="/connect/strava" variant="strava">
            <StravaMark size={16} />
            Connect Strava
            <ArrowRight />
          </Button>
          <Link
            href="/#privacy"
            className="text-[13.5px] text-white/55 underline-offset-4 hover:text-white hover:underline"
          >
            What does Cadence read?
          </Link>
        </div>
      </div>
    </GlassCard>
  );
}

function CoachStub() {
  return (
    <GlassCard className="flex flex-col p-7 md:p-8">
      <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
        STEP 02 · COACH
      </p>
      <h3 className="mt-3 text-[22px] font-medium leading-[1.1] tracking-[-0.02em] text-white">
        Your coach, listening.
      </h3>
      <p className="mt-3 text-[14px] leading-[1.55] text-white/55">
        Once a session lands, the coach reads your last 14 days, weekly load,
        and recovery before answering. Full chat ships in the next pass.
      </p>

      <div className="mt-5 flex-1 rounded-xl border border-white/10 bg-black/30 p-4">
        <div className="flex items-center gap-2 font-mono text-[10.5px] tracking-[0.16em] text-white/35">
          <span className="h-1.5 w-1.5 rounded-full bg-white/20" />
          ZZZ · LISTENING FOR ACTIVITIES
        </div>
        <div className="mt-4 space-y-2">
          <div className="h-2.5 w-3/4 rounded-full bg-white/[0.06]" />
          <div className="h-2.5 w-2/3 rounded-full bg-white/[0.05]" />
          <div className="h-2.5 w-1/2 rounded-full bg-white/[0.04]" />
        </div>
      </div>
    </GlassCard>
  );
}

function buildShimmerWeeks(): HeatmapWeek[] {
  const today = new Date();
  const dow = today.getUTCDay(); // 0..6 (Sun..Sat)
  // Convert to Mon-anchor: Mon=0..Sun=6.
  const monIdx = (dow + 6) % 7;
  const monStart = new Date(today);
  monStart.setUTCDate(monStart.getUTCDate() - monIdx);
  monStart.setUTCHours(0, 0, 0, 0);

  const weeks: HeatmapWeek[] = [];
  for (let w = -2; w <= 6; w++) {
    const week: HeatmapWeek = [];
    for (let d = 0; d < 7; d++) {
      const cellDate = new Date(monStart);
      cellDate.setUTCDate(monStart.getUTCDate() + w * 7 + d);
      const iso = cellDate.toISOString().slice(0, 10);
      const isToday =
        cellDate.getUTCFullYear() === today.getUTCFullYear() &&
        cellDate.getUTCMonth() === today.getUTCMonth() &&
        cellDate.getUTCDate() === today.getUTCDate();
      const isFuture = cellDate.getTime() > today.getTime();
      week.push({
        date: iso,
        prescribed_load: 'rest',
        is_today: isToday,
        is_future: isFuture,
      });
    }
    weeks.push(week);
  }
  return weeks;
}
