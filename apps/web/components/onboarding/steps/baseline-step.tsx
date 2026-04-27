'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useRef, useState } from 'react';
import { CalligraphicUnderline } from '@/components/onboarding/calligraphic-underline';
import {
  clearWizardStorage,
  useWizard,
  type WizardState,
} from '@/components/onboarding/wizard-context';
import { browserFetch, browserFetchSSE } from '@/lib/api-browser';
import {
  ApiError,
  type OnboardingCompleteRequest,
  type OnboardingCompleteResponse,
  type OnboardingDoneEvent,
  type OnboardingProgressEvent,
  type ProgressState,
  type ProgressStep,
} from '@/lib/api-client';

type BulletStatus = ProgressState | 'pending';

interface Bullet {
  step: ProgressStep;
  label: string;
  status: BulletStatus;
}

const INITIAL_BULLETS: Bullet[] = [
  { step: 'sync',          label: 'Reading your last 30 days',  status: 'pending' },
  { step: 'volume_curve',  label: 'Mapping your volume curve',   status: 'pending' },
  { step: 'baseline',      label: 'Asking the coach for a baseline narrative', status: 'pending' },
  { step: 'plan',          label: 'Drafting your first 8-week plan', status: 'pending' },
];

type Phase =
  | { kind: 'idle' }
  | { kind: 'submitting' }
  | { kind: 'streaming' }
  | { kind: 'done' }
  | { kind: 'error'; message: string };

/**
 * Step 5 — baseline computation. Per insights.md §4.6:
 *  1. POST /v1/me/onboarding/complete with the wizard state.
 *  2. Open SSE on /v1/me/onboarding/stream and update bullets as
 *     `step` events arrive.
 *  3. On `done` event → router.push('/home').
 *
 * Backend may not be ready when this lands — if the POST returns a
 * 5xx or the SSE handshake fails, we render an inline error block
 * but do NOT crash. The user can hit the retry button (re-runs the
 * idempotent POST + reopens the stream).
 */
