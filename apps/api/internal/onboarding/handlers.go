package onboarding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
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
	"github.com/swastikpatel7/cadence/apps/api/internal/baseline"
	"github.com/swastikpatel7/cadence/apps/api/internal/plan"
	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
)

// HandlerDeps wires the onboarding endpoints + the goal handlers (kept
// in this package to share the user_goals access pattern).
type HandlerDeps struct {
	DB      *pgxpool.Pool
	Queries *dbgen.Queries
	River   RiverInsertable
}

// RiverInsertable is the slim insert surface needed by these handlers.
type RiverInsertable interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Register mounts /v1/me/onboarding/complete + /v1/me/goal endpoints.
// The SSE handler is registered separately via RegisterSSE on the
// underlying chi.Router (it is not a Huma operation).
func Register(authed huma.API, d HandlerDeps) {
	registerComplete(authed, d)
	registerGoalGet(authed, d)
	registerGoalUpdate(authed, d)
}

// ─── POST /v1/me/onboarding/complete ──────────────────────────────────

// CompleteInput is the request body (api.md §2.1).
type CompleteInput struct {
	Body struct {
		Focus              string   `json:"focus" enum:"general,build_distance,build_speed,train_for_race"`
		WeeklyMilesTarget  int      `json:"weekly_miles_target" minimum:"5" maximum:"80"`
		DaysPerWeek        int      `json:"days_per_week" minimum:"3" maximum:"7"`
		TargetDistanceKM   *float64 `json:"target_distance_km,omitempty" minimum:"1" maximum:"100"`
		TargetPaceSecPerKM *int     `json:"target_pace_sec_per_km,omitempty" minimum:"180" maximum:"900"`
		RaceDate           *string  `json:"race_date,omitempty" format:"date"`
	}
}

// CompleteOutput is the response body.
type CompleteOutput struct {
	Body struct {
		GoalID        string `json:"goal_id"`
		BaselineJobID string `json:"baseline_job_id"`
		PlanJobID     string `json:"plan_job_id"`
	}
}

func registerComplete(api huma.API, d HandlerDeps) {
	huma.Register(api, huma.Operation{
		OperationID: "onboarding-complete",
		Method:      http.MethodPost,
		Path:        "/v1/me/onboarding/complete",
		Summary:     "Persist onboarding goal + enqueue baseline compute",
		Tags:        []string{"onboarding"},
	}, func(ctx context.Context, in *CompleteInput) (*CompleteOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}
		log := pkglogger.FromContext(ctx)

		// Validation: target_distance_km set requires target_pace_sec_per_km.
		if in.Body.TargetDistanceKM != nil && in.Body.TargetPaceSecPerKM == nil {
			return nil, huma.Error400BadRequest("target_pace_sec_per_km is required when target_distance_km is set")
		}

		raceDate := pgDateFromPtr(in.Body.RaceDate)
		if in.Body.RaceDate != nil && !raceDate.Valid {
			return nil, huma.Error400BadRequest("invalid race_date (want YYYY-MM-DD)")
		}

		params := dbgen.UpsertGoalParams{
			UserID:             userID,
			Focus:              in.Body.Focus,
			WeeklyMilesTarget:  int32(in.Body.WeeklyMilesTarget),
			DaysPerWeek:        int32(in.Body.DaysPerWeek),
			TargetDistanceKm:   pgNumericFromPtr(in.Body.TargetDistanceKM),
			TargetPaceSecPerKm: int32PtrFromIntPtr(in.Body.TargetPaceSecPerKM),
			RaceDate:           raceDate,
		}
		goal, err := d.Queries.UpsertGoal(ctx, params)
		if err != nil {
			log.Error("onboarding.complete: upsert goal", "err", err)
			return nil, huma.Error500InternalServerError("failed to save goal")
		}

		// LookupOrEnqueue baseline (api.md §8.1 item 5).
		baselineJobID, err := lookupOrEnqueueBaseline(ctx, d, userID)
		if err != nil {
			log.Error("onboarding.complete: enqueue baseline", "err", err)
			return nil, huma.Error500InternalServerError("failed to enqueue baseline compute")
		}

		// We don't enqueue InitialPlan from here — BaselineComputeWorker
		// chains it after the baseline insert (api.md §4.1 stage 5).
		// For the response, we return the most recent terminal plan job
		// id if one exists (so a retry returns stable ids); otherwise
		// the placeholder "pending" — the SSE stream surfaces real
		// progress.
		planJobID := lookupRecentPlanJobID(ctx, d, userID)

		out := &CompleteOutput{}
		out.Body.GoalID = goal.ID.String()
		out.Body.BaselineJobID = baselineJobID
		out.Body.PlanJobID = planJobID
		return out, nil
	})
}

