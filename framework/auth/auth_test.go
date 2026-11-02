package auth_test

import (
	"context"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
)

func TestGuards(t *testing.T) {
	t.Parallel()

	a := newApp(t, openDB(t), permissionsOf())
	user := ids.New()
	cookie := a.login(t, user)

	t.Run("refuse a request without credentials", func(t *testing.T) {
		t.Parallel()
		rec := a.send(t, http.MethodGet, "/any", request{})

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "unauthenticated", body(t, rec)["code"])
	})

	t.Run("let in the logged-in user of the session", func(t *testing.T) {
		t.Parallel()
		rec := a.send(t, http.MethodGet, "/any", request{cookie: cookie})

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, user.String(), body(t, rec)["id"])
		assert.Equal(t, "session", body(t, rec)["guard"])
		assert.Empty(t, body(t, rec)["tenant"])
	})

	t.Run("let in a bearer token and scope its tenant", func(t *testing.T) {
		t.Parallel()
		rec := a.send(t, http.MethodGet, "/any", bearer("tok_records:read"))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, tokenUser, body(t, rec)["id"])
		assert.Equal(t, "token", body(t, rec)["guard"])
		assert.Equal(t, "tenant-1", body(t, rec)["tenant"])
	})

	t.Run("prefer the bearer token over the session cookie", func(t *testing.T) {
		t.Parallel()
		rec := a.send(t, http.MethodGet, "/any", request{cookie: cookie, authorization: "Bearer app_1"})

		assert.Equal(t, appUser, body(t, rec)["id"])
	})

	t.Run("refuse bearer tokens on browser routes", func(t *testing.T) {
		t.Parallel()
		rec := a.send(t, http.MethodGet, "/browser", request{cookie: cookie, authorization: "Bearer app_1"})

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Equal(t, "session_required", body(t, rec)["code"])
		assert.Equal(t, http.StatusOK, a.send(t, http.MethodGet, "/browser", request{cookie: cookie}).Code)
	})

	t.Run("let first-party callers only onto account routes", func(t *testing.T) {
		t.Parallel()
		rec := a.send(t, http.MethodGet, "/account", bearer("tok_records:read"))

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Equal(t, "first_party_only", body(t, rec)["code"])
		assert.Equal(t, http.StatusOK, a.send(t, http.MethodGet, "/account", bearer("app_1")).Code)
		assert.Equal(t, http.StatusOK, a.send(t, http.MethodGet, "/account", request{cookie: cookie}).Code)
	})

	t.Run("refuse tokens nobody recognizes", func(t *testing.T) {
		t.Parallel()

		for _, header := range []string{"Bearer tok_invalid", "Bearer other_1", "Bearer  ", "bearer tok_invalid"} {
			rec := a.send(t, http.MethodGet, "/any", request{authorization: header})

			assert.Equal(t, http.StatusUnauthorized, rec.Code, header)
			assert.Equal(t, "invalid_token", body(t, rec)["code"], header)
		}
	})

	t.Run("ignore other authorization schemes", func(t *testing.T) {
		t.Parallel()
		rec := a.send(t, http.MethodGet, "/any", request{authorization: "Basic dXNlcjpwYXNz"})

		assert.Equal(t, "unauthenticated", body(t, rec)["code"])
	})
}

func TestAbilities(t *testing.T) {
	t.Parallel()

	a := newApp(t, openDB(t), permissionsOf())

	t.Run("let in a token holding the route's ability", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusOK, a.send(t, http.MethodGet, "/records", bearer("tok_records:read")).Code)
	})

	t.Run("name the ability a token lacks", func(t *testing.T) {
		t.Parallel()
		rec := a.send(t, http.MethodPost, "/records", bearer("tok_records:read"))

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Equal(t, "missing_ability", body(t, rec)["code"])
		assert.Equal(t, "records:write", body(t, rec)["required_ability"])
	})

	t.Run("let in a token holding every ability", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusOK, a.send(t, http.MethodPost, "/records", bearer("tok_*")).Code)
	})

	t.Run("never stop a session", func(t *testing.T) {
		t.Parallel()
		cookie := a.login(t, ids.New())

		assert.Equal(t, http.StatusOK, a.send(t, http.MethodPost, "/records", request{cookie: cookie}).Code)
	})

	t.Run("are copied out of an identity", func(t *testing.T) {
		t.Parallel()
		identity := &auth.Identity{Granted: []string{"records:read"}}

		identity.Abilities()[0] = "records:write"

		assert.Equal(t, []string{"records:read"}, identity.Granted)
		assert.True(t, auth.HasAbility(identity, "records:read"))
		assert.False(t, auth.HasAbility(identity, "records:write"))
	})
}

