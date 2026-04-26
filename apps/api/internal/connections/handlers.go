package connections

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/danielgtaylor/huma/v2"

	"github.com/swastikpatel7/cadence/apps/api/internal/auth"
	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
)

// HandlerDeps wires the service plus the static URLs we redirect to
// after the OAuth callback.
type HandlerDeps struct {
	Service     *Service
	WebBaseURL  string // e.g. http://localhost:3000
	SettingsURL string // path on the web host, defaults to /settings
}

// Register wires all connection-related operations.
//   - public: OAuth callback (state token IS the auth)
//   - authed: start, disconnect, sync POST/GET (Clerk JWT required)
func Register(public, authed huma.API, d HandlerDeps) {
	if d.SettingsURL == "" {
		d.SettingsURL = "/settings"
	}
	registerCallback(public, d)
	registerStart(authed, d)
	registerDisconnect(authed, d)
	registerSyncPost(authed, d)
	registerSyncGet(authed, d)
}

// ---- start ---------------------------------------------------------

type startOutput struct {
	Body struct {
		AuthorizeURL string `json:"authorize_url" doc:"Strava authorize URL the client should navigate to"`
	}
}

func registerStart(api huma.API, d HandlerDeps) {
	huma.Register(api, huma.Operation{
		OperationID: "strava-connect-start",
		Method:      http.MethodGet,
		Path:        "/v1/connections/strava/start",
		Summary:     "Begin Strava OAuth (returns the authorize URL)",
		Description: "Generates a CSRF state token and returns the Strava authorize URL. " +
			"Browser must do `window.location.href = authorize_url` to navigate.",
		Tags: []string{"connections"},
	}, func(ctx context.Context, _ *struct{}) (*startOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}
		u, err := d.Service.AuthorizeURLFor(ctx, userID)
		if err != nil {
			pkglogger.FromContext(ctx).Error("strava start failed", "err", err)
			return nil, huma.Error500InternalServerError("failed to start oauth")
		}
		out := &startOutput{}
		out.Body.AuthorizeURL = u
		return out, nil
	})
}

// ---- callback ------------------------------------------------------

type callbackInput struct {
	Code             string `query:"code" doc:"Authorization code returned by Strava"`
	State            string `query:"state" doc:"CSRF state token bound to the calling user"`
	Error            string `query:"error" doc:"Error code if user denied access"`
	ErrorDescription string `query:"error_description"`
}

type callbackOutput struct {
	Status   int
	Location string `header:"Location"`
}

func registerCallback(api huma.API, d HandlerDeps) {
	huma.Register(api, huma.Operation{
		OperationID: "strava-connect-callback",
		Method:      http.MethodGet,
		Path:        "/v1/connections/strava/callback",
		Summary:     "Strava OAuth callback (redirects to /settings)",
		Description: "Exchanges the code for tokens, encrypts and persists them, " +
			"then redirects the browser to the Settings page on the web host.",
		Tags: []string{"connections"},
	}, func(ctx context.Context, in *callbackInput) (*callbackOutput, error) {
		log := pkglogger.FromContext(ctx)
		if in.Error != "" {
			log.Warn("strava callback: user denied", "err", in.Error, "desc", in.ErrorDescription)
			return redirectTo(d, "error", "denied"), nil
		}
		res := d.Service.FinishOAuth(ctx, in.Code, in.State)
		if !res.OK {
			log.Warn("strava callback: failed", "reason", res.Reason)
			return redirectTo(d, "error", res.Reason), nil
		}
		log.Info("strava connected", "user_id", res.UserID)
		return redirectTo(d, "connected", ""), nil
	})
}

func redirectTo(d HandlerDeps, status, reason string) *callbackOutput {
	q := url.Values{}
	q.Set("strava", status)
	if reason != "" {
		q.Set("reason", reason)
	}
	return &callbackOutput{
		Status:   http.StatusFound,
		Location: d.WebBaseURL + d.SettingsURL + "?" + q.Encode(),
	}
}

// ---- disconnect ----------------------------------------------------

type disconnectOutput struct {
	Body struct {
		Status string `json:"status" example:"ok"`
	}
}

func registerDisconnect(api huma.API, d HandlerDeps) {
	huma.Register(api, huma.Operation{
		OperationID: "strava-connect-disconnect",
		Method:      http.MethodDelete,
		Path:        "/v1/connections/strava",
		Summary:     "Soft-disconnect Strava",
		Tags:        []string{"connections"},
	}, func(ctx context.Context, _ *struct{}) (*disconnectOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}
		if err := d.Service.Disconnect(ctx, userID); err != nil {
			pkglogger.FromContext(ctx).Error("strava disconnect failed", "err", err)
			return nil, huma.Error500InternalServerError("failed to disconnect")
		}
		out := &disconnectOutput{}
		out.Body.Status = "ok"
		return out, nil
	})
}

// ---- POST /me/sync -------------------------------------------------

