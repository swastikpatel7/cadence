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

// ──────────────────────────────────────────────────────────────────
// Goals
// ──────────────────────────────────────────────────────────────────

export type GoalFocus = 'general' | 'build_distance' | 'build_speed' | 'train_for_race';

export interface UserGoal {
  id: string;
  focus: GoalFocus;
  weekly_miles_target: number; // miles; 5..80
  days_per_week: number; // 3..7
  target_distance_km?: number | null;
  target_pace_sec_per_km?: number | null;
  race_date?: string | null; // YYYY-MM-DD
  created_at: string; // RFC3339
  updated_at: string;
}

export interface OnboardingCompleteRequest {
  focus: GoalFocus;
  weekly_miles_target: number;
  days_per_week: number;
  target_distance_km?: number | null;
  target_pace_sec_per_km?: number | null;
  race_date?: string | null;
}

export interface OnboardingCompleteResponse {
  goal_id: string;
  baseline_job_id: string;
  plan_job_id: string;
}

export type GoalPatchRequest = Partial<{
  focus: GoalFocus;
  weekly_miles_target: number;
  days_per_week: number;
  target_distance_km: number | null; // explicit null clears
  target_pace_sec_per_km: number | null;
  race_date: string | null;
}>;

// ──────────────────────────────────────────────────────────────────
// Baseline
// ──────────────────────────────────────────────────────────────────

export type FitnessTier = 'T1' | 'T2' | 'T3' | 'T4' | 'T5';
export type BaselineSource = 'onboarding' | 'manual_recompute' | 'sync_milestone';

export interface Baseline {
  id: string;
  computed_at: string;
  window_days: number;
  source: BaselineSource;
  fitness_tier: FitnessTier;
  weekly_volume_km_avg: number;
  weekly_volume_km_p25: number;
  weekly_volume_km_p75: number;
  avg_pace_sec_per_km: number;
  avg_pace_at_distance: Record<string, number>; // e.g. { "5": 294, "10": 319 }
  longest_run_km: number;
  consistency_score: number; // 0..100
  narrative: string;
}

export interface BaselineRecomputeRequest {
  days: 7 | 14 | 30 | 60 | 90 | -1;
}

export interface BaselineRecomputeResponse {
  baseline_job_id: string;
}

// ──────────────────────────────────────────────────────────────────
// Onboarding SSE
// ──────────────────────────────────────────────────────────────────

export type ProgressStep = 'sync' | 'volume_curve' | 'baseline' | 'plan';
export type ProgressState = 'pending' | 'in_flight' | 'done' | 'error';

export interface OnboardingProgressEvent {
  step: ProgressStep;
  state: ProgressState;
  ts: string;
  error?: string;
}

export interface OnboardingDoneEvent {
  ts: string;
}

// ──────────────────────────────────────────────────────────────────
// Plan / heatmap
// ──────────────────────────────────────────────────────────────────

export type PrescribedLoad = 'rest' | 'easy' | 'moderate' | 'hard' | 'peak';
export type PrescribedType =
  | 'easy'
  | 'tempo'
  | 'intervals'
  | 'long'
  | 'recovery'
  | 'race_pace';
export type ActualMatch = 'over' | 'under' | 'on';

export interface HeatmapActual {
  activity_id: string;
  completed: boolean;
  distance_km: number;
  avg_pace_sec_per_km: number;
  matched: ActualMatch;
}

export interface HeatmapCell {
  date: string; // YYYY-MM-DD
  prescribed_load: PrescribedLoad;
  prescribed_distance_km?: number;
  prescribed_type?: PrescribedType;
  is_today: boolean;
  is_future: boolean;
  actual?: HeatmapActual;
}

export type HeatmapWeek = HeatmapCell[]; // exactly 7 cells, Mon..Sun

export interface HeatmapResponse {
  weeks: HeatmapWeek[];
}

// ──────────────────────────────────────────────────────────────────
// Session detail (drawer)
// ──────────────────────────────────────────────────────────────────

export interface PrescribedSession {
  date: string;
  type: PrescribedType;
  distance_km: number;
  intensity: 'easy' | 'moderate' | 'hard';
  pace_target_sec_per_km?: number;
  duration_min_target?: number;
  notes_for_coach: string;
}

export interface ActualSession {
  activity_id: string;
  completed: boolean;
  distance_km: number;
  avg_pace_sec_per_km: number;
  duration_seconds: number;
  matched: ActualMatch;
  started_at: string;
}

export interface SessionDetail {
  prescribed: PrescribedSession;
  actual?: ActualSession;
  micro_summary?: string;
}

// ──────────────────────────────────────────────────────────────────
// Plan refresh
// ──────────────────────────────────────────────────────────────────

export type PlanRefreshReason = 'weekly_cron' | 'manual' | 'goal_change';

export interface PlanRefreshRequest {
  reason: PlanRefreshReason;
}

export interface PlanRefreshResponse {
  plan_job_id: string;
}
