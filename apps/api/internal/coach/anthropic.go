// Package coach owns the configured Anthropic SDK client used by all
// three Cadence model paths (baseline narrative, plan generation,
// micro-summary). It also defines the typed result envelope every
// worker persists to its respective `*_tokens` columns.
//
// The client is intentionally optional at boot — if ANTHROPIC_API_KEY
// is empty in dev the workers still register but log a clear warning
// when invoked (see Client.Available). This mirrors the graceful
// behavior used by `apps/api/internal/system/system.go` for the Strava
// env vars.
//
// SDK note: api.md §4 worker contracts use aspirational struct names
// (`anthropic.MessagesRequest`, top-level `Effort`, `Thinking{Type,
// Display}`, `OutputConfig{Format{Type,Schema}}`). The actual v1.38.0
// SDK uses:
//
//   - `anthropic.MessageNewParams{...}` → `client.Messages.New(ctx, p)`
//   - `Effort` lives under `OutputConfig.Effort` (typed
//     `anthropic.OutputConfigEffortXhigh` etc.).
//   - `Thinking` is a union (`ThinkingConfigParamUnion`) with three
//     variants: `OfEnabled`, `OfDisabled`, `OfAdaptive`. We use
//     `OfAdaptive{Display: "summarized"}` for adaptive runs.
//   - `cache_control: {type: "ephemeral"}` attaches to a `TextBlockParam`
//     via `CacheControl: anthropic.CacheControlEphemeralParam{}`. The
//     System block accepts `[]TextBlockParam`, so we attach it there.
//
// Note on JSON output: `OutputConfig.Format` (Anthropic structured
// outputs) is intentionally NOT used. Its schema validator rejects
// `minItems > 1`, `minimum`/`maximum` on number/integer types, and
// string-length bounds — half the constraints any non-trivial plan
// schema needs. Each generator instead embeds a literal JSON example
// in its system prompt and relies on Go-side post-parse validation
// (see `parsePlanBlob` / `parseNarrative`). The retry-on-parse-failure
// path in each generator covers transient strays. See
// docs/CHANGELOG.md 2026-04-30 for the migration record.
//
// Usage extracts:
//
//   - `Usage.InputTokens`, `Usage.OutputTokens`
//   - `Usage.CacheCreationInputTokens`, `Usage.CacheReadInputTokens`
//
// The SDK does not expose a discrete `thinking_tokens` field at v1.38;
// thinking tokens are billed inside `OutputTokens`. We persist 0 for
// `thinking_tokens` until the SDK surfaces it; the column is on the
// schema so we can backfill without a migration when it lands.
package coach

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Default model IDs. api.md / insights.md pin these names; the env
// overrides exist so ops can roll forward to a new minor without a
// deploy. Falling back to the defaults if env is unset keeps the spec
// canonical.
const (
	DefaultModelOpus   = "claude-opus-4-7"
	DefaultModelSonnet = "claude-sonnet-4-6"
	DefaultModelHaiku  = "claude-haiku-4-5"
)

// Client is the shared Anthropic client wrapper used by all coach
// workers. Concurrent-safe by virtue of the underlying SDK client.
type Client struct {
	api         *anthropic.Client
	logger      *slog.Logger
	modelOpus   anthropic.Model
	modelSonnet anthropic.Model
	modelHaiku  anthropic.Model
}

// Config carries the optional SDK inputs.
type Config struct {
	APIKey      string
	Logger      *slog.Logger
	ModelOpus   string
	ModelSonnet string
	ModelHaiku  string
}

// New constructs a Client. If APIKey is empty the returned Client has
// `Available() == false` and every Send call fails with ErrNotConfigured.
// This lets the caller register workers unconditionally without
// crashing on missing env in dev.
func New(cfg Config) *Client {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}

	c := &Client{
		logger:      log,
		modelOpus:   modelOrDefault(cfg.ModelOpus, DefaultModelOpus),
		modelSonnet: modelOrDefault(cfg.ModelSonnet, DefaultModelSonnet),
		modelHaiku:  modelOrDefault(cfg.ModelHaiku, DefaultModelHaiku),
	}

	if cfg.APIKey == "" {
		log.Warn("anthropic API key not configured; coach workers will no-op when invoked (dev only)")
		return c
	}

	cli := anthropic.NewClient(option.WithAPIKey(cfg.APIKey))
	c.api = &cli
	return c
}

func modelOrDefault(override, fallback string) anthropic.Model {
	if override != "" {
		return anthropic.Model(override)
	}
	return anthropic.Model(fallback)
}

