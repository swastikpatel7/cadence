'use client';

import { StepFrame } from '@/components/onboarding/step-frame';
import { useWizard } from '@/components/onboarding/wizard-context';
import type { GoalFocus } from '@/lib/api-client';
import { cn } from '@/lib/cn';

interface FocusOption {
  id: GoalFocus;
  label: string;
  blurb: string;
}

const OPTIONS: FocusOption[] = [
  {
    id: 'general',
    label: 'General fitness',
    blurb: 'Steady weekly rhythm, no race.',
  },
  {
    id: 'build_distance',
    label: 'Build distance',
    blurb: 'Going further every week, recovery-conscious.',
  },
  {
    id: 'build_speed',
    label: 'Build speed',
    blurb: 'Going faster at the same distance — intervals + tempo.',
  },
  {
    id: 'train_for_race',
    label: 'Train for a race',
    blurb: 'Pick distance + date next.',
  },
];

export function FocusStep() {
  const { state, dispatch } = useWizard();

  return (
    <StepFrame
      eyebrow="FOR YOUR FIRST PLAN"
      title={
        <>
          <span className="display">What's</span> your{' '}
          <span className="display">move?</span>
        </>
      }
      subprompt="No wrong answers — pick the closest fit, edit later."
      canContinue={state.focus !== null}
      backHref={null}
      nextHref="/onboarding/volume"
    >
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {OPTIONS.map((opt) => {
          const selected = state.focus === opt.id;
          return (
            <button
              key={opt.id}
              type="button"
              onClick={() =>
                dispatch({ type: 'SET_FIELD', field: 'focus', value: opt.id })
              }
              className={cn(
                'group relative flex min-h-[140px] flex-col items-start justify-between gap-3 overflow-hidden rounded-[var(--radius-card)] border p-5 text-left transition-all duration-300',
                'border-white/10 bg-white/[0.03] backdrop-blur-2xl',
                'shadow-[inset_0_1px_0_0_rgb(255_255_255_/_0.06)]',
                'hover:-translate-y-1 hover:border-white/20 hover:bg-white/[0.06] hover:shadow-[0_18px_40px_-18px_oklch(0.55_0.27_295_/_0.45)]',
                selected &&
                  'border-[oklch(0.55_0.27_295_/_0.6)] bg-white/[0.08] shadow-[0_0_0_1px_oklch(0.55_0.27_295_/_0.45),0_18px_40px_-18px_oklch(0.55_0.27_295_/_0.55)]',
              )}
              aria-pressed={selected}
            >
              {/* Soft inner glow layer when selected. */}
              <span
                aria-hidden="true"
                className="pointer-events-none absolute inset-0 transition-opacity duration-300"
                style={{
                  background:
                    'radial-gradient(80% 60% at 30% 0%, oklch(0.55 0.27 295 / 0.18) 0%, transparent 70%)',
                  opacity: selected ? 1 : 0,
                }}
              />
              <div className="relative">
                <h3 className="text-[20px] font-medium leading-[1.1] tracking-[-0.02em] text-white">
                  {opt.label}
                </h3>
                <p className="mt-2 max-w-[34ch] text-[13.5px] leading-[1.5] text-white/60">
                  {opt.blurb}
                </p>
              </div>
              <span
                aria-hidden="true"
                className={cn(
                  'relative inline-flex h-5 w-5 items-center justify-center rounded-full transition-all',
                  selected
                    ? 'bg-white text-black'
                    : 'border border-white/20 text-transparent',
                )}
              >
                {selected ? (
                  <svg
                    aria-hidden="true"
                    role="presentation"
                    width="11"
                    height="11"
                    viewBox="0 0 12 12"
                  >
                    <path
                      d="M2.5 6.5 5 9l4.5-5.5"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="1.6"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                  </svg>
                ) : null}
              </span>
            </button>
          );
        })}
      </div>
    </StepFrame>
  );
}
