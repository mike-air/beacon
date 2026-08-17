package http

// The gate sets.
//
// Every operation names the protection it wants. That is the one real change
// this conversion forced on the shape of the service, and it is an
// improvement: reading `Middlewares: g.orgAdmin` tells you what guards a route
// without scrolling up through however many enclosing chi.Use() calls happened
// to be in scope.
//
// The sets are ordered, and the order is load-bearing:
//
//	auth       must precede everything, because it supplies the user id
//	rateLimit  must precede real work, so a throttled caller is cheap to refuse
//	idempotent must follow auth, because the claim is keyed by user id
//	locale     must follow auth, because it reads the user's stored preference
//	org        must follow auth, because membership is per user
//	role       must follow org, because a role only exists inside an org
//
// Getting that order wrong does not fail to compile. It fails at runtime, on
// the request where a nil user id reaches a query.

import (
	"github.com/danielgtaylor/huma/v2"

	"beacon/internal/orgs"
)

// gates holds the composed middleware chains, built once when routes are
// registered so every operation shares the same limiter buckets.
type gates struct {
	// public: unauthenticated, IP-limited. Signup and login only.
	public huma.Middlewares
	// authed: a valid token, inside the tenant's rate limit, idempotency
	// checked, locale resolved.
	authed huma.Middlewares
	// orgScoped: authed, plus proven membership of {orgID}.
	orgScoped huma.Middlewares
	// orgAdmin: orgScoped, plus a role of admin or owner.
	orgAdmin huma.Middlewares
}

func (s *Server) newGates(api huma.API) gates {
	authed := huma.Middlewares{
		s.humaRequireAuth(api),
		s.humaTenantRateLimit(api, s.cfg.TenantRateLimitRPS, s.cfg.TenantRateLimitBurst),
		s.humaIdempotency(),
		s.humaLocale(),
	}

	// append() on a shared slice is a trap: two derived chains can end up
	// writing into the same backing array and silently overwrite each other's
	// last element. Each set is built from its own copy.
	orgScoped := append(append(huma.Middlewares{}, authed...), s.humaRequireOrg(api))
	orgAdmin := append(append(huma.Middlewares{}, orgScoped...), s.humaRequireRole(api, orgs.RoleAdmin))

	return gates{
		public: huma.Middlewares{
			s.humaIPRateLimit(api, s.cfg.AuthRateLimitRPS, s.cfg.AuthRateLimitBurst),
		},
		authed:    authed,
		orgScoped: orgScoped,
		orgAdmin:  orgAdmin,
	}
}
