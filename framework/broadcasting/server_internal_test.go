package broadcasting

import (
	"crypto/tls"
	"testing"

	"github.com/centrifugal/centrifuge"
	"github.com/redis/rueidis"
	"github.com/stretchr/testify/assert"
)

func TestShardConfig(t *testing.T) {
	t.Parallel()

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	base := rueidis.ClientOption{
		InitAddress: []string{"a:6379", "b:6379"},
		SelectDB:    2,
		Username:    "app",
		Password:    "pw",
		TLSConfig:   tlsConfig,
	}

	t.Run("connects to one server", func(t *testing.T) {
		t.Parallel()

		cfg := shardConfig(base, rueidis.ClientModeStandalone)

		assert.Equal(t, "a:6379", cfg.Address)
		assert.Equal(t, 2, cfg.DB)
		assert.Equal(t, "app", cfg.User)
		assert.Equal(t, "pw", cfg.Password)
		assert.Same(t, tlsConfig, cfg.TLSConfig)
	})

	t.Run("follows a cluster from its seeds", func(t *testing.T) {
		t.Parallel()

		cfg := shardConfig(base, rueidis.ClientModeCluster)

		assert.Empty(t, cfg.Address)
		assert.Equal(t, base.InitAddress, cfg.ClusterAddresses)
	})

	t.Run("asks sentinels for the primary", func(t *testing.T) {
		t.Parallel()
		option := base
		option.Sentinel = rueidis.SentinelOption{MasterSet: "primary", Username: "s", Password: "spw"}

		cfg := shardConfig(option, rueidis.ClientModeSentinel)

		assert.Equal(t, base.InitAddress, cfg.SentinelAddresses)
		assert.Equal(t, "primary", cfg.SentinelMasterName)
		assert.Equal(t, "s", cfg.SentinelUser)
		assert.Equal(t, "spw", cfg.SentinelPassword)
		assert.Equal(t, "pw", cfg.Password)
	})
}

func TestNoticed(t *testing.T) {
	t.Parallel()

	t.Run("ignores what is not an ended credential, or cannot be read", func(t *testing.T) {
		t.Parallel()
		s := &Server{}

		assert.NotPanics(t, func() {
			s.noticed(centrifuge.NotificationEvent{Op: "something_else", Data: []byte(`{}`)})
			s.noticed(centrifuge.NotificationEvent{Op: opCloseConnections, Data: []byte(`not json`)})
		})
	})
}
