package dsn_test

import (
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mailbox/dsn"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()

	raw, err := fs.ReadFile(os.DirFS("testdata"), name)
	require.NoError(t, err)

	return raw
}

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("reads every recipient and the original message id", func(t *testing.T) {
		t.Parallel()

		got, err := dsn.Parse(fixture(t, "headers.eml"))
		require.NoError(t, err)

		assert.Equal(t, "mx.shop.example", got.ReportingMTA)
		assert.Equal(t, "env-42", got.OriginalEnvelopeID)
		assert.Equal(t, "out-1.abc@shop.example", got.OriginalMessageID)
		require.Len(t, got.Recipients, 2)
		assert.Equal(t, dsn.Recipient{
			FinalRecipient: "gone@far.example", OriginalRecipient: "gone@far.example", Action: "failed",
			Status: "5.1.1", DiagnosticCode: "550 5.1.1 User unknown", RemoteMTA: "mx.far.example",
		}, got.Recipients[0])
		assert.True(t, got.Recipients[0].Permanent())
		assert.Equal(t, "delayed", got.Recipients[1].Action)
		assert.False(t, got.Recipients[1].Permanent())
	})

	t.Run("reads a report that includes the whole original message", func(t *testing.T) {
		t.Parallel()

		got, err := dsn.Parse(fixture(t, "rfc822.eml"))
		require.NoError(t, err)

		assert.Equal(t, "out-2.def@shop.example", got.OriginalMessageID)
		assert.Equal(t, "NoOne@Far.Example", got.Recipients[0].FinalRecipient)
		assert.True(t, got.Recipients[0].Permanent())
	})

	t.Run("refuses a message that is not a report", func(t *testing.T) {
		t.Parallel()

		_, err := dsn.Parse(fixture(t, "plain.eml"))

		require.ErrorIs(t, err, dsn.ErrNotDSN)
	})

	t.Run("refuses a report without recipients", func(t *testing.T) {
		t.Parallel()
		raw := "Content-Type: multipart/report; boundary=b\r\n\r\n" +
			"--b\r\nContent-Type: message/delivery-status\r\n\r\n" +
			"Reporting-MTA: dns; mx.example\r\n\r\n--b--\r\n"

		got, err := dsn.Parse([]byte(raw))

		require.ErrorIs(t, err, dsn.ErrNotDSN)
		assert.Nil(t, got)
	})
}
