package drivertest_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/hashing/drivers/bcrypt"
	"github.com/dimmihq/dimmi/framework/hashing/drivertest"
)

func TestRun(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, func(t *testing.T) hashing.Hasher {
		t.Helper()

		hasher, err := bcrypt.New(bcrypt.Options{Cost: 4})
		require.NoError(t, err)

		return hasher
	})
}
