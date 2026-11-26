package conversations_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
snooze puts the conversation to sleep for the duration and returns its
path.
*/
func (d desk) snooze(t *testing.T, id string, wait time.Duration) string {
	t.Helper()

	path := "/api/v1/conversations/" + id
	d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snoozed_until": d.tc.Clock.Now().Add(wait)}).
		AssertOK().AssertJSONPath("data.status", "snoozed")

	return path
}

/*
wake runs the job that wakes expired snoozes once.
*/
func wake(t *testing.T, tc *tests.TestCase) {
	t.Helper()

	job := &jobs.WakeExpiredSnoozes{Snoozes: sweeper(tc)}
	require.NoError(t, job.Handle(t.Context(), nil))
}

/*
sweeper returns the sweep the job runs, on the application's pool.
*/
func sweeper(tc *tests.TestCase) *conversations.WakeExpiredSnoozes {
	return &conversations.WakeExpiredSnoozes{DB: tc.App.DB, Events: tc.App.Events, Clock: tc.Clock}
}

/*
wakeUps counts the conversation's timeline entries saying a snooze
ran out.
*/
func wakeUps(t *testing.T, tc *tests.TestCase, conv ids.UUID) int {
	t.Helper()

	var n int
	require.NoError(t, tc.Operator().QueryRow(t.Context(), `SELECT count(*) FROM messages
		WHERE conversation_id = $1 AND type = 'event' AND event->>'name' = 'conversation.unsnoozed'
		  AND event->>'reason' = 'expired' AND event->'actor'->>'type' = 'system'`, conv).Scan(&n))

	return n
}

