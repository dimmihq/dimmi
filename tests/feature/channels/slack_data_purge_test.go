package channels_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/audit"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

/*
purgeSlack runs the purge of ended Slack connections for the workspace
and returns how many connections it finished.
*/
func (sc *slackCase) purgeSlack() int {
	sc.T.Helper()

	finished, err := sc.App.Actions.Inbox.PurgeSlackData.Workspace(sc.T.Context(), sc.Workspace.Workspace.ID)
	require.NoError(sc.T, err)

	return finished
}

/*
count returns how many rows the query counts.
*/
func (sc *slackCase) count(query string, args ...any) int {
	sc.T.Helper()

	var n int
	require.NoError(sc.T, sc.Operator().QueryRow(sc.T.Context(), query, args...).Scan(&n))

	return n
}

/*
withFile is a customer's message in the shared room with a file.
*/
func withFile(ts, text string) map[string]any {
	ev := message(supportRoom, "U0CUSTOMER", ts, text)
	ev["subtype"] = "file_share"
	ev["files"] = []map[string]any{{
		"id": "F1", "name": "shot.png", "mimetype": "image/png", "size": 13,
		"url_private_download": "https://files.slack.com/files-pri/T0/F1/shot.png",
	}}

	return ev
}

/*
otherWorkspaceSlack gives another workspace a live Slack connection
with a room and a kept delivery, and returns the workspace's id.
*/
func otherWorkspaceSlack(sc *slackCase) ids.UUID {
	sc.T.Helper()
	ctx, db := sc.T.Context(), sc.Operator()

	other, err := factories.Workspace(ctx, sc.App, sc.CreateUser())
	require.NoError(sc.T, err)
	inbox, err := factories.DefaultInbox(ctx, sc.App, other.Workspace)
	require.NoError(sc.T, err)

	var conn ids.UUID
	require.NoError(sc.T, db.QueryRow(ctx,
		`INSERT INTO channel_connections (workspace_id, inbox_id, type, name, status, external_id, config)
		VALUES ($1, $2, 'slack', 'Other Slack', 'active', 'A0OTHER:T0OTHER', '{"team_id": "T0OTHER"}')
		RETURNING id`, other.Workspace.ID, inbox.Inbox.ID).Scan(&conn))

	_, err = db.Exec(ctx,
		`INSERT INTO channel_rooms (workspace_id, channel_connection_id, kind, external_id, name, mode, trigger)
		VALUES ($1, $2, 'slack_channel', 'C0OTHER', '#other', 'community', 'author_window')`, other.Workspace.ID, conn)
	require.NoError(sc.T, err)

	_, err = db.Exec(ctx,
		`INSERT INTO channel_inbound_events
			(workspace_id, channel_connection_id, provider, transport, dedupe_key, kind, payload)
		VALUES ($1, $2, 'slack', 'webhook', 'other-1', 'message', '{"text": "theirs"}')`, other.Workspace.ID, conn)
	require.NoError(sc.T, err)

	return other.Workspace.ID
}

/*
emailConversation starts a conversation outside Slack and returns its
first message's id.
*/
func emailConversation(sc *slackCase) ids.UUID {
	sc.T.Helper()

	inbox, err := factories.DefaultInbox(sc.T.Context(), sc.App, sc.Workspace.Workspace)
	require.NoError(sc.T, err)

	sc.ActingAsOwner().PostJSON("/api/v1/conversations", testkit.JSON{
		"inbox_id": models.PublicID(models.PrefixInbox, inbox.Inbox.ID), "author": "contact", "subject": "My order",
		"body": "Hello from ann", "contact": testkit.JSON{"email": "ann@example.com", "name": "ann"},
	}).AssertCreated()

	var message ids.UUID
	require.NoError(sc.T, sc.Operator().QueryRow(sc.T.Context(),
		`SELECT m.id FROM messages m JOIN conversations c ON c.id = m.conversation_id
		WHERE c.workspace_id = $1 AND c.channel_connection_id <> $2 AND m.body IS NOT NULL
		ORDER BY m.created_at LIMIT 1`, sc.Workspace.Workspace.ID, sc.conn.ID).Scan(&message))

	return message
}

/*
fileOnDisk reports whether the attachment's file is still stored.
*/
func fileOnDisk(sc *slackCase, file models.Attachment) bool {
	sc.T.Helper()

	disk, err := sc.App.Storage.Disk(file.Disk)
	require.NoError(sc.T, err)

	found, err := disk.Exists(sc.T.Context(), file.Path)
	require.NoError(sc.T, err)

	return found
}

func TestSlackUninstallEndsTheConnection(t *testing.T) {
	t.Parallel()

	t.Run("an uninstall records when the connection ended and wipes its secrets", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		require.NotEmpty(t, sc.conn.CredentialsEncrypted)
		require.Nil(t, sc.conn.EndedAt)

		sc.deliver(map[string]any{"type": "app_uninstalled"})

		conn := sc.connection(sc.conn.ID)
		assert.Equal(t, "disconnected", conn.Status)
		assert.Empty(t, conn.CredentialsEncrypted)
		require.NotNil(t, conn.EndedAt)
		assert.WithinDuration(t, sc.Clock.Now(), *conn.EndedAt, time.Second)
		assert.Nil(t, conn.DataPurgedAt)
	})

	t.Run("a revoked bot token ends the connection the same way", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(map[string]any{"type": "tokens_revoked", "tokens": map[string]any{"bot": []string{botUser}}})

		conn := sc.connection(sc.conn.ID)
		assert.Empty(t, conn.CredentialsEncrypted)
		assert.NotNil(t, conn.EndedAt)
	})
}

