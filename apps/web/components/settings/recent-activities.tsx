'use client';

import { GlassCard } from '@/components/ui/glass-card';
import { useUnits } from '@/components/units/units-context';
import type { RecentActivity } from '@/lib/api-client';
import { formatDistanceMeters } from '@/lib/units';

interface Props {
  recent: RecentActivity[];
}

/**
 * Tail of the Settings page — shows the 5 most-recent activities so the
 * user can confirm their data really is in Cadence (not just a row count).
 */
export function RecentActivities({ recent }: Props) {
  const { units } = useUnits();
  if (recent.length === 0) {
    return (
      <GlassCard className="p-7">
        <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
          RECENT ACTIVITIES
        </p>
        <p className="mt-4 text-[14px] text-white/55">
          Nothing here yet. Run a sync above and your last activities will land here.
        </p>
      </GlassCard>
    );
  }

  return (
    <GlassCard className="p-7">
      <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
        RECENT ACTIVITIES
      </p>
      <ul className="mt-4 divide-y divide-white/[0.06]">
        {recent.map((a) => (
          <li key={a.id} className="flex items-center gap-4 py-3.5">
            <span
              aria-hidden
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full"
              style={{
                background: `${sportColor(a.sport_type)}`,
                color: 'white',
                fontSize: '11px',
                fontWeight: 600,
              }}
            >
              {sportInitial(a.sport_type)}
            </span>
            <div className="flex min-w-0 flex-1 flex-col">
              <p className="truncate text-[14px] font-medium text-white/90">
                {a.name}
              </p>
              <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-white/45">
                {a.sport_type} · {formatDate(a.start_time)}
              </p>
            </div>
            <p className="num shrink-0 text-[13px] text-white/75">
              {a.distance_meters && a.distance_meters > 0
                ? formatDistanceMeters(a.distance_meters, units)
                : '—'}
            </p>
          </li>
        ))}
      </ul>
    </GlassCard>
  );
}

function sportInitial(sport: string): string {
  return sport.charAt(0).toUpperCase();
}

function sportColor(sport: string): string {
  const s = sport.toLowerCase();
  if (s.includes('run')) return 'oklch(0.74 0.18 145 / 0.85)';
  if (s.includes('ride') || s.includes('cycle') || s.includes('bike')) return 'oklch(0.68 0.18 240 / 0.85)';
  if (s.includes('swim')) return 'oklch(0.74 0.14 200 / 0.85)';
  if (s.includes('weight') || s.includes('strength') || s.includes('crossfit')) return 'oklch(0.66 0.18 290 / 0.85)';
  return 'oklch(0.55 0.10 270 / 0.85)';
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: d.getFullYear() !== new Date().getFullYear() ? 'numeric' : undefined,
  });
}

