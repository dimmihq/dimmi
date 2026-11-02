package session_test

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/session"
	"github.com/dimmihq/dimmi/framework/testkit"
)

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
app is a router whose routes use the session, with the store and clock
behind it.
*/
type app struct {
	manager *session.Manager
	store   *session.PostgresStore
	clock   *clock.Fake
	handler http.Handler
}

func newApp(t *testing.T, db *database.DB, cfg session.Config) *app {
	t.Helper()

	clk := clock.NewFake(time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC))
	store := session.NewPostgresStore(db, "")
	manager := session.NewManager(store, clk, cfg)

	kernel := http.NewKernel()
	kernel.Alias("session", func(...string) (http.Middleware, error) { return manager.Middleware, nil })

	r := routing.New(routing.Options{Middleware: kernel})
	r.Group("", func(g *routing.Router) {
		g.Middleware("session")

		for path, fn := range routes(t) {
			method, path, _ := strings.Cut(path, " ")
			g.Handle(method, path, path, sessionHandler(fn))
		}
	})

	h, err := r.Handler()
	require.NoError(t, err)

	return &app{manager: manager, store: store, clock: clk, handler: h}
}

/*
handlerFunc is a route handler that receives the request's session.
*/
type handlerFunc func(w http.ResponseWriter, r *http.Request, s *session.Session)

/*
routes returns the test routes by method and path.
*/
func routes(t *testing.T) map[string]handlerFunc {
	t.Helper()

	return map[string]handlerFunc{
		"POST /put": func(w http.ResponseWriter, r *http.Request, s *session.Session) {
			s.Put(r.URL.Query().Get("key"), r.URL.Query().Get("value"))
			w.WriteHeader(http.StatusNoContent)
		},
		"POST /forget": func(_ http.ResponseWriter, r *http.Request, s *session.Session) {
			s.Forget(r.URL.Query().Get("key"))
		},
		"GET /get": func(w http.ResponseWriter, r *http.Request, s *session.Session) {
			value, _ := s.Get(r.URL.Query().Get("key"))
			_, _ = w.Write([]byte(value))
		},
		"POST /login": func(w http.ResponseWriter, r *http.Request, s *session.Session) {
			id, err := ids.Parse(r.URL.Query().Get("user"))
			require.NoError(t, err)
			s.Login(id)
			_, _ = w.Write([]byte(s.ID().String()))
		},
		"GET /me": func(w http.ResponseWriter, _ *http.Request, s *session.Session) {
			id, ok := s.UserID()
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)

				return
			}

			_, _ = w.Write([]byte(id.String() + " " + s.ID().String()))
		},
		"POST /flash": func(_ http.ResponseWriter, _ *http.Request, s *session.Session) {
			s.Flash("notice", "saved")
		},
		"GET /flashed": func(w http.ResponseWriter, _ *http.Request, s *session.Session) {
			value, _ := s.Flashed("notice")
			_, _ = w.Write([]byte(value))
		},
		"POST /confirm":    func(_ http.ResponseWriter, _ *http.Request, s *session.Session) { s.Confirm() },
		"POST /regenerate": func(_ http.ResponseWriter, _ *http.Request, s *session.Session) { s.Regenerate() },
		"POST /logout":     func(_ http.ResponseWriter, _ *http.Request, s *session.Session) { s.Destroy() },
	}
}

/*
sessionHandler adapts a function that receives the request's session.
*/
func sessionHandler(fn handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := session.From(r.Context())
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		fn(w, r, s)
	})
}

/*
send serves a request carrying the cookie, when one is given, and
returns the recorded response.
*/
func (a *app) send(t *testing.T, method, target string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(""))
	req.Header.Set("User-Agent", "test-agent")

	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)

	return rec
}

/*
sessionCookie returns the session cookie the response set, or nil.
*/
func (a *app) sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == a.manager.CookieName() {
			return c
		}
	}

	return nil
}

/*
login logs the user in and returns the session cookie.
*/
func (a *app) login(t *testing.T, user ids.UUID) *http.Cookie {
	t.Helper()

	rec := a.send(t, http.MethodPost, "/login?user="+user.String(), nil)
	require.Equal(t, http.StatusOK, rec.Code)

	cookie := a.sessionCookie(rec)
	require.NotNil(t, cookie)

	return cookie
}

func newRequest(t *testing.T) *http.Request {
	t.Helper()

	return httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", strings.NewReader(""))
}
