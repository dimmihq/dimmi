package foundation_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
greetArgs is a job that sends a greeting by mail.
*/
type greetArgs struct {
	Email string `json:"email"`
}

func (greetArgs) Kind() string { return "greet" }

/*
greetWorker sends the greeting through the application's mail and
reports each job it finished.
*/
type greetWorker struct {
	mail *mail.Manager
	done chan string
}

func (w greetWorker) Handle(ctx context.Context, job *queue.Job[greetArgs]) error {
	_, err := w.mail.To(mail.Address{Email: job.Args.Email}).Send(ctx, greeting{})
	w.done <- job.Args.Email

	return err
}

/*
greeting is a plain mailable.
*/
type greeting struct{}

func (greeting) Envelope() mail.Envelope { return mail.Envelope{Subject: "Hello from the queue"} }

func (greeting) Content(context.Context) (mail.Content, error) {
	return mail.Content{Text: "Hello."}, nil
}

/*
received reports whether Mailpit holds a message for the recipient.
*/
func received(t *testing.T, recipient string) bool {
	t.Helper()

	query := url.Values{"query": {`addressed:"` + recipient + `"`}}
	resp := mustGet(t, testkit.Env(t, "MAILPIT_API_URL")+"/api/v1/search?"+query.Encode())

	var found struct{ Messages []struct{ ID string } }
	require.NoError(t, resp.JSON(&found))

	return len(found.Messages) == 1
}

func TestApplication(t *testing.T) {
	t.Parallel()

	t.Run("boots every service, migrates and serves health on both listeners", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t)))
		require.NoError(t, app.Start(t.Context()))

		for _, base := range []string{"http://" + app.Addr(), "http://" + app.AdminAddr()} {
			assert.Equal(t, http.StatusOK, mustGet(t, base+foundation.PathUp).Status)
			ready := mustGet(t, base+foundation.PathReady)
			assert.Equal(t, http.StatusOK, ready.Status)
			assert.Contains(t, string(ready.Body), `"database":"pass"`)
			assert.Contains(t, string(ready.Body), `"valkey":"pass"`)
		}

		assert.Equal(t, http.StatusOK, mustGet(t, "http://"+app.AdminAddr()+foundation.PathMetrics).Status)
		assert.Equal(t, http.StatusNotFound, mustGet(t, "http://"+app.Addr()+foundation.PathMetrics).Status)

		for _, table := range []string{
			"sessions", "personal_access_tokens", "oauth_clients", "certmagic_storage", "idempotency_keys",
			"notifications", "river_job", "gadgets", "framework_migrations", "app_migrations",
		} {
			var exists bool
			require.NoError(t, app.DB.QueryRow(t.Context(), "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists))
			assert.True(t, exists, table)
		}
	})

	t.Run("runs a queued job end to end and sends its mail", func(t *testing.T) {
		t.Parallel()
		done := make(chan string, 1)
		register := provider{register: func(a *foundation.Application) error {
			return queue.Register(a.Queue, greetWorker{mail: a.Mail, done: done})
		}}
		app := newApp(t, options(environ(t, "APP_APP__ROLES=worker"), register))
		require.NoError(t, app.Start(t.Context()))
		recipient := testkit.UniqueName("greet") + "@example.com"

		require.NoError(t, app.Queue.Push(t.Context(), greetArgs{Email: recipient}))

		require.Eventually(t, func() bool { return len(done) == 1 }, 10*time.Second, pollEvery)
		assert.Equal(t, recipient, <-done)

		assert.Eventually(t, func() bool { return received(t, recipient) }, 5*time.Second, 50*time.Millisecond)
		assert.Empty(t, app.Addr())
	})

	t.Run("serves the application's routes through the kernel", func(t *testing.T) {
		t.Parallel()
		routes := provider{register: func(a *foundation.Application) error {
			hello := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("hello")) })
			a.Router.Handle(http.MethodGet, "/hello", "hello", hello)

			return nil
		}}
		app := newApp(t, options(environ(t, "APP_APP__ROLES=web"), routes))
		require.NoError(t, app.Start(t.Context()))

		resp := mustGet(t, "http://"+app.Addr()+"/hello")

		assert.Equal(t, "hello", string(resp.Body))
		assert.NotEmpty(t, resp.Header.Get("X-Request-Id"))
		assert.NotEmpty(t, app.AdminAddr())
	})

	t.Run("derives one stable key per purpose from the app key", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t, "APP_APP__ROLES=web")))

		codes := app.DerivedKey("codes")

		assert.Len(t, codes, 32)
		assert.Equal(t, codes, app.DerivedKey("codes"))
		assert.NotEqual(t, codes, app.DerivedKey("links"))
	})

	t.Run("shuts down cleanly in reverse and only once", func(t *testing.T) {
		t.Parallel()
		flushed := make(chan struct{}, 2)
		flush := shutdownProvider{provider{shutdown: func(context.Context) error {
			flushed <- struct{}{}

			return nil
		}}}
		app, err := foundation.New(t.Context(), options(environ(t), flush))
		require.NoError(t, err)
		require.NoError(t, app.Start(t.Context()))
		addr := app.Addr()

		require.NoError(t, app.Shutdown(t.Context()))
		require.NoError(t, app.Shutdown(t.Context()))

		assert.Len(t, flushed, 1)
		_, err = get(t, "http://"+addr+foundation.PathUp)
		require.Error(t, err)
	})

	t.Run("serves until the context ends", func(t *testing.T) {
		t.Parallel()
		app, err := foundation.New(t.Context(), options(environ(t)))
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(t.Context())
		served := make(chan error, 1)

		go func() { served <- app.Serve(ctx) }()

		require.Eventually(t, func() bool { return app.Addr() != "" }, 5*time.Second, 10*time.Millisecond)
		cancel()

		require.NoError(t, <-served)
	})

	t.Run("serves a worker without listeners until the context ends", func(t *testing.T) {
		t.Parallel()
		app, err := foundation.New(t.Context(), options(environ(t, "APP_APP__ROLES=worker")))
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		require.NoError(t, app.Serve(ctx))
		assert.Empty(t, app.Addr())
	})

	t.Run("stops serving when a listener cannot bind", func(t *testing.T) {
		t.Parallel()
		busy := newApp(t, options(environ(t, "APP_APP__ROLES=web")))
		require.NoError(t, busy.Start(t.Context()))
		env := environ(t, "APP_APP__ROLES=web", "APP_HTTP__ADDR="+busy.Addr())
		app, err := foundation.New(t.Context(), options(env))
		require.NoError(t, err)

		err = app.Serve(t.Context())

		require.ErrorContains(t, err, "listen public")
	})

	t.Run("closes what it opened when a provider fails to boot", func(t *testing.T) {
		t.Parallel()
		errBoot := errors.New("cannot boot")
		failing := provider{boot: func(context.Context, *foundation.Application) error { return errBoot }}

		_, err := foundation.New(t.Context(), options(environ(t), failing))

		require.ErrorIs(t, err, errBoot)
	})

	t.Run("reports a provider that fails to register", func(t *testing.T) {
		t.Parallel()
		errRegister := errors.New("cannot register")
		failing := provider{register: func(*foundation.Application) error { return errRegister }}

		_, err := foundation.New(t.Context(), options(environ(t), failing))

		require.ErrorIs(t, err, errRegister)
	})
}

