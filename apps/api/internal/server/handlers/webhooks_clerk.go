package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	svix "github.com/svix/svix-webhooks/go"

	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
	pkglogger "github.com/swastikpatel7/cadence/pkg/logger"
)

// Clerk webhook payload shapes. We only model what we use.
type clerkWebhookPayload struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type clerkUserData struct {
	ID                    string              `json:"id"`
	EmailAddresses        []clerkEmailAddress `json:"email_addresses"`
	PrimaryEmailAddressID string              `json:"primary_email_address_id"`
}

type clerkEmailAddress struct {
	ID           string `json:"id"`
	EmailAddress string `json:"email_address"`
}

// primaryEmail returns the primary email if matchable, else the first
// available email, else "".
func (d clerkUserData) primaryEmail() string {
	for _, e := range d.EmailAddresses {
		if e.ID == d.PrimaryEmailAddressID {
			return e.EmailAddress
		}
	}
	if len(d.EmailAddresses) > 0 {
		return d.EmailAddresses[0].EmailAddress
	}
	return ""
}

// Huma input/output for the webhook handler.
// RawBody gives us the unparsed bytes for Svix HMAC verification.
type clerkWebhookInput struct {
	RawBody       []byte
	SvixID        string `header:"svix-id" required:"true"`
	SvixTimestamp string `header:"svix-timestamp" required:"true"`
	SvixSignature string `header:"svix-signature" required:"true"`
}

type clerkWebhookOutput struct {
	Body struct {
		Status string `json:"status"`
	}
}

// RegisterClerkWebhook wires POST /v1/webhooks/clerk. If webhookSecret
// is empty, the webhook is not registered (useful for dev without
// Clerk keys yet).
func RegisterClerkWebhook(api huma.API, queries *dbgen.Queries, webhookSecret string) error {
	if webhookSecret == "" {
		return nil
	}

	wh, err := svix.NewWebhook(webhookSecret)
	if err != nil {
		return fmt.Errorf("init svix webhook: %w", err)
	}

	huma.Register(api, huma.Operation{
		OperationID: "clerk-webhook",
		Method:      http.MethodPost,
		Path:        "/v1/webhooks/clerk",
		Summary:     "Receive signed Clerk webhook events",
		Description: "Endpoint registered with Clerk's webhook delivery. " +
			"Verifies Svix-Signature and handles user.created / user.updated / user.deleted.",
		Tags: []string{"webhooks"},
	}, func(ctx context.Context, in *clerkWebhookInput) (*clerkWebhookOutput, error) {
		log := pkglogger.FromContext(ctx)

		headers := http.Header{}
		headers.Set("svix-id", in.SvixID)
		headers.Set("svix-timestamp", in.SvixTimestamp)
		headers.Set("svix-signature", in.SvixSignature)
		if err := wh.Verify(in.RawBody, headers); err != nil {
			log.Warn("clerk webhook signature invalid", "err", err)
			return nil, huma.Error401Unauthorized("invalid signature")
		}

		var payload clerkWebhookPayload
		if err := json.Unmarshal(in.RawBody, &payload); err != nil {
			return nil, huma.Error400BadRequest("invalid payload", err)
		}

		switch payload.Type {
		case "user.created", "user.updated":
			if err := upsertClerkUser(ctx, queries, payload.Data); err != nil {
				log.Error("upsert clerk user failed", "err", err, "type", payload.Type)
				return nil, huma.Error500InternalServerError("upsert failed", err)
			}
		case "user.deleted":
			if err := deleteClerkUser(ctx, queries, payload.Data); err != nil {
				log.Error("delete clerk user failed", "err", err)
				return nil, huma.Error500InternalServerError("delete failed", err)
			}
		default:
			// Unknown event type; ack and ignore.
			log.Debug("clerk webhook event ignored", "type", payload.Type)
		}

		out := &clerkWebhookOutput{}
		out.Body.Status = "ok"
		return out, nil
	})

	return nil
}

func upsertClerkUser(ctx context.Context, queries *dbgen.Queries, data json.RawMessage) error {
	var u clerkUserData
	if err := json.Unmarshal(data, &u); err != nil {
		return fmt.Errorf("unmarshal user data: %w", err)
	}
	if u.ID == "" {
		return errors.New("user id missing")
	}
	email := u.primaryEmail()
	if email == "" {
		return errors.New("user has no email")
	}
	_, err := queries.UpsertUserByClerkID(ctx, dbgen.UpsertUserByClerkIDParams{
		ClerkUserID: u.ID,
		Email:       email,
	})
	return err
}

func deleteClerkUser(ctx context.Context, queries *dbgen.Queries, data json.RawMessage) error {
	var u clerkUserData
	if err := json.Unmarshal(data, &u); err != nil {
		return fmt.Errorf("unmarshal user data: %w", err)
	}
	if u.ID == "" {
		return errors.New("user id missing")
	}
	return queries.DeleteUserByClerkID(ctx, u.ID)
}
