// Package strava is a thin wrapper over Strava's V3 API: OAuth via
// golang.org/x/oauth2 with a custom Endpoint, plus a small HTTP client
// for the three calls our manual sync needs (list, detail, streams).
//
// We deliberately don't use a generated SDK. Strava's official
// strava/go.strava is archived (Feb 2023) and predates rotating refresh
// tokens; the swagger-generated alternatives ship 200+ types we don't
// touch.
package strava

import "encoding/json"

// Athlete is the slice of the athlete blob we display on the Settings
// page. Everything else stays in the raw_athlete jsonb column on
// connected_accounts so future code can read it without re-auth.
type Athlete struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"firstname,omitempty"`
	LastName  string `json:"lastname,omitempty"`
	Profile   string `json:"profile,omitempty"`
}

// DisplayName returns the human-friendly name to show on the Settings
// page connection card. Falls back gracefully if the athlete lacks first/
// last name (Strava allows usernames without names).
func (a Athlete) DisplayName() string {
	switch {
	case a.FirstName != "" && a.LastName != "":
		return a.FirstName + " " + a.LastName
	case a.FirstName != "":
		return a.FirstName
	case a.Username != "":
		return a.Username
	default:
		return "Strava athlete"
	}
}

// SummaryActivity is the projection we read from the list endpoint AND
// the detail endpoint — DetailedActivity is a superset of SummaryActivity
// in Strava's schema, so the same struct fits both.
//
// Anything we don't project is preserved in the raw blob the caller
// passes through to the activities table, so we can derive other fields
// later without re-fetching.
type SummaryActivity struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	SportType          string  `json:"sport_type"`
	Type               string  `json:"type"` // legacy fallback if sport_type empty
	StartDate          string  `json:"start_date"`
	Distance           float64 `json:"distance"` // meters
	MovingTime         int     `json:"moving_time"`
	ElapsedTime        int     `json:"elapsed_time"`
	TotalElevationGain float64 `json:"total_elevation_gain"`
	HasHeartrate       bool    `json:"has_heartrate"`
	AverageHeartrate   float64 `json:"average_heartrate,omitempty"`
	MaxHeartrate       float64 `json:"max_heartrate,omitempty"`
	Calories           float64 `json:"calories,omitempty"` // detail only
}

// SyncProgress is the JSON shape stored in connected_accounts.sync_progress.
// Worker writes it on each activity processed; the GET /v1/me/sync handler
// reads it to populate the polling response.
//
// Cursor design: AfterTs is the fixed lower bound (start of the requested
// window). BeforeTs is the upper bound that walks down toward AfterTs as
// we process activities — each iteration of the worker queries Strava
// for `after=AfterTs&before=BeforeTs`, processes the page newest-first,
// then sets BeforeTs to the start_date of the oldest activity it just
// processed. When the query returns no rows we're done.
type SyncProgress struct {
	Processed  int   `json:"processed"`
	TotalKnown int   `json:"total_known"`
	AfterTs    int64 `json:"after_ts"`           // unix seconds; lower bound (fixed)
	BeforeTs   int64 `json:"before_ts,omitempty"` // unix seconds; upper bound (walks down)
}

// MarshalJSONOrNull returns the marshalled bytes or nil — convenient
// when persisting to a nullable jsonb column.
func MarshalJSONOrNull(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}
