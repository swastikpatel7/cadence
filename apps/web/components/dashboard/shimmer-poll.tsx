'use client';

import { useRouter } from 'next/navigation';
import { useEffect } from 'react';

/**
 * Tiny client utility for the shimmer-state dashboard. Calls
 * `router.refresh()` every 5s so the parent server component re-runs
 * its heatmap fetch and (hopefully) finds a real plan this time.
 *
 * Renders nothing — pure side effect (insights.md §17 graceful fallback
 * for "plan still generating").
 */
export function ShimmerPoll({ intervalMs = 5000 }: { intervalMs?: number }) {
  const router = useRouter();
  useEffect(() => {
    const id = window.setInterval(() => {
      router.refresh();
    }, intervalMs);
    return () => window.clearInterval(id);
  }, [router, intervalMs]);
  return null;
}
