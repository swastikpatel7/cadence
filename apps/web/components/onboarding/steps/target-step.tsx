'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useCallback, useMemo, useState } from 'react';
import { StepFrame } from '@/components/onboarding/step-frame';
import { useWizard } from '@/components/onboarding/wizard-context';
import { useUnits } from '@/components/units/units-context';
import { cn } from '@/lib/cn';
import { KM_PER_MI, type Units } from '@/lib/units';

interface DistOption {
  id: string;
  label: string;
  km: number | null; // null = none
}

const DIST_OPTIONS: DistOption[] = [
  { id: 'none', label: 'None — keep it open', km: null },
  { id: '5k',   label: '5K',                  km: 5 },
  { id: '10k',  label: '10K',                 km: 10 },
  { id: 'half', label: 'Half marathon',       km: 21.0975 },
  { id: 'full', label: 'Marathon',            km: 42.195 },
];

export function TargetStep() {
  const { state, dispatch } = useWizard();
  const { units } = useUnits();
  const router = useRouter();
  const [paceText, setPaceText] = useState<string>(() =>
    formatPace(state.target_pace_sec_per_km, units),
  );
  const [paceError, setPaceError] = useState<string | null>(null);

  const selectedDistId = useMemo(() => {
    const km = state.target_distance_km;
    if (km == null) return 'none';
    const match = DIST_OPTIONS.find(
      (d) => d.km !== null && Math.abs((d.km ?? 0) - km) < 0.01,
    );
    return match?.id ?? 'none';
  }, [state.target_distance_km]);

  const isRace = state.focus === 'train_for_race';

  const totalTime = useMemo(() => {
    const km = state.target_distance_km;
    const pace = state.target_pace_sec_per_km;
    if (!km || !pace) return null;
    const totalSec = Math.round(km * pace);
    return formatHMS(totalSec);
  }, [state.target_distance_km, state.target_pace_sec_per_km]);

  const handleSelectDist = useCallback(
    (opt: DistOption) => {
      dispatch({
        type: 'SET_FIELD',
        field: 'target_distance_km',
        value: opt.km,
      });
      if (opt.km === null) {
        // Clear pace when distance is cleared.
        dispatch({
          type: 'SET_FIELD',
          field: 'target_pace_sec_per_km',
          value: null,
        });
        setPaceText('');
      }
    },
    [dispatch],
  );

  const handlePaceBlur = useCallback(() => {
    if (!paceText.trim()) {
      dispatch({
        type: 'SET_FIELD',
        field: 'target_pace_sec_per_km',
        value: null,
      });
      setPaceError(null);
      return;
    }
    const secPerUserUnit = parsePaceToSec(paceText, units);
    if (secPerUserUnit == null) {
      setPaceError(units === 'imperial' ? 'Use M:SS — e.g. 7:15' : 'Use M:SS — e.g. 4:30');
      return;
    }
    setPaceError(null);
    // Storage is canonical sec-per-km regardless of toggle.
    const secPerKm = units === 'imperial' ? Math.round(secPerUserUnit / KM_PER_MI) : secPerUserUnit;
    dispatch({
      type: 'SET_FIELD',
      field: 'target_pace_sec_per_km',
      value: secPerKm,
    });
  }, [paceText, dispatch, units]);

  const handleSkip = useCallback(() => {
    dispatch({ type: 'SET_FIELD', field: 'target_distance_km', value: null });
    dispatch({ type: 'SET_FIELD', field: 'target_pace_sec_per_km', value: null });
    dispatch({ type: 'SET_FIELD', field: 'race_date', value: null });
    router.push('/onboarding/baseline');
  }, [dispatch, router]);

  // Distance + pace must come together. Race date is optional but
  // gated to train_for_race focus.
  const distSet = state.target_distance_km !== null;
  const paceSet = state.target_pace_sec_per_km !== null;
  const canContinue =
    (!distSet && !paceSet) || (distSet && paceSet && !paceError);

  return (
    <StepFrame
      eyebrow="ANY SPECIFIC TARGET?"
      title={
        <>
          <span className="display">Going for</span> a{' '}
          <span className="display">number?</span>
        </>
      }
      subprompt="Optional — most users skip this on day one. You can always set a target later from Settings."
      canContinue={canContinue}
      backHref="/onboarding/days"
      nextHref="/onboarding/baseline"
      secondaryAction={
        <button
          type="button"
          onClick={handleSkip}
          className="text-[13px] text-white/55 underline-offset-4 hover:text-white hover:underline"
        >
          Skip this step
        </button>
      }
    >
      <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
        <div className="rounded-[var(--radius-card)] border border-white/10 bg-white/[0.03] p-5 backdrop-blur-2xl">
          <p className="font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/45">
            DISTANCE
          </p>
          <ul className="mt-4 flex flex-col gap-2">
            {DIST_OPTIONS.map((opt) => {
              const selected = selectedDistId === opt.id;
              return (
                <li key={opt.id}>
                  <button
                    type="button"
                    onClick={() => handleSelectDist(opt)}
                    aria-pressed={selected}
                    className={cn(
                      'group flex w-full items-center justify-between rounded-full border px-4 py-2.5 text-left text-[14px] transition',
                      selected
                        ? 'border-[oklch(0.55_0.27_295_/_0.55)] bg-white/[0.08] text-white'
                        : 'border-white/10 bg-white/[0.02] text-white/75 hover:border-white/20 hover:bg-white/[0.06]',
                    )}
                  >
                    <span>{opt.label}</span>
                    <span
                      aria-hidden="true"
                      className={cn(
                        'inline-flex h-4 w-4 items-center justify-center rounded-full border transition',
                        selected
                          ? 'border-white bg-white text-black'
                          : 'border-white/20 bg-transparent',
                      )}
                    >
                      {selected ? (
                        <svg
                          aria-hidden="true"
                          role="presentation"
                          width="9"
                          height="9"
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
                      ) : null}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        </div>

        <div
          className={cn(
            'rounded-[var(--radius-card)] border border-white/10 bg-white/[0.03] p-5 backdrop-blur-2xl transition-opacity',
            !distSet && 'pointer-events-none opacity-40',
          )}
        >
          <p className="font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/45">
            TARGET PACE AT DISTANCE
          </p>
          <div className="mt-4 flex items-center gap-3">
            <input
              type="text"
              inputMode="numeric"
              autoComplete="off"
              spellCheck={false}
              placeholder={units === 'imperial' ? '7:15' : '4:30'}
              value={paceText}
              onChange={(e) => setPaceText(e.currentTarget.value)}
              onBlur={handlePaceBlur}
              className="num h-11 w-24 rounded-full border border-white/10 bg-black/30 px-4 text-center text-[18px] text-white outline-none transition focus:border-white/30 focus:ring-2 focus:ring-white/20"
              aria-label={`Target pace per ${units === 'imperial' ? 'mile' : 'kilometre'}, in M:SS`}
            />
            <span className="display text-[18px] text-white/65">/ {units === 'imperial' ? 'mi' : 'km'}</span>
          </div>

          {paceError ? (
            <p className="mt-3 text-[12px] text-[var(--color-danger)]">{paceError}</p>
          ) : null}

          {totalTime ? (
            <div className="mt-5 inline-flex items-center gap-3 rounded-full border border-white/10 bg-white/[0.04] px-4 py-1.5">
              <span className="font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/50">
                TOTAL
              </span>
              <span className="num text-[14px] text-white">{totalTime}</span>
            </div>
          ) : null}
        </div>
      </div>

      {isRace ? (
        <div className="mt-6 rounded-[var(--radius-card)] border border-white/10 bg-white/[0.03] p-5 backdrop-blur-2xl">
          <p className="font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/45">
            RACE DATE
          </p>
          <div className="mt-3 flex items-center gap-3">
            <input
              type="date"
              value={state.race_date ?? ''}
              onChange={(e) =>
                dispatch({
                  type: 'SET_FIELD',
                  field: 'race_date',
                  value: e.currentTarget.value || null,
                })
              }
              className="num h-11 rounded-full border border-white/10 bg-black/30 px-4 text-[14px] text-white outline-none transition focus:border-white/30 focus:ring-2 focus:ring-white/20"
              aria-label="Race date"
            />
            <Link
              href="/onboarding/focus"
              className="text-[12.5px] text-white/55 underline-offset-4 hover:text-white hover:underline"
            >
              not racing? change focus
            </Link>
          </div>
        </div>
      ) : null}
    </StepFrame>
  );
}

function formatPace(secPerKm: number | null | undefined, units: Units): string {
  if (!secPerKm || secPerKm <= 0) return '';
  const secPerUnit = units === 'imperial' ? Math.round(secPerKm * KM_PER_MI) : secPerKm;
  const m = Math.floor(secPerUnit / 60);
  const s = secPerUnit % 60;
  return `${m}:${s.toString().padStart(2, '0')}`;
}

// Returns seconds per *user-facing* unit (mi when imperial, km when metric).
// Caller is responsible for converting to the canonical sec-per-km storage.
function parsePaceToSec(input: string, units: Units): number | null {
  const m = /^(\d{1,2}):(\d{2})$/.exec(input.trim());
  if (!m) return null;
  const min = Number(m[1]);
  const sec = Number(m[2]);
  if (Number.isNaN(min) || Number.isNaN(sec)) return null;
  if (sec >= 60) return null;
  const total = min * 60 + sec;
  // Sanity bounds in the user's unit. Per-km: 3:00..15:00 (180..900s).
  // Per-mi: 4:50..24:08 — derived by scaling the per-km bound by KM_PER_MI.
  const lower = units === 'imperial' ? 180 * KM_PER_MI : 180;
  const upper = units === 'imperial' ? 900 * KM_PER_MI : 900;
  if (total < lower || total > upper) return null;
  return total;
}

function formatHMS(totalSec: number): string {
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (h > 0) {
    return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  }
  return `${m}:${s.toString().padStart(2, '0')}`;
}
