package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestWidgetAllowedOrigins(t *testing.T) {
	t.Parallel()

	t.Run("stores sites in their normal form", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := patch(t, tc, testkit.JSON{"allowed_origins": []string{
			"https://ACME.com", "https://acme.com:443", "https://*.acme.com", "http://localhost:3000",
			"https://münchen.de",
		}})

		var got struct {
			Data struct {
				AllowedOrigins []string `json:"allowed_origins"`
			} `json:"data"`
		}

		res.Decode(&got)
		assert.Equal(t, []string{
			"https://acme.com", "https://*.acme.com", "http://localhost:3000", "https://xn--mnchen-3ya.de",
		}, got.Data.AllowedOrigins)
	})

	for name, bad := range map[string]string{
		"a path": "https://acme.com/app", "a trailing slash": "https://acme.com/", "no scheme": "acme.com",
		"a wildcard in the middle": "https://a.*.acme.com", "a public suffix wildcard": "https://*.com",
		"null": "null",
	} {
		t.Run("refuses "+name, func(t *testing.T) {
			t.Parallel()
			tc := tests.New(t)

			owner(tc).PatchJSON(settingsPath(t, tc), testkit.JSON{"allowed_origins": []string{"https://ok.com", bad}}).
				AssertStatus(fhttp.StatusUnprocessableEntity).AssertValidationError("allowed_origins.1")
		})
	}

	t.Run("refuses more than fifty", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		many := make([]string, 51)

		for i := range many {
			many[i] = "https://s" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + ".com"
		}

		owner(tc).PatchJSON(settingsPath(t, tc), testkit.JSON{"allowed_origins": many}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
	})
}
