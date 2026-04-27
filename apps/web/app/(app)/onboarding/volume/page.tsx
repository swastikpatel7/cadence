import { VolumeStep } from '@/components/onboarding/steps/volume-step';
import type { SyncStatus } from '@/lib/api-client';
import { serverFetch } from '@/lib/api-server';

export const metadata = {
  title: 'How much running? — Cadence',
};

/**
 * Server component — fetches `/v1/me/sync` so we can read the user's
 * recent activities and surface a suggested weekly miles target
 * (last_30d_avg_miles_per_week × 1.10). Per insights.md §4.3.
 *
 * Sync fetch failures fall through to a neutral suggestion (15 mi/wk).
 */
export default async function OnboardingVolumePage() {
  let suggestedMiles: number | null = null;
  let last30dAvgMiles: number | null = null;
  try {
    const status = await serverFetch<SyncStatus>('/v1/me/sync');
    const m = deriveSuggestionFromSync(status);
    suggestedMiles = m.suggested;
    last30dAvgMiles = m.avg;
  } catch {
    // backend down — show generic copy + a 20 mi suggestion
  }
  return (
    <VolumeStep
      suggestedMiles={suggestedMiles ?? 20}
      last30dAvgMiles={last30dAvgMiles}
    />
  );
}

function deriveSuggestionFromSync(status: SyncStatus): {
  suggested: number;
  avg: number | null;
} {
  if (!status.recent || status.recent.length === 0) {
    return { suggested: 15, avg: null };
  }
  const cutoff = Date.now() - 30 * 24 * 60 * 60 * 1000;
  const totalMeters = status.recent.reduce((acc, a) => {
    if (!a.distance_meters) return acc;
    if (new Date(a.start_time).getTime() < cutoff) return acc;
    if (!a.sport_type.toLowerCase().includes('run')) return acc;
    return acc + a.distance_meters;
  }, 0);
  const totalKm = totalMeters / 1000;
  const avgKm = totalKm / 4.3; // ≈ weeks in 30 days
  const avgMiles = avgKm * 0.621371;
  if (avgMiles <= 0) return { suggested: 15, avg: null };
  const suggested = Math.max(5, Math.min(80, Math.round(avgMiles * 1.1)));
  return { suggested, avg: Math.round(avgMiles) };
}
