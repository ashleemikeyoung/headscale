package keycloak

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitIssuer(t *testing.T) {
	tests := []struct {
		issuer    string
		wantBase  string
		wantRealm string
		wantErr   bool
	}{
		{issuer: "https://id.example.com/realms/main", wantBase: "https://id.example.com", wantRealm: "main"},
		{issuer: "https://id.example.com/realms/main/", wantBase: "https://id.example.com", wantRealm: "main"},
		{issuer: "https://kc.example.com/auth/realms/staff", wantBase: "https://kc.example.com/auth", wantRealm: "staff"},
		{issuer: "https://accounts.google.com", wantErr: true},
		{issuer: "https://id.example.com/realms/", wantErr: true},
		{issuer: "https://id.example.com/realms/main/extra", wantErr: true},
		{issuer: "ftp://id.example.com/realms/main", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.issuer, func(t *testing.T) {
			base, realm, err := SplitIssuer(tt.issuer)
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantBase, base)
			assert.Equal(t, tt.wantRealm, realm)
		})
	}
}

type fakeUser struct {
	enabled bool
	roles   []string
}

// fakeKeycloak serves the token endpoint and the two admin endpoints Check uses.
func fakeKeycloak(t *testing.T, users map[string]fakeUser) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var tokens atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("POST /realms/main/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		id, secret, ok := r.BasicAuth()
		if !ok || id != "headscale-sync" || secret != "s3cret" || r.FormValue("grant_type") != "client_credentials" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		tokens.Add(1)

		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 300})
	})

	auth := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer tok" {
				w.WriteHeader(http.StatusUnauthorized)

				return
			}

			next(w, r)
		}
	}

	mux.HandleFunc("GET /admin/realms/main/users/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		u, ok := users[r.PathValue("id")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{"id": r.PathValue("id"), "enabled": u.enabled})
	}))

	mux.HandleFunc("GET /admin/realms/main/users/{id}/role-mappings/realm/composite", auth(func(w http.ResponseWriter, r *http.Request) {
		u, ok := users[r.PathValue("id")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		roles := make([]map[string]string, 0, len(u.roles))
		for _, name := range u.roles {
			roles = append(roles, map[string]string{"name": name})
		}

		_ = json.NewEncoder(w).Encode(roles)
	}))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv, &tokens
}

func TestCheck(t *testing.T) {
	srv, tokens := fakeKeycloak(t, map[string]fakeUser{
		"u-ok":       {enabled: true, roles: []string{"default-roles-main", "tailnet:user"}},
		"u-disabled": {enabled: false, roles: []string{"tailnet:user"}},
		"u-norole":   {enabled: true, roles: []string{"default-roles-main", "erpnext:System Manager"}},
	})

	c, err := New(srv.URL+"/realms/main", "headscale-sync", "s3cret", "tailnet:user", srv.Client())
	require.NoError(t, err)

	tests := []struct {
		subject string
		want    Status
	}{
		{subject: "u-ok", want: Allowed},
		{subject: "u-disabled", want: Disabled},
		{subject: "u-norole", want: LacksRole},
		{subject: "u-gone", want: Missing},
	}

	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			got, err := c.Check(context.Background(), tt.subject)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	assert.Equal(t, int32(1), tokens.Load(), "the service-account token is cached across checks")
}

func TestCheckErrorsAreNotVerdicts(t *testing.T) {
	srv, _ := fakeKeycloak(t, map[string]fakeUser{"u-ok": {enabled: true, roles: []string{"tailnet:user"}}})

	// Wrong secret: the token request fails, so the status is unknown.
	c, err := New(srv.URL+"/realms/main", "headscale-sync", "wrong", "tailnet:user", srv.Client())
	require.NoError(t, err)

	_, err = c.Check(context.Background(), "u-ok")
	require.Error(t, err)

	// Keycloak down.
	down, err := New("http://127.0.0.1:1/realms/main", "headscale-sync", "s3cret", "tailnet:user", nil)
	require.NoError(t, err)

	_, err = down.Check(context.Background(), "u-ok")
	require.Error(t, err)
}

func TestCheckRejectsPathInSubject(t *testing.T) {
	srv, _ := fakeKeycloak(t, nil)

	c, err := New(srv.URL+"/realms/main", "headscale-sync", "s3cret", "tailnet:user", srv.Client())
	require.NoError(t, err)

	for _, sub := range []string{"", "a/../b", "x?y", "x#y"} {
		_, err := c.Check(context.Background(), sub)
		require.Error(t, err, sub)
	}
}
