package baseline

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
)

// Numeric is the pure-Go output of the volume-curve / pace stages of
// the baseline compute. It feeds into both the Opus prompt (input
// context for the narrative) and the InsertBaselineParams payload.
type Numeric struct {
	WindowDays         int32
	WeeklyVolumeKMAvg  float64
	WeeklyVolumeKMP25  float64
	WeeklyVolumeKMP75  float64
	AvgPaceSecPerKM    int32
	AvgPaceAtDistance  map[int]int // distance(km) → sec/km
	LongestRunKM       float64
	ConsistencyScore   int32 // 0..100 fraction of weeks in window with at least one run
	FitnessTier        string
	ActivityCount      int
	WeeklyVolumesKM    []float64 // per-week totals oldest → newest
	RecentRunSummaries []RunSummary
}

// RunSummary is a compact per-run record for the Opus narrative
// prompt: never serialized to the DB — only handed to narrate.go.
type RunSummary struct {
	StartTime    time.Time
	DistanceKM   float64
	DurationSec  int32
	AvgPaceSecKM int32
	AvgHR        *int32
}

// ComputeNumeric runs the SQL+math derivation. windowDays = -1 means
// "all history". Pulls the activity rows once and aggregates locally —
// for v1 volumes this is cheaper than 5 separate aggregate queries.
func ComputeNumeric(
	ctx context.Context,
	queries *dbgen.Queries,
	userID uuid.UUID,
	windowDays int,
) (*Numeric, error) {
	now := time.Now().UTC()
	var windowStart time.Time
	if windowDays < 0 {
		windowStart = time.Unix(0, 0)
	} else {
		windowStart = now.AddDate(0, 0, -windowDays)
	}

	rows, err := queries.ListActivitiesInWindow(ctx, dbgen.ListActivitiesInWindowParams{
		UserID:     userID,
		StartTime:  pgtype.Timestamptz{Time: windowStart, Valid: true},
		StartTime_2: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("baseline.compute: list activities: %w", err)
	}

	out := &Numeric{
		WindowDays:        int32Bound(windowDays),
		ActivityCount:     len(rows),
		AvgPaceAtDistance: map[int]int{},
	}
	if len(rows) == 0 {
		// Snooze path; the worker checks ActivityCount before continuing.
		out.FitnessTier = "T1"
		return out, nil
	}

	// Bucket by ISO week (Monday-anchored) within the window. We use
	// Go's ISOWeek to keep this stable across year boundaries.
	weekKMs := map[isoWeekKey]float64{}
	type lastRun struct {
		distanceKM   float64
		durationSec  int32
		avgPaceSecKM int32
		startTime    time.Time
		avgHR        *int32
	}
	var (
		lastFive    []lastRun
		longestRun  float64
		paceSamples []int
	)

	for _, r := range rows {
		distM, _ := numericFloat(r.DistanceMeters)
		if distM <= 0 {
			continue
		}
		distKM := distM / 1000.0
		dur := r.DurationSeconds
		if dur <= 0 {
			continue
		}
		paceSecKM := int32(float64(dur) / distKM)

		startT := r.StartTime.Time
		y, w := startT.ISOWeek()
		weekKMs[isoWeekKey{Year: y, Week: w}] += distKM

		paceSamples = append(paceSamples, int(paceSecKM))
		if distKM > longestRun {
			longestRun = distKM
		}
		lastFive = append(lastFive, lastRun{
			distanceKM:   distKM,
			durationSec:  dur,
			avgPaceSecKM: paceSecKM,
			startTime:    startT,
			avgHR:        r.AvgHeartRate,
		})

		// Pace-at-distance buckets (5K, 10K, half, marathon). Pick the
		// closest milestone the run brackets and take the best (lowest)
		// pace observed at that distance.
		if d := nearestPaceBucket(distKM); d > 0 {
			cur, ok := out.AvgPaceAtDistance[d]
			if !ok || int(paceSecKM) < cur {
				out.AvgPaceAtDistance[d] = int(paceSecKM)
			}
		}
	}

	// Recent run summaries (last 5 by start time, newest first).
	sort.Slice(lastFive, func(i, j int) bool {
		return lastFive[i].startTime.After(lastFive[j].startTime)
	})
	if len(lastFive) > 5 {
		lastFive = lastFive[:5]
	}
	for _, r := range lastFive {
		out.RecentRunSummaries = append(out.RecentRunSummaries, RunSummary{
			StartTime:    r.startTime,
			DistanceKM:   r.distanceKM,
			DurationSec:  r.durationSec,
			AvgPaceSecKM: r.avgPaceSecKM,
			AvgHR:        r.avgHR,
		})
	}

	// Build sorted weekly-volume slice oldest → newest.
	var weeklyKMs []float64
	weekKeys := make([]isoWeekKey, 0, len(weekKMs))
	for k := range weekKMs {
		weekKeys = append(weekKeys, k)
	}
	sort.Slice(weekKeys, func(i, j int) bool {
		return weekKeys[i].Less(weekKeys[j])
	})
	for _, k := range weekKeys {
		weeklyKMs = append(weeklyKMs, weekKMs[k])
	}
	out.WeeklyVolumesKM = weeklyKMs

	out.WeeklyVolumeKMAvg = mean(weeklyKMs)
	out.WeeklyVolumeKMP25 = percentile(weeklyKMs, 0.25)
	out.WeeklyVolumeKMP75 = percentile(weeklyKMs, 0.75)
	out.LongestRunKM = round2(longestRun)
	out.AvgPaceSecPerKM = int32(intMean(paceSamples))

	// Consistency: fraction of weeks in the window that contain at
	// least one run. Coarse but interpretable.
	var totalWeeks int
	if windowDays > 0 {
		totalWeeks = windowDays / 7
		if totalWeeks <= 0 {
			totalWeeks = 1
		}
	} else if len(weeklyKMs) > 0 {
		totalWeeks = len(weeklyKMs)
	} else {
		totalWeeks = 1
	}
	out.ConsistencyScore = int32(math.Round(float64(len(weeklyKMs)) / float64(totalWeeks) * 100))
	if out.ConsistencyScore > 100 {
		out.ConsistencyScore = 100
	}
	if out.ConsistencyScore < 0 {
		out.ConsistencyScore = 0
	}

	out.FitnessTier = deriveFitnessTier(out.WeeklyVolumeKMAvg, out.LongestRunKM, out.AvgPaceSecPerKM)
	return out, nil
}

// CountActivities is the SSE-stage gate: returns the total count of
// not-deleted activities for the user. The worker uses this to decide
// whether to JobSnooze (insights.md §17 — "0 activities" graceful
// degradation).
func CountActivities(ctx context.Context, queries *dbgen.Queries, userID uuid.UUID) (int64, error) {
	return queries.CountActivitiesByUser(ctx, userID)
}

// deriveFitnessTier maps weekly volume + long run + average pace into
// a coarse 5-bucket fitness tier. Calibrated to community-running
// averages (insights.md §2). v1 is a heuristic; v2 will calibrate
// against actual user distributions.
func deriveFitnessTier(weeklyKM, longestRunKM float64, avgPaceSecKM int32) string {
	switch {
	case weeklyKM < 10 || avgPaceSecKM == 0:
		return "T1"
	case weeklyKM < 25:
		return "T2"
	case weeklyKM < 45:
		if avgPaceSecKM < 330 {
			return "T4"
		}
		return "T3"
	case weeklyKM < 70:
		if avgPaceSecKM < 300 {
			return "T5"
		}
		return "T4"
	default:
		return "T5"
	}
}

// nearestPaceBucket maps a run distance to the closest "common race"
// distance — 5K, 10K, half (21), marathon (42). Anything below 4K
// returns 0 (no bucket).
func nearestPaceBucket(distKM float64) int {
	switch {
	case distKM < 4:
		return 0
	case distKM < 7.5:
		return 5
	case distKM < 16:
		return 10
	case distKM < 32:
		return 21
	default:
		return 42
	}
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return round2(sum / float64(len(xs)))
}

func intMean(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	var sum int
	for _, x := range xs {
		sum += x
	}
	return sum / len(xs)
}

func percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	if len(cp) == 1 {
		return round2(cp[0])
	}
	idx := p * float64(len(cp)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return round2(cp[lo])
	}
	frac := idx - float64(lo)
	return round2(cp[lo]*(1-frac) + cp[hi]*frac)
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

func int32Bound(d int) int32 {
	if d < 0 {
		return -1
	}
	if d > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(d)
}

// isoWeekKey is a sortable composite of (ISO year, ISO week).
type isoWeekKey struct {
	Year int
	Week int
}

func (a isoWeekKey) Less(b isoWeekKey) bool {
	if a.Year != b.Year {
		return a.Year < b.Year
	}
	return a.Week < b.Week
}

// numericFloat extracts a float64 from a pgtype.Numeric, returning 0
// + ok=false when not Valid.
func numericFloat(n pgtype.Numeric) (float64, bool) {
	if !n.Valid {
		return 0, false
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0, false
	}
	return f.Float64, true
}
