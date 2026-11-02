package drivertest_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/encryption/drivertest"
)

func TestRunKEK(t *testing.T) {
	t.Parallel()

	drivertest.RunKEK(t, func(t *testing.T) encryption.KEK {
		t.Helper()

		key, err := encryption.ParseAppKey(encryption.GenerateAppKey())
		require.NoError(t, err)
		kek, err := encryption.NewLocalKEK(key)
		require.NoError(t, err)

		return kek
	})
}

func TestRunStore(t *testing.T) {
	t.Parallel()

	drivertest.RunStore(t, func(t *testing.T) encryption.KeysetStore {
		t.Helper()

		return drivertest.NewPostgresStore(t)
	})
}
