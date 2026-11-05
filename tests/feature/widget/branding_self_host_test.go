//go:build !cloud

package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestWidgetBranding(t *testing.T) {
	t.Parallel()

	type row struct {
		edition string
		paid    bool
		choice  any
		show    bool
		locked  bool
	}

	for name, c := range map[string]row{
		"self host unset":    {"self_host", false, nil, false, false},
		"self host opted in": {"self_host", false, true, true, false},
		"self host off":      {"self_host", false, false, false, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tc := tests.New(t)

			if c.choice != nil {
				patch(t, tc, testkit.JSON{"show_branding": c.choice})
			}

			res := tc.Client().GetJSON(configPath(t, tc)).AssertOK()
			res.AssertJSONPath("data.branding.show", c.show)
			res.AssertJSONPath("data.branding.locked", c.locked)
		})
	}

	t.Run("the badge links back with the workspace's slug", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		url := tc.Client().GetJSON(configPath(t, tc)).AssertOK().String("data.branding.url")

		assert.Contains(t, url, "utm_source=widget")
		assert.Contains(t, url, "ref="+tc.Workspace.Workspace.Slug)
	})
}
