package foundation

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/dimmihq/dimmi/framework/log"
)

/*
forwarded keeps the records a logger forwards.
*/
type forwarded struct {
	mu      sync.Mutex
	records []slog.Record
}

func (f *forwarded) Enabled(context.Context, slog.Level) bool { return true }

func (f *forwarded) Handle(_ context.Context, r slog.Record) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records = append(f.records, r)
	return nil
}

func (f *forwarded) WithAttrs([]slog.Attr) slog.Handler { return f }

func (f *forwarded) WithGroup(string) slog.Handler { return f }

func TestZapLoggerForwards(t *testing.T) {
	t.Parallel()
	out, sink := &bytes.Buffer{}, &forwarded{}
	logger, err := log.New(log.Options{Driver: "json", Level: "info", Writer: out, Forward: sink})
	require.NoError(t, err)

	zapLogger(logger).Named("river").Warn("job stuck", zap.String("queue", "mail"))

	assert.Contains(t, out.String(), "job stuck")
	require.Len(t, sink.records, 1)
	attrs := map[string]string{}
	sink.records[0].Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.String()
		return true
	})
	assert.Equal(t, "job stuck", sink.records[0].Message)
	assert.Equal(t, map[string]string{"queue": "mail", "logger": "river"}, attrs)
}
