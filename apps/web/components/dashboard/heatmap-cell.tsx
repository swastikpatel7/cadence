'use client';

import { type CSSProperties, forwardRef } from 'react';
import { useUnits } from '@/components/units/units-context';
import type { HeatmapCell as HeatmapCellType, PrescribedLoad } from '@/lib/api-client';
import { cn } from '@/lib/cn';
import { formatDistance, type Units } from '@/lib/units';

interface Props {
  cell: HeatmapCellType;
  /** Row index (0..N), used by the parent to compute stagger delay. */
  rowIdx: number;
  /** Column index (0..6), used by the parent to compute stagger delay. */
  colIdx: number;
  /** Click handler — opens the side drawer with full session detail. */
  onClick: () => void;
  /** Cell size in px; 44 on desktop, 32 on mobile. */
  size?: number;
}

/**
 * Single heatmap cell. Visual rules per insights.md §5:
 *   - Base color: `--color-load-{rest|easy|moderate|hard|peak}`.
 *   - Hover: lifts -2px, gains a 12px outer glow, tooltip slot.
 *   - Today: breathing aurora ring + 4px outer violet glow.
 *   - Past skipped (completed=false on a prescribed-non-rest day):
 *     diagonal-stripe overlay using `--color-load-skipped`.
 *   - Past completed: subtle ✓ marker in the corner.
 *   - Future: full color, prescribed (clickable for prescribed
 *     drawer detail).
 *
 * The hover tooltip is rendered as a `<span>` slot so the CSS
 * `:hover` selector can fade it in without React re-renders.
 */
export const HeatmapCellView = forwardRef<HTMLButtonElement, Props>(function HeatmapCellView(
  { cell, rowIdx, colIdx, onClick, size = 44 },
  ref,
) {
  const baseColor = LOAD_COLOR[cell.prescribed_load];
  const isPastSkipped =
    !cell.is_future &&
    !cell.is_today &&
    cell.prescribed_load !== 'rest' &&
    (!cell.actual || cell.actual.completed === false);
  const isPastCompleted = !cell.is_future && cell.actual?.completed === true;

  const ringColor = LOAD_RING[cell.prescribed_load];

  const stagger: CSSProperties = {
    animationDelay: `${rowIdx * 50 + colIdx * 18}ms`,
  };

  const { units } = useUnits();
  const tooltipText = buildTooltipText(cell, units);
  const distanceLabel = cell.prescribed_distance_km
    ? formatDistance(cell.prescribed_distance_km, units)
    : null;

  return (
    <button
      ref={ref}
      type="button"
      onClick={onClick}
      aria-label={`${cell.date} · ${cell.prescribed_load}${
        distanceLabel ? ` · ${distanceLabel}` : ''
      }`}
      className={cn(
        'group/cell relative outline-none transition-transform duration-200',
        'hover:-translate-y-0.5 focus-visible:-translate-y-0.5',
        'focus-visible:ring-2 focus-visible:ring-white/40 focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--color-bg-deep)]',
      )}
      style={{
        width: size,
        height: size,
        animation: 'heatmap-cell-in 280ms cubic-bezier(0.16, 1, 0.3, 1) both',
        ...stagger,
      }}
    >
      {/* Today's breathing aurora ring. The ring is a separate
          element so it can scale without scaling the body. */}
      {cell.is_today ? (
        <span
          aria-hidden="true"
          className="pointer-events-none absolute -inset-1 rounded-[10px] border-2"
          style={{
            borderColor: 'oklch(0.55 0.27 295 / 0.85)',
            boxShadow: '0 0 24px -4px oklch(0.55 0.27 295 / 0.85)',
            animation: 'today-breathe 2.4s ease-in-out infinite',
          }}
        />
      ) : null}

      {/* Body — load-tinted square. */}
      <span
        aria-hidden="true"
        className="absolute inset-0 rounded-[8px] border border-white/[0.06] transition-shadow duration-200 group-hover/cell:shadow-[0_0_16px_-2px_var(--cell-glow)]"
        style={
          {
            backgroundColor: baseColor,
            ['--cell-glow' as string]: ringColor,
          } as CSSProperties
        }
      />

      {/* Diagonal stripe overlay for past skipped. */}
      {isPastSkipped ? (
        <span
          aria-hidden="true"
          className="absolute inset-0 rounded-[8px]"
          style={{
            background:
              'repeating-linear-gradient(45deg, transparent 0 4px, var(--color-load-skipped) 4px 5px)',
          }}
        />
      ) : null}

      {/* Completed checkmark in the corner. */}
      {isPastCompleted ? (
        <span
          aria-hidden="true"
          className="absolute right-1 top-1 inline-flex h-3 w-3 items-center justify-center rounded-full bg-black/35 text-white/85"
        >
          <svg
            aria-hidden="true"
            role="presentation"
            width="7"
            height="7"
            viewBox="0 0 12 12"
          >
            <path
              d="M2.5 6.5 5 9l4.5-5.5"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </span>
      ) : null}

      {/* Hover tooltip — pure-CSS show/hide via group-hover. */}
      <span
        role="tooltip"
        className="pointer-events-none absolute -top-9 left-1/2 z-30 -translate-x-1/2 whitespace-nowrap rounded-full border border-white/10 bg-black/85 px-3 py-1.5 font-mono text-[10.5px] uppercase tracking-[0.16em] text-white/85 opacity-0 backdrop-blur-md transition-opacity delay-75 group-hover/cell:opacity-100"
      >
        {tooltipText}
      </span>
    </button>
  );
});

const LOAD_COLOR: Record<PrescribedLoad, string> = {
  rest: 'var(--color-load-rest)',
  easy: 'var(--color-load-easy)',
  moderate: 'var(--color-load-moderate)',
  hard: 'var(--color-load-hard)',
  peak: 'var(--color-load-peak)',
};

const LOAD_RING: Record<PrescribedLoad, string> = {
  rest: 'oklch(0.45 0.02 270 / 0.40)',
  easy: 'oklch(0.62 0.14 200 / 0.45)',
  moderate: 'oklch(0.66 0.22 295 / 0.55)',
  hard: 'oklch(0.72 0.22 45 / 0.60)',
  peak: 'oklch(0.88 0.14 60 / 0.70)',
};

function buildTooltipText(cell: HeatmapCellType, units: Units): string {
  const date = new Date(`${cell.date}T00:00:00Z`).toLocaleDateString('en-US', {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    timeZone: 'UTC',
  });
  if (cell.prescribed_load === 'rest') return `${date} · rest`;
  const dist = cell.prescribed_distance_km
    ? ` · ${formatDistance(cell.prescribed_distance_km, units)}`
    : '';
  const type = cell.prescribed_type ? ` ${cell.prescribed_type}` : '';
  return `${date}${type}${dist}`;
}
