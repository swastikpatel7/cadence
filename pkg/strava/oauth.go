package strava

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// OAuth endpoint + canonical scope.
const (
	AuthURL  = "https://www.strava.com/oauth/authorize"
	TokenURL = "https://www.strava.com/oauth/token"

	// ScopeReadAll grants access to public + private activities, including
	// privacy-zone data. This is the only scope our v1 manual sync needs.
	ScopeReadAll = "activity:read_all"
)

// Endpoint is the oauth2.Endpoint Strava uses.
var Endpoint = oauth2.Endpoint{
	AuthURL:   AuthURL,
	TokenURL:  TokenURL,
	AuthStyle: oauth2.AuthStyleInParams,
}

// NewOAuthConfig builds the *oauth2.Config our handlers use to drive
// the Strava OAuth flow. RedirectURL must match the URL registered in
// the Strava app settings (Strava is strict about prefix match).
func NewOAuthConfig(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     Endpoint,
		RedirectURL:  redirectURL,
		Scopes:       []string{ScopeReadAll},
	}
}

// AuthCodeURL builds the authorize URL with the params we want by default:
// approval_prompt=force so the user can re-grant or trim scopes between
// connections, and `state` for CSRF protection.
func AuthCodeURL(c *oauth2.Config, state string) string {
	return c.AuthCodeURL(state, oauth2.SetAuthURLParam("approval_prompt", "force"))
}

// TokenResponse is the full response shape from POST /oauth/token. We
// roll the request manually (rather than using oauth2.Config.Exchange)
// so we can capture the embedded `athlete` blob with full JSON fidelity.
type TokenResponse struct {
	AccessToken  string          `json:"access_token"`
	RefreshToken string          `json:"refresh_token"`
	ExpiresAt    int64           `json:"expires_at"` // unix seconds
	ExpiresIn    int             `json:"expires_in"`
	TokenType    string          `json:"token_type"`
	Athlete      json.RawMessage `json:"athlete,omitempty"`
	Scope        string          `json:"scope,omitempty"`
}

// AsToken returns an oauth2.Token derived from the response. Used by
// downstream code that wants to drive an oauth2.TokenSource.
func (r *TokenResponse) AsToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		TokenType:    r.TokenType,
		Expiry:       time.Unix(r.ExpiresAt, 0),
	}
}

// ExchangeCode trades an authorization code for a token pair plus the
// athlete blob. httpClient may be nil — http.DefaultClient is used.
func ExchangeCode(ctx context.Context, httpClient *http.Client, c *oauth2.Config, code string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	return postForm(ctx, httpClient, form)
}

// RefreshToken trades a refresh token for a new pair (Strava rotates
// refresh tokens — store the returned refresh_token, not the old one).
func RefreshToken(ctx context.Context, httpClient *http.Client, c *oauth2.Config, refreshToken string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	return postForm(ctx, httpClient, form)
}

func postForm(ctx context.Context, httpClient *http.Client, form url.Values) (*TokenResponse, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("strava oauth: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("strava oauth: do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("strava oauth: read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("strava oauth: %s: %s", resp.Status, string(body))
	}

	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("strava oauth: parse response: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, errors.New("strava oauth: empty access_token")
	}
	return &tr, nil
}

// RotatingTokenSource wraps an oauth2.TokenSource and notifies the
// caller via OnRotate every time a new access token is issued by a
// refresh. The caller is expected to re-encrypt the new pair and
// persist them so subsequent runs pick up the rotated refresh token.
//
// Strava rotates refresh tokens on every refresh; without persisting
// the new one, subsequent refreshes break with "Bad Request".
type RotatingTokenSource struct {
	mu       sync.Mutex
	inner    oauth2.TokenSource
	last     *oauth2.Token
	onRotate func(context.Context, *oauth2.Token) error
	ctx      context.Context
}

// NewRotatingTokenSource wraps a non-reusable inner TokenSource (one
// produced by oauth2.Config.TokenSource(ctx, t)) so that newly-issued
// tokens flow through onRotate before being returned.
func NewRotatingTokenSource(
	ctx context.Context,
	inner oauth2.TokenSource,
	initial *oauth2.Token,
	onRotate func(context.Context, *oauth2.Token) error,
) *RotatingTokenSource {
	return &RotatingTokenSource{
		inner:    inner,
		last:     initial,
		onRotate: onRotate,
		ctx:      ctx,
	}
}

// Token returns the current access token, transparently triggering a
// refresh + persist when the cached token is past its expiry.
func (r *RotatingTokenSource) Token() (*oauth2.Token, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, err := r.inner.Token()
	if err != nil {
		return nil, err
	}
	if r.last == nil || t.AccessToken != r.last.AccessToken {
		if r.onRotate != nil {
			if err := r.onRotate(r.ctx, t); err != nil {
				return nil, fmt.Errorf("strava oauth: persist rotated token: %w", err)
			}
		}
		r.last = t
	}
	return t, nil
}
