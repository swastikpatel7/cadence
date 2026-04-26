package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"github.com/swastikpatel7/cadence/apps/api/internal/auth"
	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
)

// MeBody is the response body for GET /v1/me.
type MeBody struct {
	ID          string `json:"id" example:"01975c6b-2efb-7e10-9bf8-6a1e0a1f4b2c" doc:"Internal user UUID"`
	ClerkUserID string `json:"clerk_user_id" example:"user_2abc..." doc:"Clerk's user identifier"`
	Email       string `json:"email" example:"a@b.com" doc:"Primary email"`
}

type meOutput struct {
	Body MeBody
}

// RegisterMe wires GET /v1/me. Caller is responsible for ensuring this
// route is mounted on a Huma group with auth middleware applied.
func RegisterMe(api huma.API, queries *dbgen.Queries) {
	huma.Register(api, huma.Operation{
		OperationID: "get-me",
		Method:      http.MethodGet,
		Path:        "/v1/me",
		Summary:     "Get the authenticated user",
		Tags:        []string{"users"},
	}, func(ctx context.Context, _ *struct{}) (*meOutput, error) {
		userID, ok := auth.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("not authenticated")
		}
		user, err := queries.GetUserByID(ctx, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, huma.Error404NotFound("user not found")
			}
			pkglogger.FromContext(ctx).Error("get user by id failed", "err", err, "user_id", userID)
			return nil, huma.Error500InternalServerError("failed to load user")
		}
		return &meOutput{
			Body: MeBody{
				ID:          user.ID.String(),
				ClerkUserID: user.ClerkUserID,
				Email:       user.Email,
			},
		}, nil
	})
}
