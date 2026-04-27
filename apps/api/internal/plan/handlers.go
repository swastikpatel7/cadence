package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/swastikpatel7/cadence/apps/api/internal/auth"
	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
)

// HandlerDeps wires the handlers. River insertion is via the bare
// interface so this package doesn't bind to the concrete client type.
type HandlerDeps struct {
	DB      *pgxpool.Pool
	Queries *dbgen.Queries
	River   RiverInsertable
}

// RiverInsertable is the slim insert surface this package needs.
type RiverInsertable interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Register mounts every plan-* endpoint on the supplied authed Huma
// group. The SSE handler does NOT live here — it's registered on the
// chi router by the server package because Huma assumes JSON.
func Register(authed huma.API, d HandlerDeps) {
	registerHeatmap(authed, d)
	registerSession(authed, d)
	registerRefresh(authed, d)
}

// ─── GET /v1/me/plan/heatmap ──────────────────────────────────────────

// HeatmapGetInput maps the query params from api.md §2.7.
type HeatmapGetInput struct {
	WeeksBack    int `query:"weeks_back" default:"2" minimum:"0" maximum:"8"`
	WeeksForward int `query:"weeks_forward" default:"6" minimum:"0" maximum:"12"`
}

// HeatmapGetOutput is the wire shape.
type HeatmapGetOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		Weeks [][]HeatmapCell `json:"weeks"`
	}
}

func registerHeatmap(api huma.API, d HandlerDeps) {
	huma.Register(api, huma.Operation{
		OperationID: "plan-heatmap-get",
		Method:      http.MethodGet,
		Path:        "/v1/me/plan/heatmap",
		Summary:     "Get the calendar-heatmap projection of the user's current plan",
		Tags:        []string{"plan"},
	}, func(ctx context.Context, in *HeatmapGetInput) (*HeatmapGetOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}

		now := time.Now().UTC()
		thisWeekMonday := MondayOf(now)
		windowStart := thisWeekMonday.AddDate(0, 0, -7*in.WeeksBack)
		windowEnd := thisWeekMonday.AddDate(0, 0, 7*in.WeeksForward+6) // inclusive of last Sunday

		// SQL: starts_on <= $3 AND (starts_on + ...)::date >= $2.
		// sqlc maps struct fields by struct order to $1,$2,$3 — so:
		//   StartsOn   ($2) = window_start (lower bound)
		//   StartsOn_2 ($3) = window_end   (upper bound)
		plans, err := d.Queries.GetPlanWindow(ctx, dbgen.GetPlanWindowParams{
			UserID:     userID,
			StartsOn:   pgtype.Date{Time: windowStart, Valid: true},
			StartsOn_2: pgtype.Date{Time: windowEnd, Valid: true},
		})
		if err != nil {
			pkglogger.FromContext(ctx).Error("plan.heatmap: load plans", "err", err)
			return nil, huma.Error500InternalServerError("failed to load plan window")
		}

		acts, err := d.Queries.ListActivitiesInWindow(ctx, dbgen.ListActivitiesInWindowParams{
			UserID:      userID,
			StartTime:   pgtype.Timestamptz{Time: windowStart, Valid: true},
			StartTime_2: pgtype.Timestamptz{Time: windowEnd.AddDate(0, 0, 1), Valid: true},
		})
		if err != nil {
			pkglogger.FromContext(ctx).Error("plan.heatmap: load activities", "err", err)
			return nil, huma.Error500InternalServerError("failed to load activities")
		}

		hin := HeatmapInput{
			WindowStart: windowStart,
			WindowEnd:   windowEnd,
			Today:       now,
		}
		for _, p := range plans {
			hin.Plans = append(hin.Plans, PlanRow{
				StartsOn:   p.StartsOn.Time,
				WeeksCount: int(p.WeeksCount),
				Plan:       p.Plan,
			})
		}
		for _, a := range acts {
			distM, _ := numericFloatOK(a.DistanceMeters)
			distKM := distM / 1000.0
			pace := 0
			if a.DurationSeconds > 0 && distKM > 0 {
				pace = int(float64(a.DurationSeconds) / distKM)
			}
			hin.Activities = append(hin.Activities, ActivityRow{
				ID:              a.ID.String(),
				StartTime:       a.StartTime.Time,
				DistanceKM:      distKM,
				DurationSeconds: int(a.DurationSeconds),
				AvgPaceSecPerKM: pace,
			})
		}

		out := &HeatmapGetOutput{CacheControl: "private, max-age=60"}
		out.Body.Weeks = ProjectHeatmap(hin)
		return out, nil
	})
}

