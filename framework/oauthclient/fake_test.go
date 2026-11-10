package oauthclient_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/oauthclient/drivertest"
)

func TestFakeDriver(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, func(t *testing.T) drivertest.Harness {
		t.Helper()
		m, fake := newManager(t)
		driver, err := m.Driver("chat")
		require.NoError(t, err)

		return drivertest.Harness{
			Driver: driver, Revokes: true,
			Approve: func(t *testing.T, authURL string) string {
				t.Helper()
				callback, err := fake.Approve(authURL, oauthclient.Grant{})
				require.NoError(t, err)

				return callback
			},
		}
	})
}
