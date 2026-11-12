package helpcenters_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/customdomain"

	"github.com/dimmihq/dimmi/tests"
)

func TestHelpCenterHosts(t *testing.T) {
	t.Parallel()

	t.Run("allows a certificate for a help center's host and nothing else", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_HELP_CENTERS__BASE_DOMAIN=help.example.test"))
		sub := tc.In(tc.Owner).GetJSON("/api/v1/help-centers").AssertOK().String("data.0.subdomain")

		require.NoError(t, tc.App.AuthorizeDomain(t.Context(), sub+".help.example.test"))
		for _, host := range []string{
			"unknown.help.example.test", "help.example.test", "a." + sub + ".help.example.test", "example.com",
		} {
			require.ErrorIs(t, tc.App.AuthorizeDomain(t.Context(), host), customdomain.ErrNotAuthorized, host)
		}
	})

	t.Run("allows no help center host when help centers live under the app", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		require.ErrorIs(t, tc.App.AuthorizeDomain(t.Context(), "anything.help.example.test"),
			customdomain.ErrNotAuthorized)
	})
}