func TestFakes(t *testing.T) {
	t.Parallel()

	t.Run("swaps process boundaries for fakes in the testing environment", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t, "APP_MIGRATIONS__AUTO=false")))

		mails := app.Mail.Fake()
		httpclienttest.New(app.HTTP).Stub("https://api.example.test/*", httpclient.Response{Status: http.StatusTeapot})
		app.DNS.Fake()
		app.Scanner.Fake()
		app.Social.Fake()

		_, err := app.Mail.To(mail.Address{Email: "ada@example.com"}).Send(t.Context(), greeting{})
		require.NoError(t, err)
		resp, err := app.HTTP.New().Get(t.Context(), "https://api.example.test/v1")
		require.NoError(t, err)

		assert.Len(t, mails.Sent(), 1)
		assert.Equal(t, http.StatusTeapot, resp.Status)
	})

	t.Run("refuses fakes outside the testing environment", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t, "APP_APP__ENV=local", "APP_MIGRATIONS__AUTO=false")))

		assert.PanicsWithValue(t, mail.ErrFakesNotAllowed, func() { app.Mail.Fake() })
		assert.PanicsWithValue(t, httpclient.ErrFakesNotAllowed, func() { httpclienttest.New(app.HTTP) })
		assert.Panics(t, func() { app.DNS.Fake() })
		assert.Panics(t, func() { app.Scanner.Fake() })
		assert.Panics(t, func() { app.Social.Fake() })
	})
}
