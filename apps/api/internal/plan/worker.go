package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/swastikpatel7/cadence/apps/api/internal/coach"
	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
)

const (
	planSnoozeDuration = 60 * time.Second
)

// InitialWorker is the River worker for the one-time 8-week plan
// generation. Inserted exclusively by BaselineComputeWorker.
type InitialWorker struct {
	river.WorkerDefaults[InitialJobArgs]
	queries *dbgen.Queries
	coach   *coach.Client
}

// NewInitialWorker constructs the worker.
func NewInitialWorker(queries *dbgen.Queries, cli *coach.Client) *InitialWorker {
	return &InitialWorker{queries: queries, coach: cli}
}

// Work runs one InitialPlan generation.
func (w *InitialWorker) Work(ctx context.Context, j *river.Job[InitialJobArgs]) error {
	log := pkglogger.FromContext(ctx).With(
		"user_id", j.Args.UserID, "job_id", j.ID,
		"goal_id", j.Args.GoalID, "baseline_id", j.Args.BaselineID,
	)

	if !w.coach.Available() {
		log.Warn("plan.initial: anthropic client not configured, snoozing")
		return river.JobSnooze(planSnoozeDuration)
	}

	goal, err := w.queries.GetGoalByUserID(ctx, j.Args.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warn("plan.initial: no goal yet")
			return nil // terminal — nothing to plan against
		}
		return fmt.Errorf("load goal: %w", err)
	}
	baseline, err := w.queries.GetLatestBaselineByUserID(ctx, j.Args.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warn("plan.initial: no baseline yet")
			return nil
		}
		return fmt.Errorf("load baseline: %w", err)
	}

	now := time.Now().UTC()
	firstMon := FirstMondayAfter(now)

	in := InitialPlanInput{
		GoalFocus:           goal.Focus,
		WeeklyMilesTarget:   int(goal.WeeklyMilesTarget),
		DaysPerWeek:         int(goal.DaysPerWeek),
		TargetDistanceKM:    pgNumericPtr(goal.TargetDistanceKm),
		TargetPaceSecPerKM:  ptrInt32ToInt(goal.TargetPaceSecPerKm),
		RaceDate:            pgDatePtr(goal.RaceDate),
		BaselineNarrative:   baseline.Narrative,
		WeeklyVolumeKMAvg:   numericFloat(baseline.WeeklyVolumeKmAvg),
		WeeklyVolumeKMP25:   numericFloat(baseline.WeeklyVolumeKmP25),
		WeeklyVolumeKMP75:   numericFloat(baseline.WeeklyVolumeKmP75),
		AvgPaceSecPerKM:     baseline.AvgPaceSecPerKm,
		LongestRunKM:        numericFloat(baseline.LongestRunKm),
		FitnessTier:         baseline.FitnessTier,
		FirstMonday:         firstMon,
		Today:               now,
	}

	pb, llmRes, err := GenerateInitial(ctx, w.coach, in)
	if err != nil {
		log.Error("plan.initial: generate failed", "err", err)
		if coach.IsTerminal(err) {
			return river.JobCancel(fmt.Errorf("generate initial: %w", err))
		}
		return fmt.Errorf("generate initial: %w", err)
	}

	planBytes, err := MarshalPlan(pb)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}

	_, err = w.queries.InsertPlan(ctx, dbgen.InsertPlanParams{
		UserID:         j.Args.UserID,
		GenerationKind: GenerationKindInitial8Wk,
		BaselineID:     ptrUUID(baseline.ID),
		GoalID:         ptrUUID(goal.ID),
		StartsOn:       pgtype.Date{Time: firstMon, Valid: true},
		WeeksCount:     int32(len(pb.Weeks)),
		Plan:           planBytes,
		Model:          llmRes.Model,
		InputTokens:    llmRes.InputTokens,
		OutputTokens:   llmRes.OutputTokens,
		ThinkingTokens: llmRes.ThinkingTokens,
		Reason:         strPtr(ReasonOnboarding),
	})
	if err != nil {
		return fmt.Errorf("insert plan: %w", err)
	}
	log.Info("plan.initial: complete", "weeks", len(pb.Weeks))
	return nil
}

