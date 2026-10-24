package hscontrol

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/juanfont/headscale/hscontrol/keycloak"
	"github.com/juanfont/headscale/hscontrol/types"
	"github.com/juanfont/headscale/hscontrol/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"
)

const testKeycloakIssuer = "https://id.example.com/realms/main"

var errKeycloakDown = errors.New("keycloak unreachable")

type fakeChecker map[string]keycloak.Status

func (f fakeChecker) Check(_ context.Context, sub string) (keycloak.Status, error) {
	s, ok := f[sub]
	if !ok {
		return 0, errKeycloakDown
	}

	return s, nil
}

func TestKeycloakSubject(t *testing.T) {
	tests := []struct {
		issuer, identifier string
		want               string
		ok                 bool
	}{
		{testKeycloakIssuer, testKeycloakIssuer + "/e8081e49-c120", "e8081e49-c120", true},
		{testKeycloakIssuer + "/", testKeycloakIssuer + "/abc", "abc", true},
		{testKeycloakIssuer, "https://id.example.com/realms/staff/abc", "", false},
		{testKeycloakIssuer, "https://accounts.google.com/123", "", false},
		{testKeycloakIssuer, testKeycloakIssuer + "/a/b", "", false},
		{testKeycloakIssuer, testKeycloakIssuer, "", false},
	}

	for _, tt := range tests {
		got, ok := keycloakSubject(tt.issuer, tt.identifier)
		assert.Equal(t, tt.ok, ok, tt.identifier)
		assert.Equal(t, tt.want, got, tt.identifier)
	}
}

func TestReconcileKeycloak(t *testing.T) {
	app := createTestApp(t)
	app.cfg.OIDC.Issuer = testKeycloakIssuer

	oidcUser := func(name, sub string) *types.User {
		u, _, err := app.state.CreateUser(types.User{
			Name:               name,
			Provider:           util.RegisterMethodOIDC,
			ProviderIdentifier: sql.NullString{String: testKeycloakIssuer + "/" + sub, Valid: true},
		})
		require.NoError(t, err)

		return u
	}

	register := func(u *types.User, hostname string) types.NodeID {
		pak, err := app.state.CreatePreAuthKey(u.TypedID(), false, false, nil, nil)
		require.NoError(t, err)

		nodeKey := key.NewNode()
		resp, err := app.handleRegisterWithAuthKey(tailcfg.RegisterRequest{
			Auth:     &tailcfg.RegisterResponseAuth{AuthKey: pak.Key},
			NodeKey:  nodeKey.Public(),
			Hostinfo: &tailcfg.Hostinfo{Hostname: hostname},
		}, key.NewMachine().Public())
		require.NoError(t, err)
		require.True(t, resp.MachineAuthorized)

		var id types.NodeID

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			n, ok := app.state.GetNodeByNodeKey(nodeKey.Public())
			assert.True(c, ok)

			id = n.ID()
		}, time.Second, 50*time.Millisecond)

		return id
	}

	expired := func(id types.NodeID) bool {
		n, ok := app.state.GetNodeByID(id)
		require.True(t, ok)

		exp := n.Expiry()

		return exp.Valid() && !exp.Get().After(time.Now())
	}

	allowed := register(oidcUser("allowed", "sub-allowed"), "allowed-laptop")
	disabled := register(oidcUser("disabled", "sub-disabled"), "disabled-laptop")
	noRole := register(oidcUser("norole", "sub-norole"), "norole-laptop")
	gone1 := register(oidcUser("gone", "sub-gone"), "gone-laptop")
	unknown := register(oidcUser("unknown", "sub-unknown"), "unknown-laptop")
	local := register(app.state.CreateUserForTest("local"), "local-server")

	app.keycloak = fakeChecker{
		"sub-allowed":  keycloak.Allowed,
		"sub-disabled": keycloak.Disabled,
		"sub-norole":   keycloak.LacksRole,
		"sub-gone":     keycloak.Missing,
		// sub-unknown is absent: Check errors, as when Keycloak is down
	}

	assert.Equal(t, 3, app.reconcileKeycloak(context.Background()))

	assert.False(t, expired(allowed), "allowed user keeps access")
	assert.True(t, expired(disabled), "disabled user loses access")
	assert.True(t, expired(noRole), "user without the role loses access")
	assert.True(t, expired(gone1), "deleted user loses access")
	assert.False(t, expired(unknown), "unknown status must not revoke")
	assert.False(t, expired(local), "non-OIDC users are out of scope")

	assert.Equal(t, 0, app.reconcileKeycloak(context.Background()), "already-expired nodes are left alone")
}
