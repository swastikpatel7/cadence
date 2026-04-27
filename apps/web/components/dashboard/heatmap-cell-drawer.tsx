'use client';

import { useEffect, useRef, useState } from 'react';
import { Spinner } from '@/components/ui/spinner';
import { browserFetch } from '@/lib/api-browser';
import { ApiError, type HeatmapCell, type SessionDetail } from '@/lib/api-client';
import { cn } from '@/lib/cn';

interface Props {
  cell: HeatmapCell | null;
  onClose: () => void;
}

/**
 * Slide-in detail panel for a heatmap cell. Fetches the full session
 * detail (`prescribed` + optional `actual` + optional `micro_summary`)
 * via `/v1/me/plan/session/:date` on open, so the heatmap endpoint
 * stays light.
 *
 * Esc + backdrop click close. Per-(date, plan_id) caching is left
 * server-side — the API sets `Cache-Control: private, max-age=60`.
 */
export function HeatmapCellDrawer({ cell, onClose }: Props) {
  const [detail, setDetail] = useState<SessionDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const lastDateRef = useRef<string | null>(null);

  // Fetch on cell change.
  useEffect(() => {
    if (!cell) {
      setDetail(null);
      setError(null);
      lastDateRef.current = null;
      return;
    }
    if (lastDateRef.current === cell.date && detail) {
      return; // already fetched
    }
    lastDateRef.current = cell.date;
    setLoading(true);
    setError(null);
    setDetail(null);
    let cancelled = false;
    browserFetch<SessionDetail>(`/v1/me/plan/session/${cell.date}`)
      .then((res) => {
        if (cancelled) return;
        setDetail(res);
      })
      .catch((err) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 404) {
          setError('No session detail for this date yet.');
        } else {
          setError(err instanceof Error ? err.message : 'failed to load');
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [cell, detail]);

  // Esc closes.
  useEffect(() => {
    if (!cell) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [cell, onClose]);

  const open = cell !== null;

  return (
    <>
      {/* Backdrop */}
      <div
        aria-hidden={!open}
        onClick={onClose}
        onKeyDown={(e) => e.key === 'Enter' && onClose()}
        role="presentation"
        className={cn(
          'fixed inset-0 z-50 bg-black/35 backdrop-blur-sm transition-opacity duration-300',
          open ? 'pointer-events-auto opacity-100' : 'pointer-events-none opacity-0',
        )}
      />

      {/* Drawer */}
      <aside
        aria-label="Session detail"
        className={cn(
          'fixed right-0 top-0 z-50 h-full w-full max-w-[360px] overflow-y-auto border-l border-white/[0.08] bg-[oklch(0.07_0.02_270_/_0.96)] backdrop-blur-2xl transition-transform duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]',
          open ? 'translate-x-0' : 'translate-x-full',
        )}
      >
        {cell ? (
          <DrawerBody
            cell={cell}
            detail={detail}
            loading={loading}
            error={error}
            onClose={onClose}
          />
        ) : null}
      </aside>
    </>
  );
}

function DrawerBody({
  cell,
  detail,
  loading,
  error,
  onClose,
}: {
  cell: HeatmapCell;
  detail: SessionDetail | null;
  loading: boolean;
  error: string | null;
  onClose: () => void;
}) {
  const date = new Date(`${cell.date}T00:00:00Z`).toLocaleDateString('en-US', {
    weekday: 'long',
    month: 'long',
    day: 'numeric',
    timeZone: 'UTC',
  });

  return (
    <div className="flex flex-col gap-6 p-7">
      <div className="flex items-start justify-between">
        <div>
          <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
            {cell.is_today ? 'TODAY' : cell.is_future ? 'UPCOMING' : 'PAST'}
          </p>
          <h2 className="mt-1 text-[24px] font-medium leading-[1.1] tracking-[-0.02em] text-white">
            {date}
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

      {/* Prescribed */}
      <section>
        <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
          PRESCRIBED
        </p>
        {loading ? (
          <div className="mt-3 flex items-center gap-2 text-[13px] text-white/55">
            <Spinner size={12} /> Loading session…
          </div>
        ) : detail ? (
          <PrescribedBlock detail={detail} />
        ) : cell.prescribed_load === 'rest' ? (
          <p className="mt-3 text-[14px] text-white/65">Rest day. Recovery is training.</p>
        ) : (
          <PrescribedBlockFromCell cell={cell} />
        )}
      </section>

      {/* Actual */}
      {(detail?.actual || cell.actual) ? (
        <section>
          <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
            ACTUAL
          </p>
          {detail?.actual ? (
            <ActualBlock actual={detail.actual} />
          ) : (
            <ActualBlockFromCell cell={cell} />
          )}
        </section>
      ) : null}

      {detail?.micro_summary ? (
        <section className="rounded-2xl border border-white/[0.08] bg-white/[0.03] p-4">
          <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
            COACH NOTE
          </p>
          <p className="mt-2 text-[12.5px] leading-[1.45] text-white/75">
            {detail.micro_summary}
          </p>
        </section>
      ) : null}

      {error ? (
        <p className="text-[12.5px] text-[var(--color-warning)]">{error}</p>
      ) : null}
    </div>
  );
}

function PrescribedBlock({ detail }: { detail: SessionDetail }) {
  const p = detail.prescribed;
  const pace = p.pace_target_sec_per_km ? formatPace(p.pace_target_sec_per_km) : null;
  return (
    <div className="mt-3 rounded-2xl border border-white/10 bg-white/[0.03] p-4">
      <div className="flex items-baseline gap-2">
        <span className="display text-[28px] capitalize text-white">{p.type.replace('_', ' ')}</span>
        <span className="num text-[16px] text-white/65">
          · {p.distance_km.toFixed(p.distance_km < 10 ? 1 : 0)} km
        </span>
      </div>
      <div className="mt-3 grid grid-cols-2 gap-3 text-[13px]">
        {pace ? (
          <Field label="PACE" value={`${pace}/km`} />
        ) : (
          <Field label="INTENSITY" value={p.intensity} />
        )}
        {p.duration_min_target ? (
          <Field label="DURATION" value={`${p.duration_min_target} min`} />
        ) : null}
      </div>
      {p.notes_for_coach ? (
        <p className="mt-3 text-[13px] leading-[1.5] text-white/65">{p.notes_for_coach}</p>
      ) : null}
    </div>
  );
}

function PrescribedBlockFromCell({ cell }: { cell: HeatmapCell }) {
  return (
    <div className="mt-3 rounded-2xl border border-white/10 bg-white/[0.03] p-4">
      <div className="flex items-baseline gap-2">
        <span className="display text-[24px] capitalize text-white">
          {cell.prescribed_type ?? cell.prescribed_load}
        </span>
        {cell.prescribed_distance_km ? (
          <span className="num text-[14px] text-white/65">· {cell.prescribed_distance_km} km</span>
        ) : null}
      </div>
      <p className="mt-2 text-[12.5px] text-white/55">
        Detail loads from the API on click; this is what we know from the calendar.
      </p>
    </div>
  );
}

function ActualBlock({
  actual,
}: {
  actual: NonNullable<SessionDetail['actual']>;
}) {
  return (
    <div className="mt-3 rounded-2xl border border-white/10 bg-white/[0.03] p-4">
      <div className="grid grid-cols-2 gap-3 text-[13px]">
        <Field label="DISTANCE" value={`${actual.distance_km.toFixed(2)} km`} />
        <Field label="PACE" value={`${formatPace(actual.avg_pace_sec_per_km)}/km`} />
        <Field label="DURATION" value={formatHMS(actual.duration_seconds)} />
        <Field
          label="VS PRESCRIBED"
          value={actual.matched.toUpperCase()}
          tone={
            actual.matched === 'on'
              ? 'good'
              : actual.matched === 'over'
                ? 'good'
                : 'muted'
          }
        />
      </div>
    </div>
  );
}

function ActualBlockFromCell({ cell }: { cell: HeatmapCell }) {
  if (!cell.actual) return null;
  return (
    <div className="mt-3 rounded-2xl border border-white/10 bg-white/[0.03] p-4">
      <div className="grid grid-cols-2 gap-3 text-[13px]">
        <Field label="DISTANCE" value={`${cell.actual.distance_km.toFixed(2)} km`} />
        <Field label="PACE" value={`${formatPace(cell.actual.avg_pace_sec_per_km)}/km`} />
        <Field label="VS PRESCRIBED" value={cell.actual.matched.toUpperCase()} tone="good" />
      </div>
    </div>
  );
}

function Field({
  label,
  value,
  tone = 'default',
}: {
  label: string;
  value: string;
  tone?: 'default' | 'muted' | 'good';
}) {
  const valColor =
    tone === 'good' ? 'text-[var(--color-success)]' : tone === 'muted' ? 'text-white/55' : 'text-white/95';
  return (
    <div>
      <p className="font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/40">
        {label}
      </p>
      <p className={`num mt-1 ${valColor}`}>{value}</p>
    </div>
  );
}

function formatPace(secPerKm: number): string {
  const m = Math.floor(secPerKm / 60);
  const s = secPerKm % 60;
  return `${m}:${s.toString().padStart(2, '0')}`;
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
