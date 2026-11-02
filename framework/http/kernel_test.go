package http_test

import (
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
)

/*
tag returns a middleware that appends its name to the X-Trace response
header, so tests can see which middleware ran and in which order.
*/
func tag(name string) http.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Trace", name)
			next.ServeHTTP(w, r)
		})
	}
}

func trace(t *testing.T, h http.Handler) []string {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", stdhttp.NoBody))

	return rec.Header().Values("X-Trace")
}

func ok() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

func TestKernel(t *testing.T) {
	t.Parallel()

	newKernel := func() *http.Kernel {
		k := http.NewKernel()
		k.Alias("tag", func(params ...string) (http.Middleware, error) {
			return tag(strings.Join(append([]string{"tag"}, params...), ":")), nil
		})
		k.Alias("broken", func(...string) (http.Middleware, error) { return nil, errors.New("no config") })
		k.Group("api", "tag:api,strict", "web")
		k.Group("web", "tag")
		k.Group("loop", "tag", "spin")
		k.Group("spin", "loop")

		return k
	}

	t.Run("passes parameters to the alias and expands groups in place", func(t *testing.T) {
		t.Parallel()
		mw, err := newKernel().Resolve("tag:first", "api")
		require.NoError(t, err)

		assert.Equal(t, []string{"tag:first", "tag:api:strict", "tag"}, trace(t, http.Chain(ok(), mw...)))
	})

	t.Run("refuses unknown names", func(t *testing.T) {
		t.Parallel()
		_, err := newKernel().Resolve("auth")

		require.ErrorIs(t, err, http.ErrUnknownMiddleware)
	})

	t.Run("refuses groups that list themselves", func(t *testing.T) {
		t.Parallel()
		_, err := newKernel().Resolve("loop")

		require.ErrorIs(t, err, http.ErrMiddlewareCycle)
	})

	t.Run("reports factory failures", func(t *testing.T) {
		t.Parallel()
		_, err := newKernel().Resolve("broken")

		require.ErrorContains(t, err, "no config")
	})

	t.Run("wraps every request in global middleware, first added outermost", func(t *testing.T) {
		t.Parallel()
		k := http.NewKernel()
		k.Use(tag("first"), tag("second"))
		k.Use(tag("third"))

		assert.Equal(t, []string{"first", "second", "third"}, trace(t, k.Handler(ok())))
	})
}
