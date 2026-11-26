package oauth_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestDimmiCLIApp(t *testing.T) {
	t.Parallel()

	t.Run("is a registered app the consent screen shows plainly", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		cli := app{tc: tc, id: "dimmi-cli"}

		res := tc.ActingAs(tc.Owner).GetJSON("/api/v1/oauth/authorization?query=" +
			url.QueryEscape(cli.query("settings:write offline_access"))).AssertOK()

		assert.Equal(t, "dimmi CLI", res.String("data.client_name"))
		assert.Equal(t, "false", res.String("data.self_registered"))
		assert.Equal(t, "true", res.String("data.loopback"))
	})

	t.Run("signs in on any loopback port and reaches only migrations", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		access, refresh := app{tc: tc, id: "dimmi-cli"}.connect(t, "settings:write offline_access")
		cli := tc.Client().WithHeader("Authorization", "Bearer "+access)

		assert.NotEmpty(t, refresh)
		cli.PostJSON("/api/v1/migrations", testkit.JSON{"source": "acme", "source_account": "1"}).AssertCreated()
		cli.GetJSON("/api/v1/conversations").AssertForbidden()
	})
}