// Available reports whether the SDK is wired (API key was provided).
// Workers should branch on this and short-circuit (with a clear log
// line + River JobSnooze on the next attempt) when false.
func (c *Client) Available() bool { return c != nil && c.api != nil }

// Models returns the three configured model IDs so workers don't
// re-derive them per call.
func (c *Client) Models() (opus, sonnet, haiku anthropic.Model) {
	return c.modelOpus, c.modelSonnet, c.modelHaiku
}

// ErrNotConfigured is returned by Send when the client was constructed
// without an API key. Workers translate this into a JobSnooze on first
// invocation so the queue eventually drains once an op deploys with
// the key set.
var ErrNotConfigured = errors.New("coach: anthropic API key not configured")

// Result wraps a Messages.New response with the typed token counters
// every worker persists. The SDK exposes thinking tokens implicitly via
// OutputTokens; we surface 0 until a discrete field is added (see
// package doc-comment).
type Result struct {
	// FirstText is the concatenation of every text content block in the
	// response, in order. Workers that asked for JSON-schema output
	// parse this string.
	FirstText string
	// Model is the response.Model field, mirroring the pinned model id
	// for ops dashboards.
	Model string
	// StopReason mirrors the SDK Message.StopReason ("end_turn" /
	// "max_tokens" / etc.). Workers that need to retry on truncation
	// switch on this.
	StopReason string
	// Token counters. The SDK does not expose a discrete thinking-token
	// counter at v1.38; ThinkingTokens is always 0 for now and the
	// schema column is reserved.
	InputTokens              int32
	OutputTokens             int32
	CacheReadInputTokens     int32
	CacheCreationInputTokens int32
	ThinkingTokens           int32
}

// Send wraps the SDK Messages.New call, returning the structured Result
// above. Returns ErrNotConfigured if the client was constructed without
// an API key.
func (c *Client) Send(ctx context.Context, params anthropic.MessageNewParams) (*Result, error) {
	if !c.Available() {
		return nil, ErrNotConfigured
	}
	resp, err := c.api.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("coach: messages.new: %w", err)
	}

	out := &Result{
		Model:                    string(resp.Model),
		StopReason:               string(resp.StopReason),
		InputTokens:              int32(resp.Usage.InputTokens),
		OutputTokens:             int32(resp.Usage.OutputTokens),
		CacheReadInputTokens:     int32(resp.Usage.CacheReadInputTokens),
		CacheCreationInputTokens: int32(resp.Usage.CacheCreationInputTokens),
	}

	for _, block := range resp.Content {
		if block.Type == "text" {
			out.FirstText += block.Text
		}
	}
	return out, nil
}

// IsTerminal reports whether err is a non-retryable Anthropic API error.
// We mark 4xx responses (except 408 Request Timeout, 425 Too Early, and
// 429 Too Many Requests) as terminal — those are client-side bugs that
// retrying won't fix and that previously caused River's default policy
// to thrash for ~16 hours, eventually surfacing as a Clerk-token-expiry
// 401 in the onboarding UI. Anthropic 5xx and the three transient 4xx
// codes stay retryable.
//
// Returns false on nil and on non-Anthropic errors so callers can
// branch unconditionally.
func IsTerminal(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.StatusCode {
	case 408, 425, 429:
		return false
	}
	return apiErr.StatusCode >= 400 && apiErr.StatusCode < 500
}

// SystemBlockWithCache builds the system-prompt slice every worker
// uses: a single TextBlockParam carrying the prompt with an ephemeral
// cache breakpoint attached. Concurrent users hitting the same prompt
// land on the cached entry (insights.md §8.1).
func SystemBlockWithCache(prompt string) []anthropic.TextBlockParam {
	return []anthropic.TextBlockParam{
		{
			Text:         prompt,
			CacheControl: anthropic.CacheControlEphemeralParam{},
		},
	}
}

// AdaptiveThinkingSummarized is the standard "thinking: {type:adaptive,
// display:summarized}" config used by the Opus and Sonnet workers.
func AdaptiveThinkingSummarized() anthropic.ThinkingConfigParamUnion {
	return anthropic.ThinkingConfigParamUnion{
		OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{
			Display: anthropic.ThinkingConfigAdaptiveDisplaySummarized,
		},
	}
}

// AdaptiveThinkingOmitted is used when we don't want the response to
// contain the thinking summary (smaller token bill on the response).
func AdaptiveThinkingOmitted() anthropic.ThinkingConfigParamUnion {
	return anthropic.ThinkingConfigParamUnion{
		OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{
			Display: anthropic.ThinkingConfigAdaptiveDisplayOmitted,
		},
	}
}
