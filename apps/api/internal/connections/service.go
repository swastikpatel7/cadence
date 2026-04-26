// Package connections implements the Strava OAuth flow + manual sync
// orchestration for apps/api. Domain layer only; HTTP plumbing is in
// handlers.go.
package connections

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"golang.org/x/oauth2"

	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
	pkgcrypto "github.com/swastikpatel7/cadence/pkg/crypto"
	"github.com/swastikpatel7/cadence/pkg/strava"
)

// ProviderStrava is the connected_accounts.provider value for Strava.
const ProviderStrava = "strava"

// Allowed sync windows. Constrains POST /v1/me/sync requests.
var validDays = map[int]bool{7: true, 30: true, 90: true}

// Service is the OAuth + sync façade used by handlers.go. It is a
// concrete struct rather than an interface for v1 (no test doubles
// yet); we'll lift to an interface when we write integration tests.
type Service struct {
	queries *dbgen.Queries
	state   *StateStore
	cipher  *pkgcrypto.TokenCipher
	oauth   *oauth2.Config
	http    *http.Client
	river   *river.Client[pgx.Tx]
	log     *slog.Logger
}

// Deps groups the wiring inputs.
type Deps struct {
	Queries *dbgen.Queries
	State   *StateStore
	Cipher  *pkgcrypto.TokenCipher
	OAuth   *oauth2.Config
	HTTP    *http.Client
	River   *river.Client[pgx.Tx]
	Logger  *slog.Logger
}

// NewService constructs a Service from its dependencies.
func NewService(d Deps) *Service {
	httpClient := d.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Service{
		queries: d.Queries,
		state:   d.State,
		cipher:  d.Cipher,
		oauth:   d.OAuth,
		http:    httpClient,
		river:   d.River,
		log:     d.Logger,
	}
}

// AuthorizeURLFor generates a fresh state token for userID and returns
// the full Strava authorize URL the browser should navigate to.
func (s *Service) AuthorizeURLFor(ctx context.Context, userID uuid.UUID) (string, error) {
	state, err := s.state.Generate(ctx, userID)
	if err != nil {
		return "", err
	}
	return strava.AuthCodeURL(s.oauth, state), nil
}

// CallbackResult is the outcome of FinishOAuth — used by the handler to
// build the post-callback redirect URL.
type CallbackResult struct {
	UserID uuid.UUID
	OK     bool
	Reason string
}

// FinishOAuth validates state, exchanges the code, encrypts the token
// pair, and upserts connected_accounts. Caller is expected to redirect
// the browser to the Settings page (success or failure both go there).
func (s *Service) FinishOAuth(ctx context.Context, code, state string) CallbackResult {
	if code == "" {
		return CallbackResult{OK: false, Reason: "missing code"}
	}
	userID, err := s.state.Consume(ctx, state)
	if err != nil {
		s.log.Warn("oauth callback: state invalid", "err", err)
		return CallbackResult{OK: false, Reason: "state expired"}
	}

	tok, err := strava.ExchangeCode(ctx, s.http, s.oauth, code)
	if err != nil {
		s.log.Error("oauth callback: exchange failed", "err", err, "user_id", userID)
		return CallbackResult{UserID: userID, OK: false, Reason: "exchange failed"}
	}

	accessEnc, err := s.cipher.Encrypt(tok.AccessToken)
	if err != nil {
		s.log.Error("oauth callback: encrypt access", "err", err)
		return CallbackResult{UserID: userID, OK: false, Reason: "encrypt failed"}
	}
	refreshEnc, err := s.cipher.Encrypt(tok.RefreshToken)
	if err != nil {
		s.log.Error("oauth callback: encrypt refresh", "err", err)
		return CallbackResult{UserID: userID, OK: false, Reason: "encrypt failed"}
	}

	var athlete strava.Athlete
	if len(tok.Athlete) > 0 {
		// Best-effort — ID is what we really need for provider_user_id.
		_ = json.Unmarshal(tok.Athlete, &athlete)
	}
	if athlete.ID == 0 {
		s.log.Error("oauth callback: athlete id missing in token response")
		return CallbackResult{UserID: userID, OK: false, Reason: "athlete missing"}
	}

	scopes := splitScopes(tok.Scope)

	if _, err := s.queries.UpsertConnectedAccount(ctx, dbgen.UpsertConnectedAccountParams{
		UserID:          userID,
		Provider:        ProviderStrava,
		ProviderUserID:  fmt.Sprintf("%d", athlete.ID),
		AccessTokenEnc:  accessEnc,
		RefreshTokenEnc: refreshEnc,
		ExpiresAt:       pgtype.Timestamptz{Time: time.Unix(tok.ExpiresAt, 0), Valid: true},
		Scopes:          scopes,
		RawAthlete:      []byte(tok.Athlete),
	}); err != nil {
		s.log.Error("oauth callback: upsert failed", "err", err, "user_id", userID)
		return CallbackResult{UserID: userID, OK: false, Reason: "store failed"}
	}

	return CallbackResult{UserID: userID, OK: true}
}

