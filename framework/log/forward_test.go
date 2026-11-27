package log_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/log"
)

/*
recorder is a forwarded handler that keeps every record it is given,
with the attributes it was narrowed with.
*/
type recorder struct {
	mu      *sync.Mutex
	level   slog.Level
	attrs   []slog.Attr
	records *[]map[string]any
}

func newRecorder(level slog.Level) *recorder {
	return &recorder{mu: &sync.Mutex{}, level: level, records: &[]map[string]any{}}
}

func (r *recorder) Enabled(_ context.Context, level slog.Level) bool { return level >= r.level }

func (r *recorder) Handle(_ context.Context, record slog.Record) error {
	got := map[string]any{"msg": record.Message, "level": record.Level.String()}
	for _, a := range r.attrs {
		got[a.Key] = a.Value.Resolve().Any()
	}
	record.Attrs(func(a slog.Attr) bool {
		got[a.Key] = a.Value.Resolve().Any()
		return true
	})
	r.mu.Lock()
	*r.records = append(*r.records, got)
	r.mu.Unlock()
	return nil
}

func (r *recorder) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *r
	next.attrs = append(append([]slog.Attr{}, r.attrs...), attrs...)
	return &next
}

func (r *recorder) WithGroup(string) slog.Handler { return r }

func (r *recorder) got() []map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]map[string]any(nil), *r.records...)
}

func TestForward(t *testing.T) {
	t.Parallel()

	forwarded := func(t *testing.T, level slog.Level) (log.Logger, *bytes.Buffer, *recorder) {
		t.Helper()
		out, rec := &bytes.Buffer{}, newRecorder(level)
		logger, err := log.New(log.Options{Driver: "json", Level: "debug", Writer: out, Forward: rec})
		require.NoError(t, err)
		return logger, out, rec
	}

	t.Run("writes the same output and forwards at the forwarded level", func(t *testing.T) {
		t.Parallel()
		logger, out, rec := forwarded(t, slog.LevelInfo)
		plain, plainOut := newJSON(t)

		for _, l := range []log.Logger{logger, plain} {
			ctx := log.With(t.Context(), log.String("request_id", "req_1"))
			l.DebugContext(ctx, "cache warmed")
			l.InfoContext(ctx, "user signed in", log.String("user_id", "u1"))
		}

		assert.Len(t, lines(t, out), 2, "the output keeps the debug line")
		assert.Len(t, lines(t, plainOut), 2)
		got := rec.got()
		require.Len(t, got, 1, "debug stays below the forwarded level")
		assert.Equal(t, "user signed in", got[0]["msg"])
		assert.Equal(t, "u1", got[0]["user_id"])
		assert.Equal(t, "req_1", got[0]["request_id"], "context fields are forwarded")
	})

	t.Run("scrubs what leaves the process and leaves the output alone", func(t *testing.T) {
		t.Parallel()
		logger, out, rec := forwarded(t, slog.LevelInfo)

		logger.With(log.String("token", "tok_live")).WarnContext(t.Context(), "mail logged for ana@example.com",
			log.String("recipients", "ana@example.com"), log.String("subject", "Your refund"),
			log.String("links", "https://x.test/magic?token=abc"), log.String("message", "raw mime"),
			log.String("body", "Hi"), log.Err(errors.New("550 ana@example.com rejected")),
			log.String("api_key", "k"), log.Group("contact", log.String("note", "write to bo@example.org")))

		got := rec.got()
		require.Len(t, got, 1)
		line := got[0]
		assert.Equal(t, "mail logged for [email]", line["msg"])
		for _, key := range []string{"recipients", "subject", "links", "message", "body"} {
			assert.NotContains(t, line, key)
		}
		assert.Equal(t, "550 [email] rejected", line["error"])
		assert.NotEqual(t, "k", line["api_key"])
		assert.NotEqual(t, "tok_live", line["token"])
		assert.Contains(t, fmt.Sprint(line["contact"]), "write to [email]")
		assert.Contains(t, out.String(), "ana@example.com", "the output is unchanged")
		assert.Contains(t, out.String(), "Your refund")
	})
}
