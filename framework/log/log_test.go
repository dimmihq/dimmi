package log_test

import (
	"bytes"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/log"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("writes json lines with typed attributes", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}

		logger, err := log.New(log.Options{Driver: "json", Level: "info", Writer: out})
		require.NoError(t, err)
		logger.InfoContext(t.Context(), "user signed in", log.String("user_id", "u1"), log.Int("attempts", 2))

		line := onlyLine(t, out)
		assert.Equal(t, "INFO", line["level"])
		assert.Equal(t, "user signed in", line["msg"])
		assert.Equal(t, "u1", line["user_id"])
		assert.InDelta(t, 2, line["attempts"], 0)
		assert.Contains(t, line, "time")
	})

	t.Run("writes plain text for local development", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}

		logger, err := log.New(log.Options{Driver: "text", Level: "debug", Writer: out})
		require.NoError(t, err)
		logger.DebugContext(t.Context(), "cache warmed", log.String("store", "main"), log.String("password", "p"))

		assert.Contains(t, out.String(), `level=DEBUG msg="cache warmed" store=main password=[REDACTED]`)
	})

	t.Run("drops lines below the level", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}

		logger, err := log.New(log.Options{Driver: "json", Level: "warn", Writer: out})
		require.NoError(t, err)
		logger.InfoContext(t.Context(), "quiet")
		logger.WarnContext(t.Context(), "loud")

		assert.Equal(t, "loud", onlyLine(t, out)["msg"])
	})

	t.Run("writes to stdout without a writer", func(t *testing.T) {
		t.Parallel()

		logger, err := log.New(log.Options{Driver: "json", Level: "info"})

		require.NoError(t, err)
		assert.NotNil(t, logger.Handler())
	})

	t.Run("rejects an unknown driver", func(t *testing.T) {
		t.Parallel()

		_, err := log.New(log.Options{Driver: "xml", Level: "info"})

		require.ErrorIs(t, err, log.ErrUnknownDriver)
		require.ErrorContains(t, err, `driver "xml"`)
	})

	t.Run("rejects an unknown level", func(t *testing.T) {
		t.Parallel()

		_, err := log.New(log.Options{Driver: "json", Level: "loud"})

		require.ErrorIs(t, err, log.ErrInvalidLevel)
		require.ErrorContains(t, err, `level "loud"`)
	})

	t.Run("redacts the extra names from the options", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}

		logger, err := log.New(log.Options{Driver: "json", Level: "info", Redact: []string{"tax_id"}, Writer: out})
		require.NoError(t, err)
		logger.InfoContext(t.Context(), "profile saved", log.String("tax_id", "123-45"), log.String("city", "Oslo"))

		line := onlyLine(t, out)
		assert.Equal(t, "[REDACTED]", line["tax_id"])
		assert.Equal(t, "Oslo", line["city"])
	})

	t.Run("writes whole lines from many goroutines", func(t *testing.T) {
		t.Parallel()
		logger, out := newJSON(t)

		var wg sync.WaitGroup
		for i := range 20 {
			wg.Go(func() {
				logger.InfoContext(t.Context(), "worker done", log.Int("worker", i))
			})
		}
		wg.Wait()

		assert.Len(t, lines(t, out), 20)
	})
}
