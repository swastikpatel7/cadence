package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// HealthBody is the response shape for /livez and /readyz.
type HealthBody struct {
	Status string `json:"status" example:"ok" doc:"Probe status"`
}

type healthOutput struct {
	Body HealthBody
}

// RegisterHealth wires /livez (liveness) and /readyz (readiness) onto api.
//
// /livez returns 200 as long as the process responds. Used by the platform
// to know whether to restart the container.
//
// /readyz returns 200 only if all dependencies (DB) are reachable. The
// probe is injected by internal/system.
func RegisterHealth(api huma.API, ready ReadyProbe) {
	huma.Register(api, huma.Operation{
		OperationID: "livez",
		Method:      http.MethodGet,
		Path:        "/livez",
		Summary:     "Liveness probe",
		Tags:        []string{"health"},
	}, func(_ context.Context, _ *struct{}) (*healthOutput, error) {
		return &healthOutput{Body: HealthBody{Status: "ok"}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "readyz",
		Method:      http.MethodGet,
		Path:        "/readyz",
		Summary:     "Readiness probe",
		Tags:        []string{"health"},
	}, func(ctx context.Context, _ *struct{}) (*healthOutput, error) {
		if ready != nil {
			if err := ready(ctx); err != nil {
				return nil, huma.Error503ServiceUnavailable("not ready", err)
			}
		}
		return &healthOutput{Body: HealthBody{Status: "ok"}}, nil
	})
}

// ReadyProbe is invoked on every /readyz request. Return non-nil to
// signal the service should not receive traffic.
type ReadyProbe func(ctx context.Context) error
