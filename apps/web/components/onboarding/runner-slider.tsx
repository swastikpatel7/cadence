'use client';

import { useCallback, useId, useRef, useState } from 'react';
import { cn } from '@/lib/cn';

interface Props {
  min: number;
  max: number;
  value: number;
  onChange: (next: number) => void;
  /** Optional suggested value — renders a glowing tick + tooltip. */
  suggestedValue?: number;
  /** Step granularity. Default 1. */
  step?: number;
  /** Aria label for the underlying native range input. */
  ariaLabel?: string;
  className?: string;
}

/**
 * Custom range input with a runner-icon thumb. Track is a 2px hairline
 * with the violet→strava gradient on the filled portion. Thumb scales
 * to 1.15 with an aurora ring on grab. On release, the visual bounces
 * back via a cubic-bezier easing. Snap to integers (or `step`).
 *
 * Used by the Volume step (insights.md §4.3) and reused inside the
 * Recalculate-baseline modal.
 */
export function RunnerSlider({
  min,
  max,
  value,
  onChange,
  suggestedValue,
  step = 1,
  ariaLabel,
  className,
}: Props) {
  const id = useId();
  const [grabbing, setGrabbing] = useState(false);
  const [bouncing, setBouncing] = useState(false);
  const trackRef = useRef<HTMLDivElement | null>(null);

  const pct = ((value - min) / (max - min)) * 100;
  const suggestedPct =
    suggestedValue !== undefined
      ? Math.max(0, Math.min(100, ((suggestedValue - min) / (max - min)) * 100))
      : null;

  const handleRelease = useCallback(() => {
    setGrabbing(false);
    setBouncing(true);
    // Match the bounce-back duration in the spec.
    window.setTimeout(() => setBouncing(false), 360);
  }, []);

  // Tick marks every 5 units. Uses inline width so step granularity
  // changes feel natural even for non-integer steps.
  const tickValues: number[] = [];
  for (let v = Math.ceil(min / 5) * 5; v <= max; v += 5) {
    tickValues.push(v);
  }

  return (
    <div className={cn('relative w-full select-none', className)}>
      <div
        ref={trackRef}
        className="relative mx-auto h-10 w-full"
        // The whole strip is the hit target — easier on touch.
      >
        {/* Tick marks (subtle white/15 dots every 5 units). */}
        <div className="pointer-events-none absolute inset-x-0 top-1/2 h-1 -translate-y-1/2">
          {tickValues.map((tv) => {
            const tPct = ((tv - min) / (max - min)) * 100;
            return (
              <span
                key={tv}
                aria-hidden="true"
                className="absolute h-1 w-1 -translate-x-1/2 rounded-full bg-white/15"
                style={{ left: `${tPct}%`, top: '0' }}
              />
            );
          })}
        </div>

        {/* Hairline track — the unfilled remainder. */}
        <div className="pointer-events-none absolute inset-x-0 top-1/2 h-px -translate-y-1/2 rounded-full bg-white/10" />

        {/* Filled portion — violet → strava gradient. */}
        <div
          className="pointer-events-none absolute left-0 top-1/2 h-[2px] -translate-y-1/2 rounded-full"
          style={{
            width: `${pct}%`,
            background:
              'linear-gradient(90deg, var(--color-aurora-violet-1) 0%, var(--color-strava) 100%)',
            boxShadow: '0 0 12px -2px oklch(0.55 0.27 295 / 0.45)',
          }}
        />

        {/* Suggested-value glow tick. */}
        {suggestedPct !== null ? (
          <span
            aria-hidden="true"
            className="pointer-events-none absolute top-1/2 -translate-y-1/2"
            style={{ left: `${suggestedPct}%` }}
          >
            <span className="relative -ml-[5px] block h-3 w-[2px] rounded-full bg-white/55 shadow-[0_0_10px_oklch(0.88_0.14_60_/_0.5)]" />
          </span>
        ) : null}

        {/* Runner thumb — visual only; the native input below
            captures all input. The transform mirrors the percentage
            position; on grab we scale up + add an aurora-violet
            blur ring, on release we ease-back via the bouncing
            class. */}
        <div
          aria-hidden="true"
          className="pointer-events-none absolute top-1/2"
          style={{
            left: `${pct}%`,
            transform: `translate(-50%, -50%) scale(${grabbing ? 1.15 : 1})`,
            transition: bouncing
              ? 'transform 360ms cubic-bezier(0.22, 1.6, 0.36, 1)'
              : 'transform 120ms ease-out',
          }}
        >
          <span
            className="relative inline-flex h-7 w-7 items-center justify-center rounded-full border border-white/20 bg-[oklch(0.07_0.02_270_/_0.85)] backdrop-blur-md"
            style={{
              boxShadow: grabbing
                ? '0 0 0 6px oklch(0.55 0.27 295 / 0.18), 0 8px 22px -4px oklch(0.55 0.27 295 / 0.55)'
                : '0 6px 18px -8px oklch(0 0 0 / 0.6), inset 0 1px 0 0 oklch(1 0 0 / 0.10)',
            }}
          >
            <RunnerIcon />
          </span>
        </div>

        {/* The actual input — invisible but in front, so a click
            anywhere on the track moves the thumb instantly. */}
        <input
          id={id}
          aria-label={ariaLabel}
          type="range"
          min={min}
          max={max}
          step={step}
          value={value}
          onChange={(e) => onChange(Number(e.currentTarget.value))}
          onPointerDown={() => setGrabbing(true)}
          onPointerUp={handleRelease}
          onPointerCancel={handleRelease}
          onBlur={() => grabbing && handleRelease()}
          onMouseLeave={() => grabbing && handleRelease()}
          className="absolute inset-0 h-full w-full cursor-grab appearance-none bg-transparent opacity-0 active:cursor-grabbing"
        />
      </div>
    </div>
  );
}

function RunnerIcon() {
  // Tiny silhouette of a runner. Stylized — head + torso + legs in
  // motion. Inherits color from currentColor so it can be themed.
  return (
    <svg
      aria-hidden="true"
      role="presentation"
      width="14"
      height="14"
      viewBox="0 0 16 16"
      className="text-white/85"
    >
      <circle cx="10.4" cy="2.6" r="1.4" fill="currentColor" />
      <path
        d="M5.2 5.6c1.2-.6 2.6-.7 3.6 0l1.7 1.2-1.4 2.5 1.7 2.4-1 1.5-2.6-2.6.7-2.6-1.6.4-1.7 2.6-1.4-.7L5.2 5.6Z"
        fill="currentColor"
      />
    </svg>
  );
}
