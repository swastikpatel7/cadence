'use client';

import { useEffect, useRef, useState } from 'react';
import { GlassCard } from '@/components/ui/glass-card';
import { Spinner } from '@/components/ui/spinner';
import { browserFetch } from '@/lib/api-browser';
import { ApiError } from '@/lib/api-client';
import { cn } from '@/lib/cn';

type State = 'idle' | 'confirming' | 'busy' | 'error';

const CONFIRM_TIMEOUT_MS = 6_000;

/**
 * Settings → Start over.
 *
 * Two-click reveal:
 *   idle        → "Reset onboarding" button.
 *   confirming  → inline "Confirm reset" + "Cancel" row. Auto-reverts
 *                 to idle after 6s if neither is clicked.
 *   busy        → spinner; POSTs `/v1/me/onboarding/reset` then hard-
 *                 navigates to `/onboarding/focus` so the server re-
 *                 evaluates the goal-gate cleanly. router.refresh()
 *                 would race with the redirect chain on /home.
 *
 * The API hard-deletes user_goals (so /v1/me/goal returns 404) and
 * soft-archives baselines / coach_plans / coach_insights for cost
 * auditing + future undo.
 */
export function ResetOnboardingCard() {
  const [state, setState] = useState<State>('idle');
  const [errMsg, setErrMsg] = useState<string | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Auto-revert from confirming → idle after 6s. Clears on unmount.
  useEffect(() => {
    if (state !== 'confirming') return;
    const t = setTimeout(() => setState('idle'), CONFIRM_TIMEOUT_MS);
    timerRef.current = t;
    return () => {
      clearTimeout(t);
      timerRef.current = null;
    };
  }, [state]);

  function clearTimer() {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }

  async function handleConfirm() {
    clearTimer();
    setState('busy');
    setErrMsg(null);
    try {
      await browserFetch('/v1/me/onboarding/reset', { method: 'POST' });
      // Hard nav so the server-side goal gate re-evaluates and
      // `/home` correctly redirects to `/onboarding`.
      window.location.assign('/onboarding/focus');
    } catch (err) {
      const msg =
        err instanceof ApiError
          ? `${err.status} — ${err.message}`
          : err instanceof Error
          ? err.message
          : 'failed to reset';
      setErrMsg(msg);
      setState('error');
    }
  }

  function handleStart() {
    setErrMsg(null);
    setState('confirming');
  }

  function handleCancel() {
    clearTimer();
    setState('idle');
  }

  return (
    <GlassCard className="relative overflow-hidden p-7">
      <div
        aria-hidden
        className="pointer-events-none absolute -right-14 -top-14 h-44 w-44 rounded-full opacity-40"
        style={{
          background:
            'radial-gradient(closest-side, oklch(0.66 0.22 25 / 0.34) 0%, transparent 70%)',
          filter: 'blur(36px)',
        }}
      />
      <div className="relative">
        <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
          DANGER ZONE
        </p>
        <h3 className="mt-1 text-[20px] font-medium leading-[1.1] tracking-[-0.02em] text-white">
          Start over
        </h3>
        <p className="mt-3 max-w-[58ch] text-[13.5px] leading-[1.55] text-white/55">
          Wipes your goal, baseline, plan, and coach insights so the
          onboarding wizard re-runs from scratch. Past data is archived
          (recoverable on request); your Strava connection and synced
          activities stay untouched.
        </p>

        <div className="mt-6 flex flex-wrap items-center gap-3">
          {state === 'idle' || state === 'error' ? (
            <button
              type="button"
              onClick={handleStart}
              className={cn(
                'inline-flex h-9 items-center justify-center rounded-full border px-4',
                'font-mono text-[11px] uppercase tracking-[0.18em] transition',
                'border-[var(--color-danger)]/35 bg-[var(--color-danger)]/[0.06] text-[var(--color-danger)]',
                'hover:border-[var(--color-danger)]/65 hover:bg-[var(--color-danger)]/[0.12]',
              )}
            >
              Reset onboarding
            </button>
          ) : null}

          {state === 'confirming' ? (
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={handleConfirm}
                className={cn(
                  'inline-flex h-9 items-center justify-center rounded-full px-4',
                  'font-mono text-[11px] uppercase tracking-[0.18em] transition',
                  'bg-[var(--color-danger)] text-white shadow-[0_0_0_1px_oklch(0.66_0.22_25_/_0.45),0_8px_24px_-6px_oklch(0.66_0.22_25_/_0.45)]',
                  'hover:brightness-110',
                )}
              >
                Confirm reset
              </button>
              <button
                type="button"
                onClick={handleCancel}
                className={cn(
                  'inline-flex h-9 items-center justify-center rounded-full border px-4',
                  'font-mono text-[11px] uppercase tracking-[0.18em] transition',
                  'border-white/10 bg-white/[0.03] text-white/65 hover:border-white/25 hover:text-white',
                )}
              >
                Cancel
              </button>
              <span className="font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/40">
                THIS CAN'T BE UNDONE FROM THE UI
              </span>
            </div>
          ) : null}

          {state === 'busy' ? (
            <div className="flex items-center gap-2 text-white/65">
              <Spinner size={12} />
              <span className="font-mono text-[10.5px] uppercase tracking-[0.18em]">
                RESETTING…
              </span>
            </div>
          ) : null}
        </div>

        {state === 'error' && errMsg ? (
          <p className="mt-4 text-[12.5px] text-[var(--color-warning)]">
            {errMsg}
          </p>
        ) : null}
      </div>
    </GlassCard>
  );
}
