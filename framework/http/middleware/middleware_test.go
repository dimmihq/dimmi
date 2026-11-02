package middleware_test

import (
	"context"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/view"
)

func TestRequestID(t *testing.T) {
	t.Parallel()

	var seen string
	h := middleware.RequestID("")(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = http.RequestID(r.Context())
	}))

	t.Run("gives every request an id in the context and the response", func(t *testing.T) {
		rec := serve(h, request(t, http.MethodGet, "/", ""))

		assert.Len(t, seen, 36)
		assert.Equal(t, seen, rec.Header().Get(middleware.DefaultRequestIDHeader))
	})

	t.Run("keeps an id that looks like one", func(t *testing.T) {
		r := request(t, http.MethodGet, "/", "")
		r.Header.Set("X-Request-Id", "lb-1.2:abc_D")
		serve(h, r)

		assert.Equal(t, "lb-1.2:abc_D", seen)
	})

	t.Run("replaces anything else", func(t *testing.T) {
		r := request(t, http.MethodGet, "/", "")
		r.Header.Set("X-Request-Id", "bad id\n")
		serve(h, r)

		assert.NotEqual(t, "bad id\n", seen)
		assert.Len(t, seen, 36)
	})
}

func TestRecover(t *testing.T) {
	t.Parallel()

	t.Run("answers a panic with a reported 500", func(t *testing.T) {
		t.Parallel()
		var reported error
		errs := http.Errors{Report: func(_ context.Context, err error) { reported = err }}
		h := middleware.Recover(errs)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))

		rec := serve(h, request(t, http.MethodGet, "/", ""))
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.ErrorContains(t, reported, "panic: boom")
	})

	t.Run("passes deliberate aborts on", func(t *testing.T) {
		t.Parallel()
		h := middleware.Recover(http.Errors{})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic(stdhttp.ErrAbortHandler)
		}))

		assert.PanicsWithValue(t, stdhttp.ErrAbortHandler, func() { serve(h, request(t, http.MethodGet, "/", "")) })
	})

	t.Run("lets normal requests through", func(t *testing.T) {
		t.Parallel()

		h := middleware.Recover(http.Errors{})(echo())

		assert.Equal(t, "ok", serve(h, request(t, http.MethodGet, "/", "")).Body.String())
	})
}

func TestTrustProxies(t *testing.T) {
	t.Parallel()

	mw, err := middleware.TrustProxies([]string{"10.0.0.0/8"}, "")
	require.NoError(t, err)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(http.ClientIP(r))) }))

	client := func(remote, forwarded string) string {
		r := request(t, http.MethodGet, "/", "")
		r.RemoteAddr = remote
		r.Header.Set("X-Forwarded-For", forwarded)

		return serve(h, r).Body.String()
	}

	t.Run("reads the header behind a trusted proxy, skipping trusted hops", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "203.0.113.9", client("10.0.0.1:443", "198.51.100.1, 203.0.113.9, 10.0.0.2"))
	})

	t.Run("ignores the header from anyone else", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "198.51.100.7", client("198.51.100.7:443", "203.0.113.9"))
		assert.Equal(t, "pipe", client("pipe", "203.0.113.9"))
	})

	t.Run("keeps the connection address when the header names only proxies", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "10.0.0.1", client("10.0.0.1:443", "10.0.0.3"))
	})

	t.Run("refuses addresses that do not parse", func(t *testing.T) {
		t.Parallel()
		_, err := middleware.TrustProxies([]string{"nope"}, "")

		require.Error(t, err)
	})

	t.Run("refuses an unusable header", func(t *testing.T) {
		t.Parallel()
		_, err := middleware.TrustProxies(nil, "X-Real-IP")

		require.Error(t, err)
	})
}

func TestCORS(t *testing.T) {
	t.Parallel()

	preflight := func(h http.Handler, origin string) *http.Header {
		r := request(t, http.MethodOptions, "/api/items", "")
		r.Header.Set("Origin", origin)
		r.Header.Set("Access-Control-Request-Method", http.MethodPatch)
		header := serve(h, r).Header()

		return &header
	}

	t.Run("allows matching origins", func(t *testing.T) {
		t.Parallel()
		h := middleware.CORS(middleware.CORSOptions{
			AllowedOrigins: []string{"https://*.example.com"},
			AllowedMethods: []string{http.MethodPatch},
			MaxAge:         time.Hour,
		})(echo())

		allowed := preflight(h, "https://App.Example.com")
		assert.Equal(t, "https://App.Example.com", allowed.Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "3600", allowed.Get("Access-Control-Max-Age"))
		assert.Empty(t, preflight(h, "https://example.com.evil.test").Get("Access-Control-Allow-Origin"))
		assert.Empty(t, preflight(h, "not an origin").Get("Access-Control-Allow-Origin"))
	})

	t.Run("allows any origin with a star", func(t *testing.T) {
		t.Parallel()
		h := middleware.CORS(middleware.CORSOptions{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{http.MethodPatch},
		})(echo())

		assert.Equal(t, "*", preflight(h, "https://anyone.test").Get("Access-Control-Allow-Origin"))
	})
}

