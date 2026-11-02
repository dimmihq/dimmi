package log_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/log"
)

type account struct {
	Email    string
	Password string
	APIKey   string
	Note     string `masq:"secret"`
}

func TestAttrs(t *testing.T) {
	t.Parallel()

	t.Run("writes each attribute type", func(t *testing.T) {
		t.Parallel()
		logger, out := newJSON(t)
		at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)

		logger.InfoContext(t.Context(), "all types",
			log.Int64("bytes", 1024),
			log.Float64("ratio", 0.5),
			log.Bool("ok", true),
			log.Duration("took", 2*time.Second),
			log.Time("at", at),
			log.Group("job", log.String("queue", "default")),
		)

		line := onlyLine(t, out)
		assert.InDelta(t, 1024, line["bytes"], 0)
		assert.InDelta(t, 0.5, line["ratio"], 0)
		assert.Equal(t, true, line["ok"])
		assert.InDelta(t, float64(2*time.Second), line["took"], 0)
		assert.Equal(t, "2026-03-04T05:06:07Z", line["at"])
		assert.Equal(t, map[string]any{"queue": "default"}, line["job"])
	})

	t.Run("writes errors under the error key", func(t *testing.T) {
		t.Parallel()
		logger, out := newJSON(t)
		err := fmt.Errorf("send invoice: %w", errors.New("connection reset"))

		logger.ErrorContext(t.Context(), "job failed", log.Err(err))

		line := onlyLine(t, out)
		assert.Equal(t, "ERROR", line["level"])
		assert.Equal(t, "send invoice: connection reset", line["error"])
	})

	t.Run("hides secrets by key, field name and tag", func(t *testing.T) {
		t.Parallel()
		logger, out := newJSON(t)
		user := account{Email: "a@example.com", Password: "hunter2", APIKey: "k1", Note: "private"}

		logger.InfoContext(t.Context(), "account loaded",
			log.String("Access_Token", "abc"),
			log.Any("account", user),
			log.Group("headers", log.String("authorization", "Bearer abc")),
		)

		line := onlyLine(t, out)
		assert.Equal(t, "[REDACTED]", line["Access_Token"])
		assert.Equal(t, map[string]any{
			"Email":    "a@example.com",
			"Password": "[REDACTED]",
			"APIKey":   "[REDACTED]",
			"Note":     "[REDACTED]",
		}, line["account"])
		assert.Equal(t, map[string]any{"authorization": "[REDACTED]"}, line["headers"])
	})

	t.Run("hides secrets carried by the context", func(t *testing.T) {
		t.Parallel()
		logger, out := newJSON(t)
		ctx := log.With(t.Context(), log.String("request_id", "r1"), log.String("cookie", "c1"))

		logger.InfoContext(ctx, "request done")

		line := onlyLine(t, out)
		assert.Equal(t, "r1", line["request_id"])
		assert.Equal(t, "[REDACTED]", line["cookie"])
	})
}
