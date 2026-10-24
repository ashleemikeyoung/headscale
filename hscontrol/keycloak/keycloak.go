// Package keycloak checks OIDC principals against a Keycloak realm's admin
// API, so that headscale can revoke tailnet access when Keycloak does.
//
// headscale checks oidc.allowed_groups only at login. Without this check a
// user who is disabled, deleted, or loses the role in Keycloak keeps every
// node they registered until the node expires.
package keycloak

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
)

// Status is the outcome of checking one principal.
type Status int

const (
	// Allowed means the user exists, is enabled, and holds the role.
	Allowed Status = iota
	// Missing means Keycloak has no user with this subject.
	Missing
	// Disabled means the user exists but is disabled.
	Disabled
	// LacksRole means the user does not hold the required realm role,
	// directly or through a composite role or group.
	LacksRole
)

func (s Status) String() string {
	switch s {
	case Allowed:
		return "allowed"
	case Missing:
		return "missing"
	case Disabled:
		return "disabled"
	case LacksRole:
		return "lacks-role"
	default:
		return fmt.Sprintf("Status(%d)", int(s))
	}
}

var (
	errIssuerNotKeycloak = errors.New("issuer is not a Keycloak realm URL (…/realms/<realm>)")
	errUnexpectedStatus  = errors.New("unexpected status from Keycloak")
	errInvalidSubject    = errors.New("invalid subject")
)

// tokenRefreshMargin renews the service-account token this long before it
// expires, so a request never carries a token that lapses in flight.
const tokenRefreshMargin = 30 * time.Second

// Client queries one realm's admin API with a service account (client
// credentials grant). The account needs the realm-management roles
// view-users and view-realm.
type Client struct {
	tokenURL     string
	adminURL     string
	clientID     string
	clientSecret string
	role         string
	http         *http.Client

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

// New returns a client for the realm named by issuer, for example
// https://id.example.com/realms/main. role is the realm role a principal
// must hold to keep access.
func New(issuer, clientID, clientSecret, role string, httpClient *http.Client) (*Client, error) {
	base, realm, err := SplitIssuer(issuer)
	if err != nil {
		return nil, err
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	return &Client{
		tokenURL:     base + "/realms/" + url.PathEscape(realm) + "/protocol/openid-connect/token",
		adminURL:     base + "/admin/realms/" + url.PathEscape(realm),
		clientID:     clientID,
		clientSecret: clientSecret,
		role:         role,
		http:         httpClient,
	}, nil
}

// SplitIssuer splits a Keycloak issuer URL into the server base URL
// (including any path prefix such as /auth) and the realm name.
func SplitIssuer(issuer string) (string, string, error) {
	u, err := url.Parse(strings.TrimSuffix(issuer, "/"))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", "", fmt.Errorf("%w: %q", errIssuerNotKeycloak, issuer)
	}

	prefix, realm, ok := strings.Cut(u.Path, "/realms/")
	if !ok || realm == "" || strings.Contains(realm, "/") {
		return "", "", fmt.Errorf("%w: %q", errIssuerNotKeycloak, issuer)
	}

	return u.Scheme + "://" + u.Host + prefix, realm, nil
}

// Check reports whether the user with the given subject (Keycloak user id)
// may keep tailnet access. An error means the answer is unknown, for
// example because Keycloak is unreachable; callers must not revoke on error.
func (c *Client) Check(ctx context.Context, subject string) (Status, error) {
	if subject == "" || strings.ContainsAny(subject, "/?#") {
		return 0, fmt.Errorf("%w: %q", errInvalidSubject, subject)
	}

	var user struct {
		Enabled bool `json:"enabled"`
	}

	found, err := c.get(ctx, "/users/"+url.PathEscape(subject), &user)
	if err != nil {
		return 0, err
	}

	if !found {
		return Missing, nil
	}

	if !user.Enabled {
		return Disabled, nil
	}

	var roles []struct {
		Name string `json:"name"`
	}

	hasRole := false

	// The composite view includes roles granted through groups and
	// composite roles, as the token's role claims do.
	found, err = c.get(ctx, "/users/"+url.PathEscape(subject)+"/role-mappings/realm/composite", &roles)
	if err != nil {
		return 0, err
	}

	if !found {
		return Missing, nil
	}

	for _, r := range roles {
		hasRole = hasRole || r.Name == c.role
	}

	if !hasRole {
		return LacksRole, nil
	}

	return Allowed, nil
}

// get fetches an admin API path into out. It returns false, with no error,
// when Keycloak answers 404.
func (c *Client) get(ctx context.Context, path string, out any) (bool, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.adminURL+path, nil)
	if err != nil {
		return false, fmt.Errorf("building Keycloak request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("querying Keycloak: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return false, nil
	case http.StatusUnauthorized:
		// The token may have been revoked early; fetch a new one next time.
		c.mu.Lock()
		c.token = ""
		c.mu.Unlock()

		return false, fmt.Errorf("%w: %d for %s", errUnexpectedStatus, resp.StatusCode, path)
	default:
		return false, fmt.Errorf("%w: %d for %s", errUnexpectedStatus, resp.StatusCode, path)
	}

	err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
	if err != nil {
		return false, fmt.Errorf("decoding Keycloak response for %s: %w", path, err)
	}

	return true, nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}

	form := url.Values{"grant_type": {"client_credentials"}}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("building Keycloak token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(url.QueryEscape(c.clientID), url.QueryEscape(c.clientSecret))

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting Keycloak token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: %d from token endpoint", errUnexpectedStatus, resp.StatusCode)
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&tok)
	if err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("%w: no access token in token response", errUnexpectedStatus)
	}

	c.token = tok.AccessToken
	c.tokenExp = time.Now().Add(time.Duration(tok.ExpiresIn)*time.Second - tokenRefreshMargin)

	return c.token, nil
}
