package foundation_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/routing"
)

/*
language holds the application's English and French messages, French
covering only what the tests read.
*/
func language() fstest.MapFS {
	return fstest.MapFS{
		"en/messages.json":   {Data: []byte(`{"hello": "Hello", "subject": "Welcome"}`)},
		"fr/messages.json":   {Data: []byte(`{"hello": "Bonjour", "subject": "Bienvenue"}`)},
		"fr/errors.json":     {Data: []byte(`{"invalid_cursor": "Le curseur est invalide."}`)},
		"fr/validation.json": {Data: []byte(`{"required": "Le champ {{.field}} est obligatoire."}`)},
	}
}

/*
french serves the application in English and French, listed in its
configuration file, with the messages of language.
*/
func french(t *testing.T, opts *foundation.Options) {
	t.Helper()

	file := filepath.Join(t.TempDir(), "app.yaml")
	require.NoError(t, os.WriteFile(file, []byte("app:\n  locales: [en, fr]\n"), 0o600))
	opts.ConfigFiles = []string{file}
	opts.Language = language()
}

/*
Request is a request the test routes answer.
*/
type Request = http.Request

/*
signup is the input a localized route validates.
*/
type signup struct {
	Email string `json:"email" validate:"required"`
}

/*
localizedRoutes answers with a greeting, a validation failure and a
fault, each in the request's locale; /me also reads the user's saved
locale, standing in for a route behind "auth".
*/
func localizedRoutes() provider {
	return provider{register: func(a *foundation.Application) error {
		a.Router.Handle(http.MethodGet, "/hello", "hello", http.HandlerFunc(func(w http.ResponseWriter, r *Request) {
			_, _ = w.Write([]byte(i18n.T(r.Context(), "messages.hello", nil)))
		}))
		a.Router.Handle(http.MethodGet, "/signup", "signup", http.HandlerFunc(func(w http.ResponseWriter, r *Request) {
			a.Errors.Render(w, r, a.Validator.Struct(r.Context(), signup{}))
		}))
		a.Router.Handle(http.MethodGet, "/items", "items", http.HandlerFunc(func(w http.ResponseWriter, r *Request) {
			a.Errors.Render(w, r, pagination.ErrInvalidCursor)
		}))
		a.Router.Group("/me", func(g *routing.Router) {
			g.Middleware("locale")
			g.Handle(http.MethodGet, "", "me", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(i18n.Locale(r.Context())))
			}))
		})

		return nil
	}}
}

/*
fetch requests the path from the application with the headers and
returns the status, the body and the Content-Language header.
*/
func fetch(t *testing.T, app *foundation.Application, path string, headers map[string]string) (int, string, string) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true, Timeout: 5 * time.Second})
	require.NoError(t, err)

	req := client.New()
	for name, value := range headers {
		req = req.WithHeader(name, value)
	}

	resp, err := req.Get(t.Context(), "http://"+app.Addr()+path)
	require.NoError(t, err)

	return resp.Status, string(resp.Body), resp.Header.Get("Content-Language")
}

/*
welcomeMail is a mailable whose subject is translated in the locale it
renders in.
*/
type welcomeMail struct{}

func (welcomeMail) Envelope() mail.Envelope { return mail.Envelope{} }

func (welcomeMail) Content(ctx context.Context) (mail.Content, error) {
	return mail.Content{Subject: i18n.T(ctx, "messages.subject", nil), Text: "."}, nil
}

func TestLocalization(t *testing.T) {
	t.Parallel()

	t.Run("serves requests in the preferred, negotiated or default locale", func(t *testing.T) {
		t.Parallel()
		opts := options(environ(t, "APP_APP__ROLES=web", "APP_MIGRATIONS__AUTO=false"),
			localizedRoutes())
		french(t, &opts)
		opts.LocalePreference = func(r *http.Request) string { return r.Header.Get("X-User-Locale") }
		app := newApp(t, opts)
		require.NoError(t, app.Start(t.Context()))

		_, body, contentLanguage := fetch(t, app, "/hello", map[string]string{"Accept-Language": "fr-FR,fr;q=0.9"})
		assert.Equal(t, "Bonjour", body)
		assert.Equal(t, "fr", contentLanguage)

		_, body, contentLanguage = fetch(t, app, "/hello", map[string]string{"Accept-Language": "de"})
		assert.Equal(t, "Hello", body)
		assert.Equal(t, "en", contentLanguage)

		_, body, _ = fetch(t, app, "/me", map[string]string{"Accept-Language": "en", "X-User-Locale": "fr"})
		assert.Equal(t, "fr", body)
	})

	t.Run("localizes validation and error messages but never codes or fields", func(t *testing.T) {
		t.Parallel()
		opts := options(environ(t, "APP_APP__ROLES=web", "APP_MIGRATIONS__AUTO=false"),
			localizedRoutes())
		french(t, &opts)
		app := newApp(t, opts)
		require.NoError(t, app.Start(t.Context()))
		fr := map[string]string{"Accept-Language": "fr"}

		status, body, _ := fetch(t, app, "/signup", fr)
		assert.Equal(t, http.StatusUnprocessableEntity, status)
		assert.JSONEq(t, `{"message":"Le champ email est obligatoire.","code":"validation_failed",`+
			`"errors":{"email":["Le champ email est obligatoire."]}}`, body)

		_, body, _ = fetch(t, app, "/signup", nil)
		assert.JSONEq(t, `{"message":"The email field is required.","code":"validation_failed",`+
			`"errors":{"email":["The email field is required."]}}`, body)

		_, body, _ = fetch(t, app, "/items", fr)
		assert.JSONEq(t, `{"message":"Le curseur est invalide.","code":"invalid_cursor"}`, body)
	})

	t.Run("sends queued mail in the recipient's locale", func(t *testing.T) {
		t.Parallel()
		opts := options(environ(t))
		french(t, &opts)
		app := newApp(t, opts)
		mail.Register[welcomeMail](app.Mail, "welcome")
		mails := app.Mail.Fake()
		sender := i18n.WithLocale(t.Context(), "en")

		zoe := mail.Address{Email: "zoe@example.com"}
		require.NoError(t, app.Mail.To(zoe).Locale("fr").Queue(sender, welcomeMail{}))
		require.Eventually(t, func() bool {
			return app.Queue.Drain(t.Context()) == nil && len(mails.Sent()) == 1
		}, maintenanceWait, pollEvery)
		assert.Equal(t, "Bienvenue", mails.Sent()[0].Message.Subject)
	})

	t.Run("defaults to english and serves the configured locales", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t, "APP_MIGRATIONS__AUTO=false")))

		assert.Equal(t, "en", app.Config.App.Locale)
		assert.Equal(t, []string{"en"}, app.Lang.Locales())
		_, err := app.Kernel.Resolve("locale")
		require.NoError(t, err)
		assert.Equal(t, "The given data was invalid.", app.Lang.T(t.Context(), "errors.validation_failed", nil))
	})
}
