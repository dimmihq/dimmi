//go:build cloud

package web_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/providers"
	"github.com/dimmihq/dimmi/tests"
)

func TestCloudBootGuards(t *testing.T) {
	t.Parallel()

	t.Run("refuses quietly wrong defaults in production and names each", func(t *testing.T) {
		t.Parallel()

		_, _, err := tests.Boot(t, tests.WithEnv(
			"DIMMI_APP__ENV=production",
			"DIMMI_APP__URL=http://localhost:8080",
			"DIMMI_MAIL__MAILERS__SYSTEM__DRIVER=log",
		))

		require.ErrorIs(t, err, providers.ErrCloudMisconfigured)
		keys := []string{"filesystems.default", "mail.default", "app.url"}
		for _, key := range keys {
			assert.ErrorContains(t, err, key)
		}
	})

	t.Run("passes a production configuration that names real services", func(t *testing.T) {
		t.Parallel()

		_, _, err := tests.Boot(t, tests.WithEnv(
			"DIMMI_APP__ENV=production",
			"DIMMI_APP__URL=https://staging.example.com",
			"DIMMI_FILESYSTEMS__DEFAULT=s3",
			"DIMMI_FILESYSTEMS__DISKS__S3__DRIVER=s3",
			"DIMMI_FILESYSTEMS__DISKS__S3__BUCKET=files",
			"DIMMI_MAIL__MAILERS__SYSTEM__DRIVER=smtp",
			"DIMMI_MAIL__MAILERS__SYSTEM__HOST=localhost",
			"DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN=inbound.example.com",
		))

		assert.NotErrorIs(t, err, providers.ErrCloudMisconfigured)
	})
}