// lookupOrEnqueueBaseline finds the most recent terminal baseline job
// for the user (last 24h, trigger=onboarding) and returns its id. If
// none, inserts a fresh job (with a 5-min uniqueness window) and
// returns that id.
func lookupOrEnqueueBaseline(ctx context.Context, d HandlerDeps, userID uuid.UUID) (string, error) {
	if id, ok, err := plan.FindRecentTerminalJob(ctx, d.DB, baseline.JobKind, userID, "trigger", baseline.TriggerOnboarding); err != nil {
		return "", err
	} else if ok {
		return id, nil
	}
	args := baseline.JobArgs{
		UserID:  userID,
		Window:  30,
		Trigger: baseline.TriggerOnboarding,
	}
	res, err := d.River.Insert(ctx, args, &river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: 5 * time.Minute,
		},
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", res.Job.ID), nil
}

// lookupRecentPlanJobID returns the most recent terminal initial-plan
// job id, or a sentinel "pending" string if none exist yet. The SSE
// stream is the source of truth for live progress.
func lookupRecentPlanJobID(ctx context.Context, d HandlerDeps, userID uuid.UUID) string {
	id, ok, err := plan.FindRecentTerminalJob(ctx, d.DB, plan.InitialJobKind, userID, "", "")
	if err != nil || !ok {
		// Try to find an in-flight job too — kind=plan_initial,
		// state in (available, running, retryable, scheduled).
		const q = `
SELECT id FROM river_job
WHERE kind = $1
  AND state IN ('available','running','retryable','scheduled')
  AND args->>'user_id' = $2
ORDER BY id DESC
LIMIT 1`
		var jid int64
		qErr := d.DB.QueryRow(ctx, q, plan.InitialJobKind, userID.String()).Scan(&jid)
		if qErr == nil {
			return fmt.Sprintf("%d", jid)
		}
		return "pending"
	}
	return id
}

// ─── GET /v1/me/goal ──────────────────────────────────────────────────

