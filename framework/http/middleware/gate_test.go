package middleware_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
)

func TestGate(t *testing.T) {
	t.Parallel()

	opts := middleware.GateOptions{
		Password: "correct horse",
		Key:      "ci-key",
		Secret:   []byte("0123456789abcdef0123456789abcdef"),
		Except:   []string{"/webhooks/*", "/assets/app.js"},
	}
	gated := middleware.Gate(opts)(echo())

	signIn := func(t *testing.T, password, next string) *http.Request {
		t.Helper()
		form := url.Values{"password": {password}, "next": {next}}
		r := request(t, http.MethodPost, middleware.GatePath, form.Encode())
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		return r
	}

	t.Run("refuses a visitor with the password form and noindex", func(t *testing.T) {
		t.Parallel()
		rec := serve(gated, request(t, http.MethodGet, "/d/settings?tab=1", ""))

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Body.String(), `action="/__gate"`)
		assert.Contains(t, rec.Body.String(), `value="/d/settings?tab=1"`)
		assert.Equal(t, "noindex, nofollow", rec.Header().Get("X-Robots-Tag"))
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	})

	t.Run("lets excepted paths and the key through", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusOK, serve(gated, request(t, http.MethodPost, "/webhooks/mail/inbound", "")).Code)
		assert.Equal(t, http.StatusOK, serve(gated, request(t, http.MethodGet, "/assets/app.js", "")).Code)

		withKey := request(t, http.MethodGet, "/d", "")
		withKey.Header.Set(middleware.GateKeyHeader, "ci-key")
		assert.Equal(t, http.StatusOK, serve(gated, withKey).Code)

		wrongKey := request(t, http.MethodGet, "/d", "")
		wrongKey.Header.Set(middleware.GateKeyHeader, "guess")
		assert.Equal(t, http.StatusUnauthorized, serve(gated, wrongKey).Code)
	})

	t.Run("keeps search engines out", func(t *testing.T) {
		t.Parallel()
		rec := serve(gated, request(t, http.MethodGet, "/robots.txt", ""))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "User-agent: *\nDisallow: /\n", rec.Body.String())
	})

	t.Run("the right password sets a host cookie that opens every page", func(t *testing.T) {
		t.Parallel()
		rec := serve(gated, signIn(t, "correct horse", "/d/settings?tab=1"))

		require.Equal(t, http.StatusSeeOther, rec.Code)
		assert.Equal(t, "/d/settings?tab=1", rec.Header().Get("Location"))
		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "__Host-gate", cookies[0].Name)
		assert.True(t, cookies[0].Secure)
		assert.True(t, cookies[0].HttpOnly)
		assert.NotContains(t, cookies[0].Value, "correct")

		next := request(t, http.MethodGet, "/d", "")
		next.AddCookie(cookies[0])
		assert.Equal(t, http.StatusOK, serve(gated, next).Code)
	})

	t.Run("a wrong password or a forged cookie stays out", func(t *testing.T) {
		t.Parallel()
		rec := serve(gated, signIn(t, "wrong", "/d"))

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Body.String(), "That password is not right.")
		assert.Empty(t, rec.Result().Cookies())

		forged := request(t, http.MethodGet, "/d", "")
		forged.AddCookie(&http.Cookie{Name: "__Host-gate", Value: "anything"})
		assert.Equal(t, http.StatusUnauthorized, serve(gated, forged).Code)
	})

	t.Run("a changed password signs everyone out", func(t *testing.T) {
		t.Parallel()
		cookie := serve(gated, signIn(t, "correct horse", "/")).Result().Cookies()[0]
		changed := opts
		changed.Password = "new password"

		r := request(t, http.MethodGet, "/d", "")
		r.AddCookie(cookie)
		assert.Equal(t, http.StatusUnauthorized, serve(middleware.Gate(changed)(echo()), r).Code)
	})

	t.Run("never sends anyone to another host after signing in", func(t *testing.T) {
		t.Parallel()

		for _, next := range []string{"https://evil.example/", "//evil.example/x", "/\\evil.example", "relative", ""} {
			rec := serve(gated, signIn(t, "correct horse", next))
			assert.Equal(t, "/", rec.Header().Get("Location"), next)
		}
	})
}

func TestGatePage(t *testing.T) {
	t.Parallel()

	gated := middleware.Gate(middleware.GateOptions{
		Password: "correct horse",
		Secret:   []byte("0123456789abcdef0123456789abcdef"),
		Page:     true,
	})(echo())

	page := func(t *testing.T, method, path, accept string) *http.Request {
		t.Helper()
		r := request(t, method, path, "")
		r.Header.Set("Accept", accept)

		return r
	}

	t.Run("sends a browser asking for a page to the application's password page", func(t *testing.T) {
		t.Parallel()
		rec := serve(gated, page(t, http.MethodGet, "/d/settings?tab=1", "text/html,application/xhtml+xml"))

		assert.Equal(t, http.StatusSeeOther, rec.Code)
		assert.Equal(t, "/__gate?next=%2Fd%2Fsettings%3Ftab%3D1", rec.Header().Get("Location"))
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	})

	t.Run("lets the password page itself reach the application", func(t *testing.T) {
		t.Parallel()
		rec := serve(gated, page(t, http.MethodGet, middleware.GatePath+"?next=%2Fd", "text/html"))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "noindex, nofollow", rec.Header().Get("X-Robots-Tag"))
	})

	t.Run("still refuses calls that are not pages", func(t *testing.T) {
		t.Parallel()

		api := page(t, http.MethodGet, "/api/v1/me", "application/json")
		form := page(t, http.MethodPost, "/d", "text/html")

		assert.Equal(t, http.StatusUnauthorized, serve(gated, api).Code)
		assert.Equal(t, http.StatusUnauthorized, serve(gated, form).Code)
	})

	t.Run("returns a wrong password to the page and lets the right one through", func(t *testing.T) {
		t.Parallel()
		post := func(password string) *http.Request {
			body := url.Values{"password": {password}, "next": {"/d"}}.Encode()
			r := request(t, http.MethodPost, middleware.GatePath, body)
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			return r
		}

		wrong := serve(gated, post("guess"))
		assert.Equal(t, http.StatusSeeOther, wrong.Code)
		assert.Equal(t, "/__gate?failed=1&next=%2Fd", wrong.Header().Get("Location"))
		assert.Empty(t, wrong.Result().Cookies())

		right := serve(gated, post("correct horse"))
		assert.Equal(t, http.StatusSeeOther, right.Code)
		assert.Equal(t, "/d", right.Header().Get("Location"))
		require.Len(t, right.Result().Cookies(), 1)
	})
}
