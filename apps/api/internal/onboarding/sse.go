package onboarding

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/swastikpatel7/cadence/apps/api/internal/auth"
	"github.com/swastikpatel7/cadence/apps/api/internal/baseline"
	"github.com/swastikpatel7/cadence/apps/api/internal/plan"
	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
)

// SSEDeps wires the SSE handler.
type SSEDeps struct {
	DB *pgxpool.Pool
}

// pollInterval is the cadence the SSE handler hits river_job at. 500ms
// per api.md §3 ("polling cadence on the server: 500 ms").
const pollInterval = 500 * time.Millisecond

// progressStep IDs (api.md §3.2).
const (
	stepSync         = "sync"
	stepVolumeCurve  = "volume_curve"
	stepBaseline     = "baseline"
	stepPlan         = "plan"
	statePending     = "pending"
	stateInFlight    = "in_flight"
	stateDone        = "done"
	stateError       = "error"
)

// stepEvent matches api.md §3.4 OnboardingStepEvent.
type stepEvent struct {
	Step  string `json:"step"`
	State string `json:"state"`
	TS    string `json:"ts"`
	Error string `json:"error,omitempty"`
}

type doneEvent struct {
	TS string `json:"ts"`
}

// progressTracker remembers the last-emitted state per step so we don't
// re-emit identical lines.
type progressTracker map[string]string

// jobStateRow is the minimal shape we read from river_job for one job.
type jobStateRow struct {
	state    string
	hasError bool
	errMsg   string
}

// HandleStream is the chi-style http.HandlerFunc for the SSE endpoint.
// Mounted on the chi router with the verifier middleware applied —
// see server/server.go.
func (d SSEDeps) HandleStream() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserID(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		log := pkglogger.FromContext(r.Context()).With("user_id", userID)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache, no-transform")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		tracker := progressTracker{
			stepSync:        statePending,
			stepVolumeCurve: statePending,
			stepBaseline:    statePending,
			stepPlan:        statePending,
		}
		// Initial snapshot per step so the client renders something
		// immediately.
		for _, step := range []string{stepSync, stepVolumeCurve, stepBaseline, stepPlan} {
			emitStep(w, flusher, step, statePending, "")
		}

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				updates, baselineErr, planErr, terminal := pollOnboardingProgress(r.Context(), d.DB, userID, tracker)
				for _, ev := range updates {
					emitStep(w, flusher, ev.Step, ev.State, ev.Error)
				}
				if baselineErr != "" {
					log.Warn("onboarding.sse: baseline error", "err", baselineErr)
				}
				if planErr != "" {
					log.Warn("onboarding.sse: plan error", "err", planErr)
				}
				if terminal {
					emitDone(w, flusher)
					return
				}
			}
		}
	}
}

// pollOnboardingProgress reads the latest baseline + plan job rows for
// the user and reconciles them against the tracker. Returns the events
// to emit, optional error messages from any discarded jobs, and a
// terminal flag (both jobs in completed/discarded state).
func pollOnboardingProgress(
	ctx context.Context,
	db *pgxpool.Pool,
	userID uuid.UUID,
	tracker progressTracker,
) (updates []stepEvent, baselineErr, planErr string, terminal bool) {
	baselineRow, baselineExists := loadLatestJob(ctx, db, baseline.JobKind, userID)
	planRow, planExists := loadLatestJob(ctx, db, plan.InitialJobKind, userID)

	syncDone := tracker[stepSync] == stateDone
	volumeDone := tracker[stepVolumeCurve] == stateDone

	// Stage 1: sync. Marked done as soon as the baseline job has been
	// observed at least once in any state — its first stage is the
	// sync verification (api.md §4.1 stage 1). Equivalently: any
	// baseline job row in `available`/`running`/`completed` means the
	// worker has at minimum seen 1+ activities.
	if baselineExists && !syncDone {
		// Heuristic: if baseline is running, retryable, or completed,
		// the sync stage has happened (because the worker only proceeds
		// past the initial CountActivities check when count > 0).
		// scheduled or available pre-run-1 might be sync-pending → we
		// still emit in_flight. `cancelled` is treated like `discarded`
		// here: the worker reached at least the start of stage 3 to
		// surface an Anthropic 4xx, so sync was satisfied.
		switch baselineRow.state {
		case "running", "completed", "retryable", "discarded", "cancelled":
			updates = appendIfChanged(updates, tracker, stepSync, stateDone)
			syncDone = true
			tracker[stepSync] = stateDone
		default:
			updates = appendIfChanged(updates, tracker, stepSync, stateInFlight)
		}
	}

	// Stage 2: volume_curve. Same heuristic — if baseline is running
	// or beyond, the curve is computed (it's stage 2 inside the same
	// worker; we don't have a finer signal without persisting
	// intermediate state).
	if syncDone && !volumeDone {
		switch baselineRow.state {
		case "running":
			updates = appendIfChanged(updates, tracker, stepVolumeCurve, stateInFlight)
		case "completed", "retryable", "discarded", "cancelled":
			updates = appendIfChanged(updates, tracker, stepVolumeCurve, stateDone)
			tracker[stepVolumeCurve] = stateDone
		}
	}

	// Stage 3: baseline narrative. Maps directly to job state. `cancelled`
	// is the terminal-failure signal we now use for Anthropic 4xx — same
	// UX as `discarded` (the older retry-exhausted path).
	if baselineExists {
		switch baselineRow.state {
		case "available", "scheduled":
			updates = appendIfChanged(updates, tracker, stepBaseline, statePending)
		case "running", "retryable":
			updates = appendIfChanged(updates, tracker, stepBaseline, stateInFlight)
		case "completed":
			updates = appendIfChanged(updates, tracker, stepBaseline, stateDone)
		case "discarded", "cancelled":
			baselineErr = baselineRow.errMsg
			updates = appendIfChangedWithError(updates, tracker, stepBaseline, stateError, baselineErr)
		}
	}

	// Stage 4: plan.
	if planExists {
		switch planRow.state {
		case "available", "scheduled":
			updates = appendIfChanged(updates, tracker, stepPlan, statePending)
		case "running", "retryable":
			updates = appendIfChanged(updates, tracker, stepPlan, stateInFlight)
		case "completed":
			updates = appendIfChanged(updates, tracker, stepPlan, stateDone)
		case "discarded", "cancelled":
			planErr = planRow.errMsg
			updates = appendIfChangedWithError(updates, tracker, stepPlan, stateError, planErr)
		}
	}

	// Terminal: both jobs reached a terminal state. If plan job hasn't
	// even shown up yet, we are not terminal — the chained insert from
	// BaselineComputeWorker may be in flight.
	baselineTerminal := baselineExists && isJobTerminal(baselineRow.state)
	planTerminal := planExists && isJobTerminal(planRow.state)
	if baselineTerminal && planTerminal {
		terminal = true
	}
	if baselineTerminal && isJobFailed(baselineRow.state) && !planExists {
		// Baseline failed and never produced a plan; terminate the stream.
		terminal = true
	}
	return
}