func TestGuardRegistry(t *testing.T) {
	t.Parallel()

	t.Run("refuses routes naming an unknown guard", func(t *testing.T) {
		t.Parallel()
		guards := auth.NewGuards(http.Errors{})

		_, err := guards.Middleware("missing")

		require.ErrorIs(t, err, auth.ErrUnknownGuard)
	})

	t.Run("refuses with the default fault when a set names none", func(t *testing.T) {
		t.Parallel()
		guards := auth.NewGuards(http.Errors{})
		guards.Register("token", auth.BearerGuard(auth.TokenResolverFunc(resolveTestToken)))
		guards.Register("token", auth.BearerGuard(auth.TokenResolverFunc(resolveTestToken)))
		guards.Define("nobody", nil)

		rec := serveGuarded(t, guards, "nobody", bearer("app_1"))

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Equal(t, "credential_not_accepted", body(t, rec)["code"])
		assert.Equal(t, http.StatusOK, serveGuarded(t, guards, "token", bearer("tok_x")).Code)
	})

	t.Run("combines guards so the first with a credential answers", func(t *testing.T) {
		t.Parallel()
		guards := auth.NewGuards(http.Errors{})
		none := auth.GuardFunc(func(*http.Request) (auth.Principal, error) { return nil, auth.ErrNoCredentials })
		guards.Register("token", auth.Any(none, auth.BearerGuard(auth.TokenResolverFunc(resolveTestToken))))

		assert.Equal(t, appUser, body(t, serveGuarded(t, guards, "", bearer("app_1")))["id"])
		assert.Equal(t, "unauthenticated", body(t, serveGuarded(t, guards, "", request{}))["code"])
	})

	t.Run("passes a guard's failure through", func(t *testing.T) {
		t.Parallel()
		guards := auth.NewGuards(http.Errors{})
		broken := errors.New("store down")
		guards.Register("broken", auth.GuardFunc(func(*http.Request) (auth.Principal, error) { return nil, broken }))

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", strings.NewReader(""))
		_, err := guards.Authenticate(req)

		require.ErrorIs(t, err, broken)
		assert.Equal(t, http.StatusInternalServerError, serveGuarded(t, guards, "", request{}).Code)
	})
}

/*
serveGuarded runs a request through the guards' middleware with the
parameter, outside any route.
*/
func serveGuarded(t *testing.T, guards *auth.Guards, param string, req request) *httptest.ResponseRecorder {
	t.Helper()

	var params []string
	if param != "" {
		params = []string{param}
	}

	mw, err := guards.Middleware(params...)
	require.NoError(t, err)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", strings.NewReader(""))
	r.Header.Set("Authorization", req.authorization)

	rec := httptest.NewRecorder()
	mw(whoami()).ServeHTTP(rec, r)

	return rec
}

func TestConfirmation(t *testing.T) {
	t.Parallel()

	a := newApp(t, openDB(t), permissionsOf())

	t.Run("lets in a caller who just logged in", func(t *testing.T) {
		t.Parallel()
		cookie := a.login(t, ids.New())

		assert.Equal(t, http.StatusOK, a.send(t, http.MethodPost, "/password", request{cookie: cookie}).Code)
	})

	t.Run("asks again once the window has passed, until confirmed", func(t *testing.T) {
		t.Parallel()
		b := newApp(t, openDB(t), permissionsOf())
		cookie := b.login(t, ids.New())
		b.clock.Travel(confirmWindow + time.Second)

		rec := b.send(t, http.MethodPost, "/password", request{cookie: cookie})

		assert.Equal(t, stdhttp.StatusLocked, rec.Code)
		assert.Equal(t, "password_confirmation_required", body(t, rec)["code"])
		b.send(t, http.MethodPost, "/confirm", request{cookie: cookie})
		assert.Equal(t, http.StatusOK, b.send(t, http.MethodPost, "/password", request{cookie: cookie}).Code)
	})

	t.Run("asks a token that cannot tell", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, stdhttp.StatusLocked, a.send(t, http.MethodPost, "/password", bearer("tok_*")).Code)
	})

	t.Run("refuses a request nobody authenticated", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusUnauthorized, a.send(t, http.MethodGet, "/guest", request{}).Code)
	})

	t.Run("counts the window's last moment as recent", func(t *testing.T) {
		t.Parallel()
		at := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
		p := &auth.Identity{Confirmed: at}

		assert.True(t, auth.Confirmed(p, at.Add(confirmWindow), confirmWindow))
		assert.False(t, auth.Confirmed(p, at.Add(confirmWindow+time.Nanosecond), confirmWindow))
		assert.False(t, auth.Confirmed(&auth.Identity{}, at, confirmWindow))
	})
}

