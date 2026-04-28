'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { RunnerSlider } from '@/components/onboarding/runner-slider';
import { Button } from '@/components/ui/button';
import { GlassCard } from '@/components/ui/glass-card';
import { Spinner } from '@/components/ui/spinner';
import { useUnits } from '@/components/units/units-context';
import { browserFetch } from '@/lib/api-browser';
import type {
  GoalFocus,
  GoalPatchRequest,
  UserGoal,
} from '@/lib/api-client';
import { cn } from '@/lib/cn';
import { formatPace, formatWeeklyVolume } from '@/lib/units';

interface Props {
  initial: UserGoal | null;
}

const FOCUSES: { id: GoalFocus; label: string }[] = [
  { id: 'general',         label: 'General'        },
  { id: 'build_distance',  label: 'Distance'       },
  { id: 'build_speed',     label: 'Speed'          },
  { id: 'train_for_race',  label: 'Race'           },
];

const DAYS = [3, 4, 5, 6, 7];

/**
 * Settings → Edit goal. The card itself is a quiet summary; the
 * "Edit" button opens a modal with a compact one-page form. On
 * submit: PATCH /v1/me/goal → router.refresh() + show "plan
 * refreshing…" pill until the next /home render lands.
 */
export function EditGoalCard({ initial }: Props) {
  const [open, setOpen] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const router = useRouter();
  const { units } = useUnits();

  if (!initial) {
    return (
      <GlassCard className="p-7">
        <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
          GOAL
        </p>
        <p className="mt-3 text-[14px] text-white/55">
          You haven't onboarded yet.{' '}
          <a href="/onboarding" className="text-white underline-offset-4 hover:underline">
            Build a plan →
          </a>
        </p>
      </GlassCard>
    );
  }

  const focus = FOCUSES.find((f) => f.id === initial.focus)?.label ?? initial.focus;

  return (
    <>
      <GlassCard className="relative overflow-hidden p-7">
        <div
          className="pointer-events-none absolute -right-12 -top-12 h-44 w-44 rounded-full opacity-50"
          style={{
            background:
              'radial-gradient(closest-side, oklch(0.72 0.22 45 / 0.40) 0%, transparent 70%)',
            filter: 'blur(36px)',
          }}
        />
        <div className="relative">
          <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
            GOAL
          </p>
          <h3 className="mt-1 text-[20px] font-medium leading-[1.1] tracking-[-0.02em] text-white">
            {focus} · {formatWeeklyVolume(initial.weekly_miles_target, units)} · {initial.days_per_week}d
          </h3>
          {initial.target_distance_km && initial.target_pace_sec_per_km ? (
            <p className="mt-2 font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/55">
              TARGET ·{' '}
              <span className="num">{initial.target_distance_km.toFixed(1)}</span>KM @{' '}
              <span className="num">{formatPace(initial.target_pace_sec_per_km, units)}</span>
            </p>
          ) : null}

          <div className="mt-5 flex flex-wrap items-center gap-3">
            <Button onClick={() => setOpen(true)}>Edit goal</Button>
            {refreshing ? (
              <span className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.04] px-3 py-1.5 backdrop-blur-md">
                <Spinner size={12} />
                <span className="font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/65">
                  PLAN REFRESHING…
                </span>
              </span>
            ) : null}
          </div>
        </div>
      </GlassCard>

      {open ? (
        <EditGoalModal
          initial={initial}
          onClose={() => setOpen(false)}
          onSaved={() => {
            setOpen(false);
            setRefreshing(true);
            router.refresh();
            // Drop the pill after a few seconds — the user can refresh
            // themselves if the plan is still working.
            window.setTimeout(() => setRefreshing(false), 8000);
          }}
        />
      ) : null}
    </>
  );
}

