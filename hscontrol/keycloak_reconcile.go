package hscontrol

import (
	"context"
	"strings"
	"time"

	"github.com/juanfont/headscale/hscontrol/keycloak"
	"github.com/juanfont/headscale/hscontrol/types"
	"github.com/juanfont/headscale/hscontrol/util"
	"github.com/rs/zerolog/log"
)

// principalChecker answers whether an OIDC subject may keep tailnet access.
// [keycloak.Client] implements it; tests substitute a fake.
type principalChecker interface {
	Check(ctx context.Context, subject string) (keycloak.Status, error)
}

// keycloakSubject returns the Keycloak user id from a user's provider
// identifier, which [types.OIDCClaims.Identifier] builds as issuer + "/" + sub.
// It returns false for users from any other issuer.
func keycloakSubject(issuer, identifier string) (string, bool) {
	prefix := types.CleanIdentifier(strings.TrimSuffix(issuer, "/")) + "/"

	sub, ok := strings.CutPrefix(identifier, prefix)
	if !ok || sub == "" || strings.Contains(sub, "/") {
		return "", false
	}

	return sub, true
}

// reconcileKeycloak expires every node owned by an OIDC user whom Keycloak
// no longer allows (deleted, disabled, or without the required role). It
// returns the number of nodes expired.
//
// A user whose status cannot be determined is skipped: a Keycloak outage
// must not take every user off the tailnet. Expiry, not deletion, is used so
// that an admin can restore a node and a reinstated user can log in again.
// Tagged nodes have no owning user and are never touched.
func (h *Headscale) reconcileKeycloak(ctx context.Context) int {
	users, err := h.state.ListAllUsers()
	if err != nil {
		log.Error().Err(err).Msg("keycloak reconcile: listing users")

		return 0
	}

	expired := 0

	for _, user := range users {
		if user.Provider != util.RegisterMethodOIDC || !user.ProviderIdentifier.Valid {
			continue
		}

		sub, ok := keycloakSubject(h.cfg.OIDC.Issuer, user.ProviderIdentifier.String)
		if !ok {
			continue
		}

		status, err := h.keycloak.Check(ctx, sub)
		if err != nil {
			log.Warn().Err(err).Uint("user.id", user.ID).Msg("keycloak reconcile: status unknown, leaving nodes as they are")

			continue
		}

		if status == keycloak.Allowed {
			continue
		}

		now := time.Now()
		nodes := h.state.ListNodesByUser(types.UserID(user.ID))

		for i := range nodes.Len() {
			node := nodes.At(i)

			if exp := node.Expiry(); exp.Valid() && !exp.Get().After(now) {
				continue
			}

			_, c, err := h.state.SetNodeExpiry(node.ID(), &now)
			h.Change(c)

			if err != nil {
				log.Error().Err(err).Uint64("node.id", node.ID().Uint64()).Msg("keycloak reconcile: expiring node")

				continue
			}

			expired++

			log.Info().
				Uint64("node.id", node.ID().Uint64()).
				Str("node.name", node.Hostname()).
				Uint("user.id", user.ID).
				Str("keycloak.status", status.String()).
				Msg("keycloak reconcile: expired node")
		}
	}

	return expired
}
