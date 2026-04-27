package plan

import "github.com/google/uuid"

// Job kinds. The strings are wire-stable: insert sites and worker
// dispatchers must use these constants, not literals.
const (
	InitialJobKind          = "plan_initial"
	WeeklyRefreshJobKind    = "plan_weekly_refresh"
	SessionMicroSummaryKind = "session_micro_summary"
)

// Refresh reasons land on coach_plans.reason. Three legal values.
const (
	ReasonOnboarding = "onboarding"
	ReasonWeeklyCron = "weekly_cron"
	ReasonGoalChange = "goal_change"
	ReasonManual     = "manual"
)

// Generation kinds align with the schema CHECK constraint.
const (
	GenerationKindInitial8Wk    = "initial_8wk"
	GenerationKindWeeklyRefresh = "weekly_refresh"
)

// InitialJobArgs is the River JobArgs for the one-time 8-week plan
// generation. Inserted exclusively by BaselineComputeWorker after a
// successful onboarding-trigger baseline narrative.
type InitialJobArgs struct {
	UserID     uuid.UUID `json:"user_id"`
	GoalID     uuid.UUID `json:"goal_id"`
	BaselineID uuid.UUID `json:"baseline_id"`
}

// Kind satisfies river.JobArgs.
func (InitialJobArgs) Kind() string { return InitialJobKind }

// WeeklyRefreshArgs is the River JobArgs for the recurring weekly plan
// refresh. Inserted by the periodic cron, the plan-refresh handler, and
// the goal-PATCH handler.
type WeeklyRefreshArgs struct {
	UserID    uuid.UUID `json:"user_id"`
	WeekStart string    `json:"week_start"` // YYYY-MM-DD, Monday-anchored
	Reason    string    `json:"reason"`
}

// Kind satisfies river.JobArgs.
func (WeeklyRefreshArgs) Kind() string { return WeeklyRefreshJobKind }

// SessionMicroSummaryArgs is the River JobArgs for the lazy Haiku
// micro-summary the session-detail handler enqueues when a past day's
// drawer is opened with a matched activity but no `coach_insights` row.
type SessionMicroSummaryArgs struct {
	UserID     uuid.UUID `json:"user_id"`
	ActivityID uuid.UUID `json:"activity_id"`
	// Prescribed is serialized inline so the worker doesn't have to
	// re-join the plan blob to know what the user was supposed to do.
	Prescribed PrescribedSessionArgs `json:"prescribed"`
}

// PrescribedSessionArgs is the inline copy of the planned session the
// worker passes into the prompt. Values match the wire shape on
// PrescribedSessionDTO (api.md §2.8).
type PrescribedSessionArgs struct {
	Date               string  `json:"date"`
	Type               string  `json:"type"`
	DistanceKM         float64 `json:"distance_km"`
	Intensity          string  `json:"intensity"`
	PaceTargetSecPerKM *int    `json:"pace_target_sec_per_km,omitempty"`
	DurationMinTarget  *int    `json:"duration_min_target,omitempty"`
	NotesForCoach      string  `json:"notes_for_coach"`
}

// Kind satisfies river.JobArgs.
func (SessionMicroSummaryArgs) Kind() string { return SessionMicroSummaryKind }
