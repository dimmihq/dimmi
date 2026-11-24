package widget_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/tests"
)

func TestPurgeExpiredWidgetSessions(t *testing.T) {
	t.Parallel()

	t.Run("deletes anonymous sessions past their expiry and keeps the ones still alive", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		gone, alive := startVisitor(t, tc), startVisitor(t, tc)
		goneID := gone.session(t).ID
		tc.Clock.Travel(tc.App.Config.Widget.AnonymousSessionTTL - time.Hour)
		alive.client().GetJSON("/widget/v1/session").AssertOK()
		tc.Clock.Travel(2 * time.Hour)

		require.NoError(t, tc.App.Queue.Push(t.Context(), jobs.PurgeExpiredAccessRecordsArgs{}))
		tc.Drain()

		tc.Table("widget_sessions").AssertCount(1)
		tc.Table("widget_sessions").AssertCount(0, testkit.Row{"id": goneID})
		alive.client().GetJSON("/widget/v1/session").AssertOK()
	})
}
