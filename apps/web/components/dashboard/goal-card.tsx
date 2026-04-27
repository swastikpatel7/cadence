import { GlassCard } from '@/components/ui/glass-card';
import type { Baseline, HeatmapWeek, UserGoal } from '@/lib/api-client';

interface Props {
  goal: UserGoal;
  baseline?: Baseline | null;
  /** This week's row (Mon..Sun). Used to derive completed-volume so far. */
  thisWeek: HeatmapWeek | null;
}

const KM_PER_MILE = 1.609344;

/**
 * Goal card on /home. Two surfaces:
 *   1. A weekly-volume progress bar — completed-this-week (km converted
 *      to mi) vs `weekly_miles_target`.
 *   2. An optional pace-gap pill — only shown when the goal carries a
 *      `target_pace_sec_per_km`. Reads `baseline.avg_pace_sec_per_km`
 *      and shows the delta as a +Xs/km warm pill (slower) or -Xs/km
 *      cool pill (faster). The pill is also a soft cue for the user
 *      that the plan is in service of a number.
 */
export function GoalCard({ goal, baseline, thisWeek }: Props) {
  const completedKm = thisWeek
    ? thisWeek.reduce((acc, c) => acc + (c.actual?.distance_km ?? 0), 0)
    : 0;
  const completedMiles = completedKm / KM_PER_MILE;
  const targetMiles = goal.weekly_miles_target;
  const pct = Math.min(100, Math.max(0, (completedMiles / targetMiles) * 100));

  const focusLabel = FOCUS_LABEL[goal.focus];

  // Pace gap.
  const paceDeltaSec =
    goal.target_pace_sec_per_km && baseline?.avg_pace_sec_per_km
      ? goal.target_pace_sec_per_km - baseline.avg_pace_sec_per_km
      : null;

  return (
    <GlassCard className="relative overflow-hidden p-7">
      <div
        className="pointer-events-none absolute -right-12 -top-12 h-44 w-44 rounded-full opacity-50"
        style={{
          background:
            'radial-gradient(closest-side, oklch(0.55 0.27 295 / 0.45) 0%, transparent 70%)',
          filter: 'blur(36px)',
        }}
      />
      <div className="relative">
        <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
          THIS WEEK · {focusLabel}
        </p>
        <h3 className="mt-2 text-[20px] font-medium leading-[1.1] tracking-[-0.02em] text-white">
          {completedMiles.toFixed(1)}{' '}
          <span className="text-white/40">/ {targetMiles}</span>{' '}
          <span className="display text-[22px] text-white/55">mi</span>
        </h3>

        <div className="mt-5 h-1.5 w-full overflow-hidden rounded-full bg-white/[0.06]">
          <div
            className="h-full rounded-full"
            style={{
              width: `${pct}%`,
              background:
                'linear-gradient(90deg, var(--color-aurora-violet-1) 0%, var(--color-strava) 100%)',
              boxShadow: '0 0 12px -2px oklch(0.55 0.27 295 / 0.45)',
              transition: 'width 600ms var(--ease-out-expo)',
            }}
          />
        </div>

        <div className="mt-3 flex flex-wrap items-center justify-between gap-3 font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/45">
          <span>{Math.round(pct)}% COMPLETE</span>
          <span>{(targetMiles - completedMiles).toFixed(1)} MI TO GO</span>
        </div>

        {paceDeltaSec !== null ? (
          <PaceGapPill deltaSec={paceDeltaSec} />
        ) : null}
      </div>
    </GlassCard>
  );
}

function PaceGapPill({ deltaSec }: { deltaSec: number }) {
  const slower = deltaSec > 0;
  const abs = Math.abs(deltaSec);
  const sign = slower ? '+' : '−';
  return (
    <div className="mt-5 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.03] px-3 py-1.5 backdrop-blur-md">
      <span className="font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/45">
        PACE GAP
      </span>
      <span
        className="num text-[12.5px]"
        style={{ color: slower ? 'var(--color-warning)' : 'var(--color-success)' }}
      >
        {sign}
        {abs}s/km
      </span>
      <span className="font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/45">
        {slower ? 'TO TARGET' : 'AHEAD'}
      </span>
    </div>
  );
}

const FOCUS_LABEL: Record<UserGoal['focus'], string> = {
  general: 'GENERAL FITNESS',
  build_distance: 'BUILDING DISTANCE',
  build_speed: 'BUILDING SPEED',
  train_for_race: 'RACE PREP',
};
