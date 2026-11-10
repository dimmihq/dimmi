package channels_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
slackBroadcast writes a Slack broadcast to the support channel and sends
it, running every job it queues.
*/
func slackBroadcast(sc *slackCase) string {
	sc.T.Helper()

	owner := sc.ActingAsOwner()
	id := owner.PostJSON("/api/v1/broadcasts", testkit.JSON{"name": "Release notes", "channel": "slack"}).
		AssertCreated().String("data.id")
	owner.PatchJSON("/api/v1/broadcasts/"+id, testkit.JSON{
		"subject":   "New: audit log export",
		"body":      "Hi {{ company.name | default: \"team\" }}, exports now include the audit log.",
		"room_ids":  []string{models.PublicID(models.PrefixChannelRoom, sc.room(supportRoom).ID)},
		"sender_id": models.PublicID(models.PrefixMember, sc.Workspace.Owner.ID),
	}).AssertOK()
	owner.PostJSON("/api/v1/broadcasts/"+id+"/send", nil).AssertOK()

	for range 4 {
		sc.DrainLater()
	}

	return id
}

func TestSlackBroadcasts(t *testing.T) {
	t.Parallel()

	t.Run("posts into each picked customer channel and counts it", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "every_message")

		id := slackBroadcast(sc)

		posts := sc.slack.called("chat.postMessage")
		require.NotEmpty(t, posts)
		sent, err := url.QueryUnescape(posts[len(posts)-1].Form.Encode())
		require.NoError(t, err)
		assert.Contains(t, sent, "exports now include the audit log")
		assert.Contains(t, sent, "New: audit log export")

		report := sc.ActingAsOwner().GetJSON("/api/v1/broadcasts/" + id + "/report").AssertOK()
		report.AssertJSONPath("data.broadcast.status", "sent")
		report.AssertJSONPath("data.counts.sent", float64(1))
	})

	t.Run("a reply in the post's thread opens a conversation that starts with the post", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "every_message")

		id := slackBroadcast(sc)
		sc.deliver(reply(message(supportRoom, "U0CUSTOMER", "1730000600.000100", "Can we get it as CSV?"),
			"1730000500.000200"))

		convs := sc.conversations()
		require.Len(t, convs, 1)

		msgs := sc.messages(convs[0])
		require.GreaterOrEqual(t, len(msgs), 2)
		assert.Contains(t, body(msgs[0]), "exports now include the audit log", "the post comes first")
		assert.Equal(t, "member", msgs[0].AuthorType)
		assert.Contains(t, body(msgs[len(msgs)-1]), "Can we get it as CSV?")

		sc.DrainLater()
		sc.ActingAsOwner().GetJSON("/api/v1/broadcasts/"+id+"/report").AssertOK().
			AssertJSONPath("data.counts.replied", float64(1))
	})

	t.Run("counts reactions on the post toward the report", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "every_message")

		id := slackBroadcast(sc)
		reaction := func(kind string) map[string]any {
			return map[string]any{
				"type": kind, "user": "U0CUSTOMER", "reaction": "tada",
				"item": map[string]any{"type": "message", "channel": supportRoom, "ts": "1730000500.000200"},
			}
		}
		sc.deliver(reaction("reaction_added"))
		sc.DrainLater()

		report := sc.ActingAsOwner().GetJSON("/api/v1/broadcasts/" + id + "/report").AssertOK()
		report.AssertJSONPath("data.counts.reactions", float64(1))
		report.AssertJSONPath("data.counts.reacted", float64(1))
	})
}