// Disconnect flags the user's Strava connection as user-disconnected.
// Idempotent — safe to call when no connection exists (no rows updated).
func (s *Service) Disconnect(ctx context.Context, userID uuid.UUID) error {
	return s.queries.DisconnectAccount(ctx, dbgen.DisconnectAccountParams{
		UserID:   userID,
		Provider: ProviderStrava,
	})
}

// ErrNoConnection signals there's no Strava row to act on.
var ErrNoConnection = errors.New("connections: no active Strava connection")

// ErrSyncInProgress signals a sync is already running for this user.
var ErrSyncInProgress = errors.New("connections: sync already in progress")

// ErrInvalidDays signals an out-of-range days value.
var ErrInvalidDays = errors.New("connections: invalid days (must be 7, 30, or 90)")

// StartSync marks the user's connection as syncing and inserts a River
// job to process activities + streams over the given window.
func (s *Service) StartSync(ctx context.Context, userID uuid.UUID, days int) error {
	if !validDays[days] {
		return ErrInvalidDays
	}

	conn, err := s.queries.GetConnectedAccount(ctx, dbgen.GetConnectedAccountParams{
		UserID:   userID,
		Provider: ProviderStrava,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoConnection
		}
		return fmt.Errorf("connections: load row: %w", err)
	}
	if conn.LastError != nil && *conn.LastError != "" {
		return ErrNoConnection
	}
	if conn.SyncStartedAt.Valid {
		return ErrSyncInProgress
	}

	afterTs := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	progress, err := json.Marshal(strava.SyncProgress{AfterTs: afterTs})
	if err != nil {
		return fmt.Errorf("connections: marshal progress: %w", err)
	}

	if err := s.queries.SetSyncStarted(ctx, dbgen.SetSyncStartedParams{
		UserID:       userID,
		Provider:     ProviderStrava,
		SyncProgress: progress,
	}); err != nil {
		return fmt.Errorf("connections: mark started: %w", err)
	}

	args := strava.SyncJobArgs{UserID: userID, AfterTs: afterTs}
	if _, err := s.river.Insert(ctx, args, nil); err != nil {
		// Best-effort rollback so the user can retry without a 409.
		_ = s.queries.ClearSyncStartedFailure(ctx, dbgen.ClearSyncStartedFailureParams{
			ID:        conn.ID,
			LastError: ptr("enqueue failed"),
		})
		return fmt.Errorf("connections: enqueue: %w", err)
	}
	return nil
}

// Status is the rich state document returned by GET /v1/me/sync. The
// handler maps this to the JSON body.
type Status struct {
	Connection      *ConnectionView
	Syncing         bool
	StartedAt       *time.Time
	Processed       int
	TotalKnown      int
	LastSyncAt      *time.Time
	LastError       *string
	TotalActivities int64
	SportBreakdown  map[string]int64
	Recent          []RecentActivity
}

