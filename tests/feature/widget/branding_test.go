//go:build !cloud

package widget_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestSelfHostedBranding(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		choice any
		show   bool
	}{
		"hides the badge unless asked": {nil, false},
		"shows the badge when asked":   {true, true},
		"hides the badge when told":    {false, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tc := tests.New(t)

			if c.choice != nil {
				patch(t, tc, testkit.JSON{"show_branding": c.choice})
			}

			res := tc.Client().GetJSON(configPath(t, tc)).AssertOK()
			res.AssertJSONPath("data.branding.show", c.show).AssertJSONPath("data.branding.locked", false)
		})
	}
}