export function BaselineStep() {
  const router = useRouter();
  const { state, dispatch } = useWizard();
  const [bullets, setBullets] = useState<Bullet[]>(INITIAL_BULLETS);
  const [phase, setPhase] = useState<Phase>({ kind: 'idle' });
  const [hasReady, setHasReady] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const startedRef = useRef(false);

  useEffect(() => {
    if (startedRef.current) return;
    const complete = checkStateComplete(state);
    if (!complete) return; // wait for hydration
    startedRef.current = true;
    void runOnboarding(complete, {
      onPhaseChange: setPhase,
      onBullet: (step, status) =>
        setBullets((prev) =>
          prev.map((b) => (b.step === step ? { ...b, status } : b)),
        ),
      registerAbort: (ctrl) => {
        abortRef.current = ctrl;
      },
      onDone: () => {
        setPhase({ kind: 'done' });
        setHasReady(true);
        clearWizardStorage();
        dispatch({ type: 'RESET' });
        // 1.2s ride-out so the "ready." flourish lands before nav.
        window.setTimeout(() => router.push('/home'), 1200);
      },
    });
    return () => {
      abortRef.current?.abort();
    };
  }, [state, dispatch, router]);

  const isError = phase.kind === 'error';
  const errorMessage = phase.kind === 'error' ? phase.message : null;

  return (
    <div className="relative mx-auto flex max-w-[760px] flex-col items-center text-center">
      <p className="font-mono text-[11px] uppercase tracking-[0.22em] text-white/45">
        READING YOUR LAST 30 DAYS
      </p>
      <h1 className="mt-4 max-w-[18ch] text-[40px] font-medium leading-[0.98] tracking-[-0.03em] text-white md:text-[56px]">
        We're <span className="display">calibrating.</span>
      </h1>
      <CalligraphicUnderline className="mt-2 text-white/65" delayMs={400} />

      <ShimmerBand />

      <ul className="mt-2 flex w-full max-w-[480px] flex-col gap-3 text-left">
        {bullets.map((b) => (
          <BulletRow key={b.step} bullet={b} />
        ))}
      </ul>

      <p className="mt-10 font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/40">
        TAKES 8&ndash;15 SECONDS
      </p>

      {hasReady ? (
        <p
          className="display mt-6 text-[40px] text-white/95"
          style={{ animation: 'fade-in 600ms ease-out 240ms both' }}
        >
          ready.
        </p>
      ) : null}

      {isError ? (
        <div className="mt-8 rounded-[var(--radius-card)] border border-[var(--color-danger)]/30 bg-[var(--color-danger)]/[0.06] p-5 text-left">
          <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-[var(--color-danger)]">
            COULDN'T FINISH SETUP
          </p>
          <p className="mt-2 text-[14px] text-white/75">{errorMessage}</p>
          <p className="mt-3 text-[13px] text-white/55">
            The backend may still be coming up. Your wizard answers are saved.
          </p>
          <div className="mt-4 flex items-center gap-3">
            <button
              type="button"
              onClick={() => {
                startedRef.current = false;
                setBullets(INITIAL_BULLETS);
                setPhase({ kind: 'idle' });
                // re-trigger the effect by mutating one state field.
                dispatch({ type: 'SET_FIELD', field: 'focus', value: state.focus });
              }}
              className="inline-flex h-10 items-center gap-2 rounded-full bg-white px-5 text-[13px] font-medium text-black transition hover:bg-white/90"
            >
              Retry
            </button>
            <button
              type="button"
              onClick={() => router.push('/home')}
              className="inline-flex h-10 items-center rounded-full border border-white/10 px-5 text-[13px] text-white/65 transition hover:border-white/25 hover:text-white"
            >
              Back to /home
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function BulletRow({ bullet }: { bullet: Bullet }) {
  const icon =
    bullet.status === 'done'
      ? '✓'
      : bullet.status === 'in_flight'
        ? '◌'
        : bullet.status === 'error'
          ? '×'
          : '○';
  const tone =
    bullet.status === 'done'
      ? 'text-[var(--color-success)]'
      : bullet.status === 'in_flight'
        ? 'text-white'
        : bullet.status === 'error'
          ? 'text-[var(--color-danger)]'
          : 'text-white/30';

  return (
    <li className="flex items-center gap-3">
      <span className={`num inline-block w-5 text-center text-[14px] ${tone}`}>{icon}</span>
      <span
        className={`text-[14px] ${
          bullet.status === 'done'
            ? 'text-white/85'
            : bullet.status === 'in_flight'
              ? 'text-white'
              : 'text-white/45'
        }`}
      >
        {bullet.label}
        {bullet.status === 'in_flight' ? <span className="ml-1 animate-pulse text-white/55">…</span> : null}
      </span>
    </li>
  );
}

function ShimmerBand() {
  // Vertical-running aurora. Two stacked gradients, animated via the
  // shimmer-vertical keyframe defined in globals.css. Pure CSS — sells
  // the "the system is alive and working" feeling without a spinner.
  return (
    <div
      aria-hidden="true"
      className="relative my-8 h-[2px] w-full max-w-[480px] overflow-hidden rounded-full bg-white/[0.06]"
    >
      <span
        className="absolute inset-x-0 h-[60%] rounded-full"
        style={{
          background:
            'linear-gradient(180deg, transparent 0%, var(--color-aurora-violet-1) 50%, transparent 100%)',
          animation: 'shimmer-vertical 1.6s ease-in-out infinite',
        }}
      />
      <span
        className="absolute inset-x-0 h-[40%] rounded-full"
        style={{
          background:
            'linear-gradient(180deg, transparent 0%, var(--color-strava) 50%, transparent 100%)',
          animation: 'shimmer-vertical 1.6s ease-in-out infinite 0.5s',
          opacity: 0.65,
        }}
      />
    </div>
  );
}

function checkStateComplete(s: WizardState): CompleteState | null {
  if (
    s.focus === null ||
    s.weekly_miles_target === null ||
    s.days_per_week === null
  ) {
    return null;
  }
  return {
    focus: s.focus,
    weekly_miles_target: s.weekly_miles_target,
    days_per_week: s.days_per_week,
    target_distance_km: s.target_distance_km,
    target_pace_sec_per_km: s.target_pace_sec_per_km,
    race_date: s.race_date,
  };
}

interface RunHandlers {
  onPhaseChange: (p: Phase) => void;
  onBullet: (step: ProgressStep, status: BulletStatus) => void;
  registerAbort: (ctrl: AbortController) => void;
  onDone: () => void;
}

interface CompleteState {
  focus: NonNullable<WizardState['focus']>;
  weekly_miles_target: number;
  days_per_week: number;
  target_distance_km: number | null;
  target_pace_sec_per_km: number | null;
  race_date: string | null;
}

async function runOnboarding(state: CompleteState, handlers: RunHandlers) {
  const { onPhaseChange, onBullet, registerAbort, onDone } = handlers;
  const ctrl = new AbortController();
  registerAbort(ctrl);
  onPhaseChange({ kind: 'submitting' });
  try {
    const body: OnboardingCompleteRequest = {
      focus: state.focus,
      weekly_miles_target: state.weekly_miles_target,
      days_per_week: state.days_per_week,
      target_distance_km: state.target_distance_km,
      target_pace_sec_per_km: state.target_pace_sec_per_km,
      race_date: state.race_date,
    };
    await browserFetch<OnboardingCompleteResponse>('/v1/me/onboarding/complete', {
      method: 'POST',
      body,
    });
  } catch (err) {
    const msg =
      err instanceof ApiError
        ? `${err.status} · ${describeError(err.body)}`
        : err instanceof Error
          ? err.message
          : 'unknown error';
    onPhaseChange({ kind: 'error', message: msg });
    return;
  }

  onPhaseChange({ kind: 'streaming' });
  try {
    await browserFetchSSE(
      '/v1/me/onboarding/stream',
      (event, data) => {
        if (event === 'step') {
          const ev = data as OnboardingProgressEvent;
          onBullet(ev.step, ev.state);
        } else if (event === 'error') {
          const ev = data as OnboardingProgressEvent;
          onBullet(ev.step, 'error');
        } else if (event === 'done') {
          // Defer; the stream may emit any remaining `step done`
          // events on the same tick. The reader will exit naturally.
          void (data as OnboardingDoneEvent | null);
        }
      },
      ctrl.signal,
    );
    // Stream closed cleanly → done.
    onDone();
  } catch (err) {
    if (ctrl.signal.aborted) return; // unmount, not an error
    const msg = err instanceof Error ? err.message : 'sse failed';
    onPhaseChange({ kind: 'error', message: msg });
  }
}

function describeError(body: unknown): string {
  if (body && typeof body === 'object' && 'detail' in body) {
    const d = (body as { detail?: unknown }).detail;
    if (typeof d === 'string') return d;
  }
  return 'request failed';
}
