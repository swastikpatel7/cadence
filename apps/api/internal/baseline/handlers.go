package baseline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/swastikpatel7/cadence/apps/api/internal/auth"
	"github.com/swastikpatel7/cadence/apps/api/internal/plan"
	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
)

// HandlerDeps wires the GET/POST handlers.
type HandlerDeps struct {
	DB      *pgxpool.Pool
	Queries *dbgen.Queries
	River   RiverHandlerInsertable
}

// RiverHandlerInsertable is the same shape as the worker's
// RiverInsertable but kept separate so the file imports are clean.
type RiverHandlerInsertable interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Register mounts both endpoints.
func Register(authed huma.API, d HandlerDeps) {
	registerGet(authed, d)
	registerRecompute(authed, d)
}

// ─── GET /v1/me/baseline ──────────────────────────────────────────────

// BaselineDTO is the wire shape (api.md §2.4).
type BaselineDTO struct {
	ID                string         `json:"id"`
	ComputedAt        string         `json:"computed_at"`
	WindowDays        int            `json:"window_days"`
	Source            string         `json:"source"`
	FitnessTier       string         `json:"fitness_tier"`
	WeeklyVolumeKMAvg float64        `json:"weekly_volume_km_avg"`
	WeeklyVolumeKMP25 float64        `json:"weekly_volume_km_p25"`
	WeeklyVolumeKMP75 float64        `json:"weekly_volume_km_p75"`
	AvgPaceSecPerKM   int            `json:"avg_pace_sec_per_km"`
	AvgPaceAtDistance map[string]int `json:"avg_pace_at_distance"`
	LongestRunKM      float64        `json:"longest_run_km"`
	ConsistencyScore  int            `json:"consistency_score"`
	Narrative         string         `json:"narrative"`
}

// BaselineGetOutput wraps the body.
type BaselineGetOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		Baseline BaselineDTO `json:"baseline"`
	}
}

func registerGet(api huma.API, d HandlerDeps) {
	huma.Register(api, huma.Operation{
		OperationID: "baseline-get",
		Method:      http.MethodGet,
		Path:        "/v1/me/baseline",
		Summary:     "Get the latest baseline for the authenticated user",
		Tags:        []string{"baseline"},
	}, func(ctx context.Context, _ *struct{}) (*BaselineGetOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}
		row, err := d.Queries.GetLatestBaselineByUserID(ctx, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, huma.Error404NotFound("no baseline yet")
			}
			pkglogger.FromContext(ctx).Error("baseline.get: load row", "err", err)
			return nil, huma.Error500InternalServerError("failed to load baseline")
		}

		paceAtDistance := map[string]int{}
		if len(row.AvgPaceAtDistance) > 0 {
			_ = json.Unmarshal(row.AvgPaceAtDistance, &paceAtDistance)
		}

		out := &BaselineGetOutput{CacheControl: "private, max-age=60"}
		out.Body.Baseline = BaselineDTO{
			ID:                row.ID.String(),
			ComputedAt:        row.ComputedAt.Time.UTC().Format("2006-01-02T15:04:05Z07:00"),
			WindowDays:        int(row.WindowDays),
			Source:            row.Source,
			FitnessTier:       row.FitnessTier,
			WeeklyVolumeKMAvg: numericToFloat(row.WeeklyVolumeKmAvg),
			WeeklyVolumeKMP25: numericToFloat(row.WeeklyVolumeKmP25),
			WeeklyVolumeKMP75: numericToFloat(row.WeeklyVolumeKmP75),
			AvgPaceSecPerKM:   int(row.AvgPaceSecPerKm),
			AvgPaceAtDistance: paceAtDistance,
			LongestRunKM:      numericToFloat(row.LongestRunKm),
			ConsistencyScore:  int(row.ConsistencyScore),
			Narrative:         row.Narrative,
		}
		return out, nil
	})
}

// ─── POST /v1/me/baseline/recompute ───────────────────────────────────

// BaselineRecomputeInput is the request body.
type BaselineRecomputeInput struct {
	Body struct {
		Days int `json:"days" enum:"7,14,30,60,90,-1"`
	}
}

// BaselineRecomputeOutput returns 202 + the job id.
type BaselineRecomputeOutput struct {
	Status int
	Body   struct {
		BaselineJobID string `json:"baseline_job_id"`
	}
}

func registerRecompute(api huma.API, d HandlerDeps) {
	huma.Register(api, huma.Operation{
		OperationID: "baseline-recompute",
		Method:      http.MethodPost,
		Path:        "/v1/me/baseline/recompute",
		Summary:     "Enqueue a baseline recompute over a chosen window",
		Tags:        []string{"baseline"},
	}, func(ctx context.Context, in *BaselineRecomputeInput) (*BaselineRecomputeOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}
		if !validRecomputeDays(in.Body.Days) {
			return nil, huma.Error400BadRequest("days must be one of 7,14,30,60,90,-1")
		}
		if d.River == nil {
			return nil, huma.Error500InternalServerError("queue not configured")
		}

		inFlight, err := plan.IsBaselineInFlight(ctx, d.DB, userID)
		if err != nil {
			pkglogger.FromContext(ctx).Error("baseline.recompute: check in-flight", "err", err)
			return nil, huma.Error500InternalServerError("failed to check job state")
		}
		if inFlight {
			return nil, huma.Error409Conflict("a baseline recompute is already in flight")
		}

		args := JobArgs{
			UserID:  userID,
			Window:  in.Body.Days,
			Trigger: TriggerManualRecompute,
		}
		res, err := d.River.Insert(ctx, args, &river.InsertOpts{
			UniqueOpts: river.UniqueOpts{
				ByArgs:   true,
				ByPeriod: 5 * time.Minute,
			},
		})
		if err != nil {
			pkglogger.FromContext(ctx).Error("baseline.recompute: enqueue", "err", err)
			return nil, huma.Error500InternalServerError("failed to enqueue baseline recompute")
		}
		out := &BaselineRecomputeOutput{Status: http.StatusAccepted}
		out.Body.BaselineJobID = fmt.Sprintf("%d", res.Job.ID)
		return out, nil
	})
}

func validRecomputeDays(d int) bool {
	switch d {
	case 7, 14, 30, 60, 90, -1:
		return true
	}
	return false
}

// numericToFloat renders a pgtype.Numeric as a JSON-friendly float64.
// Zero on invalid / null.
func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}
