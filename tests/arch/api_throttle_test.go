package arch_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/tests"
)

func TestAPIThrottle(t *testing.T) {
	t.Parallel()

	t.Run("every signed-in api route passes the api rate limits", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		checked := 0
		for _, route := range tc.App.Router.Routes() {
			names := route.MiddlewareNames()
			signedIn := slices.ContainsFunc(names, func(name string) bool {
				return name == "auth" || strings.HasPrefix(name, "auth:")
			})

			if !strings.HasPrefix(route.Path(), "/api/v1/") || !signedIn {
				continue
			}

			checked++
			assert.Contains(t, names, "api.throttle", "%s %s", route.Method(), route.Path())
		}

		assert.Positive(t, checked)
	})
}
