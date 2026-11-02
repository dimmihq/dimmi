package middleware_test

import (
	stdhttp "net/http"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/i18n"
)

/*
greeter answers with the request's locale and a greeting in it, or with
the localized not found error on /missing.
*/
func greeter() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.Errors{}.Render(w, r, http.StatusError(http.StatusNotFound))

			return
		}

		_, _ = w.Write([]byte(i18n.Locale(r.Context()) + ": " + i18n.T(r.Context(), "messages.hello", nil)))
	})
}

/*
userLocale stands for the user's saved preference, read here from a
header the test sets.
*/
func userLocale(r *http.Request) string {
	return r.Header.Get("X-User-Locale")
}

func newLocaleKernel(t *testing.T) http.Handler {
	t.Helper()

	lang := fstest.MapFS{
		"en/messages.json": {Data: []byte(`{"hello": "Hello"}`)},
		"fr/messages.json": {Data: []byte(`{"hello": "Bonjour"}`)},
		"fr/errors.json":   {Data: []byte(`{"not_found": "Introuvable."}`)},
		"es/messages.json": {Data: []byte(`{"hello": "Hola"}`)},
	}
	translator, err := i18n.New(i18n.Config{Locale: "en", Locales: []string{"en", "fr", "es"}}, http.Messages(), lang)
	require.NoError(t, err)

	k := http.NewKernel()
	k.Use(middleware.Locale(translator, nil))
	k.Alias("locale", func(...string) (http.Middleware, error) {
		return middleware.Locale(translator, userLocale), nil
	})

	routed, err := k.Resolve("locale")
	require.NoError(t, err)

	return k.Handler(http.Chain(greeter(), routed...))
}

func TestLocale(t *testing.T) {
	t.Parallel()

	get := func(t *testing.T, h http.Handler, path string, headers map[string]string) (int, string, stdhttp.Header) {
		t.Helper()
		r := request(t, http.MethodGet, path, "")
		for name, value := range headers {
			r.Header.Set(name, value)
		}

		rec := serve(h, r)

		return rec.Code, rec.Body.String(), rec.Header()
	}

	t.Run("serves the best match for accept-language", func(t *testing.T) {
		t.Parallel()
		h := newLocaleKernel(t)

		_, body, header := get(t, h, "/", map[string]string{"Accept-Language": "fr-CH, fr;q=0.9, en;q=0.8"})

		assert.Equal(t, "fr: Bonjour", body)
		assert.Equal(t, "fr", header.Get("Content-Language"))
		assert.Equal(t, []string{"Accept-Language"}, header.Values("Vary"))
	})

	t.Run("falls back to the default locale", func(t *testing.T) {
		t.Parallel()
		h := newLocaleKernel(t)

		_, none, _ := get(t, h, "/", nil)
		_, unserved, header := get(t, h, "/", map[string]string{"Accept-Language": "de-DE, de;q=0.9"})

		assert.Equal(t, "en: Hello", none)
		assert.Equal(t, "en: Hello", unserved)
		assert.Equal(t, "en", header.Get("Content-Language"))
	})

	t.Run("prefers the user's locale over accept-language", func(t *testing.T) {
		t.Parallel()
		h := newLocaleKernel(t)

		_, body, header := get(t, h, "/", map[string]string{"Accept-Language": "fr", "X-User-Locale": "es-MX"})

		assert.Equal(t, "es: Hola", body)
		assert.Equal(t, "es", header.Get("Content-Language"))
		assert.Equal(t, []string{"Accept-Language"}, header.Values("Vary"))
	})

	t.Run("ignores a preference the application does not serve", func(t *testing.T) {
		t.Parallel()
		h := newLocaleKernel(t)

		_, body, _ := get(t, h, "/", map[string]string{"Accept-Language": "fr", "X-User-Locale": "de"})

		assert.Equal(t, "fr: Bonjour", body)
	})

	t.Run("writes error envelopes in the request's locale with a stable code", func(t *testing.T) {
		t.Parallel()
		h := newLocaleKernel(t)

		status, fr, _ := get(t, h, "/missing", map[string]string{"Accept-Language": "fr"})
		_, en, _ := get(t, h, "/missing", nil)

		assert.Equal(t, http.StatusNotFound, status)
		assert.JSONEq(t, `{"message":"Introuvable.","code":"not_found"}`, fr)
		assert.JSONEq(t, `{"message":"Not Found.","code":"not_found"}`, en)
	})
}