func TestCSRF(t *testing.T) {
	t.Parallel()

	mw, err := middleware.CSRF("https://dashboard.example.com")
	require.NoError(t, err)
	h := mw(echo())

	post := func(headers map[string]string) int {
		r := request(t, http.MethodPost, "https://app.example.com/settings", "")
		for name, value := range headers {
			r.Header.Set(name, value)
		}

		return serve(h, r).Code
	}

	t.Run("refuses cross-site unsafe requests", func(t *testing.T) {
		t.Parallel()
		r := request(t, http.MethodPost, "https://app.example.com/settings", "")
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		rec := serve(h, r)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Equal(t, "cross_origin_request", code(t, rec.Body))
	})

	t.Run("allows same-origin, trusted and non-browser requests", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusOK, post(map[string]string{"Sec-Fetch-Site": "same-origin"}))
		assert.Equal(t, http.StatusOK, post(map[string]string{"Origin": "https://dashboard.example.com"}))
		assert.Equal(t, http.StatusOK, post(nil))
	})

	t.Run("lets bearer requests through", func(t *testing.T) {
		t.Parallel()

		bearer := map[string]string{"Sec-Fetch-Site": "cross-site", "Authorization": "Bearer t"}

		assert.Equal(t, http.StatusOK, post(bearer))
	})

	t.Run("refuses trusted origins that are not origins", func(t *testing.T) {
		t.Parallel()
		_, err := middleware.CSRF("https://example.com/path")

		require.Error(t, err)
	})
}

func TestSigned(t *testing.T) {
	t.Parallel()

	r := routing.New(routing.Options{})
	r.Handle(http.MethodGet, "/downloads/{file}", "downloads.show", echo())
	urls := routing.NewURL(r, "", []byte("key"), clock.System())
	h := middleware.Signed(urls)(echo())

	t.Run("lets signed links through", func(t *testing.T) {
		t.Parallel()
		link, err := urls.SignedRoute("downloads.show", map[string]string{"file": "a"}, time.Hour)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, serve(h, request(t, http.MethodGet, link, "")).Code)
	})

	t.Run("refuses anything else", func(t *testing.T) {
		t.Parallel()
		rec := serve(h, request(t, http.MethodGet, "/downloads/a", ""))

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Equal(t, "invalid_signature", code(t, rec.Body))
	})
}

/*
fileMode is maintenance mode switched on by a file existing, as a
deployment switches it.
*/
type fileMode struct{ path string }

func (m fileMode) Active(context.Context) (bool, error) {
	_, err := os.Stat(m.path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return err == nil, err
}

func TestMaintenance(t *testing.T) {
	t.Parallel()

	newMode := func(t *testing.T) (fileMode, http.Handler) {
		t.Helper()
		mode := fileMode{path: filepath.Join(t.TempDir(), "down")}

		return mode, middleware.Maintenance(mode, 90*time.Second, "/up", "/internal/*")(echo())
	}

	t.Run("answers 503 while down, except the listed paths", func(t *testing.T) {
		t.Parallel()
		mode, h := newMode(t)
		require.NoError(t, os.WriteFile(mode.path, nil, 0o600))

		rec := serve(h, request(t, http.MethodGet, "/", ""))
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "90", rec.Header().Get("Retry-After"))
		assert.Equal(t, "maintenance", code(t, rec.Body))
		assert.Equal(t, http.StatusOK, serve(h, request(t, http.MethodGet, "/up", "")).Code)
		assert.Equal(t, http.StatusOK, serve(h, request(t, http.MethodGet, "/internal/tls/ask", "")).Code)
	})

	t.Run("serves while up", func(t *testing.T) {
		t.Parallel()
		_, h := newMode(t)

		assert.Equal(t, http.StatusOK, serve(h, request(t, http.MethodGet, "/", "")).Code)
	})

	t.Run("stays up when the switch cannot be read", func(t *testing.T) {
		t.Parallel()
		file := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(file, nil, 0o600))
		h := middleware.Maintenance(fileMode{path: filepath.Join(file, "down")}, 0)(echo())

		assert.Equal(t, http.StatusOK, serve(h, request(t, http.MethodGet, "/", "")).Code)
	})
}

func TestBodyLimit(t *testing.T) {
	t.Parallel()

	h := middleware.BodyLimit(5)(echo())

	t.Run("refuses bodies announced as too large", func(t *testing.T) {
		t.Parallel()
		rec := serve(h, request(t, http.MethodPost, "/", "123456"))

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
		assert.Equal(t, "payload_too_large", code(t, rec.Body))
	})

	t.Run("stops bodies that grow past the limit", func(t *testing.T) {
		t.Parallel()
		r := request(t, http.MethodPost, "/", "123456")
		r.ContentLength = -1
		rec := serve(h, r)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	})

	t.Run("lets small bodies through", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "12345", serve(h, request(t, http.MethodPost, "/", "12345")).Body.String())
	})
}