type syncPostInput struct {
	Body struct {
		Days int `json:"days" enum:"7,30,90" doc:"Sync window in days"`
	}
}

type syncPostOutput struct {
	Status int
	Body   struct {
		Enqueued bool `json:"enqueued"`
	}
}

func registerSyncPost(api huma.API, d HandlerDeps) {
	huma.Register(api, huma.Operation{
		OperationID: "me-sync-start",
		Method:      http.MethodPost,
		Path:        "/v1/me/sync",
		Summary:     "Enqueue a manual Strava sync",
		Description: "409 if a sync is already in flight for this user. " +
			"Frontend should poll GET /v1/me/sync until syncing=false.",
		Tags: []string{"connections"},
	}, func(ctx context.Context, in *syncPostInput) (*syncPostOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}
		if err := d.Service.StartSync(ctx, userID, in.Body.Days); err != nil {
			switch {
			case errors.Is(err, ErrNoConnection):
				return nil, huma.Error409Conflict("no active Strava connection")
			case errors.Is(err, ErrSyncInProgress):
				return nil, huma.Error409Conflict("a sync is already in progress")
			case errors.Is(err, ErrInvalidDays):
				return nil, huma.Error400BadRequest(err.Error())
			default:
				pkglogger.FromContext(ctx).Error("strava sync enqueue failed", "err", err)
				return nil, huma.Error500InternalServerError("failed to enqueue sync")
			}
		}
		out := &syncPostOutput{Status: http.StatusAccepted}
		out.Body.Enqueued = true
		return out, nil
	})
}

// ---- GET /me/sync --------------------------------------------------

type syncGetOutput struct {
	Body SyncStatusBody
}

// SyncStatusBody is the JSON response for GET /v1/me/sync. Combines
// connection state, sync progress, recent activity, and breakdown so
// the Settings page can render in one round-trip.
type SyncStatusBody struct {
	Syncing         bool                `json:"syncing"`
	StartedAt       *string             `json:"started_at,omitempty" doc:"RFC3339 if syncing"`
	Processed       int                 `json:"processed"`
	TotalKnown      int                 `json:"total_known"`
	LastSyncAt      *string             `json:"last_sync_at,omitempty"`
	LastError       *string             `json:"last_error,omitempty"`
	TotalActivities int64               `json:"total_activities"`
	SportBreakdown  map[string]int64    `json:"sport_breakdown"`
	Recent          []RecentActivityDTO `json:"recent"`
	Connection      *ConnectionDTO      `json:"connection"`
}

// RecentActivityDTO is the wire shape of a recent activity row.
type RecentActivityDTO struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	SportType      string   `json:"sport_type"`
	StartTime      string   `json:"start_time"`
	DistanceMeters *float64 `json:"distance_meters,omitempty"`
}

// ConnectionDTO is the connection-card payload.
type ConnectionDTO struct {
	Connected   bool     `json:"connected"`
	AthleteName string   `json:"athlete_name"`
	Scopes      []string `json:"scopes"`
	ConnectedAt string   `json:"connected_at"`
}

func registerSyncGet(api huma.API, d HandlerDeps) {
	huma.Register(api, huma.Operation{
		OperationID: "me-sync-status",
		Method:      http.MethodGet,
		Path:        "/v1/me/sync",
		Summary:     "Get Strava connection + sync status",
		Tags:        []string{"connections"},
	}, func(ctx context.Context, _ *struct{}) (*syncGetOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}
		st, err := d.Service.GetStatus(ctx, userID)
		if err != nil {
			pkglogger.FromContext(ctx).Error("strava status failed", "err", err)
			return nil, huma.Error500InternalServerError("failed to load status")
		}
		out := &syncGetOutput{Body: toBody(st)}
		return out, nil
	})
}

func toBody(s *Status) SyncStatusBody {
	body := SyncStatusBody{
		Syncing:         s.Syncing,
		Processed:       s.Processed,
		TotalKnown:      s.TotalKnown,
		LastError:       s.LastError,
		TotalActivities: s.TotalActivities,
		SportBreakdown:  s.SportBreakdown,
	}
	if s.StartedAt != nil {
		v := s.StartedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		body.StartedAt = &v
	}
	if s.LastSyncAt != nil {
		v := s.LastSyncAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		body.LastSyncAt = &v
	}
	body.Recent = make([]RecentActivityDTO, 0, len(s.Recent))
	for _, r := range s.Recent {
		body.Recent = append(body.Recent, RecentActivityDTO{
			ID:             r.ID.String(),
			Name:           r.Name,
			SportType:      r.SportType,
			StartTime:      r.StartTime.UTC().Format("2006-01-02T15:04:05Z07:00"),
			DistanceMeters: r.DistanceMeters,
		})
	}
	if s.Connection != nil {
		body.Connection = &ConnectionDTO{
			Connected:   s.Connection.Connected,
			AthleteName: s.Connection.AthleteName,
			Scopes:      s.Connection.Scopes,
			ConnectedAt: s.Connection.ConnectedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return body
}

