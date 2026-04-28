package baseline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/swastikpatel7/cadence/apps/api/internal/coach"
	plan "github.com/swastikpatel7/cadence/apps/api/internal/plan"
	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
)

// snoozeDuration is the delay applied when (a) the user has 0 activities
// (sync hasn't completed yet), or (b) Anthropic returns 429/503. River
// re-runs the job after this window. Matches insights.md §17.
const snoozeDuration = 60 * time.Second

// Worker is the River worker for JobArgs.
//
// Stages map directly to the SSE step IDs the onboarding handler emits
// (api.md §3.2):
//
//  1. sync          — at least one activity row exists for the user
//  2. volume_curve  — weekly mileage curve computed (no LLM call)
//  3. baseline      — Opus 4.7 narrative returned + persisted
//  4. (only on Trigger=onboarding) enqueue InitialPlan
//
// On terminal errors we return nil so River discards rather than
// retrying — matches the strava_sync.go convention.
type Worker struct {
	river.WorkerDefaults[JobArgs]
	db      *pgxpool.Pool
	queries *dbgen.Queries
	coach   *coach.Client
	river   RiverInsertable
}

// NewWorker builds a Worker with all dependencies.
func NewWorker(
	db *pgxpool.Pool,
	queries *dbgen.Queries,
	cli *coach.Client,
	riverInsertable RiverInsertable,
) *Worker {
	return &Worker{
		db:      db,
		queries: queries,
		coach:   cli,
		river:   riverInsertable,
	}
}

