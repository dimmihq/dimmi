/*
|--------------------------------------------------------------------------
| Account Contract
|--------------------------------------------------------------------------
|
| Every connected account must behave the same to the application: a
| cursor starts now, sent mail comes first, a stale cursor is told apart
| from a failure, a lost grant is told apart from both, and raw bytes come
| back untouched. A driver's tests answer as its provider would.
|
*/

package drivertest

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mailbox"
)

/*
AccountHarness is an account under test with its provider answering.

Serve makes the provider hold the messages as new, after the returned
cursor. Stale makes the cursor's feed answer as forgotten, Revoked makes
every call refuse the token, and Limited makes every call ask to slow
down. Raw makes the provider hold a raw message.
*/
type AccountHarness struct {
	Account mailbox.Account
	Session mailbox.Session
	Serve   func(t *testing.T, received, sent string) []byte
	Stale   func(t *testing.T) []byte
	Revoked func(t *testing.T) []byte
	Limited func(t *testing.T) []byte
	Raw     func(t *testing.T, id string, raw []byte)
}

/*
NewAccountHarness returns an account no other test uses.
*/
type NewAccountHarness func(t *testing.T) AccountHarness

/*
RunAccount runs the contract against accounts from the constructor.
*/
func RunAccount(t *testing.T, newHarness NewAccountHarness) {
	t.Helper()

	t.Run("feed", func(t *testing.T) { t.Parallel(); runAccountFeed(t, newHarness) })
	t.Run("failures", func(t *testing.T) { t.Parallel(); runAccountFailures(t, newHarness) })
}

func runAccountFeed(t *testing.T, newHarness NewAccountHarness) {
	t.Helper()

	t.Run("lists sent mail before received mail", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		page, err := h.Account.Changes(t.Context(), h.Session, h.Serve(t, "in-1", "out-1"))
		require.NoError(t, err)

		require.Len(t, page.Items, 2)
		assert.Equal(t, mailbox.FolderSent, page.Items[0].Folder)
		assert.Equal(t, "out-1", page.Items[0].ID)
		assert.Equal(t, mailbox.FolderReceived, page.Items[1].Folder)
		assert.Equal(t, "in-1", page.Items[1].ID)
		assert.NotEmpty(t, page.Cursor)
	})

	t.Run("hands back the raw bytes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		raw := NewDelivery().Raw
		h.Raw(t, "m-1", raw)

		got, err := h.Account.Raw(t.Context(), h.Session, "m-1")

		require.NoError(t, err)
		assert.Equal(t, raw, got)
	})
}

func runAccountFailures(t *testing.T, newHarness NewAccountHarness) {
	t.Helper()

	t.Run("tells a stale cursor apart from a failure", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		_, err := h.Account.Changes(t.Context(), h.Session, h.Stale(t))

		require.ErrorIs(t, err, mailbox.ErrCursorExpired)
	})

	t.Run("tells a cursor it cannot read as stale", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		_, err := h.Account.Changes(t.Context(), h.Session, []byte("nonsense"))

		require.ErrorIs(t, err, mailbox.ErrCursorExpired)
	})

	t.Run("tells a lost grant apart from both", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		_, err := h.Account.Changes(t.Context(), h.Session, h.Revoked(t))

		require.ErrorIs(t, err, mailbox.ErrGrantRevoked)
	})

	t.Run("passes on how long to slow down", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		_, err := h.Account.Changes(t.Context(), h.Session, h.Limited(t))

		limited, ok := errors.AsType[*mailbox.RateLimitedError](err)
		require.True(t, ok, "%v", err)
		assert.Equal(t, 30*time.Second, limited.RetryAfter)
		assert.Contains(t, limited.Error(), "30s")
	})
}
