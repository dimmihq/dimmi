//go:build cloud

package widget_test

import (
	"context"
	"testing"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/tests"
)

/*
paidStatus answers whether the workspace paid, standing in for the
billing provider, which is a true process boundary.
*/
type paidStatus bool

/*
HasPaid answers what the status says.
*/
func (p paidStatus) HasPaid(context.Context, ids.UUID) (bool, error) {
	return bool(p), nil
}

func TestCloudBranding(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		paid   bool
		choice any
		show   bool
		locked bool
	}{
		"a workspace that never paid shows a locked badge": {false, nil, true, true},
		"a paying workspace hides the badge unless asked":  {true, nil, false, false},
		"a paying workspace shows the badge when asked":    {true, true, true, false},
		"a paying workspace hides the badge when told":     {true, false, false, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tc := tests.New(t, tests.Before(func(a *app.App) { a.BillingStatus = paidStatus(c.paid) }))

			if c.choice != nil {
				patch(t, tc, testkit.JSON{"show_branding": c.choice})
			}

			res := tc.Client().GetJSON(configPath(t, tc)).AssertOK()
			res.AssertJSONPath("data.branding.show", c.show).AssertJSONPath("data.branding.locked", c.locked)
		})
	}

	t.Run("a workspace that never paid cannot hide the badge", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.BillingStatus = paidStatus(false) }))

		owner(tc).PatchJSON(settingsPath(t, tc), testkit.JSON{"show_branding": false}).
			AssertStatus(fhttp.StatusForbidden).AssertJSONPath("code", "branding_locked")
	})
}
