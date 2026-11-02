package encryption_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/encryption/drivertest"
)

func appKey(t *testing.T) []byte {
	t.Helper()

	key, err := encryption.ParseAppKey(encryption.GenerateAppKey())
	require.NoError(t, err)

	return key
}

func localKEK(t *testing.T) *encryption.LocalKEK {
	t.Helper()

	kek, err := encryption.NewLocalKEK(appKey(t))
	require.NoError(t, err)

	return kek
}

func newKeys(t *testing.T, opts encryption.KeysOptions) *encryption.Keys {
	t.Helper()

	if opts.KEK == nil {
		opts.KEK = localKEK(t)
	}

	if opts.Store == nil {
		opts.Store = drivertest.NewPostgresStore(t)
	}

	keys, err := encryption.NewKeys(opts)
	require.NoError(t, err)

	return keys
}

func tenant(id string) encryption.Scope {
	return encryption.Scope{Kind: "tenant", ID: id}
}
