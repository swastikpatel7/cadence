'use client';

import { StepFrame } from '@/components/onboarding/step-frame';
import { useWizard } from '@/components/onboarding/wizard-context';
import { cn } from '@/lib/cn';

const DAYS = [3, 4, 5, 6, 7];

const SUBCAPTION: Record<number, string> = {
  3: 'easy',
  4: 'most',
  5: 'serious',
  6: 'high',
  7: 'daily',
};

export function DaysStep() {
  const { state, dispatch } = useWizard();
  const value = state.days_per_week;

  return (
    <StepFrame
      eyebrow="CADENCE"
      title={
        <>
          <span className="display">How often</span> per{' '}
          <span className="display">week?</span>
        </>
      }
      subprompt="Rest days matter as much as run days. Pick the rhythm you can actually sustain."
      canContinue={value !== null && value >= 3 && value <= 7}
      backHref="/onboarding/volume"
      nextHref="/onboarding/target"
    >
      <div className="flex flex-col items-center gap-8">
        <div className="flex flex-wrap items-center justify-center gap-3">
          {DAYS.map((d) => {
            const selected = value === d;
            return (
              <button
                key={d}
                type="button"
                onClick={() =>
                  dispatch({ type: 'SET_FIELD', field: 'days_per_week', value: d })
                }
                aria-pressed={selected}
                className={cn(
                  'group relative flex h-[88px] w-[78px] flex-col items-center justify-center gap-2 overflow-hidden rounded-2xl border backdrop-blur-md transition-all duration-300',
                  'shadow-[inset_0_1px_0_0_rgb(255_255_255_/_0.06)]',
                  selected
                    ? 'border-[oklch(0.55_0.27_295_/_0.55)] bg-white/[0.10] shadow-[0_0_0_1px_oklch(0.55_0.27_295_/_0.4),0_18px_38px_-18px_oklch(0.55_0.27_295_/_0.55)]'
                    : 'border-white/10 bg-white/[0.04] hover:-translate-y-0.5 hover:border-white/20 hover:bg-white/[0.07]',
                )}
              >
                <span className="num text-[28px] font-medium tracking-[-0.03em] text-white">
                  {d}
                </span>
                <span
                  className={cn(
                    'pointer-events-none absolute -inset-px rounded-2xl transition-opacity',
                    selected ? 'opacity-100' : 'opacity-0',
                  )}
                  style={{
                    background:
                      'radial-gradient(60% 50% at 50% 0%, oklch(0.55 0.27 295 / 0.18) 0%, transparent 70%)',
                  }}
                />
              </button>
            );
          })}
        </div>

        <p
          className="font-mono text-[11px] uppercase tracking-[0.22em] text-white/55"
          aria-live="polite"
        >
          {value !== null ? SUBCAPTION[value] ?? '' : 'pick a rhythm'}
        </p>
      </div>
    </StepFrame>
  );
}