/*
record is a resource the gate tests authorize against.
*/
type record struct {
	owner ids.UUID
}

func TestGate(t *testing.T) {
	t.Parallel()

	owner := &auth.Identity{User: ids.New(), Via: "session"}
	stranger := &auth.Identity{User: ids.New(), Via: "session"}
	gate := auth.NewGate(permissionsOf("records.update", "records.delete", "reports.view"), http.Errors{})
	auth.RegisterPolicy(gate, "records.update", func(_ context.Context, p auth.Principal, r *record) (bool, error) {
		return r.owner == p.ID(), nil
	})
	gate.Define("records.delete", func(context.Context, auth.Principal, any) (bool, error) {
		return false, fault.New(fault.NotFound, "record_not_found", "No such record.")
	})
	gate.Define("records.update", func(context.Context, auth.Principal, any) (bool, error) { return false, nil })

	t.Run("allows a held permission without a rule", func(t *testing.T) {
		t.Parallel()
		allowed, err := gate.Can(t.Context(), stranger, "reports.view", nil)

		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("refuses a permission the caller does not hold", func(t *testing.T) {
		t.Parallel()
		allowed, err := gate.Can(t.Context(), owner, "records.create", nil)

		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("asks the policy for the resource's type", func(t *testing.T) {
		t.Parallel()
		r := &record{owner: owner.User}

		allowed, err := gate.Can(t.Context(), owner, "records.update", r)
		require.NoError(t, err)
		assert.True(t, allowed)

		allowed, err = gate.Can(t.Context(), stranger, "records.update", r)
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("falls back to the permission's own rule", func(t *testing.T) {
		t.Parallel()
		allowed, err := gate.Can(t.Context(), owner, "records.update", "something else")

		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("authorizes the caller in the context", func(t *testing.T) {
		t.Parallel()
		ctx := http.WithPrincipal(t.Context(), owner)

		require.NoError(t, gate.Authorize(ctx, "records.update", &record{owner: owner.User}))
		require.ErrorIs(t, gate.Authorize(ctx, "records.create", nil), auth.ErrForbidden)
		assert.Equal(t, fault.NotFound, fault.KindOf(gate.Authorize(ctx, "records.delete", &record{})))
		require.ErrorIs(t, gate.Authorize(t.Context(), "reports.view", nil), auth.ErrUnauthenticated)
	})

	t.Run("surfaces a resolver failure", func(t *testing.T) {
		t.Parallel()
		broken := errors.New("roles unavailable")
		g := auth.NewGate(auth.PermissionsFunc(func(context.Context, auth.Principal) ([]string, error) {
			return nil, broken
		}), http.Errors{})

		require.ErrorIs(t, g.Authorize(http.WithPrincipal(t.Context(), owner), "reports.view", nil), broken)
	})

	t.Run("needs a permission to build its middleware", func(t *testing.T) {
		t.Parallel()
		_, err := gate.Middleware()

		require.ErrorIs(t, err, auth.ErrNoPermission)
	})
}

func TestPermissionMiddleware(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("lets in a caller holding the permission", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, permissionsOf("reports.view"))

		assert.Equal(t, http.StatusOK, a.send(t, http.MethodGet, "/reports", bearer("tok_*")).Code)
	})

	t.Run("refuses a caller without it", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, permissionsOf())
		rec := a.send(t, http.MethodGet, "/reports", bearer("tok_*"))

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Equal(t, "forbidden", body(t, rec)["code"])
	})
}
