import { cn } from '@/lib/cn';

/**
 * Cadence wordmark — `cadence.` in Geist Medium with the period rendered in
 * the brand violet. The period is the rhythm marker, not decoration; it's
 * the visual signature you'll find in chrome (rails / nav) and at scale on
 * the landing hero.
 *
 * The "cadence" letterform inherits color from the parent so it adapts to
 * dark / light contexts; the period always renders in violet. Pass
 * `monochrome` to drop the violet (useful inside an already-violet pill).
 */
export function WordMark({
  size = 16,
  monochrome = false,
  className,
}: {
  size?: number;
  monochrome?: boolean;
  className?: string;
}) {
  return (
    <span
      className={cn(
        'inline-flex items-baseline font-medium leading-none tracking-[-0.04em]',
        className,
      )}
      style={{ fontSize: `${size}px` }}
    >
      cadence
      <span
        style={
          monochrome ? undefined : { color: 'var(--color-aurora-violet-1)' }
        }
      >
        .
      </span>
    </span>
  );
}

/**
 * Cadence icon mark — the wordmark compressed to its initial. A square
 * violet-gradient pill with white `c.` inside. Used for app launchers,
 * favicons, and the Strava app directory listing. The wordmark itself
 * is the brand; this is the brand boiled down to a single character.
 */
export function BrandMark({
  size = 28,
  className,
}: {
  size?: number;
  className?: string;
}) {
  return (
    <span
      aria-hidden
      className={cn(
        'inline-flex items-center justify-center font-medium leading-none text-white',
        className,
      )}
      style={{
        width: `${size}px`,
        height: `${size}px`,
        borderRadius: `${Math.round(size * 0.22)}px`,
        fontSize: `${size * 0.62}px`,
        letterSpacing: '-0.04em',
        background:
          'linear-gradient(135deg, oklch(0.55 0.27 295) 0%, oklch(0.62 0.22 265) 100%)',
        boxShadow:
          'inset 0 1px 0 0 rgb(255 255 255 / 0.20), 0 6px 14px -8px oklch(0.55 0.27 295 / 0.7)',
      }}
    >
      c.
    </span>
  );
}