func TestSnoozeExpiry(t *testing.T) {
	t.Parallel()

	t.Run("wakes a snooze that ran out exactly once", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		unsnoozed := record[appevents.ConversationUnsnoozed](d.tc)
		id := d.customer(t, "ann", "Billing", "Charged twice")
		path := d.snooze(t, id, 2*time.Minute)

		d.tc.Clock.Travel(2*time.Minute + time.Second)
		wake(t, d.tc)
		wake(t, d.tc)

		d.owner.GetJSON(path).AssertOK().AssertJSONPath("data.status", "open").AssertJSONPath("data.snoozed_until", nil)
		assert.Equal(t, 1, wakeUps(t, d.tc, decode(t, d, models.PrefixConversation, id)))

		seen := unsnoozed.all()
		require.Len(t, seen, 1)
		assert.Equal(t, conversations.ReasonExpired, seen[0].Reason)
		assert.Equal(t, "system", seen[0].Actor.Type)
		assert.Equal(t, d.tc.Workspace.Workspace.ID, seen[0].WorkspaceID)
	})

	t.Run("leaves a snooze that has not run out asleep", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		unsnoozed := record[appevents.ConversationUnsnoozed](d.tc)
		path := d.snooze(t, d.customer(t, "ann", "Billing", "Charged twice"), time.Hour)

		d.tc.Clock.Travel(59 * time.Minute)
		wake(t, d.tc)

		d.owner.GetJSON(path).AssertOK().AssertJSONPath("data.status", "snoozed")
		assert.Empty(t, unsnoozed.all())
	})

	t.Run("wakes each workspace's snoozes inside that workspace", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		unsnoozed := record[appevents.ConversationUnsnoozed](d.tc)
		other, err := factories.Workspace(t.Context(), d.tc.App, d.tc.Owner)
		require.NoError(t, err)
		otherInbox, err := factories.DefaultInbox(t.Context(), d.tc.App, other.Workspace)
		require.NoError(t, err)
		elsewhere := desk{
			tc: d.tc, inbox: otherInbox,
			owner: d.tc.ActingAs(d.tc.Owner).
				WithHeader("Dimmi-Workspace", models.PublicID(models.PrefixWorkspace, other.Workspace.ID)),
		}

		here := d.customer(t, "ann", "Here", "Hi")
		there := elsewhere.customer(t, "bob", "There", "Hello")
		later := elsewhere.customer(t, "cat", "Later", "Hey")
		d.snooze(t, here, 2*time.Minute)
		elsewhere.snooze(t, there, 2*time.Minute)
		elsewhere.snooze(t, later, time.Hour)

		d.tc.Clock.Travel(3 * time.Minute)
		wake(t, d.tc)

		d.owner.GetJSON("/api/v1/conversations/"+here).AssertOK().AssertJSONPath("data.status", "open")
		elsewhere.owner.GetJSON("/api/v1/conversations/"+there).AssertOK().AssertJSONPath("data.status", "open")
		elsewhere.owner.GetJSON("/api/v1/conversations/"+later).AssertOK().AssertJSONPath("data.status", "snoozed")

		byWorkspace := map[ids.UUID]ids.UUID{}
		for _, e := range unsnoozed.all() {
			byWorkspace[e.ConversationID] = e.WorkspaceID
		}
		assert.Equal(t, map[ids.UUID]ids.UUID{
			decode(t, d, models.PrefixConversation, here):  d.tc.Workspace.Workspace.ID,
			decode(t, d, models.PrefixConversation, there): other.Workspace.ID,
		}, byWorkspace)

		var crossed int
		require.NoError(t, d.tc.Operator().QueryRow(t.Context(), `SELECT count(*) FROM messages m
			JOIN conversations c ON c.id = m.conversation_id
			WHERE m.event->>'name' = 'conversation.unsnoozed' AND m.workspace_id <> c.workspace_id`).Scan(&crossed))
		assert.Zero(t, crossed)
	})

	t.Run("never wakes a conversation twice when two workers sweep at once", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		unsnoozed := record[appevents.ConversationUnsnoozed](d.tc)
		var convs []ids.UUID
		for range 12 {
			id := d.customer(t, "ann", "Billing", "Charged twice")
			d.snooze(t, id, 2*time.Minute)
			convs = append(convs, decode(t, d, models.PrefixConversation, id))
		}

		d.tc.Clock.Travel(3 * time.Minute)
		var wg sync.WaitGroup
		errs := make(chan error, 4)
		for range 4 {
			wg.Go(func() { errs <- sweeper(d.tc).Sweep(t.Context()) })
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}

		for _, c := range convs {
			assert.Equal(t, 1, wakeUps(t, d.tc, c))
		}
		assert.Len(t, unsnoozed.all(), len(convs))
	})

	t.Run("skips a conversation another worker holds, and wakes it next time", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		held := d.customer(t, "ann", "Held", "Hi")
		free := d.customer(t, "bob", "Free", "Hello")
		d.snooze(t, held, 2*time.Minute)
		d.snooze(t, free, 2*time.Minute)
		d.tc.Clock.Travel(3 * time.Minute)

		ws := d.tc.Workspace.Workspace.ID
		ctx := tenancy.With(t.Context(), ws.String())
		locked, release := make(chan struct{}), make(chan struct{})
		done := make(chan error, 1)
		role := d.tc.ApplicationRole()
		go func() {
			done <- role.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
				_, err := models.New(tx).LockConversation(ctx, models.LockConversationParams{
					WorkspaceID: ws, ID: decode(t, d, models.PrefixConversation, held),
				})
				close(locked)
				<-release

				return err
			})
		}()
		<-locked

		wake(t, d.tc)
		d.owner.GetJSON("/api/v1/conversations/"+free).AssertOK().AssertJSONPath("data.status", "open")
		d.owner.GetJSON("/api/v1/conversations/"+held).AssertOK().AssertJSONPath("data.status", "snoozed")

		close(release)
		require.NoError(t, <-done)
		wake(t, d.tc)
		d.owner.GetJSON("/api/v1/conversations/"+held).AssertOK().AssertJSONPath("data.status", "open")
	})

	t.Run("tells the holder in the bell and reaches webhooks", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.customer(t, "ann", "Billing", "Charged twice")
		path := "/api/v1/conversations/" + id
		d.owner.PatchJSON(path, testkit.JSON{"assignee_id": d.ownerID()}).AssertOK()
		d.owner.PostJSON("/api/v1/webhook-endpoints", testkit.JSON{
			"url": "https://hooks.example.com/dimmi", "event_types": []string{"conversation.unsnoozed"},
		}).AssertCreated()
		d.snooze(t, id, 2*time.Minute)
		d.tc.Drain()

		d.tc.Clock.Travel(3 * time.Minute)
		wake(t, d.tc)
		d.tc.Drain()

		bell := d.owner.GetJSON("/api/v1/notifications").AssertOK()
		assert.Contains(t, sliceOf(bell, "data.#.trigger"), "snooze_expired")
		d.tc.Table("webhook_deliveries").AssertCount(1, testkit.Row{"event_type": "conversation.unsnoozed"})
	})
}