func TestSlackDataPurge(t *testing.T) {
	t.Parallel()

	t.Run("after the grace period what Slack brought in is gone and nothing else", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		ws := sc.Workspace.Workspace.ID

		sc.deliver(withFile(rootTS, "The export fails"))
		conv := sc.conversations()[0]
		sc.replyAsOwner(conv, "Looking into it")
		first := sc.messages(conv)[0]
		files := query(sc, func(ctx context.Context, q *models.Queries) ([]models.Attachment, error) {
			return q.ListMessageAttachments(ctx, models.ListMessageAttachmentsParams{
				WorkspaceID: ws, MessageIds: []ids.UUID{first.ID},
			})
		})
		require.Len(t, files, 1)
		require.True(t, fileOnDisk(sc, files[0]))

		kept := emailConversation(sc)
		other := otherWorkspaceSlack(sc)
		require.Positive(t, sc.count(`SELECT count(*) FROM contact_identities WHERE workspace_id = $1
			AND type = 'slack_user'`, ws))

		sc.deliver(map[string]any{"type": "app_uninstalled"})

		sc.Clock.Travel(9 * 24 * time.Hour)
		assert.Zero(t, sc.purgeSlack(), "nothing goes before the grace period is over")
		assert.Equal(t, "The export fails", body(sc.messages(conv)[0]))
		assert.Positive(t, sc.count(`SELECT count(*) FROM channel_inbound_events WHERE channel_connection_id = $1`,
			sc.conn.ID))
		assert.True(t, fileOnDisk(sc, files[0]))

		sc.Clock.Travel(2 * 24 * time.Hour)
		assert.Equal(t, 1, sc.purgeSlack())

		msgs := sc.messages(conv)
		require.Len(t, msgs, 3, "the customer's emptied message, the teammate's reply and the assignment event stay")
		asked, replied := msgs[0], msgs[1]
		assert.Nil(t, asked.Body)
		assert.Nil(t, asked.BodyHtml)
		assert.JSONEq(t, `{}`, string(asked.Metadata))
		assert.NotNil(t, asked.DeletedAt)
		assert.Equal(t, "Looking into it", body(replied), "a reply written in dimmi keeps its words")
		assert.Nil(t, replied.DeletedAt)
		for _, msg := range msgs[:2] {
			assert.Nil(t, msg.ChannelMessageID)
			assert.Nil(t, msg.DedupeKey)
		}

		assert.False(t, fileOnDisk(sc, files[0]))
		assert.Zero(t, sc.count(`SELECT count(*) FROM attachments WHERE workspace_id = $1`, ws))

		shell := sc.conversations()[0]
		assert.Nil(t, shell.ExternalThreadKey)
		assert.Nil(t, shell.Subject)
		assert.JSONEq(t, `{}`, string(shell.Source))
		assert.NotEqual(t, conv.ContactID, shell.ContactID)
		assert.Equal(t, 1, sc.count(`SELECT count(*) FROM contacts WHERE id = $1 AND placeholder`, shell.ContactID))
		assert.Zero(t, sc.count(`SELECT count(*) FROM contacts WHERE id = $1`, conv.ContactID),
			"a contact known only from Slack is deleted")
		assert.Zero(t, sc.count(`SELECT count(*) FROM contact_identities WHERE workspace_id = $1
			AND type = 'slack_user'`, ws))

		for _, table := range []string{
			"channel_inbound_events", "channel_rooms", "channel_message_refs", "channel_member_links",
			"channel_conversation_posts",
		} {
			assert.Zero(t, sc.count(`SELECT count(*) FROM `+table+` WHERE channel_connection_id = $1`, sc.conn.ID),
				table)
		}

		conn := sc.connection(sc.conn.ID)
		assert.NotNil(t, conn.DataPurgedAt)
		assert.NotNil(t, conn.DeletedAt)
		assert.Nil(t, conn.ExternalID)
		assert.NotContains(t, string(conn.Config), installTeam)
		assert.NotContains(t, string(conn.Config), botUser)

		assert.Equal(t, 1, sc.count(`SELECT count(*) FROM messages WHERE id = $1 AND body = 'Hello from ann'`, kept))
		assert.Equal(t, 1, sc.count(`SELECT count(*) FROM contacts WHERE workspace_id = $1
			AND email = 'ann@example.com'`, ws))
		assert.Equal(t, 1, sc.count(`SELECT count(*) FROM channel_rooms WHERE workspace_id = $1`, other))
		assert.Equal(t, 1, sc.count(`SELECT count(*) FROM channel_inbound_events WHERE workspace_id = $1`, other))
		assert.Equal(t, 1, sc.count(`SELECT count(*) FROM channel_connections WHERE workspace_id = $1
			AND type = 'slack' AND status = 'active' AND deleted_at IS NULL`, other))

		entries := `SELECT count(*) FROM audit_logs WHERE workspace_id = $1 AND event = $2 AND subject_id = $3`
		assert.Equal(t, 1, sc.count(entries+` AND (changes->>'messages')::int = 2
			AND (changes->>'attachments')::int = 1 AND (changes->>'conversations')::int = 1
			AND (changes->>'contacts')::int = 1 AND (changes->>'rooms')::int > 0
			AND (changes->>'channel_inbound_events')::int > 0`, ws, audit.EventSlackDataDeleted, sc.conn.ID))
		assert.Zero(t, sc.count(`SELECT count(*) FROM audit_logs WHERE workspace_id = $1 AND event = $2
			AND changes::text LIKE '%export fails%'`, ws, audit.EventSlackDataDeleted), "the entry holds no content")

		assert.Zero(t, sc.purgeSlack(), "a second round finds nothing left")
		assert.Equal(t, 1, sc.count(entries, ws, audit.EventSlackDataDeleted, sc.conn.ID))
		assert.Len(t, sc.messages(conv), 3)
	})

	t.Run("connecting Slack again before the purge keeps everything", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(message(supportRoom, "U0CUSTOMER", rootTS, "The export fails"))
		conv := sc.conversations()[0]
		sc.deliver(map[string]any{"type": "app_uninstalled"})
		sc.Clock.Travel(5 * 24 * time.Hour)

		sc.ActingAsOwner().PostJSON("/api/v1/channels/"+sc.publicID()+"/credentials", map[string]any{
			"bot_token": slackBot, "signing_secret": slackSigning,
		}).AssertOK()
		sc.Drain()

		conn := sc.connection(sc.conn.ID)
		assert.Equal(t, "active", conn.Status)
		assert.Nil(t, conn.EndedAt)

		sc.Clock.Travel(10 * 24 * time.Hour)
		assert.Zero(t, sc.purgeSlack())
		assert.Equal(t, "The export fails", body(sc.messages(conv)[0]))
		assert.Nil(t, sc.connection(sc.conn.ID).DeletedAt)
		assert.Zero(t, sc.count(`SELECT count(*) FROM audit_logs WHERE event = $1`, audit.EventSlackDataDeleted))
	})

	t.Run("a purged connection is never brought back", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(map[string]any{"type": "app_uninstalled"})
		sc.Clock.Travel(11 * 24 * time.Hour)
		require.Equal(t, 1, sc.purgeSlack())

		sc.ActingAsOwner().PostJSON("/api/v1/channels/"+sc.publicID()+"/credentials", map[string]any{
			"bot_token": slackBot, "signing_secret": slackSigning,
		}).AssertNotFound()
	})

	t.Run("the shared app a teammate disconnected comes back, and starts fresh once purged", func(t *testing.T) {
		t.Parallel()
		sc, conv := installedConversation(t)
		ws, install := sc.Workspace.Workspace.ID, *sc.conn.IntegrationInstallationID
		gone := appevents.IntegrationDisconnected{
			Base:           appevents.Base{ID: ids.New(), WorkspaceID: ws, OccurredAt: sc.Clock.Now()},
			InstallationID: install, Provider: "slack", Reason: "disconnected",
		}
		back := appevents.IntegrationConnected{
			Base:           appevents.Base{ID: ids.New(), WorkspaceID: ws, OccurredAt: sc.Clock.Now()},
			InstallationID: install, Provider: "slack", ExternalAccountID: installTeam, ExternalAppID: appID,
			Features: []string{"channels"}, Reconnected: true,
		}

		sc.dispatch(gone)
		require.NotNil(t, sc.connection(sc.conn.ID).EndedAt)

		sc.Clock.Travel(3 * 24 * time.Hour)
		sc.dispatch(back)
		sc.Drain()
		assert.Equal(t, "active", sc.connection(sc.conn.ID).Status)
		assert.Nil(t, sc.connection(sc.conn.ID).EndedAt)

		sc.Clock.Travel(20 * 24 * time.Hour)
		assert.Zero(t, sc.purgeSlack(), "a connection that came back is not purged")
		assert.Equal(t, "The export fails", body(sc.messages(conv)[0]))

		gone.ID = ids.New()
		sc.dispatch(gone)
		sc.Clock.Travel(11 * 24 * time.Hour)
		require.Equal(t, 1, sc.purgeSlack())
		assert.Nil(t, sc.messages(conv)[0].Body)

		back.ID = ids.New()
		sc.dispatch(back)
		sc.Drain()
		fresh := query(sc, func(ctx context.Context, q *models.Queries) (models.ChannelConnection, error) {
			return q.FindInstallationChannelConnection(ctx, models.FindInstallationChannelConnectionParams{
				WorkspaceID: ws, IntegrationInstallationID: &install,
			})
		})
		assert.NotEqual(t, sc.conn.ID, fresh.ID)
		assert.Equal(t, "active", fresh.Status)
		assert.Nil(t, fresh.EndedAt)
	})
}
