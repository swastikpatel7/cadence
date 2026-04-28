'use client';

import { GlassCard } from '@/components/ui/glass-card';
import { useUnits } from '@/components/units/units-context';
import type { HeatmapCell, HeatmapWeek, PrescribedLoad } from '@/lib/api-client';
import { formatDistance } from '@/lib/units';

interface Props {
  /** All weeks from the heatmap response — we find today inside. */
  weeks: HeatmapWeek[];
}

/**
 * Big GlassCard for today's prescribed workout. Reads from the heatmap
 * data — no extra fetch (insights.md §12).
 *
 * Three states:
 *   1. Today is a prescribed run → render type, distance, pace target.
 *   2. Today is a rest day → "rest. recovery is training." flourish.
 *   3. No "today" found in the window → render a neutral fallback so the
 *      slot doesn't disappear (defensive — server may have shifted the
 *      window).
 */
export function TodaysSession({ weeks }: Props) {
  const { units } = useUnits();
  const today = findToday(weeks);
  if (!today) {
    return (
      <GlassCard className="p-7">
        <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
          TODAY
        </p>
        <p className="mt-3 text-[14px] text-white/55">
          The plan window doesn't include today yet — refresh in a moment.
        </p>
      </GlassCard>
    );
  }

  const dateLabel = new Date(`${today.date}T00:00:00Z`).toLocaleDateString('en-US', {
    weekday: 'long',
    month: 'long',
    day: 'numeric',
    timeZone: 'UTC',
  });

  if (today.prescribed_load === 'rest') {
    return (
      <GlassCard className="relative overflow-hidden p-7 md:p-8">
        <div
          className="pointer-events-none absolute -left-10 -top-10 h-44 w-44 rounded-full opacity-50"
          style={{
            background:
              'radial-gradient(closest-side, oklch(0.55 0.27 295 / 0.40) 0%, transparent 70%)',
            filter: 'blur(36px)',
          }}
        />
        <div className="relative">
          <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
            TODAY · {dateLabel.toUpperCase()}
          </p>
          <h2 className="mt-3 text-[40px] font-medium leading-[1.0] tracking-[-0.025em] text-white md:text-[48px]">
            <span className="display">Rest.</span>
          </h2>
          <p className="mt-3 max-w-[44ch] text-[14px] leading-[1.55] text-white/55">
            Recovery is training. Walk, stretch, sleep — your next hard day
            depends on what you do today.
          </p>
        </div>
      </GlassCard>
    );
  }

  return (
    <GlassCard className="relative overflow-hidden p-7 md:p-8">
      <div
        className="pointer-events-none absolute -right-16 -top-12 h-56 w-56 rounded-full opacity-55"
        style={{
          background: `radial-gradient(closest-side, ${LOAD_GLOW[today.prescribed_load]} 0%, transparent 70%)`,
          filter: 'blur(36px)',
        }}
      />
      <div className="relative">
        <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
          TODAY · {dateLabel.toUpperCase()}
        </p>
        <h2 className="mt-3 text-[40px] font-medium leading-[1.0] tracking-[-0.025em] text-white md:text-[56px]">
          <span className="display capitalize">{(today.prescribed_type ?? today.prescribed_load).replace('_', ' ')}</span>
          {today.prescribed_distance_km ? (
            <>
              {' '}
              <span className="num text-white/85">{formatDistance(today.prescribed_distance_km, units)}</span>
            </>
          ) : null}
        </h2>
        <div className="mt-5 flex flex-wrap items-center gap-2 text-[12.5px]">
          <Tag>
            LOAD&nbsp;<span className="num">{today.prescribed_load.toUpperCase()}</span>
          </Tag>
          {today.actual?.completed ? <Tag tone="success">DONE&nbsp;✓</Tag> : null}
        </div>
      </div>
    </GlassCard>
  );
}

function Tag({
  children,
  tone = 'default',
}: {
  children: React.ReactNode;
  tone?: 'default' | 'success';
}) {
  const cls =
    tone === 'success'
      ? 'border-[var(--color-success)]/30 bg-[var(--color-success)]/[0.10] text-[var(--color-success)]'
      : 'border-white/10 bg-white/[0.04] text-white/75';
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 font-mono uppercase tracking-[0.18em] ${cls}`}
    >
      {children}
    </span>
  );
}

const LOAD_GLOW: Record<PrescribedLoad, string> = {
  rest: 'oklch(0.55 0.27 295 / 0.4)',
  easy: 'oklch(0.62 0.14 200 / 0.55)',
  moderate: 'oklch(0.66 0.22 295 / 0.55)',
  hard: 'oklch(0.72 0.22 45 / 0.55)',
  peak: 'oklch(0.88 0.14 60 / 0.55)',
};

function findToday(weeks: HeatmapWeek[]): HeatmapCell | null {
  for (const week of weeks) {
    for (const cell of week) {
      if (cell.is_today) return cell;
    }
  }
  return null;
}
