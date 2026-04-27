package plan

// PlanBlob is the JSON shape stored in `coach_plans.plan`. Both the
// initial-plan generator and the weekly-refresh generator must produce
// this structure — it's the single contract the heatmap projector
// reads.
type PlanBlob struct {
	Weeks []Week `json:"weeks"`
}

// Week is one week of the periodized plan.
type Week struct {
	WeekIndex int       `json:"week_index"`           // 0-based; matches starts_on +N weeks
	StartDate string    `json:"start_date,omitempty"` // YYYY-MM-DD, Monday
	TotalKM   float64   `json:"total_km"`
	Sessions  []Session `json:"sessions"`
}

// Session is one prescribed run on a given calendar date.
type Session struct {
	Date               string  `json:"date"` // YYYY-MM-DD
	Type               string  `json:"type"` // easy|tempo|intervals|long|recovery|race_pace
	DistanceKM         float64 `json:"distance_km"`
	Intensity          string  `json:"intensity"` // easy|moderate|hard
	PaceTargetSecPerKM *int    `json:"pace_target_sec_per_km,omitempty"`
	DurationMinTarget  *int    `json:"duration_min_target,omitempty"`
	NotesForCoach      string  `json:"notes_for_coach,omitempty"`
	// Load is the heatmap-cell color bucket. Generators set it from
	// (intensity, type) — rest days have no Session row at all so we
	// don't need a "rest" load on this struct.
	Load string `json:"load"` // easy|moderate|hard|peak
}

// PrescribedLoadRest is the cell load value for days without a
// scheduled run. It's surfaced only in the heatmap projection — never
// stored in PlanBlob.
const PrescribedLoadRest = "rest"

// Wire-level prescribed_type / load enums (api.md §2.7).
const (
	LoadEasy     = "easy"
	LoadModerate = "moderate"
	LoadHard     = "hard"
	LoadPeak     = "peak"

	TypeEasy      = "easy"
	TypeTempo     = "tempo"
	TypeIntervals = "intervals"
	TypeLong      = "long"
	TypeRecovery  = "recovery"
	TypeRacePace  = "race_pace"
)
