'use client';

import { useEffect, useRef, useState } from 'react';
import { GlassCard } from '@/components/ui/glass-card';
import { Select } from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { browserFetch } from '@/lib/api-browser';
import type { SyncStatus } from '@/lib/api-client';

const POLL_INTERVAL_MS = 2000;

interface Props {
  initial: SyncStatus;
}

/**
 * Manual-sync controls + live progress. While `syncing=true` we poll
 * GET /v1/me/sync every 2s and lift state up. The sport-type breakdown
 * + total + recent activities are derived from the polled status.
 */
export function SyncCard({ initial }: Props) {
  const [status, setStatus] = useState<SyncStatus>(initial);
  const [days, setDays] = useState<number>(30);
  const [enqueueing, setEnqueueing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Poll while a sync is in flight.
  useEffect(() => {
    if (!status.syncing) {
      if (pollRef.current) {
        clearInterval(pollRef.current);
        pollRef.current = null;
      }
      return;
    }
    pollRef.current = setInterval(async () => {
      try {
        const next = await browserFetch<SyncStatus>('/v1/me/sync');
        setStatus(next);
      } catch (err) {
        // transient — keep polling, but surface
        setError(err instanceof Error ? err.message : 'poll failed');
      }
    }, POLL_INTERVAL_MS);
    return () => {
      if (pollRef.current) {
        clearInterval(pollRef.current);
        pollRef.current = null;
      }
    };
  }, [status.syncing]);

  const isConnected = status.connection?.connected ?? false;

  async function handleSync() {
    if (!isConnected) return;
    setEnqueueing(true);
    setError(null);
    try {
      await browserFetch('/v1/me/sync', { method: 'POST', body: { days } });
      // optimistic — flip to syncing immediately so the UI shows the bar
      setStatus((s) => ({ ...s, syncing: true, processed: 0 }));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to start sync');
    } finally {
      setEnqueueing(false);
    }
  }

  return (
    <GlassCard className="flex flex-col p-7 md:p-8">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
            ACTIVITY SYNC
          </p>
          <h3 className="mt-1 text-[20px] font-medium leading-[1.1] tracking-[-0.02em] text-white">
            {status.total_activities > 0
              ? `${status.total_activities.toLocaleString()} activities synced`
              : 'No activities yet'}
          </h3>
          {status.last_sync_at ? (
            <p className="mt-1 font-mono text-[11px] uppercase tracking-[0.16em] text-white/45">
              LAST SYNC · {formatRelative(status.last_sync_at)}
            </p>
          ) : null}
        </div>

        {Object.keys(status.sport_breakdown).length > 0 ? (
          <SportBreakdown breakdown={status.sport_breakdown} />
        ) : null}
      </div>

      <div className="mt-7 flex flex-wrap items-center gap-3 border-t border-white/[0.06] pt-6">
        <div className="flex items-center gap-2 text-[12.5px] text-white/55">
          <span className="font-mono uppercase tracking-[0.16em]">WINDOW</span>
          <Select
            aria-label="Sync window in days"
            value={days}
            onChange={(e) => setDays(Number(e.currentTarget.value))}
            disabled={status.syncing || !isConnected}
          >
            <option value={7}>Last 7 days</option>
            <option value={30}>Last 30 days</option>
            <option value={90}>Last 90 days</option>
          </Select>
        </div>

        <button
          type="button"
          onClick={handleSync}
          disabled={!isConnected || status.syncing || enqueueing}
          className="inline-flex h-10 items-center gap-2 rounded-full bg-white px-5 text-[13px] font-medium text-black shadow-[0_8px_24px_-8px_rgb(255_255_255_/_0.35)] transition hover:bg-white/90 disabled:opacity-40 disabled:shadow-none"
        >
          {(enqueueing || status.syncing) ? <Spinner size={12} /> : null}
          {status.syncing ? 'Syncing…' : 'Sync now'}
        </button>

        {!isConnected ? (
          <p className="font-mono text-[10.5px] uppercase tracking-[0.16em] text-white/40">
            CONNECT STRAVA TO ENABLE
          </p>
        ) : null}
      </div>

      {status.syncing ? (
        <div className="mt-5 rounded-xl border border-white/10 bg-black/30 p-4">
          <div className="flex items-baseline justify-between font-mono text-[11px] tracking-[0.16em] text-white/55">
            <span>SYNCING · {status.processed} processed</span>
            {status.last_error ? (
              <span className="text-[var(--color-warning)]">{status.last_error}</span>
            ) : null}
          </div>
          <div className="mt-3 h-1.5 w-full overflow-hidden rounded-full bg-white/[0.06]">
            <div
              className="h-full rounded-full bg-white/40"
              style={{
                width: status.processed > 0 ? `${Math.min(100, (status.processed / Math.max(status.processed + 4, 8)) * 100)}%` : '8%',
                transition: 'width 600ms var(--ease-out-expo)',
              }}
            />
          </div>
        </div>
      ) : null}

      {error ? (
        <p className="mt-4 text-[12.5px] text-[var(--color-danger)]">{error}</p>
      ) : null}

      {status.last_error && !status.syncing ? (
        <p className="mt-4 font-mono text-[11px] uppercase tracking-[0.16em] text-[var(--color-warning)]">
          LAST ERROR · {status.last_error}
        </p>
      ) : null}
    </GlassCard>
  );
}

function SportBreakdown({ breakdown }: { breakdown: Record<string, number> }) {
  const items = Object.entries(breakdown).slice(0, 4);
  return (
    <div className="hidden gap-2 sm:flex sm:flex-wrap">
      {items.map(([sport, count]) => (
        <span
          key={sport}
          className="inline-flex items-center gap-1.5 rounded-full border border-white/10 bg-white/[0.04] px-2.5 py-1 text-[11.5px] text-white/75"
        >
          <span
            aria-hidden
            className="h-1.5 w-1.5 rounded-full"
            style={{ backgroundColor: sportColor(sport) }}
          />
          <span className="num text-white/85">{count}</span>
          <span className="text-white/50">{sport}</span>
        </span>
      ))}
    </div>
  );
}

function sportColor(sport: string): string {
  const s = sport.toLowerCase();
  if (s.includes('run')) return 'var(--color-sport-run)';
  if (s.includes('ride') || s.includes('cycle') || s.includes('bike')) return 'var(--color-sport-ride)';
  if (s.includes('swim')) return 'var(--color-sport-swim)';
  if (s.includes('weight') || s.includes('strength') || s.includes('crossfit')) return 'var(--color-sport-strength)';
  return 'var(--color-fg-muted)';
}

function formatRelative(iso: string): string {
  const then = new Date(iso).getTime();
  const now = Date.now();
  const diffSec = Math.floor((now - then) / 1000);
  if (diffSec < 60) return 'just now';
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`;
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`;
  if (diffSec < 7 * 86400) return `${Math.floor(diffSec / 86400)}d ago`;
  return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}