// GoalDTO mirrors api.md §2.5.
type GoalDTO struct {
	ID                 string   `json:"id"`
	Focus              string   `json:"focus"`
	WeeklyMilesTarget  int      `json:"weekly_miles_target"`
	DaysPerWeek        int      `json:"days_per_week"`
	TargetDistanceKM   *float64 `json:"target_distance_km,omitempty"`
	TargetPaceSecPerKM *int     `json:"target_pace_sec_per_km,omitempty"`
	RaceDate           *string  `json:"race_date,omitempty"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
}

// GoalGetOutput wraps the body.
type GoalGetOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		Goal GoalDTO `json:"goal"`
	}
}

func registerGoalGet(api huma.API, d HandlerDeps) {
	huma.Register(api, huma.Operation{
		OperationID: "goal-get",
		Method:      http.MethodGet,
		Path:        "/v1/me/goal",
		Summary:     "Get the authenticated user's goal",
		Tags:        []string{"goals"},
	}, func(ctx context.Context, _ *struct{}) (*GoalGetOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}
		row, err := d.Queries.GetGoalByUserID(ctx, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, huma.Error404NotFound("no goal yet")
			}
			pkglogger.FromContext(ctx).Error("goal.get: load row", "err", err)
			return nil, huma.Error500InternalServerError("failed to load goal")
		}
		out := &GoalGetOutput{CacheControl: "private, max-age=60"}
		out.Body.Goal = goalToDTO(row)
		return out, nil
	})
}

// ─── PATCH /v1/me/goal ────────────────────────────────────────────────

// GoalUpdateInput captures the PATCH /v1/me/goal body. Because Huma v2
// silently leaves `RawBody []byte` nil when it sits alongside a typed
// `Body` struct, we record explicit JSON nulls via a custom
// UnmarshalJSON on the body type (api.md §2.6 implementation note).
// The unmarshal does the parse twice in one call: once into the typed
// struct (preserves Huma validation tags), once into a `map[string]
// json.RawMessage` to detect literal `"null"` for the three nullable
// columns. The captured booleans drive the companion ClearGoalNullable
// query.
type GoalUpdateInput struct {
	Body goalPatchBody
}

type goalPatchBody struct {
	Focus              *string  `json:"focus,omitempty" enum:"general,build_distance,build_speed,train_for_race"`
	WeeklyMilesTarget  *int     `json:"weekly_miles_target,omitempty" minimum:"5" maximum:"80"`
	DaysPerWeek        *int     `json:"days_per_week,omitempty" minimum:"3" maximum:"7"`
	TargetDistanceKM   *float64 `json:"target_distance_km,omitempty"`
	TargetPaceSecPerKM *int     `json:"target_pace_sec_per_km,omitempty"`
	RaceDate           *string  `json:"race_date,omitempty" format:"date"`

	// Captured during UnmarshalJSON; not serialized.
	clearDistance bool `json:"-"`
	clearPace     bool `json:"-"`
	clearRace     bool `json:"-"`
}

// UnmarshalJSON parses the body twice: once into the typed alias to
// preserve Huma validation, once into a raw-message map to detect
// explicit JSON null on each nullable field. The `alias` indirection
// is the standard trick to call json.Unmarshal recursively without
// stack-overflowing.
func (b *goalPatchBody) UnmarshalJSON(data []byte) error {
	type alias goalPatchBody
	if err := json.Unmarshal(data, (*alias)(b)); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		b.clearDistance = string(raw["target_distance_km"]) == "null"
		b.clearPace = string(raw["target_pace_sec_per_km"]) == "null"
		b.clearRace = string(raw["race_date"]) == "null"
	}
	return nil
}

