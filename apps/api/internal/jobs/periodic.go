package jobs

import (
	"time"

	"github.com/riverqueue/river"

	"github.com/swastikpatel7/cadence/apps/api/internal/plan"
)

// SundayAt18UTCSchedule fires the next Sunday at 18:00 UTC. If today
// is Sunday past 18:00, returns next Sunday. Implements River's
// PeriodicSchedule interface (Next(time.Time) time.Time).
//
// River v0.35 ships only PeriodicInterval; cron-style schedules are
// expected to come from third-party packages. We hand-roll the one
// schedule we actually need to keep the dependency surface small.
type SundayAt18UTCSchedule struct{}

// Next returns the next Sunday-at-18:00-UTC after `current`.
func (SundayAt18UTCSchedule) Next(current time.Time) time.Time {
	c := current.UTC()
	// Days until next Sunday: Sunday=0, so (0-wd+7)%7 days. If today is
	// Sunday and we're past 18:00, advance 7 days.
	wd := int(c.Weekday())
	daysUntilSun := (0 - wd + 7) % 7
	candidate := time.Date(c.Year(), c.Month(), c.Day(), 18, 0, 0, 0, time.UTC).AddDate(0, 0, daysUntilSun)
	if !candidate.After(current) {
		candidate = candidate.AddDate(0, 0, 7)
	}
	return candidate
}

// NewWeeklyRefreshPeriodic returns the river.PeriodicJob entry that
// the system wiring layer adds to its periodic-job slice.
//
// v1 limitation: the periodic job inserts a single tick row with a
// nil UserID; per-user fan-out is deferred to the coach v1 cron work
// (api.md §8.1 item 1 — UTC-only for v1, user-local later). The
// WeeklyRefreshWorker no-ops on UserID == uuid.Nil so the tick row
// simply passes through. The user-facing path that matters today is
// the manual /v1/me/plan/refresh + the goal-PATCH-driven enqueue.
func NewWeeklyRefreshPeriodic() *river.PeriodicJob {
	return river.NewPeriodicJob(
		SundayAt18UTCSchedule{},
		func() (river.JobArgs, *river.InsertOpts) {
			args := plan.WeeklyRefreshArgs{
				WeekStart: nextWeekMonday().Format("2006-01-02"),
				Reason:    plan.ReasonWeeklyCron,
			}
			return args, nil
		},
		&river.PeriodicJobOpts{RunOnStart: false, ID: "weekly_refresh_cron"},
	)
}

// nextWeekMonday returns the Monday at the start of the *next* week.
func nextWeekMonday() time.Time {
	now := time.Now().UTC()
	wd := int(now.Weekday()) // Sun=0, Mon=1, ..., Sat=6
	days := (1 - wd + 7) % 7
	if days == 0 {
		days = 7
	}
	out := now.AddDate(0, 0, days)
	return time.Date(out.Year(), out.Month(), out.Day(), 0, 0, 0, 0, time.UTC)
}
