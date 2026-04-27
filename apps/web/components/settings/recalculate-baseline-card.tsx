'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { GlassCard } from '@/components/ui/glass-card';
import { Spinner } from '@/components/ui/spinner';
import { browserFetch } from '@/lib/api-browser';
import {
  ApiError,
  type Baseline,
  type BaselineRecomputeRequest,
  type BaselineRecomputeResponse,
} from '@/lib/api-client';
import { cn } from '@/lib/cn';

const DAYS_OPTIONS: { id: BaselineRecomputeRequest['days']; label: string }[] = [
  { id: 7,   label: '7d' },
  { id: 14,  label: '14d' },
  { id: 30,  label: '30d' },
  { id: 60,  label: '60d' },
  { id: 90,  label: '90d' },
  { id: -1,  label: 'All' },
];

interface Props {
  initial: Baseline | null;
}

type Toast = {
  kind: 'success' | 'error';
  message: string;
} | null;

/**
 * Settings → Recalculate baseline. Chip-row picks the lookback window;
 * the button enqueues `POST /v1/me/baseline/recompute` and surfaces the
 * 200/202/409 outcomes as a transient toast row.
 *
 * On success, polls `GET /v1/me/baseline` every 2s until `computed_at`
 * advances (or the user navigates away), then triggers a
 * `router.refresh()` so the dashboard's `GoalCard` and any baseline
 * narrative re-render.
 */
export function RecalculateBaselineCard({ initial }: Props) {
  const router = useRouter();
  const [days, setDays] = useState<BaselineRecomputeRequest['days']>(30);
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState<Toast>(null);

  const initialComputedAt = initial?.computed_at ?? null;

  async function handleRecompute() {
    setBusy(true);
    setToast(null);
    try {
      await browserFetch<BaselineRecomputeResponse>('/v1/me/baseline/recompute', {
        method: 'POST',
        body: { days } satisfies BaselineRecomputeRequest,
      });
      setToast({
        kind: 'success',
        message: `Recomputing — we'll refresh when the new baseline lands.`,
      });
      void pollUntilFresh(initialComputedAt, () => {
        setToast({
          kind: 'success',
          message: 'Baseline updated. Reloading…',
        });
        router.refresh();
        setBusy(false);
      });
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setToast({
          kind: 'error',
          message: 'A baseline recompute is already running.',
        });
      } else {
        setToast({
          kind: 'error',
          message: err instanceof Error ? err.message : 'failed to start',
        });
      }
      setBusy(false);
    }
  }

  return (
    <GlassCard className="relative overflow-hidden p-7">
      <div
        className="pointer-events-none absolute -right-12 -top-12 h-44 w-44 rounded-full opacity-50"
        style={{
          background:
            'radial-gradient(closest-side, oklch(0.55 0.27 295 / 0.40) 0%, transparent 70%)',
          filter: 'blur(36px)',
        }}
      />
      <div className="relative">
        <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
          BASELINE
        </p>
        <h3 className="mt-1 text-[20px] font-medium leading-[1.1] tracking-[-0.02em] text-white">
          Recalculate from a fresh window
        </h3>
        <p className="mt-3 max-w-[52ch] text-[13.5px] leading-[1.55] text-white/55">
          We last calibrated{' '}
          <span className="text-white/85">{formatRelative(initial?.computed_at)}</span>
          . Recompute over a tighter or wider window to retune your tier and
          weekly volume targets.
        </p>

        <div
          role="radiogroup"
          aria-label="Lookback window"
          className="mt-5 flex flex-wrap gap-2"
        >
          {DAYS_OPTIONS.map((opt) => {
            const selected = days === opt.id;
            return (
              <button
                key={opt.id}
                type="button"
                onClick={() => setDays(opt.id)}
                aria-pressed={selected}
                disabled={busy}
                className={cn(
                  'inline-flex h-9 items-center justify-center rounded-full border px-3 font-mono text-[11px] uppercase tracking-[0.18em] transition',
                  selected
                    ? 'border-[oklch(0.55_0.27_295_/_0.55)] bg-white/[0.10] text-white'
                    : 'border-white/10 bg-white/[0.03] text-white/55 hover:border-white/25 hover:text-white',
                  busy && 'cursor-wait opacity-60',
                )}
              >
                {opt.label}
              </button>
            );
          })}
        </div>

        <div className="mt-6 flex flex-wrap items-center gap-3">
          <Button onClick={handleRecompute} disabled={busy}>
            {busy ? <Spinner size={12} /> : null}
            {busy ? 'Recomputing…' : 'Recalculate baseline'}
          </Button>
          {initial?.fitness_tier ? (
            <span className="font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/45">
              CURRENT TIER · {initial.fitness_tier}
            </span>
          ) : null}
        </div>

        {toast ? (
          <p
            className={cn(
              'mt-4 text-[12.5px]',
              toast.kind === 'success'
                ? 'text-[var(--color-success)]'
                : 'text-[var(--color-warning)]',
            )}
          >
            {toast.message}
          </p>
        ) : null}
      </div>
    </GlassCard>
  );
}

async function pollUntilFresh(
  initialComputedAt: string | null,
  onFresh: () => void,
) {
  const deadline = Date.now() + 2 * 60 * 1000;
  while (Date.now() < deadline) {
    await new Promise((r) => setTimeout(r, 2000));
    try {
      const res = await browserFetch<{ baseline: Baseline }>('/v1/me/baseline');
      if (
        res.baseline.computed_at &&
        res.baseline.computed_at !== initialComputedAt
      ) {
        onFresh();
        return;
      }
    } catch {
      // ignore; just keep polling.
    }
  }
}

function formatRelative(iso: string | null | undefined): string {
  if (!iso) return 'never';
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return 'recently';
  const diffSec = Math.floor((Date.now() - then) / 1000);
  if (diffSec < 60) return 'just now';
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`;
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`;
  return `${Math.floor(diffSec / 86400)}d ago`;
}
