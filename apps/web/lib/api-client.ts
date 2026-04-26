/**
 * Shared types + ApiError for both server and client fetchers.
 *
 * The actual fetchers live in two sibling files so that Clerk's
 * server-only and client-only modules don't end up in the wrong bundle:
 *   - lib/api-server.ts → serverFetch (uses @clerk/nextjs/server)
 *   - lib/api-browser.ts → browserFetch (uses window.Clerk)
 *
 * Both server and client components are free to import this file.
 */

export class ApiError extends Error {
  status: number;
  body: unknown;
  constructor(status: number, body: unknown, message?: string) {
    super(message ?? `API error ${status}`);
    this.status = status;
    this.body = body;
  }
}

// ──────────────────────────────────────────────────────────────────
// Wire types — keep in sync with apps/api/internal/connections
// (handlers.go SyncStatusBody and friends).
// ──────────────────────────────────────────────────────────────────

export interface SyncStatus {
  syncing: boolean;
  started_at?: string | null;
  processed: number;
  total_known: number;
  last_sync_at?: string | null;
  last_error?: string | null;
  total_activities: number;
  sport_breakdown: Record<string, number>;
  recent: RecentActivity[];
  connection: Connection | null;
}

export interface RecentActivity {
  id: string;
  name: string;
  sport_type: string;
  start_time: string;
  distance_meters?: number | null;
}

export interface Connection {
  connected: boolean;
  athlete_name: string;
  scopes: string[];
  connected_at: string;
}

export interface StartOAuthResponse {
  authorize_url: string;
}
