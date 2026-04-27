package plan

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// HeatmapInput is everything ProjectHeatmap needs.
type HeatmapInput struct {
	WindowStart time.Time
	WindowEnd   time.Time
	Today       time.Time
	Plans       []PlanRow
	Activities  []ActivityRow
}

// PlanRow is the slice of coach_plans we need to project. Mapped from
// the sqlc CoachPlan struct in handlers.go to keep this package free
// of the dbgen import.
type PlanRow struct {
	StartsOn   time.Time // Monday of week 1
	WeeksCount int
	Plan       []byte // raw JSONB
}

// ActivityRow is the slice of activities we need to merge.
type ActivityRow struct {
	ID              string
	StartTime       time.Time
	DistanceKM      float64
	DurationSeconds int
	AvgPaceSecPerKM int
}

// HeatmapCell mirrors the wire shape (api.md §2.7).
type HeatmapCell struct {
	Date                 string         `json:"date"`
	PrescribedLoad       string         `json:"prescribed_load"`
	PrescribedDistanceKM *float64       `json:"prescribed_distance_km,omitempty"`
	PrescribedType       *string        `json:"prescribed_type,omitempty"`
	IsToday              bool           `json:"is_today"`
	IsFuture             bool           `json:"is_future"`
	Actual               *HeatmapActual `json:"actual,omitempty"`
}

// HeatmapActual mirrors the wire shape.
type HeatmapActual struct {
	ActivityID      string  `json:"activity_id"`
	Completed       bool    `json:"completed"`
	DistanceKM      float64 `json:"distance_km"`
	AvgPaceSecPerKM int     `json:"avg_pace_sec_per_km"`
	Matched         string  `json:"matched"`
}

// ProjectHeatmap stitches plan + activities into rows of 7 cells
// (Mon..Sun) covering [windowStart, windowEnd].
//
// Skeleton mode: if plans is empty, every cell prescribes 'rest' but
// is_today / is_future are still set correctly. UI shows shimmer.
func ProjectHeatmap(in HeatmapInput) [][]HeatmapCell {
	weeks := mondaySpan(in.WindowStart, in.WindowEnd)
	out := make([][]HeatmapCell, 0, len(weeks))

	// Index sessions by date string for O(1) lookup.
	sessionByDate := map[string]Session{}
	for _, p := range in.Plans {
		var pb PlanBlob
		if err := json.Unmarshal(p.Plan, &pb); err != nil {
			continue
		}
		for _, w := range pb.Weeks {
			for _, s := range w.Sessions {
				if _, exists := sessionByDate[s.Date]; !exists {
					sessionByDate[s.Date] = s
				}
			}
		}
	}

	// Index activities by ISO date (longest run wins).
	actByDate := map[string]ActivityRow{}
	for _, a := range in.Activities {
		k := a.StartTime.UTC().Format("2006-01-02")
		if cur, ok := actByDate[k]; !ok || a.DistanceKM > cur.DistanceKM {
			actByDate[k] = a
		}
	}

	todayStr := in.Today.UTC().Format("2006-01-02")

	for _, wkStart := range weeks {
		row := make([]HeatmapCell, 7)
		for i := 0; i < 7; i++ {
			d := wkStart.AddDate(0, 0, i)
			ds := d.Format("2006-01-02")
			cell := HeatmapCell{
				Date:           ds,
				PrescribedLoad: PrescribedLoadRest,
				IsToday:        ds == todayStr,
				IsFuture:       d.After(in.Today),
			}
			if s, ok := sessionByDate[ds]; ok {
				load := s.Load
				if load == "" {
					load = inferLoadFromIntensity(s.Intensity)
				}
				cell.PrescribedLoad = load
				if s.DistanceKM > 0 {
					dk := round2(s.DistanceKM)
					cell.PrescribedDistanceKM = &dk
				}
				if s.Type != "" {
					t := s.Type
					cell.PrescribedType = &t
				}
			}
			if !cell.IsFuture {
				if a, ok := actByDate[ds]; ok {
					cell.Actual = &HeatmapActual{
						ActivityID:      a.ID,
						Completed:       true,
						DistanceKM:      round2(a.DistanceKM),
						AvgPaceSecPerKM: a.AvgPaceSecPerKM,
						Matched:         matchedFor(cell.PrescribedDistanceKM, a.DistanceKM),
					}
				}
			}
			row[i] = cell
		}
		out = append(out, row)
	}
	return out
}

// MondayOf returns the Monday on or before t.
func MondayOf(t time.Time) time.Time {
	t = t.UTC()
	wd := int(t.Weekday()) // Sun=0, Mon=1, ..., Sat=6
	delta := wd - 1
	if wd == 0 {
		delta = 6 // Sunday → previous Monday is 6 days back
	}
	out := t.AddDate(0, 0, -delta)
	return time.Date(out.Year(), out.Month(), out.Day(), 0, 0, 0, 0, time.UTC)
}

// mondaySpan returns the list of Mondays anchoring each week between
// windowStart and windowEnd (inclusive of both edge weeks).
func mondaySpan(start, end time.Time) []time.Time {
	if !end.After(start) {
		return nil
	}
	startMon := MondayOf(start)
	endMon := MondayOf(end)
	out := []time.Time{}
	for m := startMon; !m.After(endMon); m = m.AddDate(0, 0, 7) {
		out = append(out, m)
	}
	return out
}

// inferLoadFromIntensity is the fallback if the generator omitted Load.
func inferLoadFromIntensity(intensity string) string {
	switch intensity {
	case "easy":
		return LoadEasy
	case "moderate":
		return LoadModerate
	case "hard":
		return LoadHard
	default:
		return LoadEasy
	}
}

// matchedFor decides "over" / "under" / "on" against the prescribed
// distance. Threshold ±15% (insights.md §5).
func matchedFor(prescribedKM *float64, actualKM float64) string {
	if prescribedKM == nil || *prescribedKM <= 0 {
		return "on"
	}
	delta := actualKM - *prescribedKM
	tol := *prescribedKM * 0.15
	if delta > tol {
		return "over"
	}
	if delta < -tol {
		return "under"
	}
	return "on"
}

// SessionForDate finds the prescribed session in a plan blob for a
// given YYYY-MM-DD. Used by the session-detail handler.
func SessionForDate(planBytes []byte, date string) (*Session, error) {
	var pb PlanBlob
	if err := json.Unmarshal(planBytes, &pb); err != nil {
		return nil, fmt.Errorf("decode plan: %w", err)
	}
	for _, w := range pb.Weeks {
		for _, s := range w.Sessions {
			if s.Date == date {
				cp := s
				return &cp, nil
			}
		}
	}
	return nil, nil // not found is not an error
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }
