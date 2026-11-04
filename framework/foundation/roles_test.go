package foundation_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/jwt"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
tickArgs is a periodic job.
*/
type tickArgs struct{}

func (tickArgs) Kind() string { return "tick" }

/*
tickWorker reports every tick it ran.
*/
type tickWorker struct {
	ticks chan struct{}
}

func (w tickWorker) Handle(context.Context, *queue.Job[tickArgs]) error {
	w.ticks <- struct{}{}

	return nil
}

func TestRoles(t *testing.T) {
	t.Parallel()

	t.Run("pushes periodic jobs from the scheduler for workers to run", func(t *testing.T) {
		t.Parallel()
		ticks := make(chan struct{}, 10)
		env := environ(t)
		schedule := provider{
			register: func(a *foundation.Application) error {
				return queue.Register(a.Queue, tickWorker{ticks: ticks})
			},
			boot: func(_ context.Context, a *foundation.Application) error {
				a.Schedule.Job(tickArgs{}).EveryMinute().RunOnStart()

				return nil
			},
		}
		scheduler := newApp(t, options(append(env, "APP_APP__ROLES=scheduler"), schedule))
		worker := newApp(t, options(append(env, "APP_APP__ROLES=worker"), schedule))

		require.NoError(t, scheduler.Start(t.Context()))
		require.NoError(t, worker.Start(t.Context()))

		require.Eventually(t, func() bool { return len(ticks) > 0 }, 20*time.Second, pollEvery)

		assert.Empty(t, scheduler.Addr())
		assert.Empty(t, worker.AdminAddr())
	})

	t.Run("serves the web role without an admin listener when it is off", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t, "APP_APP__ROLES=web", "APP_HTTP__ADMIN_ADDR=")))

		require.NoError(t, app.Start(t.Context()))

		assert.NotEmpty(t, app.Addr())
		assert.Empty(t, app.AdminAddr())
	})

	t.Run("refuses a schedule it cannot apply", func(t *testing.T) {
		t.Parallel()
		broken := provider{boot: func(_ context.Context, a *foundation.Application) error {
			a.Schedule.Job(tickArgs{}).DailyAt("25:99")

			return nil
		}}
		app := newApp(t, options(environ(t, "APP_APP__ROLES=worker"), broken))

		require.ErrorContains(t, app.Start(t.Context()), "apply schedule")
	})

	t.Run("refuses routes it cannot build", func(t *testing.T) {
		t.Parallel()
		broken := provider{register: func(a *foundation.Application) error {
			a.Router.Handle(http.MethodGet, "/a", "same", http.NotFoundHandler())
			a.Router.Handle(http.MethodGet, "/b", "same", http.NotFoundHandler())

			return nil
		}}
		app := newApp(t, options(environ(t, "APP_APP__ROLES=web"), broken))

		require.ErrorContains(t, app.Start(t.Context()), "build routes")
	})

	t.Run("reports an admin listener that cannot bind", func(t *testing.T) {
		t.Parallel()
		busy := newApp(t, options(environ(t, "APP_APP__ROLES=web")))
		require.NoError(t, busy.Start(t.Context()))
		app := newApp(t, options(environ(t, "APP_APP__ROLES=web", "APP_HTTP__ADMIN_ADDR="+busy.Addr())))

		require.ErrorContains(t, app.Start(t.Context()), "listen admin")
	})
}

func TestMaintenance(t *testing.T) {
	t.Parallel()

	t.Run("takes every replica down and brings it back", func(t *testing.T) {
		t.Parallel()
		env := environ(t, "APP_APP__ROLES=web")
		first := newApp(t, options(env))
		second := newApp(t, options(append(env, "APP_APP__DATA_DIR="+t.TempDir())))
		require.NoError(t, second.Start(t.Context()))

		require.NoError(t, first.Maintenance.Down(t.Context(), "upgrading"))

		require.Eventually(t, func() bool {
			return mustGet(t, "http://"+second.Addr()+"/anything").Status == http.StatusServiceUnavailable
		}, 5*time.Second, 20*time.Millisecond)
		resp := mustGet(t, "http://"+second.Addr()+"/anything")
		assert.Equal(t, "60", resp.Header.Get("Retry-After"))
		ready := mustGet(t, "http://"+second.Addr()+foundation.PathReady)
		assert.Equal(t, http.StatusOK, ready.Status)
		assert.Contains(t, string(ready.Body), `"maintenance":"warn"`)
		state, err := second.Maintenance.State(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "upgrading", state.Message)

		require.NoError(t, first.Maintenance.Up(t.Context()))

		require.Eventually(t, func() bool {
			return mustGet(t, "http://"+second.Addr()+"/anything").Status == http.StatusNotFound
		}, 5*time.Second, 20*time.Millisecond)
	})

	t.Run("stays down across a restart and for a replica on another disk", func(t *testing.T) {
		t.Parallel()
		env := environ(t, "APP_MIGRATIONS__AUTO=false")
		before := newApp(t, options(env))
		require.NoError(t, before.Maintenance.Down(t.Context(), "moving"))
		require.NoError(t, before.Shutdown(t.Context()))

		after := newApp(t, options(append(env, "APP_APP__DATA_DIR="+t.TempDir())))

		state, err := after.Maintenance.State(t.Context())
		require.NoError(t, err)
		assert.True(t, state.Enabled)
		assert.Equal(t, "moving", state.Message)
		assert.False(t, state.Since.IsZero())
	})

	t.Run("comes back up for a replica started after", func(t *testing.T) {
		t.Parallel()
		env := environ(t, "APP_MIGRATIONS__AUTO=false")
		first := newApp(t, options(env))
		require.NoError(t, first.Maintenance.Down(t.Context(), ""))
		require.NoError(t, first.Maintenance.Up(t.Context()))

		later := newApp(t, options(env))

		active, err := later.Maintenance.Active(t.Context())
		require.NoError(t, err)
		assert.False(t, active)
	})
}

func TestJWKS(t *testing.T) {
	t.Parallel()

	t.Run("fetches published keys through the guarded client", func(t *testing.T) {
		t.Parallel()
		jwks := `{"keys":[{"kty":"oct","kid":"k1","alg":"HS256","k":"c2VjcmV0LXNlY3JldC1zZWNyZXQtc2VjcmV0LXNlY3JldA"}]}`
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/keys" {
				w.WriteHeader(http.StatusNotFound)

				return
			}

			_, _ = w.Write([]byte(jwks))
		}))
		t.Cleanup(server.Close)
		app := newApp(t, options(environ(t, "APP_MIGRATIONS__AUTO=false")))

		_, err := app.JWKS(server.URL+"/keys", jwt.JWKSOptions{}).Keys(t.Context())
		require.NoError(t, err)

		_, err = app.JWKS(server.URL+"/missing", jwt.JWKSOptions{}).Keys(t.Context())
		require.ErrorContains(t, err, "status 404")

		_, err = app.JWKS("http://127.0.0.1:1/keys", jwt.JWKSOptions{}).Keys(t.Context())
		require.Error(t, err)
	})
}
