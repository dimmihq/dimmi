package arf_test

import (
	"bytes"
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mailbox/arf"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()

	raw, err := fs.ReadFile(os.DirFS("testdata"), name)
	require.NoError(t, err)

	return raw
}

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("reads the complaint and the reported message", func(t *testing.T) {
		t.Parallel()

		got, err := arf.Parse(fixture(t, "abuse.eml"))
		require.NoError(t, err)

		assert.Equal(t, &arf.Report{
			FeedbackType: arf.TypeAbuse, UserAgent: "ISP-FBL/1.0", Version: "1",
			OriginalMailFrom: "bounces@shop.example", OriginalRcptTo: []string{"reader@isp.example"},
			ArrivalDate: "Mon, 02 Nov 2026 10:04:00 +0000", SourceIP: "192.0.2.5",
			ReportedDomain: "shop.example", OriginalMessageID: "out-3.ghi@shop.example",
		}, got)
		assert.True(t, got.Complaint())
	})

	t.Run("tells a not spam report from a complaint", func(t *testing.T) {
		t.Parallel()
		raw := bytes.Replace(fixture(t, "abuse.eml"), []byte("Type: Abuse"), []byte("Type: not-spam"), 1)

		got, err := arf.Parse(raw)
		require.NoError(t, err)

		assert.False(t, got.Complaint())
	})

	t.Run("refuses a message that is not a report", func(t *testing.T) {
		t.Parallel()

		_, err := arf.Parse(fixture(t, "plain.eml"))

		require.ErrorIs(t, err, arf.ErrNotARF)
	})

	t.Run("refuses a report without a feedback type", func(t *testing.T) {
		t.Parallel()
		raw := bytes.Replace(fixture(t, "abuse.eml"), []byte("Feedback-Type: Abuse"), []byte("X-Note: none"), 1)

		_, err := arf.Parse(raw)

		require.ErrorIs(t, err, arf.ErrNotARF)
	})
}
