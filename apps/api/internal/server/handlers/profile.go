package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/swastikpatel7/cadence/apps/api/internal/auth"
	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
)

const (
	unitsMetric        = "metric"
	unitsImperial      = "imperial"
	defaultTimezoneStr = "UTC"
)

// ProfileBody is the wire shape for both GET and PATCH responses.
type ProfileBody struct {
	DisplayName string `json:"display_name" doc:"User's display name (empty if unset)"`
	Timezone    string `json:"timezone" doc:"IANA timezone, defaults to UTC"`
	Units       string `json:"units" enum:"metric,imperial" doc:"Distance display preference"`
}

type profileGetOutput struct {
	Body ProfileBody
}

type profilePatchInput struct {
	Body profilePatchBody
}

// profilePatchBody is the only-units PATCH body for now. New fields can
// be added as additional optional pointers; the handler will merge them
// against the existing row before upsert.
type profilePatchBody struct {
	Units       *string `json:"units,omitempty" enum:"metric,imperial"`
	DisplayName *string `json:"display_name,omitempty"`
	Timezone    *string `json:"timezone,omitempty"`

	clearDisplayName bool `json:"-"`
}

// UnmarshalJSON mirrors the goalPatchBody pattern in onboarding/handlers.go:
// JSON `null` and a missing field both decode to a nil pointer, so the
// only way to tell the client wants display_name *cleared* is to look at
// the raw payload. Units never accepts null (the column is NOT NULL with
// a CHECK constraint).
func (b *profilePatchBody) UnmarshalJSON(data []byte) error {
	type alias profilePatchBody
	if err := json.Unmarshal(data, (*alias)(b)); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		b.clearDisplayName = string(raw["display_name"]) == "null"
	}
	return nil
}

// RegisterProfile wires GET + PATCH /v1/me/profile. Mounted on the same
// authed group as RegisterMe.
func RegisterProfile(api huma.API, queries *dbgen.Queries) {
	huma.Register(api, huma.Operation{
		OperationID: "get-profile",
		Method:      http.MethodGet,
		Path:        "/v1/me/profile",
		Summary:     "Get the authenticated user's profile (display name, timezone, units)",
		Tags:        []string{"users"},
	}, func(ctx context.Context, _ *struct{}) (*profileGetOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}
		body, err := loadProfileWithDefaults(ctx, queries, userID)
		if err != nil {
			pkglogger.FromContext(ctx).Error("profile.get: load failed", "err", err, "user_id", userID)
			return nil, huma.Error500InternalServerError("failed to load profile")
		}
		return &profileGetOutput{Body: body}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "patch-profile",
		Method:      http.MethodPatch,
		Path:        "/v1/me/profile",
		Summary:     "Partially update the authenticated user's profile",
		Tags:        []string{"users"},
	}, func(ctx context.Context, in *profilePatchInput) (*profileGetOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}
		log := pkglogger.FromContext(ctx)

		current, err := loadProfileWithDefaults(ctx, queries, userID)
		if err != nil {
			log.Error("profile.patch: load failed", "err", err, "user_id", userID)
			return nil, huma.Error500InternalServerError("failed to load profile")
		}

		merged := current
		if in.Body.Units != nil {
			if *in.Body.Units != unitsMetric && *in.Body.Units != unitsImperial {
				return nil, huma.Error422UnprocessableEntity("units must be 'metric' or 'imperial'")
			}
			merged.Units = *in.Body.Units
		}
		if in.Body.Timezone != nil {
			merged.Timezone = *in.Body.Timezone
		}
		if in.Body.clearDisplayName {
			merged.DisplayName = ""
		} else if in.Body.DisplayName != nil {
			merged.DisplayName = *in.Body.DisplayName
		}

		var displayPtr *string
		if merged.DisplayName != "" {
			s := merged.DisplayName
			displayPtr = &s
		}
		if _, err := queries.UpsertUserProfile(ctx, dbgen.UpsertUserProfileParams{
			UserID:      userID,
			DisplayName: displayPtr,
			Timezone:    merged.Timezone,
			Units:       merged.Units,
		}); err != nil {
			log.Error("profile.patch: upsert failed", "err", err, "user_id", userID)
			return nil, huma.Error500InternalServerError("failed to save profile")
		}
		return &profileGetOutput{Body: merged}, nil
	})
}

// loadProfileWithDefaults returns the user's profile body, lazy-defaulting
// the row if no profile exists yet (no DB write — the row is created on
// the next PATCH). Mirrors the lazy-provision pattern in resolveUserID.
func loadProfileWithDefaults(ctx context.Context, queries *dbgen.Queries, userID uuid.UUID) (ProfileBody, error) {
	row, err := queries.GetUserProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProfileBody{
				DisplayName: "",
				Timezone:    defaultTimezoneStr,
				Units:       unitsMetric,
			}, nil
		}
		return ProfileBody{}, err
	}
	display := ""
	if row.DisplayName != nil {
		display = *row.DisplayName
	}
	return ProfileBody{
		DisplayName: display,
		Timezone:    row.Timezone,
		Units:       row.Units,
	}, nil
}
