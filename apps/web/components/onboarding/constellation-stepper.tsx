import { cn } from '@/lib/cn';

interface Props {
  /** 1-indexed current step. */
  current: number;
  /** Total step count. Default 5 (focus / volume / days / target / baseline). */
  total?: number;
  className?: string;
}

/**
 * 5-dot horizontal step indicator with a soft hairline connector.
 * Per insights.md §3:
 *  - Current dot: 6px solid white + 12px aurora-violet bloom + 1.6s pulse.
 *  - Past dots:   4px white/40.
 *  - Future dots: 3px white/15.
 *
 * Used in the wizard chrome (top bar) and as the sole identifier on
 * the baseline-compute page (no nav, just progress).
 */
export function ConstellationStepper({ current, total = 5, className }: Props) {
  const steps = Array.from({ length: total }, (_, i) => i + 1);
  return (
    <div
      className={cn(
        'flex items-center gap-3 font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/50',
        className,
      )}
    >
      <div className="flex items-center gap-2">
        {steps.map((n, idx) => {
          const isCurrent = n === current;
          const isPast = n < current;
          const isFuture = n > current;
          return (
            <span key={n} className="relative inline-flex items-center">
              {idx > 0 ? (
                <span
                  aria-hidden="true"
                  className="mr-2 inline-block h-px w-3 bg-white/[0.06]"
                />
              ) : null}
              <span
                aria-hidden="true"
                className="relative inline-flex items-center justify-center"
                style={{
                  width: isCurrent ? 12 : 8,
                  height: isCurrent ? 12 : 8,
                }}
              >
                <span
                  className={cn(
                    'rounded-full',
                    isCurrent && 'bg-white',
                    isPast && 'bg-white/40',
                    isFuture && 'bg-white/15',
                  )}
                  style={{
                    width: isCurrent ? 6 : isPast ? 4 : 3,
                    height: isCurrent ? 6 : isPast ? 4 : 3,
                    animation: isCurrent
                      ? 'constellation-pulse 1.6s ease-in-out infinite'
                      : undefined,
                    borderRadius: '9999px',
                  }}
                />
              </span>
            </span>
          );
        })}
      </div>
      <span className="text-white/40">
        STEP {current} OF {total}
      </span>
    </div>
  );
}
