package email_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/mailbox"

	"github.com/dimmihq/dimmi/tests"
)

func TestProviderKeptRaw(t *testing.T) {
	t.Parallel()

	t.Run("reads a message the provider keeps from the provider's disk", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, tests.WithEnv(
			"DIMMI_FILESYSTEMS__DISKS__LOCAL__DRIVER=local",
			"DIMMI_FILESYSTEMS__DISKS__LOCAL__ROOT="+t.TempDir(),
			"DIMMI_FILESYSTEMS__DISKS__PROVIDER__DRIVER=local",
			"DIMMI_FILESYSTEMS__DISKS__PROVIDER__ROOT="+t.TempDir(),
		))
		raw, address := inboundRaw(tc)

		disk, err := tc.App.Storage.Disk("provider")
		require.NoError(t, err)
		require.NoError(t, disk.Put(t.Context(), "raw/kept.eml", raw, filesystem.PutOptions{}))

		_, err = tc.App.Actions.ReceiveInboundEmail.Handle(t.Context(), mailbox.Envelope{
			Source: "test", ProviderMessageID: "kept-1", Recipients: []string{address},
			RawRef: &mailbox.RawRef{Disk: "provider", Path: "raw/kept.eml"}, SizeBytes: int64(len(raw)),
		})
		require.NoError(t, err)

		assertBecameConversation(tc, "test")
		assert.Equal(t, "provider", lastInbound(tc, defaultAddress(tc)).RawDisk)
	})
}
