package webhook_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/internal/webhook"
)

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("broken") }

func TestRead(t *testing.T) {
	t.Parallel()

	t.Run("returns a body up to the limit", func(t *testing.T) {
		t.Parallel()

		body, err := webhook.Read(mailbox.Delivery{Body: bytes.NewReader([]byte("1234"))}, 4)

		require.NoError(t, err)
		assert.Equal(t, []byte("1234"), body)
	})

	t.Run("refuses a body over the limit", func(t *testing.T) {
		t.Parallel()

		_, err := webhook.Read(mailbox.Delivery{Body: bytes.NewReader([]byte("12345"))}, 4)

		require.ErrorIs(t, err, mailbox.ErrTooLarge)
	})

	t.Run("treats no body as empty", func(t *testing.T) {
		t.Parallel()

		body, err := webhook.Read(mailbox.Delivery{}, 4)

		require.NoError(t, err)
		assert.Nil(t, body)
	})

	t.Run("reports a body that cannot be read", func(t *testing.T) {
		t.Parallel()

		_, err := webhook.Read(mailbox.Delivery{Body: brokenReader{}}, 4)

		require.Error(t, err)
		assert.NotErrorIs(t, err, mailbox.ErrRejected)
	})
}

func TestForm(t *testing.T) {
	t.Parallel()

	t.Run("reads a url-encoded form when the type is missing", func(t *testing.T) {
		t.Parallel()

		values, err := webhook.Form("", []byte("a=b&c=d"))

		require.NoError(t, err)
		assert.Equal(t, "d", values.Get("c"))
	})

	t.Run("rejects a form that cannot be read", func(t *testing.T) {
		t.Parallel()

		_, err := webhook.Form("", []byte("%zz"))

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("rejects a multipart body cut short", func(t *testing.T) {
		t.Parallel()

		_, err := webhook.Form("multipart/form-data; boundary=x", []byte("--x\r\nContent-Dispo"))

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})
}

func TestVerdicts(t *testing.T) {
	t.Parallel()

	t.Run("maps a provider's words", func(t *testing.T) {
		t.Parallel()

		for word, want := range map[string]mailbox.Verdict{
			"pass (ok)": mailbox.VerdictPass, "SoftFail": mailbox.VerdictFail, "neutral": mailbox.VerdictNone,
			"temperror": mailbox.VerdictUnknown, "other": "", "": "",
		} {
			assert.Equal(t, want, webhook.Verdict(word), word)
		}
	})

	t.Run("maps a spam answer", func(t *testing.T) {
		t.Parallel()

		for word, want := range map[string]mailbox.Verdict{
			"Yes, score=9": mailbox.VerdictFail, "No": mailbox.VerdictPass, "maybe": "", "": "",
		} {
			assert.Equal(t, want, webhook.SpamVerdict(word), word)
		}
	})

	t.Run("reads no headers from a message too broken to read", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, webhook.First(webhook.Headers([]byte("not a message")), "Subject"))
	})

	t.Run("believes any top header when no server is asked for", func(t *testing.T) {
		t.Parallel()

		raw := []byte("Authentication-Results: mx.any.example; dkim=fail; dkim=pass\r\n\r\n")
		verdicts, ok := webhook.TopAuthResults(raw, "")

		assert.True(t, ok)
		assert.Equal(t, mailbox.VerdictPass, verdicts.DKIM)
	})

	t.Run("believes no top header it cannot parse", func(t *testing.T) {
		t.Parallel()

		_, ok := webhook.TopAuthResults([]byte("Authentication-Results: ;;;\r\n\r\n"), "")

		assert.False(t, ok)
	})
}

func TestHelpers(t *testing.T) {
	t.Parallel()

	t.Run("opens the raw bytes again and again", func(t *testing.T) {
		t.Parallel()
		open := webhook.Opener([]byte("raw"))

		for range 2 {
			reader, err := open(t.Context())
			require.NoError(t, err)
			content, err := io.ReadAll(reader)
			require.NoError(t, err)
			assert.Equal(t, []byte("raw"), content)
		}
	})

	t.Run("keeps each recipient once and drops blanks", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{"a@x", "b@x"}, webhook.Recipients([]string{" a@x", ""}, []string{"a@x", "b@x"}))
	})

	t.Run("reads the null sender as empty", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, webhook.Sender("<>"))
		assert.Equal(t, "a@x", webhook.Sender(" <a@x> "))
	})
}