// RiverInsertable is the slim insert surface this package needs.
// Defined here so the worker file does not depend on the concrete
// pgx-tx generic argument of *river.Client[pgx.Tx].
type RiverInsertable interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Work runs one BaselineCompute attempt.
func (w *Worker) Work(ctx context.Context, j *river.Job[JobArgs]) error {
	log := pkglogger.FromContext(ctx).With(
		"user_id", j.Args.UserID, "job_id", j.ID,
		"trigger", j.Args.Trigger, "window", j.Args.Window,
	)

	// Stage 1: sync verified.
	count, err := CountActivities(ctx, w.queries, j.Args.UserID)
	if err != nil {
		return fmt.Errorf("count activities: %w", err)
	}
	if count == 0 {
		log.Info("baseline: 0 activities yet, snoozing")
		return river.JobSnooze(snoozeDuration)
	}

	// Stage 2: volume curve.
	num, err := ComputeNumeric(ctx, w.queries, j.Args.UserID, j.Args.Window)
	if err != nil {
		return fmt.Errorf("compute numeric: %w", err)
	}

	// Stage 3: baseline narrative. If Anthropic is not configured we
	// snooze; in dev this means the worker waits until the env is set.
	if !w.coach.Available() {
		log.Warn("baseline: anthropic client not configured, snoozing")
		return river.JobSnooze(snoozeDuration)
	}

	narrative, llmRes, err := Narrate(ctx, w.coach, num)
	if err != nil {
		log.Error("baseline: narrate failed", "err", err)
		// Terminal Anthropic 4xx (e.g. schema-validation 400, auth 401):
		// retrying won't fix it. Cancel the job so River discards it
		// immediately — the SSE handler then surfaces a clean error to
		// the user instead of letting the JWT expire under retry storm.
		if coach.IsTerminal(err) {
			return river.JobCancel(fmt.Errorf("narrate: %w", err))
		}
		return fmt.Errorf("narrate: %w", err)
	}

	// Honor the Opus-decided fitness tier when present, otherwise
	// fall back to the heuristic.
	tier := narrative.FitnessTier
	if tier == "" {
		tier = num.FitnessTier
	}
	consistency := int32(narrative.ConsistencyScore)
	if consistency == 0 {
		consistency = num.ConsistencyScore
	}

	// Merge the narrative-supplied avg_pace_at_distance with the
	// numeric-derived one (numeric wins on conflict — it's grounded
	// in the actual data).
	avgPaceJSON := mergePaceMaps(num.AvgPaceAtDistance, narrative.AvgPaceAtDistance)
	avgPaceBytes, err := json.Marshal(avgPaceJSON)
	if err != nil {
		return fmt.Errorf("marshal avg_pace_at_distance: %w", err)
	}

	src := j.Args.Trigger
	if src == "" {
		src = TriggerManualRecompute
	}

	// Stage 4: persist baselines + (if onboarding) enqueue InitialPlan
	// transactionally so a failed enqueue doesn't leave a baselines row
	// without the chained plan job (api.md §4.1 stage 5).
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := w.queries.WithTx(tx)

	row, err := q.InsertBaseline(ctx, dbgen.InsertBaselineParams{
		UserID:            j.Args.UserID,
		WindowDays:        num.WindowDays,
		FitnessTier:       tier,
		WeeklyVolumeKmAvg: numericFromFloat(num.WeeklyVolumeKMAvg),
		WeeklyVolumeKmP25: numericFromFloat(num.WeeklyVolumeKMP25),
		WeeklyVolumeKmP75: numericFromFloat(num.WeeklyVolumeKMP75),
		AvgPaceSecPerKm:   num.AvgPaceSecPerKM,
		AvgPaceAtDistance: avgPaceBytes,
		LongestRunKm:      numericFromFloat(num.LongestRunKM),
		ConsistencyScore:  consistency,
		Narrative:         narrative.Narrative,
		Source:            src,
		Model:             llmRes.Model,
		InputTokens:       llmRes.InputTokens,
		OutputTokens:      llmRes.OutputTokens,
		ThinkingTokens:    llmRes.ThinkingTokens,
	})
	if err != nil {
		return fmt.Errorf("insert baseline: %w", err)
	}

	if j.Args.Trigger == TriggerOnboarding {
		// Onboarding-only chain. Look up the goal so we can pin the
		// goal_id on the plan job args.
		goal, gerr := q.GetGoalByUserID(ctx, j.Args.UserID)
		if gerr != nil {
			return fmt.Errorf("load goal post-baseline: %w", gerr)
		}
		// We commit the baselines row and let the plan-job insert use
		// the standalone river client (Insert isn't transactional with
		// our pgxpool unless we go through InsertTx with a concrete
		// pgx.Tx — kept simple for v1; the prior commit means the
		// InitialPlan worker's pre-condition (baseline exists) holds).
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit baseline: %w", err)
		}
		args := plan.InitialJobArgs{
			UserID:     j.Args.UserID,
			GoalID:     goal.ID,
			BaselineID: row.ID,
		}
		if _, err := w.river.Insert(ctx, args, nil); err != nil {
			log.Error("baseline: enqueue initial plan failed", "err", err)
			// Don't return error — the baseline is already saved; the
			// frontend SSE will time out waiting for plan and the user
			// can retry via /v1/me/plan/refresh.
			return nil
		}
		log.Info("baseline: complete (onboarding) + initial plan enqueued")
		return nil
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit baseline: %w", err)
	}
	log.Info("baseline: complete")
	return nil
}

// mergePaceMaps prefers numeric (data-grounded) over llm (free-form
// guess) on conflict.
func mergePaceMaps(numeric map[int]int, llm map[string]int) map[string]int {
	out := make(map[string]int, len(numeric))
	for d, p := range numeric {
		out[fmt.Sprintf("%d", d)] = p
	}
	for k, v := range llm {
		if _, has := out[k]; !has {
			out[k] = v
		}
	}
	return out
}

// numericFromFloat converts a Go float64 to a pgtype.Numeric with two
// decimal places (matches the numeric(6,2) columns).
func numericFromFloat(f float64) pgtype.Numeric {
	if !isFiniteAndNonNegative(f) {
		return pgtype.Numeric{Valid: false}
	}
	scaled := big.NewInt(int64(math.Round(f * 100)))
	return pgtype.Numeric{
		Int:   scaled,
		Exp:   -2,
		Valid: true,
	}
}

func isFiniteAndNonNegative(f float64) bool {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return false
	}
	return f >= 0
}

// ErrSnooze is exported so callers can detect the snooze case if they
// want to log differently. Currently unused outside the package.
var ErrSnooze = errors.New("baseline: snooze")
