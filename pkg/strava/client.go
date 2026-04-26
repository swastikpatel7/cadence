package strava

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/oauth2"
)

const (
	apiBase = "https://www.strava.com/api/v3"

	// Default 15-minute window. We snooze for slightly longer so we land
	// on the far side of the rolling reset.
	rateLimitWindow = 15 * time.Minute
)

// DefaultStreamKeys is the set we ask for on /streams. Order matches the
// Strava docs; key_by_type=true folds them into a map by type name.
var DefaultStreamKeys = []string{
	"time", "latlng", "distance", "altitude",
	"velocity_smooth", "heartrate", "cadence",
	"watts", "temp", "moving", "grade_smooth",
}

// RateLimitedError is returned when Strava signals 429. Callers (e.g.
// the worker job) inspect RetryAfter and snooze accordingly.
type RateLimitedError struct {
	StatusCode int
	RetryAfter time.Duration
	Headers    http.Header
}

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("strava: rate limited (retry after %v)", e.RetryAfter)
}

// IsRateLimited reports whether err (or any wrapped error) is a
// RateLimitedError.
func IsRateLimited(err error) (*RateLimitedError, bool) {
	var rl *RateLimitedError
	if errors.As(err, &rl) {
		return rl, true
	}
	return nil, false
}

// AuthError surfaces a non-recoverable 401 (e.g. user revoked access on
// Strava's end) so the worker can mark the connection failed.
type AuthError struct {
	StatusCode int
	Body       string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("strava: auth failed (%d): %s", e.StatusCode, e.Body)
}

// Client wraps a *http.Client preconfigured with Strava OAuth (via
// oauth2.NewClient) and the helpers we actually use.
type Client struct {
	httpClient *http.Client
	base       string
}

// NewClient builds a Strava API client driven by the supplied
// TokenSource. Use oauth2.NewClient for token-aware HTTP client
// construction; the result auto-injects the bearer header on every
// request and triggers refreshes as needed.
func NewClient(ctx context.Context, ts oauth2.TokenSource) *Client {
	return &Client{
		httpClient: oauth2.NewClient(ctx, ts),
		base:       apiBase,
	}
}

// ListAthleteActivities pages Strava's /athlete/activities endpoint.
//
// `after` (unix seconds, 0 = no lower bound) and `before` (unix seconds,
// 0 = no upper bound) constrain activity start_date. perPage caps at
// Strava's 200; page is 1-indexed.
//
// The intended pagination loop uses `after` as a fixed lower bound for
// the whole sync and walks `before` downward as the cursor advances —
// see SyncProgress doc for the full algorithm.
//
// Returns the raw JSON elements (one per activity) so the caller can
// both decode the projection AND persist the raw bytes for future
// enrichment without re-fetching.
func (c *Client) ListAthleteActivities(ctx context.Context, after, before int64, perPage, page int) ([]json.RawMessage, error) {
	q := url.Values{}
	if after > 0 {
		q.Set("after", strconv.FormatInt(after, 10))
	}
	if before > 0 {
		q.Set("before", strconv.FormatInt(before, 10))
	}
	if perPage <= 0 || perPage > 200 {
		perPage = 200
	}
	q.Set("per_page", strconv.Itoa(perPage))
	if page <= 0 {
		page = 1
	}
	q.Set("page", strconv.Itoa(page))

	var out []json.RawMessage
	if err := c.getJSON(ctx, "/athlete/activities?"+q.Encode(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetActivityDetail fetches the DetailedActivity for a single activity.
// Returns the raw JSON so the caller can persist it as-is into the
// activities.raw column AND decode the projected fields it needs.
func (c *Client) GetActivityDetail(ctx context.Context, activityID int64) (json.RawMessage, error) {
	return c.getRaw(ctx, fmt.Sprintf("/activities/%d", activityID))
}

// GetActivityStreams fetches the streams blob for a single activity.
// keys nil ⇒ DefaultStreamKeys. Always uses key_by_type=true so the
// response is a map { "time": {...}, "heartrate": {...}, … } which is
// directly usable in the activity_streams table.
func (c *Client) GetActivityStreams(ctx context.Context, activityID int64, keys []string) (json.RawMessage, error) {
	if len(keys) == 0 {
		keys = DefaultStreamKeys
	}
	q := url.Values{}
	q.Set("keys", joinComma(keys))
	q.Set("key_by_type", "true")
	return c.getRaw(ctx, fmt.Sprintf("/activities/%d/streams?%s", activityID, q.Encode()))
}

// getJSON does a GET and JSON-decodes into out.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	body, err := c.getRaw(ctx, path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// getRaw does a GET and returns the raw bytes. Surfaces 429 as a typed
// error and 401 as AuthError; all other 4xx/5xx come back as a generic
// error containing the status + body for log triage.
func (c *Client) getRaw(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, fmt.Errorf("strava: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("strava: do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("strava: read body: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, &RateLimitedError{
			StatusCode: resp.StatusCode,
			RetryAfter: parseRetryAfter(resp.Header),
			Headers:    resp.Header.Clone(),
		}
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, &AuthError{StatusCode: resp.StatusCode, Body: string(body)}
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf("strava: %s %s: %s", req.Method, path, resp.Status)
	}

	return body, nil
}

// parseRetryAfter inspects the response headers to compute when we can
// retry. Strava doesn't always emit Retry-After; falls back to the
// 15-minute window.
func parseRetryAfter(h http.Header) time.Duration {
	if ra := h.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	// Fallback: bucket resets at the next 15-min boundary, so worst case
	// wait is one full window.
	return rateLimitWindow + 30*time.Second
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ","
		}
		out += v
	}
	return out
}