func isJobTerminal(state string) bool {
	return state == "completed" || state == "discarded" || state == "cancelled"
}

func isJobFailed(state string) bool {
	return state == "discarded" || state == "cancelled"
}

// loadLatestJob returns the most recent river_job row for (kind,
// user_id). ok=false if none.
func loadLatestJob(ctx context.Context, db *pgxpool.Pool, kind string, userID uuid.UUID) (jobStateRow, bool) {
	const q = `
SELECT state::text, COALESCE((errors[array_length(errors, 1)] ->> 'error')::text, '')
FROM river_job
WHERE kind = $1 AND args->>'user_id' = $2
ORDER BY id DESC
LIMIT 1`
	var row jobStateRow
	err := db.QueryRow(ctx, q, kind, userID.String()).Scan(&row.state, &row.errMsg)
	if err != nil {
		if !isNoRows(err) {
			pkglogger.FromContext(ctx).Warn("onboarding.sse: load river_job failed",
				"kind", kind, "err", err)
		}
		return jobStateRow{}, false
	}
	row.hasError = row.errMsg != ""
	return row, true
}

func isNoRows(err error) bool {
	return err != nil && err == pgx.ErrNoRows
}

// monotonicGuard returns true if the step has already reached a terminal
// state (`done` or `error`). Once terminal, we never regress — even if
// River briefly reports `available`/`retryable` for a job that just
// finished and got re-snoozed by another worker pass. The frontend
// renders past-terminal as "✓ done"; flipping it back to "in_flight"
// would look like a glitch.
func monotonicGuard(cur string) bool {
	return cur == stateDone || cur == stateError
}

func appendIfChanged(updates []stepEvent, tracker progressTracker, step, state string) []stepEvent {
	cur := tracker[step]
	if monotonicGuard(cur) || cur == state {
		return updates
	}
	tracker[step] = state
	return append(updates, stepEvent{Step: step, State: state, TS: nowRFC3339()})
}

func appendIfChangedWithError(updates []stepEvent, tracker progressTracker, step, state, msg string) []stepEvent {
	cur := tracker[step]
	// Allow transition INTO `error` from any non-terminal state, but
	// don't let a later `error` overwrite an earlier `done`.
	if cur == stateDone || cur == state {
		return updates
	}
	tracker[step] = state
	return append(updates, stepEvent{Step: step, State: state, TS: nowRFC3339(), Error: msg})
}

func emitStep(w http.ResponseWriter, flusher http.Flusher, step, state, errMsg string) {
	ev := stepEvent{Step: step, State: state, TS: nowRFC3339(), Error: errMsg}
	body, _ := json.Marshal(ev)
	if _, err := fmt.Fprintf(w, "event: step\ndata: %s\n\n", body); err != nil {
		return
	}
	flusher.Flush()
}

func emitDone(w http.ResponseWriter, flusher http.Flusher) {
	ev := doneEvent{TS: nowRFC3339()}
	body, _ := json.Marshal(ev)
	if _, err := fmt.Fprintf(w, "event: done\ndata: %s\n\n", body); err != nil {
		return
	}
	flusher.Flush()
}

func nowRFC3339() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z07:00")
}
