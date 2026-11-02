package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/resilience"
)

var errDown = errors.New("provider down")

func TestBreaker(t *testing.T) {
	t.Parallel()

	t.Run("passes calls and their errors through while closed", func(t *testing.T) {
		t.Parallel()
		b := resilience.NewBreaker("llm", resilience.Settings{})

		require.NoError(t, b.Execute(func() error { return nil }))
		require.ErrorIs(t, b.Execute(func() error { return errDown }), errDown)
	})

	t.Run("opens after the failures in a row and stops calling", func(t *testing.T) {
		t.Parallel()
		b := resilience.NewBreaker("llm", resilience.Settings{Failures: 2})
		calls := 0
		fail := func() error { calls++; return errDown }

		require.ErrorIs(t, b.Execute(fail), errDown)
		require.ErrorIs(t, b.Execute(fail), errDown)
		err := b.Execute(fail)

		require.ErrorIs(t, err, resilience.ErrOpen)
		assert.Contains(t, err.Error(), "llm")
		assert.Equal(t, 2, calls)
	})

	t.Run("lets a probe through after the cooldown and closes on success", func(t *testing.T) {
		t.Parallel()
		b := resilience.NewBreaker("llm", resilience.Settings{Failures: 1, Cooldown: 20 * time.Millisecond})

		require.ErrorIs(t, b.Execute(func() error { return errDown }), errDown)
		require.ErrorIs(t, b.Execute(func() error { return nil }), resilience.ErrOpen)
		clock.System().Sleep(40 * time.Millisecond)

		require.NoError(t, b.Execute(func() error { return nil }))
		require.NoError(t, b.Execute(func() error { return nil }))
	})

	t.Run("ignores errors the settings do not count as failures", func(t *testing.T) {
		t.Parallel()
		errBadInput := errors.New("bad input")
		b := resilience.NewBreaker("llm", resilience.Settings{
			Failures:  1,
			IsFailure: func(err error) bool { return !errors.Is(err, errBadInput) },
		})

		require.ErrorIs(t, b.Execute(func() error { return errBadInput }), errBadInput)
		require.ErrorIs(t, b.Execute(func() error { return errBadInput }), errBadInput)
		require.ErrorIs(t, b.Execute(func() error { return errDown }), errDown)
		require.ErrorIs(t, b.Execute(func() error { return nil }), resilience.ErrOpen)
	})

	t.Run("never counts a canceled context", func(t *testing.T) {
		t.Parallel()
		b := resilience.NewBreaker("llm", resilience.Settings{Failures: 1})

		for range 3 {
			require.ErrorIs(t, b.Execute(func() error { return context.Canceled }), context.Canceled)
		}

		require.NoError(t, b.Execute(func() error { return nil }))
	})
}
