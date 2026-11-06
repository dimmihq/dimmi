package accesspolicy_test

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

func TestAccessRuleOrder(t *testing.T) {
	t.Parallel()

	t.Run("the first rule failed is the one reported, in order, and no database is needed", func(t *testing.T) {
		t.Parallel()
		policy := appauth.DefaultAccessPolicy(clock.NewFake(tests.Epoch()))
		connection := ids.New()
		maxAge := 60

		settings, err := json.Marshal(map[string]any{"security": models.SecuritySettings{
			RequireTwoFactor:         true,
			AllowedSignInMethods:     []string{"google"},
			GoogleAllowedDomains:     []string{"a.example"},
			SessionMaxAge:            &maxAge,
			DashboardAllowlist:       []models.AllowedAddress{{CIDR: "203.0.113.0/24", Label: "x"}},
			SSOEnforcedConnectionIDs: []ids.UUID{connection},
		}})
		require.NoError(t, err)

		w := models.Workspace{Settings: settings}
		caller := &appauth.Principal{
			Kind: appauth.KindSession, IP: netip.MustParseAddr("198.51.100.1"),
			SignIn: appauth.SignIn{Method: "password", Methods: []string{"password"}},
		}

		expect := func(code string) {
			t.Helper()

			err := policy.Evaluate(t.Context(), caller, w, enums.RoleMember)
			var f *fault.Error
			require.ErrorAs(t, err, &f)
			assert.Equal(t, code, f.Code())
		}

		expect("ip_not_allowed")
		caller.IP = netip.MustParseAddr("203.0.113.1")
		expect("sso_required")
		caller.SSOConnectionID = connection
		expect("sign_in_method_required")
		caller.SignIn.Method = "google"
		expect("sign_in_method_required")
		caller.AuthDomain = "a.example"
		expect("reauthentication_required")
		caller.AuthenticatedAt = tests.Epoch()
		expect("two_factor_required")
	})

	t.Run("a session from another connection does not satisfy the rule", func(t *testing.T) {
		t.Parallel()
		enforced := ids.New()
		rule := appauth.SSORule{Redirect: appauth.SSORedirectPath}
		check := appauth.AccessCheck{
			Principal: &appauth.Principal{Kind: appauth.KindSession, SSOConnectionID: ids.New()},
			Settings:  models.SecuritySettings{SSOEnforcedConnectionIDs: []ids.UUID{enforced}},
		}

		err := rule.Check(check)
		var f *fault.Error
		require.ErrorAs(t, err, &f)
		assert.Equal(t, "sso_required", f.Code())
		assert.Contains(t, f.Meta()["redirect_url"], "/auth/sso/ssoc_")

		check.Principal.SSOConnectionID = enforced
		require.NoError(t, rule.Check(check))
	})

	t.Run("two factor passes for a session through one of the organization's connections only", func(t *testing.T) {
		t.Parallel()
		connection := ids.New()
		rule := appauth.RequireTwoFactor{ActionURL: "/security"}
		check := appauth.AccessCheck{
			Principal: &appauth.Principal{Kind: appauth.KindSession, SSOConnectionID: connection},
			Settings:  models.SecuritySettings{RequireTwoFactor: true, SSOConnectionIDs: []ids.UUID{connection}},
		}

		require.NoError(t, rule.Check(check))

		check.Settings.SSOConnectionIDs = []ids.UUID{ids.New()}
		require.Error(t, rule.Check(check))
	})
}
