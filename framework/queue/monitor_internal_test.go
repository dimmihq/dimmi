package queue

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMonitors(t *testing.T) {
	t.Parallel()

	t.Run("checks in a monitored kind's run and its end", func(t *testing.T) {
		t.Parallel()
		var started []Monitor
		var ended []error
		m := monitors{byKind: map[string]Monitor{}, checkIn: func(monitor Monitor) func(error) {
			started = append(started, monitor)

			return func(err error) { ended = append(ended, err) }
		}}
		monitor := Monitor{Slug: "prune-audit-logs", Schedule: "10 3 * * *", Timezone: "UTC"}
		m.add("prune_audit_logs", monitor)

		failed := errors.New("database gone")
		m.start("prune_audit_logs")(failed)
		m.start("send_email")(nil)

		assert.Equal(t, []Monitor{monitor}, started)
		assert.Equal(t, []error{failed}, ended)
	})

	t.Run("checks in a postponed run as a success", func(t *testing.T) {
		t.Parallel()
		failed := errors.New("database gone")

		assert.NoError(t, checkInError(nil))
		assert.NoError(t, checkInError(Postpone(time.Second)))
		assert.Equal(t, failed, checkInError(failed))
		assert.Error(t, checkInError(Cancel(failed)))
		assert.Error(t, checkInError(RetryAfter(time.Second)))
	})

	t.Run("does nothing without a check-in", func(t *testing.T) {
		t.Parallel()
		m := monitors{byKind: map[string]Monitor{}}
		m.add("prune_audit_logs", Monitor{Slug: "prune-audit-logs"})

		m.start("prune_audit_logs")(nil)
	})
}