// WeeklyRefreshWorker is the River worker for the recurring weekly
// plan refresh. Driven by the periodic cron, /v1/me/plan/refresh, and
// PATCH /v1/me/goal.
type WeeklyRefreshWorker struct {
	river.WorkerDefaults[WeeklyRefreshArgs]
	db      *pgxpool.Pool
	queries *dbgen.Queries
	coach   *coach.Client
}

// NewWeeklyRefreshWorker constructs the worker.
func NewWeeklyRefreshWorker(db *pgxpool.Pool, queries *dbgen.Queries, cli *coach.Client) *WeeklyRefreshWorker {
	return &WeeklyRefreshWorker{db: db, queries: queries, coach: cli}
}

// Work runs one weekly refresh.
func (w *WeeklyRefreshWorker) Work(ctx context.Context, j *river.Job[WeeklyRefreshArgs]) error {
	log := pkglogger.FromContext(ctx).With(
		"user_id", j.Args.UserID, "job_id", j.ID, "reason", j.Args.Reason,
	)
	// Periodic-cron tick rows arrive with UserID == uuid.Nil — per-user
	// fan-out is deferred (jobs/periodic.go). No-op until that lands.
	if j.Args.UserID == (uuid.UUID{}) {
		log.Info("plan.weekly_refresh: cron tick (no per-user fan-out yet); skipping")
		return nil
	}
	if !w.coach.Available() {
		log.Warn("plan.weekly_refresh: anthropic client not configured, snoozing")
		return river.JobSnooze(planSnoozeDuration)
	}

	currentPlan, err := w.queries.GetCurrentPlanByUserID(ctx, j.Args.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Info("plan.weekly_refresh: no current plan; skipping")
			return nil
		}
		return fmt.Errorf("load current plan: %w", err)
	}
	goal, err := w.queries.GetGoalByUserID(ctx, j.Args.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Info("plan.weekly_refresh: no goal; skipping")
			return nil
		}
		return fmt.Errorf("load goal: %w", err)
	}
	baseline, err := w.queries.GetLatestBaselineByUserID(ctx, j.Args.UserID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load baseline: %w", err)
	}

	nextWeekStart, err := parseDate(j.Args.WeekStart)
	if err != nil {
		nextWeekStart = FirstMondayAfter(time.Now().UTC())
	}
	priorStart := nextWeekStart.AddDate(0, 0, -7)

	// Compose prior-week prescribed/actual.
	prescribed := lastWeekFromPlan(currentPlan.Plan, priorStart)
	actual, err := lastWeekActuals(ctx, w.queries, j.Args.UserID, priorStart, nextWeekStart)
	if err != nil {
		return fmt.Errorf("load prior week activities: %w", err)
	}

	in := WeeklyRefreshInput{
		NextWeekStart:       nextWeekStart,
		GoalFocus:           goal.Focus,
		WeeklyMilesTarget:   int(goal.WeeklyMilesTarget),
		DaysPerWeek:         int(goal.DaysPerWeek),
		BaselineFitnessTier: stringOr(baseline.FitnessTier, "T2"),
		BaselineNarrative:   baseline.Narrative,
		PriorWeekPrescribed: prescribed,
		PriorWeekActual:     actual,
		Reason:              j.Args.Reason,
	}

	pb, llmRes, err := GenerateWeekly(ctx, w.coach, in)
	if err != nil {
		log.Error("plan.weekly_refresh: generate failed", "err", err)
		if coach.IsTerminal(err) {
			return river.JobCancel(fmt.Errorf("generate weekly: %w", err))
		}
		return fmt.Errorf("generate weekly: %w", err)
	}
	planBytes, err := MarshalPlan(pb)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}

	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := w.queries.WithTx(tx)

	row, err := q.InsertPlan(ctx, dbgen.InsertPlanParams{
		UserID:         j.Args.UserID,
		GenerationKind: GenerationKindWeeklyRefresh,
		BaselineID:     ptrUUIDFromPtr(currentPlan.BaselineID),
		GoalID:         ptrUUID(goal.ID),
		StartsOn:       pgtype.Date{Time: nextWeekStart, Valid: true},
		WeeksCount:     1,
		Plan:           planBytes,
		Model:          llmRes.Model,
		InputTokens:    llmRes.InputTokens,
		OutputTokens:   llmRes.OutputTokens,
		ThinkingTokens: llmRes.ThinkingTokens,
		Reason:         strPtr(j.Args.Reason),
	})
	if err != nil {
		return fmt.Errorf("insert plan: %w", err)
	}
	if err := q.MarkPlanSuperseded(ctx, dbgen.MarkPlanSupersededParams{
		UserID:       j.Args.UserID,
		SupersededBy: ptrUUID(row.ID),
	}); err != nil {
		return fmt.Errorf("mark superseded: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit weekly refresh: %w", err)
	}
	log.Info("plan.weekly_refresh: complete")
	return nil
}

