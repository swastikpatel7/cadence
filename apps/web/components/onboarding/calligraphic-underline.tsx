'use client';

import { cn } from '@/lib/cn';

interface Props {
  /** Width in px; the SVG scales fluidly. */
  width?: number;
  /** Stroke color — pass any CSS color (e.g. `currentColor`, `var(--color-aurora-violet-1)`). */
  color?: string;
  className?: string;
  /** Animation duration in ms. */
  durationMs?: number;
  /** Animation delay in ms (so the underline lands AFTER the headline rises in). */
  delayMs?: number;
}

/**
 * Hand-drawn underline that paints itself in via `stroke-dasharray` +
 * `stroke-dashoffset` animation. Sits beneath onboarding section H1s
 * and the baseline-page headline (insights.md §3).
 *
 * The path is a single sweeping stroke with a subtle upward lift on
 * the right end — feels like a calligrapher's flick, not a CSS
 * underline. The dash length is intentionally larger than the path
 * so the draw-in goes from "nothing" → "full stroke".
 */
export function CalligraphicUnderline({
  width = 220,
  color = 'currentColor',
  className,
  durationMs = 1200,
  delayMs = 200,
}: Props) {
  return (
    <svg
      aria-hidden="true"
      role="presentation"
      width={width}
      height={14}
      viewBox="0 0 220 14"
      fill="none"
      className={cn('block', className)}
    >
      <path
        d="M2 9 C 30 4, 70 12, 110 7 S 180 3, 218 6"
        stroke={color}
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
        style={{
          strokeDasharray: 280,
          strokeDashoffset: 280,
          animation: `underline-draw ${durationMs}ms cubic-bezier(0.16, 1, 0.3, 1) ${delayMs}ms forwards`,
        }}
      />
      <style>{`
        @keyframes underline-draw {
          to { stroke-dashoffset: 0; }
        }
      `}</style>
    </svg>
  );
}
