package channels_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/erasures"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
holdEverything is a retention guard that keeps every record.
*/
type holdEverything struct{}

/*
MayDestroy holds everything.
*/
func (holdEverything) MayDestroy(context.Context, ids.UUID, contracts.RetentionSubject) (bool, error) {
	return false, nil
}

/*
seedDelivery keeps a delivery on the connection in the status, received
the given number of days ago, and returns its id.
*/
func seedDelivery(sc *slackCase, key, status string, daysAgo int) ids.UUID {
	sc.T.Helper()

	var id ids.UUID
	received := sc.Clock.Now().Add(-time.Duration(daysAgo) * 24 * time.Hour)
	require.NoError(sc.T, sc.Operator().QueryRow(sc.T.Context(), `
		INSERT INTO channel_inbound_events (workspace_id, channel_connection_id, provider, transport, dedupe_key,
			kind, payload, status, received_at)
		VALUES ($1, $2, 'slack', 'webhook', $3, 'message', '{"event":{"user":"U0SEED"}}', $4, $5)
		RETURNING id`,
		sc.Workspace.Workspace.ID, sc.conn.ID, key, status, received).Scan(&id))

	return id
}

/*
pruneJob is the nightly prune with the default periods and the guard.
*/
func pruneJob(sc *slackCase, guard contracts.RetentionGuard) *jobs.PruneChannelInboundEvents {
	return &jobs.PruneChannelInboundEvents{
		DB: sc.App.DB, Clock: sc.Clock, Retention: guard, ProcessedRetentionDays: 7, FailedRetentionDays: 30,
	}
}

func TestSlackInboundEventRetention(t *testing.T) {
	t.Parallel()

	t.Run("prunes finished deliveries after a week and failed or held ones after a month", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		recent := seedDelivery(sc, "recent", "processed", 2)
		old := seedDelivery(sc, "old", "processed", 8)
		ignored := seedDelivery(sc, "ignored", "ignored", 8)
		failed := seedDelivery(sc, "failed", "failed", 8)
		held := seedDelivery(sc, "held", "held", 20)
		staleFailed := seedDelivery(sc, "stale-failed", "failed", 31)
		staleHeld := seedDelivery(sc, "stale-held", "held", 31)
		pending := seedDelivery(sc, "pending", "pending", 40)

		require.NoError(t, pruneJob(sc, sc.App.Retention).Handle(t.Context(), nil))

		events := sc.Table("channel_inbound_events")
		for _, kept := range []ids.UUID{recent, failed, held, pending} {
			events.AssertCount(1, testkit.Row{"id": kept})
		}
		for _, gone := range []ids.UUID{old, ignored, staleFailed, staleHeld} {
			events.AssertCount(0, testkit.Row{"id": gone})
		}
	})

	t.Run("keeps every delivery of a workspace the retention guard holds", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		old := seedDelivery(sc, "old", "processed", 60)

		require.NoError(t, pruneJob(sc, holdEverything{}).Handle(t.Context(), nil))

		sc.Table("channel_inbound_events").AssertCount(1, testkit.Row{"id": old})
	})

	t.Run("erasing a customer deletes the deliveries that name them and no others", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.deliver(message(supportRoom, "U0CUSTOMER", rootTS, "My card is 4111"))
		sc.deliver(message(supportRoom, "U0OTHER", secondTS, "Unrelated question"))
		convs := sc.conversations()
		require.Len(t, convs, 2)
		sc.Table("channel_inbound_events").AssertCount(2, testkit.Row{"workspace_id": sc.Workspace.Workspace.ID})

		contact := sc.App.IDs.Encode(string(models.PrefixContact), convs[0].ContactID)
		erasure := sc.ActingAsOwner().Delete("/api/v1/contacts/" + contact + "?erase=true").
			AssertStatus(202).String("data.id")
		sc.Clock.Travel(8 * 24 * time.Hour)
		erasureID, err := sc.App.IDs.Decode(string(models.PrefixContactErasure), erasure)
		require.NoError(t, err)

		outcome, err := sc.App.Actions.Inbox.EraseContact.Handle(t.Context(), erasures.EraseArgs{
			WorkspaceID: sc.Workspace.Workspace.ID, ErasureID: erasureID,
		})
		require.NoError(t, err)
		assert.Equal(t, erasures.Done, outcome)

		var customer, other int
		require.NoError(t, sc.Operator().QueryRow(t.Context(), `
			SELECT count(*) FILTER (WHERE payload->'event'->>'user' = 'U0CUSTOMER'),
				count(*) FILTER (WHERE payload->'event'->>'user' = 'U0OTHER')
			FROM channel_inbound_events WHERE workspace_id = $1`,
			sc.Workspace.Workspace.ID).Scan(&customer, &other))
		assert.Zero(t, customer)
		assert.Equal(t, 1, other)
	})

	t.Run("a held erasure keeps the customer's deliveries", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.deliver(message(supportRoom, "U0CUSTOMER", rootTS, "My card is 4111"))
		convs := sc.conversations()
		require.Len(t, convs, 1)

		contact := sc.App.IDs.Encode(string(models.PrefixContact), convs[0].ContactID)
		erasure := sc.ActingAsOwner().Delete("/api/v1/contacts/" + contact + "?erase=true").
			AssertStatus(202).String("data.id")
		sc.Clock.Travel(8 * 24 * time.Hour)
		erasureID, err := sc.App.IDs.Decode(string(models.PrefixContactErasure), erasure)
		require.NoError(t, err)

		held := *sc.App.Actions.Inbox.EraseContact
		held.Retention = holdEverything{}
		outcome, err := held.Handle(t.Context(), erasures.EraseArgs{
			WorkspaceID: sc.Workspace.Workspace.ID, ErasureID: erasureID,
		})
		require.NoError(t, err)
		assert.Equal(t, erasures.Held, outcome)

		sc.Table("channel_inbound_events").AssertCount(1, testkit.Row{"workspace_id": sc.Workspace.Workspace.ID})
	})
}

func TestSlackHeldDeliveries(t *testing.T) {
	t.Parallel()

	t.Run("a suspended workspace's deliveries wait and are processed once it is back", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		slug := sc.Workspace.Workspace.Slug

		code, _ := sc.Command("workspace:suspend", slug, "--reason", "unpaid invoice")
		require.Zero(t, code)

		sc.deliver(message(supportRoom, "U0CUSTOMER", rootTS, "First while away"))
		sc.Clock.Travel(time.Second)
		sc.deliver(message(supportRoom, "U0OTHER", secondTS, "Second while away"))
		assert.Empty(t, sc.conversations())
		events := sc.Table("channel_inbound_events")
		events.AssertCount(2, testkit.Row{"status": "held"})

		code, _ = sc.Command("workspace:unsuspend", slug)
		require.Zero(t, code)
		sc.Drain()

		events.AssertCount(0, testkit.Row{"status": "held"})
		events.AssertCount(2, testkit.Row{"status": "processed"})
		var bodies []string
		for _, conv := range sc.conversations() {
			for _, msg := range sc.messages(conv) {
				bodies = append(bodies, body(msg))
			}
		}
		assert.ElementsMatch(t, []string{"First while away", "Second while away"}, bodies)
	})
}