// SessionMicroSummaryWorker is the River worker for the lazy Haiku
// micro-summary. Inserted by the session-detail handler when a past
// day's drawer is opened with a matched activity but no insight row.
type SessionMicroSummaryWorker struct {
	river.WorkerDefaults[SessionMicroSummaryArgs]
	queries *dbgen.Queries
	coach   *coach.Client
}

// NewSessionMicroSummaryWorker constructs the worker.
func NewSessionMicroSummaryWorker(queries *dbgen.Queries, cli *coach.Client) *SessionMicroSummaryWorker {
	return &SessionMicroSummaryWorker{queries: queries, coach: cli}
}

// Work runs one micro-summary generation. Failures here are non-
// critical (the user just sees the drawer without the line); we
// always return nil on terminal errors.
func (w *SessionMicroSummaryWorker) Work(ctx context.Context, j *river.Job[SessionMicroSummaryArgs]) error {
	log := pkglogger.FromContext(ctx).With(
		"user_id", j.Args.UserID, "activity_id", j.Args.ActivityID, "job_id", j.ID,
	)
	if !w.coach.Available() {
		log.Info("plan.micro_summary: anthropic client not configured, skipping")
		return nil
	}

	// Load activity row to get the actuals.
	dayStart, err := parseDate(j.Args.Prescribed.Date)
	if err != nil {
		log.Warn("plan.micro_summary: bad date in args", "err", err)
		return nil
	}
	row, err := w.queries.GetActivityForDate(ctx, dbgen.GetActivityForDateParams{
		UserID:    j.Args.UserID,
		StartTime: pgtype.Timestamptz{Time: dayStart, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Info("plan.micro_summary: activity vanished")
			return nil
		}
		log.Warn("plan.micro_summary: load activity failed", "err", err)
		return nil
	}
	distM, _ := numericFloatOK(row.DistanceMeters)
	distKM := distM / 1000.0
	pace := 0
	if row.DurationSeconds > 0 && distKM > 0 {
		pace = int(float64(row.DurationSeconds) / distKM)
	}

	in := MicroSummaryInput{
		Prescribed: j.Args.Prescribed,
		ActualKM:   distKM,
		ActualPace: pace,
		ActualDur:  int(row.DurationSeconds),
		ActualHR:   row.AvgHeartRate,
		StartedAt:  row.StartTime.Time,
	}

	body, llmRes, err := GenerateMicroSummary(ctx, w.coach, in)
	if err != nil {
		log.Warn("plan.micro_summary: generate failed", "err", err)
		return nil
	}

	if _, err := w.queries.InsertInsight(ctx, dbgen.InsertInsightParams{
		UserID:       j.Args.UserID,
		ActivityID:   ptrUUID(row.ID),
		Kind:         "micro_summary",
		Body:         body,
		Model:        llmRes.Model,
		InputTokens:  llmRes.InputTokens,
		OutputTokens: llmRes.OutputTokens,
	}); err != nil {
		log.Warn("plan.micro_summary: insert insight failed", "err", err)
		return nil
	}
	log.Info("plan.micro_summary: complete")
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────

func lastWeekFromPlan(planBytes []byte, weekStart time.Time) []SessionRecap {
	var pb PlanBlob
	if err := pbUnmarshal(planBytes, &pb); err != nil {
		return nil
	}
	wkEnd := weekStart.AddDate(0, 0, 7)
	var out []SessionRecap
	for _, w := range pb.Weeks {
		for _, s := range w.Sessions {
			d, err := parseDate(s.Date)
			if err != nil {
				continue
			}
			if !d.Before(weekStart) && d.Before(wkEnd) {
				out = append(out, SessionRecap{
					Date:       s.Date,
					Type:       s.Type,
					DistanceKM: s.DistanceKM,
					Intensity:  s.Intensity,
				})
			}
		}
	}
	return out
}

func lastWeekActuals(
	ctx context.Context,
	queries *dbgen.Queries,
	userID uuid.UUID,
	from, to time.Time,
) ([]ActivityRecap, error) {
	rows, err := queries.ListActivitiesInWindow(ctx, dbgen.ListActivitiesInWindowParams{
		UserID:      userID,
		StartTime:   pgtype.Timestamptz{Time: from, Valid: true},
		StartTime_2: pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	out := make([]ActivityRecap, 0, len(rows))
	for _, r := range rows {
		distM, _ := numericFloatOK(r.DistanceMeters)
		distKM := distM / 1000.0
		pace := 0
		if r.DurationSeconds > 0 && distKM > 0 {
			pace = int(float64(r.DurationSeconds) / distKM)
		}
		out = append(out, ActivityRecap{
			Date:            r.StartTime.Time.UTC().Format("2006-01-02"),
			DistanceKM:      distKM,
			AvgPaceSecPerKM: pace,
			DurationSec:     int(r.DurationSeconds),
		})
	}
	return out, nil
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	return time.Parse("2006-01-02", s)
}

func pgNumericPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

func pgDatePtr(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	v := d.Time.UTC().Format("2006-01-02")
	return &v
}

func ptrInt32ToInt(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }

func ptrUUIDFromPtr(p *uuid.UUID) *uuid.UUID {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func strPtr(s string) *string { return &s }

func stringOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func numericFloat(n pgtype.Numeric) float64 {
	v, _ := numericFloatOK(n)
	return v
}

func numericFloatOK(n pgtype.Numeric) (float64, bool) {
	if !n.Valid {
		return 0, false
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0, false
	}
	return f.Float64, true
}

// pbUnmarshal is a tiny indirection so the generator-and-worker tests
// can swap in a deterministic decoder later. Currently json.Unmarshal.
func pbUnmarshal(b []byte, v *PlanBlob) error {
	return jsonUnmarshalPB(b, v)
}

// jsonUnmarshalPB is split out to keep the import list local.
var jsonUnmarshalPB = func(b []byte, v *PlanBlob) error {
	return json.Unmarshal(b, v)
}

// numericFromKM converts kilometers (as float64) to a pgtype.Numeric
// with two-decimal scale (matches numeric(6,2) columns). Used by
// callers that want to write distances back to the DB. Currently
// unused by this file but kept for symmetry with baseline/worker.go.
//
// Exported to satisfy lints that flag dead code: the heatmap path
// writes via different helpers, and the workers don't write distances
// back into activities.
func numericFromKM(km float64) pgtype.Numeric {
	// math.Round, not int64-truncate — see onboarding.pgNumericFromPtr.
	scaled := big.NewInt(int64(math.Round(km * 100)))
	return pgtype.Numeric{Int: scaled, Exp: -2, Valid: true}
}

// _ = numericFromKM keeps the helper available for future workers
// that want to round-trip a distance. Without this, golangci-lint
// flags it as unused; we keep it explicit so the symbol is reachable
// from tests later.
var _ = numericFromKM
