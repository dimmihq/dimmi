package mailbox_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestBudget(t *testing.T) {
	t.Parallel()

	const wait = 20 * time.Millisecond

	t.Run("counts a message for twice its size", func(t *testing.T) {
		t.Parallel()
		budget := mailbox.NewBudget(100)

		release, err := budget.Acquire(t.Context(), 40, wait)
		require.NoError(t, err)
		defer release()

		second, err := budget.Acquire(t.Context(), 10, wait)
		require.NoError(t, err)
		defer second()

		_, err = budget.Acquire(t.Context(), 1, wait)
		require.ErrorIs(t, err, mailbox.ErrBudgetBusy)
	})

	t.Run("lets the largest message through alone", func(t *testing.T) {
		t.Parallel()
		budget := mailbox.NewBudget(100)

		release, err := budget.Acquire(t.Context(), 1000, wait)
		require.NoError(t, err)

		_, err = budget.Acquire(t.Context(), 0, wait)
		require.ErrorIs(t, err, mailbox.ErrBudgetBusy)

		release()

		again, err := budget.Acquire(t.Context(), -1, wait)
		require.NoError(t, err)
		again()
	})
}

func TestStatus(t *testing.T) {
	t.Parallel()

	t.Run("maps each result to what a provider expects", func(t *testing.T) {
		t.Parallel()

		for err, want := range map[error]int{
			mailbox.ErrRejected:     http.StatusUnauthorized,
			mailbox.ErrTooLarge:     http.StatusRequestEntityTooLarge,
			mailbox.ErrUnsubscribed: http.StatusOK,
			errors.New("disk"):      http.StatusServiceUnavailable,
		} {
			assert.Equal(t, want, mailbox.DefaultStatus(err))
		}

		assert.Equal(t, http.StatusOK, mailbox.Status(hook{}, nil))
	})
}

func TestReplays(t *testing.T) {
	t.Parallel()

	newReplays := func(t *testing.T) *mailbox.Replays {
		t.Helper()

		store, err := cache.Open(testkit.NewValkey(t), cache.Options{Keyspace: "cache"})
		require.NoError(t, err)
		t.Cleanup(store.Close)

		return mailbox.NewReplays(store)
	}

	t.Run("refuses only a token already accepted", func(t *testing.T) {
		t.Parallel()
		replays := newReplays(t)

		require.NoError(t, replays.Check(t.Context(), "hook", "token"))
		require.NoError(t, replays.Accepted("hook", "token")(t.Context()))

		require.ErrorIs(t, replays.Check(t.Context(), "hook", "token"), mailbox.ErrRejected)
		require.NoError(t, replays.Check(t.Context(), "other", "token"))
	})

	t.Run("fails closed when the store is down", func(t *testing.T) {
		t.Parallel()
		client := testkit.NewValkey(t)
		store, err := cache.Open(client, cache.Options{Keyspace: "cache"})
		require.NoError(t, err)
		replays := mailbox.NewReplays(store)
		store.Close()
		client.Close()

		err = replays.Check(t.Context(), "hook", "token")
		require.Error(t, err)
		assert.NotErrorIs(t, err, mailbox.ErrRejected)
		require.Error(t, replays.Accepted("hook", "token")(t.Context()))
	})
}

func TestMisconfiguredError(t *testing.T) {
	t.Parallel()

	t.Run("names the source and the setting", func(t *testing.T) {
		t.Parallel()

		err := &mailbox.MisconfiguredError{Source: "hook", Setting: "raw on"}

		assert.Contains(t, err.Error(), "hook")
		assert.Contains(t, err.Error(), "raw on")
	})
}

func TestDriverDefault(t *testing.T) {
	t.Parallel()

	t.Run("takes the driver from the name when it is left out", func(t *testing.T) {
		t.Parallel()
		m := mailbox.NewManager(mailbox.Config{Sources: map[string]config.Section{"hook": {}}})
		m.Extend("hook", func(name string, _ config.Section) (mailbox.Source, error) { return hook{name: name}, nil })

		source, err := m.Source("hook")

		require.NoError(t, err)
		assert.Equal(t, "hook", source.Name())
	})
}