func registerGoalUpdate(api huma.API, d HandlerDeps) {
	huma.Register(api, huma.Operation{
		OperationID: "goal-update",
		Method:      http.MethodPatch,
		Path:        "/v1/me/goal",
		Summary:     "Partially update the authenticated user's goal",
		Tags:        []string{"goals"},
	}, func(ctx context.Context, in *GoalUpdateInput) (*GoalGetOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}
		log := pkglogger.FromContext(ctx)

		// Explicit-JSON-null detection happens inside goalPatchBody's
		// UnmarshalJSON (see the type definition above). JSON null vs
		// field-omitted collapse to the same `nil` pointer in the typed
		// struct, so the boolean flags are the only way to know which
		// nullable columns the client wants cleared.
		clearDistance := in.Body.clearDistance
		clearPace := in.Body.clearPace
		clearRace := in.Body.clearRace

		// Apply update + clears in a single tx.
		tx, err := d.DB.Begin(ctx)
		if err != nil {
			log.Error("goal.update: begin tx", "err", err)
			return nil, huma.Error500InternalServerError("tx error")
		}
		defer func() { _ = tx.Rollback(ctx) }()
		q := d.Queries.WithTx(tx)

		// First, ensure the goal exists.
		if _, err := q.GetGoalByUserID(ctx, userID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, huma.Error404NotFound("no goal — finish onboarding first")
			}
			log.Error("goal.update: get existing", "err", err)
			return nil, huma.Error500InternalServerError("failed to load goal")
		}

		params := dbgen.UpdateGoalPartialParams{
			UserID:             userID,
			Focus:              in.Body.Focus,
			WeeklyMilesTarget:  ptrIntToInt32(in.Body.WeeklyMilesTarget),
			DaysPerWeek:        ptrIntToInt32(in.Body.DaysPerWeek),
			TargetDistanceKm:   pgNumericFromPtr(in.Body.TargetDistanceKM),
			TargetPaceSecPerKm: int32PtrFromIntPtr(in.Body.TargetPaceSecPerKM),
			RaceDate:           pgDateFromPtr(in.Body.RaceDate),
		}
		row, err := q.UpdateGoalPartial(ctx, params)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, huma.Error404NotFound("no goal — finish onboarding first")
			}
			log.Error("goal.update: update", "err", err)
			return nil, huma.Error500InternalServerError("failed to update goal")
		}

		if clearDistance || clearPace || clearRace {
			if err := q.ClearGoalNullable(ctx, dbgen.ClearGoalNullableParams{
				UserID:        userID,
				ClearDistance: clearDistance,
				ClearPace:     clearPace,
				ClearRaceDate: clearRace,
			}); err != nil {
				log.Error("goal.update: clear nullable", "err", err)
				return nil, huma.Error500InternalServerError("failed to clear nullable fields")
			}
			// Re-read to pick up the cleared fields.
			row, err = q.GetGoalByUserID(ctx, userID)
			if err != nil {
				return nil, huma.Error500InternalServerError("failed to reload goal")
			}
		}

		if err := tx.Commit(ctx); err != nil {
			log.Error("goal.update: commit", "err", err)
			return nil, huma.Error500InternalServerError("commit failed")
		}

		// Best-effort enqueue of a weekly refresh with reason=goal_change.
		// Skip if a refresh is already in flight (5-min uniqueness handles
		// the rest).
		if d.River != nil {
			nextWeek := plan.FirstMondayAfter(time.Now().UTC())
			args := plan.WeeklyRefreshArgs{
				UserID:    userID,
				WeekStart: nextWeek.Format("2006-01-02"),
				Reason:    plan.ReasonGoalChange,
			}
			if _, err := d.River.Insert(ctx, args, &river.InsertOpts{
				UniqueOpts: river.UniqueOpts{
					ByArgs:   true,
					ByPeriod: 5 * time.Minute,
				},
			}); err != nil {
				log.Warn("goal.update: enqueue refresh failed", "err", err)
			}
		}

		out := &GoalGetOutput{CacheControl: "private, max-age=60"}
		out.Body.Goal = goalToDTO(row)
		return out, nil
	})
}

func goalToDTO(row dbgen.UserGoal) GoalDTO {
	dto := GoalDTO{
		ID:                row.ID.String(),
		Focus:             row.Focus,
		WeeklyMilesTarget: int(row.WeeklyMilesTarget),
		DaysPerWeek:       int(row.DaysPerWeek),
		CreatedAt:         row.CreatedAt.Time.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:         row.UpdatedAt.Time.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if row.TargetDistanceKm.Valid {
		f, err := row.TargetDistanceKm.Float64Value()
		if err == nil && f.Valid {
			v := f.Float64
			dto.TargetDistanceKM = &v
		}
	}
	if row.TargetPaceSecPerKm != nil {
		v := int(*row.TargetPaceSecPerKm)
		dto.TargetPaceSecPerKM = &v
	}
	if row.RaceDate.Valid {
		v := row.RaceDate.Time.UTC().Format("2006-01-02")
		dto.RaceDate = &v
	}
	return dto
}

// ─── tiny converters ──────────────────────────────────────────────────

func pgNumericFromPtr(p *float64) pgtype.Numeric {
	if p == nil {
		return pgtype.Numeric{Valid: false}
	}
	// math.Round, NOT int64-truncate. Half-marathon distance 21.0975 →
	// 2110 (≡ 21.10), not 2109 (truncated 21.09). Float→int cast in Go
	// drops the fractional part regardless of sign; rounding-half-away-
	// from-zero is what `numeric(6,2)` round-trips need.
	return pgtype.Numeric{
		Int:   big.NewInt(int64(math.Round(*p * 100))),
		Exp:   -2,
		Valid: true,
	}
}

func pgDateFromPtr(p *string) pgtype.Date {
	if p == nil || *p == "" {
		return pgtype.Date{Valid: false}
	}
	t, err := time.Parse("2006-01-02", *p)
	if err != nil {
		return pgtype.Date{Valid: false}
	}
	return pgtype.Date{Time: t, Valid: true}
}

func int32PtrFromIntPtr(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}

func ptrIntToInt32(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}
