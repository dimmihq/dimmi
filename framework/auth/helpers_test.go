package auth_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/session"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
sessionRequired is the refusal for a bearer caller on a browser route.
*/
func sessionRequired() *fault.Error {
	return fault.New(fault.Forbidden, "session_required", "Open the dashboard to continue.")
}

/*
firstPartyOnly is the refusal for a token on an account route.
*/
func firstPartyOnly() *fault.Error {
	return fault.New(fault.Forbidden, "first_party_only", "Only first-party apps may call this.")
}

/*
confirmWindow is how recently a caller must have proved who they are
for the sensitive route.
*/
const confirmWindow = 15 * time.Minute

/*
The users the test token resolver acts for.
*/
const (
	tokenUser = "019a4a40-0000-7000-8000-000000000001"
	appUser   = "019a4a40-0000-7000-8000-000000000002"
)

func mustID(s string) ids.UUID {
	id, _ := ids.Parse(s)

	return id
}

/*
openDB returns a small pool on a fresh database migrated with the
sessions table.
*/
func openDB(t *testing.T) *database.DB {
	t.Helper()

	url := testkit.NewDatabase(t).URL
	_, err := migrate.Runner{URL: url, Framework: session.Migrations()}.Up(t.Context())
	require.NoError(t, err)

	db, err := database.Open(t.Context(), database.Config{URL: url, MaxConns: 2, PoolMode: database.PoolModeSession})
	require.NoError(t, err)
	t.Cleanup(db.Close)

	return db
}

/*
resolveTestToken resolves the tokens of the test application: "tok_"
tokens act as a third party bound to a tenant with the abilities after
the prefix, "app_" tokens as the first-party app. Other prefixes are not
its kind.
*/
func resolveTestToken(_ *http.Request, token string) (auth.Principal, error) {
	switch {
	case token == "tok_invalid":
		return nil, auth.ErrInvalidToken
	case strings.HasPrefix(token, "tok_"):
		abilities := strings.Split(strings.TrimPrefix(token, "tok_"), "+")

		return &auth.Identity{User: mustID(tokenUser), Via: "token", TenantID: "tenant-1", Granted: abilities}, nil
	case strings.HasPrefix(token, "app_"):
		return &auth.Identity{User: mustID(appUser), Via: "app", Granted: []string{auth.AllAbilities}}, nil
	default:
		return nil, auth.ErrNoCredentials
	}
}

/*
app is the test application: its router, session manager and clock.
*/
type app struct {
	manager *session.Manager
	clock   *clock.Fake
	routes  *routing.Router
	handler http.Handler
}

func newApp(t *testing.T, db *database.DB, permissions auth.PermissionResolver) *app {
	t.Helper()

	clk := clock.NewFake(time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC))
	manager := session.NewManager(session.NewPostgresStore(db, ""), clk, session.Config{})

	guards := auth.NewGuards(http.Errors{})
	guards.Register("token", auth.BearerGuard(auth.TokenResolverFunc(resolveTestToken)))
	guards.Register("session", auth.SessionGuard("session"))
	guards.Define("session", sessionRequired(), "session")
	guards.Define("user", firstPartyOnly(), "session", "app")

	gate := auth.NewGate(permissions, http.Errors{})

	kernel := http.NewKernel()
	kernel.Alias("session", func(...string) (http.Middleware, error) { return manager.Middleware, nil })
	kernel.Alias("auth", guards.Middleware)
	kernel.Alias("can", gate.Middleware)

	confirmed := auth.RequireConfirmation(clk, confirmWindow, http.Errors{})

	r := routing.New(routing.Options{Middleware: kernel})
	r.Group("", func(g *routing.Router) {
		g.Middleware("session")
		g.Handle(http.MethodPost, "/login", "login", login(t))
		g.Handle(http.MethodPost, "/confirm", "confirm", confirm())
		g.Handle(http.MethodGet, "/guest", "guest", whoami()).Use(confirmed)
		g.Handle(http.MethodGet, "/any", "any", whoami()).Middleware("auth")
		g.Handle(http.MethodGet, "/browser", "browser", whoami()).Middleware("auth:session")
		g.Handle(http.MethodGet, "/account", "account", whoami()).Middleware("auth:user")
		g.Handle(http.MethodGet, "/records", "records.index", whoami()).Middleware("auth").Ability("records:read")
		g.Handle(http.MethodPost, "/records", "records.store", whoami()).Middleware("auth").Ability("records:write")
		g.Handle(http.MethodGet, "/reports", "reports", whoami()).Middleware("auth", "can:reports.view")
		g.Handle(http.MethodPost, "/password", "password", whoami()).Middleware("auth").Use(confirmed)
	})

	h, err := r.Handler()
	require.NoError(t, err)

	return &app{manager: manager, clock: clk, routes: r, handler: h}
}

/*
caller is what the whoami route answers with.
*/
type caller struct {
	ID     string `json:"id"`
	Guard  string `json:"guard"`
	Tenant string `json:"tenant"`
}

/*
whoami answers with the principal in the context and the tenant the
database would be scoped to.
*/
func whoami() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.From(r.Context())
		if !ok {
			http.WriteJSON(w, http.StatusOK, caller{})

			return
		}

		tenant, _ := tenancy.From(r.Context())
		http.WriteJSON(w, http.StatusOK, caller{ID: p.ID().String(), Guard: p.Guard(), Tenant: tenant})
	})
}

func login(t *testing.T) http.Handler {
	t.Helper()

	return http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		s, ok := session.From(r.Context())
		require.True(t, ok)

		id, err := ids.Parse(r.URL.Query().Get("user"))
		require.NoError(t, err)
		s.Login(id)
	})
}

func confirm() http.Handler {
	return http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if s, ok := session.From(r.Context()); ok {
			s.Confirm()
		}
	})
}

/*
request describes a test request: a session cookie and an Authorization
header, either of which may be empty.
*/
type request struct {
	cookie        *http.Cookie
	authorization string
}

func (a *app) send(t *testing.T, method, target string, req request) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(""))
	if req.cookie != nil {
		r.AddCookie(req.cookie)
	}

	if req.authorization != "" {
		r.Header.Set("Authorization", req.authorization)
	}

	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, r)

	return rec
}

/*
login logs the user in and returns the session cookie.
*/
func (a *app) login(t *testing.T, user ids.UUID) *http.Cookie {
	t.Helper()

	rec := a.send(t, http.MethodPost, "/login?user="+user.String(), request{})
	for _, c := range rec.Result().Cookies() {
		if c.Name == a.manager.CookieName() {
			return c
		}
	}

	t.Fatal("login set no session cookie")

	return nil
}

/*
body decodes a JSON response into a map.
*/
func body(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	return out
}

/*
permissionsOf returns a resolver that grants every caller the
permissions.
*/
func permissionsOf(permissions ...string) auth.PermissionResolver {
	return auth.PermissionsFunc(func(context.Context, auth.Principal) ([]string, error) {
		return permissions, nil
	})
}

func bearer(token string) request {
	return request{authorization: "Bearer " + token}
}
