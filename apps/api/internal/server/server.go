// Package server constructs the HTTP server (chi router + Huma adapter)
// used by the API service. It does not own its dependencies — those are
// passed in by internal/system. Keep this file focused on wiring.
package server

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/swastikpatel7/cadence/apps/api/internal/auth"
	"github.com/swastikpatel7/cadence/apps/api/internal/config"
	"github.com/swastikpatel7/cadence/apps/api/internal/connections"
	"github.com/swastikpatel7/cadence/apps/api/internal/server/handlers"
	mw "github.com/swastikpatel7/cadence/apps/api/internal/server/middleware"
	dbgen "github.com/swastikpatel7/cadence/pkg/db/generated"
)

// Deps holds the typed dependencies a server needs. Passed in from
// internal/system; server.New does not construct them itself.
type Deps struct {
	Config        *config.Config
	Logger        *slog.Logger
	Queries       *dbgen.Queries
	Verifier      *auth.Verifier
	ReadyProbe    handlers.ReadyProbe
	WebhookSecret string
	Connections   *connections.Service
	WebBaseURL    string
}

// New builds the *http.Server. Caller is responsible for ListenAndServe
// and Shutdown.
func New(d Deps) (*http.Server, error) {
	r := chi.NewRouter()

	// Middleware order: outermost → innermost.
	// CORS sits on top so OPTIONS preflights short-circuit before auth or
	// any per-request bookkeeping. Browsers preflight any request that
	// carries `Authorization`, which is every authed call we make from the
	// web app on a different origin (localhost:3000 → :8080 in dev,
	// distinct Railway subdomains in prod).
	allowedOrigins := []string{d.Config.WebBaseURL}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Requested-With"},
		ExposedHeaders:   []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	// RequestID first so subsequent middleware can attach the ID.
	// Recover next so we catch panics in Logger and handlers.
	// Logger last so it sees the final status code.
	r.Use(mw.RequestID)
	r.Use(mw.Recover(d.Logger))
	r.Use(mw.Logger(d.Logger))

	api := humachi.New(r, huma.DefaultConfig("Cadence API", "0.1.0"))

	// Public health probes.
	handlers.RegisterHealth(api, d.ReadyProbe)

	// Public webhook (Svix-signature checked at the handler level).
	if err := handlers.RegisterClerkWebhook(api, d.Queries, d.WebhookSecret); err != nil {
		return nil, err
	}

	// Authed routes — Huma group with the Clerk JWT verifier middleware.
	// If the verifier wasn't constructed (e.g. dev without Clerk keys),
	// authed routes are not registered, and the API still starts.
	if d.Verifier != nil {
		authed := huma.NewGroup(api)
		authed.UseMiddleware(d.Verifier.HumaMiddleware(api))
		handlers.RegisterMe(authed, d.Queries)

		// Strava OAuth callback is registered on the public api (no Clerk
		// JWT — the OAuth state token is the auth) while start, disconnect,
		// and the /me/sync endpoints sit on the authed group.
		if d.Connections != nil {
			connections.Register(api, authed, connections.HandlerDeps{
				Service:    d.Connections,
				WebBaseURL: d.WebBaseURL,
			})
		}
	}

	addr := net.JoinHostPort("", strconv.Itoa(d.Config.PortAPI))
	return &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}, nil
}
