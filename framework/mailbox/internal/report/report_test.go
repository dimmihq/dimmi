package report_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mailbox/internal/report"
)

func TestRead(t *testing.T) {
	t.Parallel()

	t.Run("finds the machine part and the original header anywhere", func(t *testing.T) {
		t.Parallel()
		raw := "Content-Type: multipart/report; boundary=b\r\n\r\n" +
			"--b\r\nContent-Type: message/x-status\r\n\r\nA: 1\r\n\r\n\r\nB: 2\r\n" +
			"--b\r\nContent-Type: text/rfc822-headers\r\n\r\nMessage-ID: < m1@example.com >\r\n--b--\r\n"

		got, err := report.Read([]byte(raw), "message/x-status")
		require.NoError(t, err)

		require.Len(t, got.Fields, 2)
		assert.Equal(t, "1", got.Fields[0].Get("A"))
		assert.Equal(t, "2", got.Fields[1].Get("B"))
		assert.Equal(t, "m1@example.com", got.MessageID())
	})

	t.Run("reports no message id without the original", func(t *testing.T) {
		t.Parallel()
		raw := "Content-Type: multipart/report; boundary=b\r\n\r\n" +
			"--b\r\nContent-Type: message/x-status\r\n\r\nA: 1\r\n--b--\r\n"

		got, err := report.Read([]byte(raw), "message/x-status")
		require.NoError(t, err)

		assert.Empty(t, got.MessageID())
	})

	t.Run("treats an unreadable original header as absent", func(t *testing.T) {
		t.Parallel()
		raw := "Content-Type: multipart/report; boundary=b\r\n\r\n" +
			"--b\r\nContent-Type: message/x-status\r\n\r\nA: 1\r\n" +
			"--b\r\nContent-Type: message/rfc822\r\n\r\nno header here\r\n--b--\r\n"

		got, err := report.Read([]byte(raw), "message/x-status")
		require.NoError(t, err)

		assert.Nil(t, got.Original)
	})

	t.Run("refuses fields it cannot read", func(t *testing.T) {
		t.Parallel()
		raw := "Content-Type: multipart/report; boundary=b\r\n\r\n" +
			"--b\r\nContent-Type: message/x-status\r\n\r\nnot a field\r\n--b--\r\n"

		_, err := report.Read([]byte(raw), "message/x-status")

		require.ErrorIs(t, err, report.ErrNotReport)
	})

	t.Run("refuses a report without the machine part", func(t *testing.T) {
		t.Parallel()
		raw := "Content-Type: multipart/report; boundary=b\r\n\r\n" +
			"--b\r\nContent-Type: text/plain\r\n\r\nhi\r\n--b--\r\n"

		_, err := report.Read([]byte(raw), "message/x-status")

		require.ErrorIs(t, err, report.ErrNotReport)
	})

	t.Run("refuses a broken document", func(t *testing.T) {
		t.Parallel()

		_, err := report.Read([]byte("Content-Type: multipart/report; boundary=b\r\n\r\n--b\r\n"), "x")

		require.ErrorIs(t, err, report.ErrNotReport)
	})
}

func TestValue(t *testing.T) {
	t.Parallel()

	t.Run("drops the type label of a field", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "ana@example.com", report.Value("rfc822; ana@example.com"))
		assert.Equal(t, "plain", report.Value(" plain "))
	})
}