// ConnectionView is the connection-card slice of Status.
type ConnectionView struct {
	Connected   bool
	AthleteName string
	Scopes      []string
	ConnectedAt time.Time
}

// RecentActivity is one row in Status.Recent.
type RecentActivity struct {
	ID             uuid.UUID
	Name           string
	SportType      string
	StartTime      time.Time
	DistanceMeters *float64
}

// GetStatus assembles the settings-page payload in one round-trip.
// Connection may be nil if the user has never connected Strava.
func (s *Service) GetStatus(ctx context.Context, userID uuid.UUID) (*Status, error) {
	out := &Status{}

	conn, err := s.queries.GetConnectedAccount(ctx, dbgen.GetConnectedAccountParams{
		UserID:   userID,
		Provider: ProviderStrava,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("connections: load row: %w", err)
	}

	if err == nil {
		view := &ConnectionView{
			Connected:   conn.LastError == nil || *conn.LastError == "",
			Scopes:      conn.Scopes,
			ConnectedAt: conn.ConnectedAt.Time,
		}
		if len(conn.RawAthlete) > 0 {
			var a strava.Athlete
			if jerr := json.Unmarshal(conn.RawAthlete, &a); jerr == nil {
				view.AthleteName = a.DisplayName()
			}
		}
		out.Connection = view

		out.Syncing = conn.SyncStartedAt.Valid
		if out.Syncing {
			t := conn.SyncStartedAt.Time
			out.StartedAt = &t
		}
		if len(conn.SyncProgress) > 0 {
			var p strava.SyncProgress
			if jerr := json.Unmarshal(conn.SyncProgress, &p); jerr == nil {
				out.Processed = p.Processed
				out.TotalKnown = p.TotalKnown
			}
		}
		if conn.LastSyncAt.Valid {
			t := conn.LastSyncAt.Time
			out.LastSyncAt = &t
		}
		if conn.LastError != nil && *conn.LastError != "" {
			e := *conn.LastError
			out.LastError = &e
		}
	}

	count, err := s.queries.CountActivitiesByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("connections: count activities: %w", err)
	}
	out.TotalActivities = count

	breakdown, err := s.queries.BreakdownActivitiesBySport(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("connections: breakdown: %w", err)
	}
	out.SportBreakdown = make(map[string]int64, len(breakdown))
	for _, b := range breakdown {
		out.SportBreakdown[b.SportType] = b.Total
	}

	recent, err := s.queries.ListRecentActivitiesByUser(ctx, dbgen.ListRecentActivitiesByUserParams{
		UserID: userID,
		Limit:  5,
	})
	if err != nil {
		return nil, fmt.Errorf("connections: list recent: %w", err)
	}
	out.Recent = make([]RecentActivity, 0, len(recent))
	for _, r := range recent {
		ra := RecentActivity{
			ID:        r.ID,
			Name:      r.Name,
			SportType: r.SportType,
			StartTime: r.StartTime.Time,
		}
		if r.DistanceMeters.Valid {
			f, ferr := r.DistanceMeters.Float64Value()
			if ferr == nil && f.Valid {
				v := f.Float64
				ra.DistanceMeters = &v
			}
		}
		out.Recent = append(out.Recent, ra)
	}

	return out, nil
}

func ptr[T any](v T) *T { return &v }

// splitScopes turns Strava's space-delimited scope string into a slice.
func splitScopes(s string) []string {
	if s == "" {
		return []string{strava.ScopeReadAll}
	}
	out := []string{}
	cur := ""
	for _, ch := range s {
		if ch == ' ' || ch == ',' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(ch)
	}
	if cur != "" {
		out = append(out, cur)
	}
	if len(out) == 0 {
		out = []string{strava.ScopeReadAll}
	}
	return out
}
