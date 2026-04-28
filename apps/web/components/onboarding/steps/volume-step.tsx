'use client';

import { useEffect } from 'react';
import { RunnerSlider } from '@/components/onboarding/runner-slider';
import { StepFrame } from '@/components/onboarding/step-frame';
import { useWizard } from '@/components/onboarding/wizard-context';
import { useUnits } from '@/components/units/units-context';
import { milesToKm, type Units } from '@/lib/units';

interface Props {
  /** Last_30d_avg × 1.10, clamped to [5, 80]. */
  suggestedMiles: number;
  /** Avg from last 30 days; null if no recent runs. */
  last30dAvgMiles: number | null;
}

export function VolumeStep({ suggestedMiles, last30dAvgMiles }: Props) {
  const { state, dispatch } = useWizard();
  const { units } = useUnits();

  // Default the slider to the suggested value the first time the user
  // lands on this step. Preserves any value they already chose.
  useEffect(() => {
    if (state.weekly_miles_target == null) {
      dispatch({
        type: 'SET_FIELD',
        field: 'weekly_miles_target',
        value: suggestedMiles,
      });
    }
  }, [state.weekly_miles_target, suggestedMiles, dispatch]);

  const value = state.weekly_miles_target ?? suggestedMiles;
  const showHighVolumeNote = value > 40;

  const subprompt = buildSubprompt(units, last30dAvgMiles, suggestedMiles);

  return (
    <StepFrame
      eyebrow="VOLUME PER WEEK"
      title={
        <>
          <span className="display">How much</span> running feels{' '}
          <span className="display">right?</span>
        </>
      }
      subprompt={subprompt}
      canContinue={value >= 5 && value <= 80}
      backHref="/onboarding/focus"
      nextHref="/onboarding/days"
    >
      <div className="flex flex-col items-center gap-8">
        <ValueReadout value={value} units={units} />

        <div className="w-full max-w-[640px]">
          <RunnerSlider
            min={5}
            max={80}
            value={value}
            suggestedValue={suggestedMiles}
            onChange={(n) =>
              dispatch({
                type: 'SET_FIELD',
                field: 'weekly_miles_target',
                value: n,
              })
            }
            ariaLabel="Weekly volume target"
          />
          <div className="mt-3 flex items-center justify-between font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/40">
            <span>{units === 'imperial' ? '5 mi' : `${Math.round(milesToKm(5))} km`}</span>
            <span>{units === 'imperial' ? '80 mi' : `${Math.round(milesToKm(80))} km`}</span>
          </div>
        </div>

        <p className="max-w-[58ch] text-center text-[13.5px] leading-[1.55] text-white/55">
          {units === 'imperial' ? (
            <>
              Most beginners thrive at 15&ndash;25 mi/wk. Ramps over 30 mi need
              more recovery work — we'll pace it for you.
            </>
          ) : (
            <>
              Most beginners thrive at 25&ndash;40 km/wk. Ramps over 50 km need
              more recovery work — we'll pace it for you.
            </>
          )}
        </p>

        {showHighVolumeNote ? (
          <p className="rounded-full border border-white/10 bg-white/[0.04] px-4 py-1.5 font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/65 backdrop-blur-md">
            HIGH VOLUME · WE'LL ADD AN EXTRA RECOVERY DAY
          </p>
        ) : null}
      </div>
    </StepFrame>
  );
}

function ValueReadout({ value, units }: { value: number; units: Units }) {
  const display = units === 'imperial' ? value : Math.round(milesToKm(value));
  const label = units === 'imperial' ? 'mi/wk' : 'km/wk';
  return (
    <div className="flex items-baseline gap-3 text-white">
      <span
        className="num text-[80px] font-medium tracking-[-0.04em] leading-none text-white/95"
        aria-live="polite"
      >
        {display}
      </span>
      <span className="display text-[28px] text-white/65">{label}</span>
    </div>
  );
}

function buildSubprompt(
  units: Units,
  last30dAvgMiles: number | null,
  suggestedMiles: number,
): string {
  if (units === 'imperial') {
    return last30dAvgMiles
      ? `We saw ~${last30dAvgMiles} miles/week in your last 30 days. Most plans progress +10% per week — we suggest ${suggestedMiles} to start.`
      : 'Most plans progress +10% per week. Pick a number that feels both honest and a little ambitious — we can always adjust.';
  }
  const last30Km = last30dAvgMiles != null ? Math.round(last30dAvgMiles * 1.609344) : null;
  const suggestedKm = Math.round(milesToKm(suggestedMiles));
  return last30Km
    ? `We saw ~${last30Km} km/week in your last 30 days. Most plans progress +10% per week — we suggest ${suggestedKm} to start.`
    : 'Most plans progress +10% per week. Pick a number that feels both honest and a little ambitious — we can always adjust.';
}