func TestTimeout(t *testing.T) {
	t.Parallel()

	t.Run("answers 503 when the handler gave up without writing", func(t *testing.T) {
		t.Parallel()
		h := middleware.Timeout(10 * time.Millisecond)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		rec := serve(h, request(t, http.MethodGet, "/", ""))

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "request_timeout", code(t, rec.Body))
	})

	t.Run("keeps what the handler wrote", func(t *testing.T) {
		t.Parallel()
		h := middleware.Timeout(10 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
			w.WriteHeader(http.StatusAccepted)
		}))

		assert.Equal(t, http.StatusAccepted, serve(h, request(t, http.MethodGet, "/", "")).Code)
	})

	t.Run("lets fast requests through", func(t *testing.T) {
		t.Parallel()

		h := middleware.Timeout(time.Minute)(echo())

		assert.Equal(t, "ok", serve(h, request(t, http.MethodGet, "/", "")).Body.String())
	})
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	t.Run("sets safe defaults", func(t *testing.T) {
		t.Parallel()
		h := middleware.SecurityHeaders(middleware.SecurityHeadersOptions{
			ContentSecurityPolicy: "default-src 'self'",
			HSTSMaxAge:            time.Hour,
			PermissionsPolicy:     "camera=()",
		})(echo())
		header := serve(h, request(t, http.MethodGet, "/", "")).Header()

		assert.Equal(t, "nosniff", header.Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", header.Get("X-Frame-Options"))
		assert.Equal(t, "strict-origin-when-cross-origin", header.Get("Referrer-Policy"))
		assert.Equal(t, "max-age=3600; includeSubDomains", header.Get("Strict-Transport-Security"))
		assert.Equal(t, "camera=()", header.Get("Permissions-Policy"))
		assert.Equal(t, "default-src 'self'", header.Get("Content-Security-Policy"))
	})

	t.Run("gives each request a fresh nonce shared with templates", func(t *testing.T) {
		t.Parallel()
		h := middleware.SecurityHeaders(middleware.SecurityHeadersOptions{
			ContentSecurityPolicy: "script-src 'nonce-{nonce}'",
			FrameOptions:          "SAMEORIGIN",
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(view.Nonce(r.Context())))
		}))

		first := serve(h, request(t, http.MethodGet, "/", ""))
		second := serve(h, request(t, http.MethodGet, "/", ""))
		assert.Equal(t, "script-src 'nonce-"+first.Body.String()+"'", first.Header().Get("Content-Security-Policy"))
		assert.NotEqual(t, first.Body.String(), second.Body.String())
		assert.Equal(t, "SAMEORIGIN", first.Header().Get("X-Frame-Options"))
	})
}

func TestAPIVersion(t *testing.T) {
	t.Parallel()

	h := middleware.APIVersion("Api-Version", "2026-11-01", []string{"2026-06-01"})(echo())
	send := func(version string) *httptest.ResponseRecorder {
		r := request(t, http.MethodGet, "/", "")
		if version != "" {
			r.Header.Set("Api-Version", version)
		}

		return serve(h, r)
	}

	t.Run("names the current version on every response", func(t *testing.T) {
		t.Parallel()
		rec := send("")

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "2026-11-01", rec.Header().Get("Api-Version"))
	})

	t.Run("accepts the current and supported versions", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusOK, send("2026-11-01").Code)
		assert.Equal(t, http.StatusOK, send("2026-06-01").Code)
	})

	t.Run("refuses other versions with the supported list", func(t *testing.T) {
		t.Parallel()
		rec := send("2020-01-01")

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.JSONEq(t, `{"message":"This API version is not supported.","code":"invalid_api_version",`+
			`"supported_versions":["2026-11-01","2026-06-01"]}`, rec.Body.String())
	})
}

func TestClientVersion(t *testing.T) {
	t.Parallel()

	h := middleware.ClientVersion("Client-Version",
		map[string]string{"ios": "1.4.0", "cli": "bad"},
		map[string]string{"ios": "https://apps.example.com/ios"},
	)(echo())
	send := func(version string) *httptest.ResponseRecorder {
		r := request(t, http.MethodGet, "/", "")
		r.Header.Set("Client-Version", version)

		return serve(h, r)
	}

	t.Run("refuses clients below the minimum", func(t *testing.T) {
		t.Parallel()
		rec := send("ios/1.3.9")

		assert.Equal(t, http.StatusUpgradeRequired, rec.Code)
		assert.JSONEq(t, `{"message":"This version of the app is no longer supported. Please upgrade.",`+
			`"code":"client_upgrade_required","min_version":"1.4.0","upgrade_url":"https://apps.example.com/ios"}`,
			rec.Body.String())
	})

	t.Run("lets current, unknown and malformed clients through", func(t *testing.T) {
		t.Parallel()

		for _, version := range []string{
			"", "ios/1.4.0", "ios/1.10", "ios/2.0.0-beta.1", "android/0.1.0", "ios",
			"ios/1.x", "ios/1.2.3.4", "ios/-1", "cli/0.1.0",
		} {
			assert.Equal(t, http.StatusOK, send(version).Code, version)
		}
	})

	t.Run("compares partial versions", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusUpgradeRequired, send("ios/1").Code)
	})
}
