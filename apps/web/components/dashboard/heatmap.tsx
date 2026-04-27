'use client';

import { useCallback, useState } from 'react';
import { HeatmapCellDrawer } from '@/components/dashboard/heatmap-cell-drawer';
import { HeatmapCellView } from '@/components/dashboard/heatmap-cell';
import { GlassCard } from '@/components/ui/glass-card';
import type { HeatmapCell, HeatmapWeek } from '@/lib/api-client';

interface Props {
  weeks: HeatmapWeek[];
  /** When true, render a shimmering skeleton — the plan isn't ready yet. */
  shimmering?: boolean;
}

const DAY_LABELS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

/**
 * Calendar heatmap. Centerpiece of /home (insights.md §5). Renders a
 * grid of 44px (desktop) / 32px (mobile) cells, one per day. Past
 * weeks fade slightly via opacity, today's row gets the breathing
 * aurora ring on the today-cell, future weeks render full-color.
 *
 * Cell click → opens the side drawer with prescribed + actual + the
 * Haiku micro-summary.
 */
export function Heatmap({ weeks, shimmering = false }: Props) {
  const [drawerCell, setDrawerCell] = useState<HeatmapCell | null>(null);

  const handleCellClick = useCallback((cell: HeatmapCell) => {
    setDrawerCell(cell);
  }, []);

  const handleClose = useCallback(() => {
    setDrawerCell(null);
  }, []);

  const todayWeekIdx = weeks.findIndex((w) => w.some((c) => c.is_today));

  return (
    <GlassCard className="relative overflow-hidden p-6 md:p-7">
      <div className="flex items-end justify-between">
        <div>
          <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
            CALENDAR
          </p>
          <h3 className="mt-1 text-[20px] font-medium leading-[1.1] tracking-[-0.02em] text-white">
            Your training rhythm
          </h3>
        </div>
        <Legend />
      </div>

      <div className="-mx-2 mt-6 overflow-x-auto pb-2">
        <div className="inline-flex min-w-full flex-col gap-1.5 px-2">
          {/* Day-of-week header */}
          <div className="flex items-center gap-1.5 pl-12">
            {DAY_LABELS.map((d) => (
              <span
                key={d}
                className="w-11 text-center font-mono text-[10px] uppercase tracking-[0.18em] text-white/35 md:w-11"
              >
                {d}
              </span>
            ))}
          </div>

          {weeks.map((week, rowIdx) => {
            const isPast = rowIdx < todayWeekIdx;
            const isThisWeek = rowIdx === todayWeekIdx;
            return (
              <div
                key={week[0]?.date ?? rowIdx}
                className="flex items-center gap-1.5"
                style={{ opacity: shimmering ? 0.4 : isPast ? 0.62 : 1 }}
              >
                <span className="w-10 shrink-0 text-right font-mono text-[10px] uppercase tracking-[0.16em] text-white/35">
                  {weekLabel(rowIdx, todayWeekIdx)}
                </span>
                {/* Optional caret to today. */}
                <span className="flex w-2 items-center justify-center text-white/40" aria-hidden="true">
                  {isThisWeek ? (
                    <svg width="6" height="9" viewBox="0 0 6 9" aria-hidden="true" role="presentation">
                      <path d="M1 1l4 3.5L1 8" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                    </svg>
                  ) : null}
                </span>
                {week.map((cell, colIdx) => (
                  <HeatmapCellView
                    key={cell.date}
                    cell={cell}
                    rowIdx={rowIdx}
                    colIdx={colIdx}
                    onClick={() => handleCellClick(cell)}
                  />
                ))}
              </div>
            );
          })}
        </div>
      </div>

      {shimmering ? (
        <p className="mt-4 font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/40">
          ◌ DRAFTING YOUR PLAN — SHIMMERING
        </p>
      ) : null}

      <HeatmapCellDrawer cell={drawerCell} onClose={handleClose} />
    </GlassCard>
  );
}

function weekLabel(rowIdx: number, todayWeekIdx: number): string {
  if (todayWeekIdx === -1) return `W${rowIdx + 1}`;
  const delta = rowIdx - todayWeekIdx;
  if (delta === 0) return 'NOW';
  if (delta < 0) return `W${delta}`;
  return `W+${delta}`;
}

function Legend() {
  return (
    <div className="hidden items-center gap-2 sm:flex">
      <span className="font-mono text-[10px] uppercase tracking-[0.18em] text-white/35">
        LOAD
      </span>
      {(['rest', 'easy', 'moderate', 'hard', 'peak'] as const).map((load) => (
        <span
          key={load}
          className="inline-flex items-center gap-1.5"
          title={load}
        >
          <span
            aria-hidden="true"
            className="h-2.5 w-2.5 rounded-[3px] border border-white/10"
            style={{ backgroundColor: `var(--color-load-${load})` }}
          />
          <span className="sr-only">{load}</span>
        </span>
      ))}
    </div>
  );
}