function EditGoalModal({
  initial,
  onClose,
  onSaved,
}: {
  initial: UserGoal;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { units } = useUnits();
  const [focus, setFocus] = useState<GoalFocus>(initial.focus);
  const [miles, setMiles] = useState<number>(initial.weekly_miles_target);
  const [daysPerWeek, setDaysPerWeek] = useState<number>(initial.days_per_week);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  async function handleSubmit() {
    setSubmitting(true);
    setError(null);
    const body: GoalPatchRequest = {};
    if (focus !== initial.focus) body.focus = focus;
    if (miles !== initial.weekly_miles_target) body.weekly_miles_target = miles;
    if (daysPerWeek !== initial.days_per_week) body.days_per_week = daysPerWeek;
    if (Object.keys(body).length === 0) {
      onClose();
      return;
    }
    try {
      await browserFetch('/v1/me/goal', { method: 'PATCH', body });
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to save');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      <div
        aria-hidden="true"
        onClick={onClose}
        onKeyDown={(e) => e.key === 'Enter' && onClose()}
        role="presentation"
        className="fixed inset-0 z-50 bg-black/55 backdrop-blur-sm"
      />
      <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
        <GlassCard className="w-full max-w-[560px] p-7">
          <div className="flex items-start justify-between">
            <div>
              <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
                EDIT GOAL
              </p>
              <h2 className="mt-2 text-[24px] font-medium leading-[1.1] tracking-[-0.02em] text-white">
                Tune your <span className="display">plan.</span>
              </h2>
            </div>
            <button
              type="button"
              onClick={onClose}
              aria-label="Close"
              className="inline-flex h-8 w-8 items-center justify-center rounded-full border border-white/10 bg-white/[0.04] text-white/65 transition hover:border-white/20 hover:text-white"
            >
              <svg
                aria-hidden="true"
                role="presentation"
                width="10"
                height="10"
                viewBox="0 0 10 10"
              >
                <path
                  d="M1.5 1.5l7 7M8.5 1.5l-7 7"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                />
              </svg>
            </button>
          </div>

          <div className="mt-6 flex flex-col gap-6">
            <Section label="FOCUS">
              <div className="flex flex-wrap gap-2">
                {FOCUSES.map((f) => {
                  const sel = focus === f.id;
                  return (
                    <button
                      key={f.id}
                      type="button"
                      onClick={() => setFocus(f.id)}
                      aria-pressed={sel}
                      className={cn(
                        'inline-flex h-9 items-center justify-center rounded-full border px-3 text-[12.5px] transition',
                        sel
                          ? 'border-[oklch(0.55_0.27_295_/_0.55)] bg-white/[0.10] text-white'
                          : 'border-white/10 bg-white/[0.03] text-white/65 hover:border-white/25 hover:text-white',
                      )}
                    >
                      {f.label}
                    </button>
                  );
                })}
              </div>
            </Section>

            <Section label={`WEEKLY VOLUME · ${formatWeeklyVolume(miles, units).toUpperCase()}`}>
              <RunnerSlider
                min={5}
                max={80}
                value={miles}
                onChange={setMiles}
                ariaLabel="Weekly miles target"
              />
            </Section>

            <Section label="DAYS PER WEEK">
              <div className="flex flex-wrap gap-2">
                {DAYS.map((d) => {
                  const sel = daysPerWeek === d;
                  return (
                    <button
                      key={d}
                      type="button"
                      onClick={() => setDaysPerWeek(d)}
                      aria-pressed={sel}
                      className={cn(
                        'inline-flex h-10 w-10 items-center justify-center rounded-full border text-[14px] transition',
                        sel
                          ? 'border-[oklch(0.55_0.27_295_/_0.55)] bg-white/[0.10] text-white'
                          : 'border-white/10 bg-white/[0.03] text-white/65 hover:border-white/25 hover:text-white',
                      )}
                    >
                      <span className="num">{d}</span>
                    </button>
                  );
                })}
              </div>
            </Section>
          </div>

          <div className="mt-7 flex items-center justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              className="inline-flex h-10 items-center rounded-full border border-white/10 px-5 text-[13px] text-white/65 transition hover:border-white/25 hover:text-white"
            >
              Cancel
            </button>
            <Button onClick={handleSubmit} disabled={submitting}>
              {submitting ? <Spinner size={12} /> : null}
              Save changes
            </Button>
          </div>

          {error ? (
            <p className="mt-4 text-[12.5px] text-[var(--color-danger)]">{error}</p>
          ) : null}
        </GlassCard>
      </div>
    </>
  );
}

function Section({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <p className="font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/45">
        {label}
      </p>
      <div className="mt-3">{children}</div>
    </div>
  );
}