// ─── GET /v1/me/plan/session/:date ────────────────────────────────────

// SessionGetInput is the path param.
type SessionGetInput struct {
	Date string `path:"date" pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
}

// PrescribedSessionDTO mirrors api.md §2.8.
type PrescribedSessionDTO struct {
	Date               string  `json:"date"`
	Type               string  `json:"type"`
	DistanceKM         float64 `json:"distance_km"`
	Intensity          string  `json:"intensity"`
	PaceTargetSecPerKM *int    `json:"pace_target_sec_per_km,omitempty"`
	DurationMinTarget  *int    `json:"duration_min_target,omitempty"`
	NotesForCoach      string  `json:"notes_for_coach"`
}

// ActualSessionDTO mirrors api.md §2.8.
type ActualSessionDTO struct {
	ActivityID      string  `json:"activity_id"`
	Completed       bool    `json:"completed"`
	DistanceKM      float64 `json:"distance_km"`
	AvgPaceSecPerKM int     `json:"avg_pace_sec_per_km"`
	DurationSeconds int     `json:"duration_seconds"`
	Matched         string  `json:"matched"`
	StartedAt       string  `json:"started_at"`
}

// SessionGetOutput wraps the body.
type SessionGetOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		Prescribed   PrescribedSessionDTO `json:"prescribed"`
		Actual       *ActualSessionDTO    `json:"actual,omitempty"`
		MicroSummary *string              `json:"micro_summary,omitempty"`
	}
}

func registerSession(api huma.API, d HandlerDeps) {
	huma.Register(api, huma.Operation{
		OperationID: "plan-session-get",
		Method:      http.MethodGet,
		Path:        "/v1/me/plan/session/{date}",
		Summary:     "Get the prescribed + actual detail for a single calendar date",
		Tags:        []string{"plan"},
	}, func(ctx context.Context, in *SessionGetInput) (*SessionGetOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}
		log := pkglogger.FromContext(ctx)

		dayStart, err := time.Parse("2006-01-02", in.Date)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid date format (want YYYY-MM-DD)")
		}

		current, err := d.Queries.GetCurrentPlanByUserID(ctx, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, huma.Error404NotFound("no plan yet")
			}
			log.Error("plan.session: load plan", "err", err)
			return nil, huma.Error500InternalServerError("failed to load plan")
		}

		sess, err := SessionForDate(current.Plan, in.Date)
		if err != nil {
			log.Error("plan.session: decode plan", "err", err)
			return nil, huma.Error500InternalServerError("failed to decode plan")
		}
		if sess == nil {
			return nil, huma.Error404NotFound("date is outside the current plan window")
		}

		out := &SessionGetOutput{CacheControl: "private, max-age=60"}
		out.Body.Prescribed = PrescribedSessionDTO{
			Date:               sess.Date,
			Type:               sess.Type,
			DistanceKM:         sess.DistanceKM,
			Intensity:          sess.Intensity,
			PaceTargetSecPerKM: sess.PaceTargetSecPerKM,
			DurationMinTarget:  sess.DurationMinTarget,
			NotesForCoach:      sess.NotesForCoach,
		}

		// Actual + micro-summary lookup.
		row, actErr := d.Queries.GetActivityForDate(ctx, dbgen.GetActivityForDateParams{
			UserID:    userID,
			StartTime: pgtype.Timestamptz{Time: dayStart, Valid: true},
		})
		if actErr == nil {
			distM, _ := numericFloatOK(row.DistanceMeters)
			distKM := distM / 1000.0
			pace := 0
			if row.DurationSeconds > 0 && distKM > 0 {
				pace = int(float64(row.DurationSeconds) / distKM)
			}
			actID := row.ID
			out.Body.Actual = &ActualSessionDTO{
				ActivityID:      actID.String(),
				Completed:       true,
				DistanceKM:      round2(distKM),
				AvgPaceSecPerKM: pace,
				DurationSeconds: int(row.DurationSeconds),
				Matched:         matchedFor(prescribedKMPtr(sess.DistanceKM), distKM),
				StartedAt:       row.StartTime.Time.UTC().Format("2006-01-02T15:04:05Z07:00"),
			}

			// Lazy micro-summary: read cache; if miss, enqueue a job.
			insight, ierr := d.Queries.GetInsightForActivity(ctx, dbgen.GetInsightForActivityParams{
				ActivityID: ptrUUID(actID),
				Kind:       "micro_summary",
			})
			if ierr == nil {
				body := insight.Body
				out.Body.MicroSummary = &body
			} else if errors.Is(ierr, pgx.ErrNoRows) && d.River != nil {
				// Only enqueue if past day and we actually have an activity.
				if dayStart.Before(time.Now().UTC().AddDate(0, 0, 1)) {
					_, enq := d.River.Insert(ctx, SessionMicroSummaryArgs{
						UserID:     userID,
						ActivityID: actID,
						Prescribed: PrescribedSessionArgs{
							Date:               sess.Date,
							Type:               sess.Type,
							DistanceKM:         sess.DistanceKM,
							Intensity:          sess.Intensity,
							PaceTargetSecPerKM: sess.PaceTargetSecPerKM,
							DurationMinTarget:  sess.DurationMinTarget,
							NotesForCoach:      sess.NotesForCoach,
						},
					}, &river.InsertOpts{
						UniqueOpts: river.UniqueOpts{
							ByArgs:   true,
							ByPeriod: 5 * time.Minute,
						},
					})
					if enq != nil {
						log.Warn("plan.session: enqueue micro_summary failed", "err", enq)
					}
				}
			} else if !errors.Is(ierr, pgx.ErrNoRows) {
				log.Warn("plan.session: load insight failed", "err", ierr)
			}
		} else if !errors.Is(actErr, pgx.ErrNoRows) {
			log.Warn("plan.session: load activity failed", "err", actErr)
		}

		return out, nil
	})
}

// ─── POST /v1/me/plan/refresh ─────────────────────────────────────────

// PlanRefreshInput is the request body.
type PlanRefreshInput struct {
	Body struct {
		Reason string `json:"reason" enum:"weekly_cron,manual,goal_change"`
	}
}

// PlanRefreshOutput returns 202 with the job id.
type PlanRefreshOutput struct {
	Status int
	Body   struct {
		PlanJobID string `json:"plan_job_id"`
	}
}

func registerRefresh(api huma.API, d HandlerDeps) {
	huma.Register(api, huma.Operation{
		OperationID: "plan-refresh",
		Method:      http.MethodPost,
		Path:        "/v1/me/plan/refresh",
		Summary:     "Enqueue a weekly plan refresh",
		Tags:        []string{"plan"},
	}, func(ctx context.Context, in *PlanRefreshInput) (*PlanRefreshOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}
		if d.River == nil {
			return nil, huma.Error500InternalServerError("queue not configured")
		}

		// 409 if a refresh is already in-flight for this user.
		inFlight, err := IsRefreshInFlight(ctx, d.DB, userID)
		if err != nil {
			pkglogger.FromContext(ctx).Error("plan.refresh: check in-flight", "err", err)
			return nil, huma.Error500InternalServerError("failed to check job state")
		}
		if inFlight {
			return nil, huma.Error409Conflict("a plan refresh is already in flight")
		}

		nextWeek := FirstMondayAfter(time.Now().UTC())
		args := WeeklyRefreshArgs{
			UserID:    userID,
			WeekStart: nextWeek.Format("2006-01-02"),
			Reason:    in.Body.Reason,
		}
		res, err := d.River.Insert(ctx, args, &river.InsertOpts{
			UniqueOpts: river.UniqueOpts{
				ByArgs:   true,
				ByPeriod: 5 * time.Minute,
			},
		})
		if err != nil {
			pkglogger.FromContext(ctx).Error("plan.refresh: enqueue", "err", err)
			return nil, huma.Error500InternalServerError("failed to enqueue plan refresh")
		}

		out := &PlanRefreshOutput{Status: http.StatusAccepted}
		out.Body.PlanJobID = fmt.Sprintf("%d", res.Job.ID)
		return out, nil
	})
}

// IsRefreshInFlight queries river_job for an active weekly_refresh job
// for the user. Used by both the refresh handler and the goal-PATCH
// handler.
func IsRefreshInFlight(ctx context.Context, db *pgxpool.Pool, userID uuid.UUID) (bool, error) {
	const q = `
SELECT EXISTS (
  SELECT 1 FROM river_job
  WHERE kind = $1
    AND state IN ('available','running','retryable','scheduled')
    AND args->>'user_id' = $2
)`
	var exists bool
	if err := db.QueryRow(ctx, q, WeeklyRefreshJobKind, userID.String()).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// IsBaselineInFlight queries river_job for an active baseline job. Used
// by the baseline-recompute handler to enforce 409 semantics.
func IsBaselineInFlight(ctx context.Context, db *pgxpool.Pool, userID uuid.UUID) (bool, error) {
	const q = `
SELECT EXISTS (
  SELECT 1 FROM river_job
  WHERE kind = 'baseline_compute'
    AND state IN ('available','running','retryable','scheduled')
    AND args->>'user_id' = $1
)`
	var exists bool
	if err := db.QueryRow(ctx, q, userID.String()).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// FindRecentTerminalJob is the helper hinted by api.md §8.1 item 5:
// look up the most recent terminal (completed/discarded) job for the
// user matching kind + a JSONB args predicate, within the last 24h.
// Returns the job id (as a string) and ok.
func FindRecentTerminalJob(
	ctx context.Context,
	db *pgxpool.Pool,
	kind string,
	userID uuid.UUID,
	extraArgPath string, // e.g. "trigger" -> looks up args->>'trigger' = $3
	extraArgValue string,
) (string, bool, error) {
	if extraArgPath != "" {
		const q = `
SELECT id FROM river_job
WHERE kind = $1
  AND state IN ('completed','discarded')
  AND args->>'user_id' = $2
  AND args->>$3 = $4
  AND finalized_at >= now() - interval '24 hours'
ORDER BY finalized_at DESC
LIMIT 1`
		var id int64
		err := db.QueryRow(ctx, q, kind, userID.String(), extraArgPath, extraArgValue).Scan(&id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return "", false, nil
			}
			return "", false, err
		}
		return fmt.Sprintf("%d", id), true, nil
	}
	const q = `
SELECT id FROM river_job
WHERE kind = $1
  AND state IN ('completed','discarded')
  AND args->>'user_id' = $2
  AND finalized_at >= now() - interval '24 hours'
ORDER BY finalized_at DESC
LIMIT 1`
	var id int64
	err := db.QueryRow(ctx, q, kind, userID.String()).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return fmt.Sprintf("%d", id), true, nil
}

// ─── tiny helpers ─────────────────────────────────────────────────────

func prescribedKMPtr(km float64) *float64 {
	if km <= 0 {
		return nil
	}
	v := km
	return &v
}

// MarshalActualForLog is a tiny convenience used by some callers when
// logging the heatmap shape. Currently unused but kept exported so the
// shape is part of the public API.
//
//nolint:unused // exported for future use
func MarshalActualForLog(a *HeatmapActual) []byte {
	if a == nil {
		return nil
	}
	b, _ := json.Marshal(a)
	return b
}
