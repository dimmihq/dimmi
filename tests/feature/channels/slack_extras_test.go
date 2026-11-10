package channels_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
classifier answers every message with one verdict, standing in for the
request classifier, which calls a language model.
*/
type classifier struct {
	verdict channels.RequestVerdict
	err     error
}

/*
Classify returns the verdict.
*/
func (c classifier) Classify(context.Context, channels.ClassifyInput) (channels.RequestVerdict, error) {
	return c.verdict, c.err
}

/*
grants hands out a teammate's own Slack token, standing in for the
integrations' token service.
*/
type grants struct{}

/*
BotToken returns the installation's bot token.
*/
func (grants) BotToken(context.Context, ids.UUID, ids.UUID) (string, error) {
	return "xoxb-fixture", nil
}

/*
UserToken returns the teammate's own token.
*/
func (grants) UserToken(context.Context, ids.UUID, ids.UUID, ids.UUID) (channels.UserGrant, bool, error) {
	return channels.UserGrant{Token: "xoxp-" + "fixture", UserID: "U0TEAMMATE"}, true, nil
}

func TestSlackRequestDetection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		verdict channels.RequestVerdict
		err     error
		want    int
	}{
		{"a confident request opens a conversation", channels.RequestVerdict{Kind: "request", Confidence: 0.9}, nil, 1},
		{"an unsure request opens nothing", channels.RequestVerdict{Kind: "request", Confidence: 0.5}, nil, 0},
		{"chatter opens nothing", channels.RequestVerdict{Kind: "chatter", Confidence: 0.9}, nil, 0},
		{"a follow-up joins the author's conversation", channels.RequestVerdict{Kind: "followup"}, nil, 1},
		{"a failing classifier groups by author", channels.RequestVerdict{}, errors.New("timeout"), 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			sc := connectedCase(t)
			sc.setRoom(supportRoom, "shared", "ai")
			sc.App.Actions.Slack.Channel.Classifier = classifier{verdict: c.verdict, err: c.err}

			sc.deliver(message(supportRoom, "U0CUSTOMER", rootTS, "The export fails"))

			assert.Len(t, sc.conversations(), c.want)
		})
	}
}

func TestSlackFiles(t *testing.T) {
	t.Parallel()

	t.Run("a customer's files are downloaded from Slack, oversized ones named instead", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		ev := message(supportRoom, "U0CUSTOMER", rootTS, "Screenshot attached")
		ev["subtype"] = "file_share"
		ev["files"] = []map[string]any{
			{
				"id": "F1", "name": "shot.png", "mimetype": "image/png", "size": 13,
				"url_private_download": "https://files.slack.com/files-pri/T0/F1/shot.png",
			},
			{
				"id": "F2", "name": "huge.zip", "mimetype": "application/zip", "size": 90 << 20,
				"url_private_download": "https://files.slack.com/files-pri/T0/F2/huge.zip",
			},
		}
		sc.deliver(ev)

		msg := sc.messages(sc.conversations()[0])[0]
		assert.Contains(t, body(msg), "[huge.zip, too large to import]")

		files := query(sc, func(ctx context.Context, q *models.Queries) ([]models.Attachment, error) {
			return q.ListMessageAttachments(ctx, models.ListMessageAttachmentsParams{
				WorkspaceID: msg.WorkspaceID, MessageIds: []ids.UUID{msg.ID},
			})
		})
		require.Len(t, files, 1)
		assert.Equal(t, "shot.png", files[0].Filename)
	})

	t.Run("a reply's files are shared into the thread after its text", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		driver, err := sc.App.Channels.Driver(slackdriver.Type)
		require.NoError(t, err)

		ctx := tenancy.With(sc.T.Context(), sc.Workspace.Workspace.ID.String())
		result, err := driver.Send(ctx, sc.conn, contracts.OutboundMessage{
			MessageID: sc.messages(conv)[0].ID, ExternalThreadKey: *conv.ExternalThreadKey,
			Attachments: []contracts.OutboundAttachment{{
				Filename: "report.txt", SizeBytes: 5,
				Open: func(context.Context) (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("hello")), nil
				},
			}},
		})
		require.NoError(t, err)
		assert.Equal(t, "file:F0FILE", result.ChannelMessageID)
		assert.Len(t, sc.slack.called("files.completeUploadExternal"), 1)
		assert.Empty(t, sc.slack.called("chat.postMessage"))

		again, err := driver.Send(ctx, sc.conn, contracts.OutboundMessage{
			MessageID: sc.messages(conv)[0].ID, ExternalThreadKey: *conv.ExternalThreadKey,
		})
		require.NoError(t, err)
		assert.Equal(t, result.ChannelMessageID, again.ChannelMessageID)

		_, err = driver.Send(ctx, sc.conn, contracts.OutboundMessage{MessageID: ids.New(), ExternalThreadKey: "nope"})
		require.ErrorIs(t, err, contracts.ErrPermanent)
	})
}

func TestSlackInstallations(t *testing.T) {
	t.Parallel()

	t.Run("an installation for channels makes its connection, replies go as the teammate", func(t *testing.T) {
		t.Parallel()
		sc := newSlackCase(t)
		sc.App.Actions.Slack.Channel.Secrets.Installations = grants{}
		ws := sc.Workspace.Workspace.ID

		install := query(sc, func(ctx context.Context, q *models.Queries) (models.IntegrationInstallation, error) {
			return q.CreateIntegrationInstallation(ctx, models.CreateIntegrationInstallationParams{
				ID: ids.New(), WorkspaceID: ws, Provider: "slack", ExternalAccountID: new(installTeam),
				ExternalAppID: new(appID), Name: "Acme", Source: "oauth", Scopes: []string{},
				CredentialsEncrypted: []byte{1}, Metadata: json.RawMessage(`{"bot_user_id": "U0BOT"}`),
				Now: sc.Clock.Now(),
			})
		})
		connected := appevents.IntegrationConnected{
			Base:           appevents.Base{ID: ids.New(), WorkspaceID: ws, OccurredAt: sc.Clock.Now()},
			InstallationID: install.ID, Provider: "slack", ExternalAccountID: installTeam, ExternalAppID: appID,
			Features: []string{"channels"},
		}
		sc.dispatch(connected)
		sc.dispatch(connected)
		sc.Drain()

		conn := query(sc, func(ctx context.Context, q *models.Queries) (models.ChannelConnection, error) {
			return q.FindInstallationChannelConnection(ctx, models.FindInstallationChannelConnectionParams{
				WorkspaceID: ws, IntegrationInstallationID: &install.ID,
			})
		})
		sc.conn = conn
		assert.Equal(t, "active", conn.Status)
		assert.Equal(t, channels.ModeShared, channels.FactsOf(conn).Mode)
		assert.Equal(t, string(enums.RoomModeShared), sc.room(supportRoom).Mode)

		*sc.App.Actions.Slack.Channel.Shared = channels.SharedApp{AppID: appID, SigningSecret: "shared"}
		sc.signed("/webhooks/slack/events", "shared", "application/json",
			envelope("Ev1", message(supportRoom, "U0CUSTOMER", rootTS, "The export fails"))).AssertOK()
		sc.Drain()

		conv := sc.conversations()[0]
		id := sc.replyAsOwner(conv, "On it")
		posts := sc.slack.called("chat.postMessage")
		require.Len(t, posts, 1)
		assert.Empty(t, posts[0].Form.Get("username"))
		assert.Equal(t, "xoxp-fixture", posts[0].Token)

		sc.ActingAsOwner().Delete(
			"/api/v1/conversations/" + models.PublicID(models.PrefixConversation, conv.ID) + "/messages/" + id,
		)
		sc.Drain()
		deletes := sc.slack.called("chat.delete")
		require.Len(t, deletes, 1)
		assert.Equal(t, "xoxp-fixture", deletes[0].Token)

		sc.dispatch(appevents.IntegrationDisconnected{
			Base:           appevents.Base{ID: ids.New(), WorkspaceID: ws, OccurredAt: sc.Clock.Now()},
			InstallationID: install.ID, Provider: "slack", Reason: "uninstalled",
		})
		assert.Equal(t, "disconnected", sc.connection(conn.ID).Status)

		connected.ID, connected.Reconnected = ids.New(), true
		sc.dispatch(connected)
		assert.Equal(t, "active", sc.connection(conn.ID).Status)
	})
}

/*
dispatch dispatches the event in a transaction of the workspace.
*/
func (sc *slackCase) dispatch(e interface{ EventName() string }) {
	sc.T.Helper()

	ctx := tenancy.With(sc.T.Context(), sc.Workspace.Workspace.ID.String())
	err := sc.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return sc.App.Events.Dispatch(ctx, tx, e)
	})
	require.NoError(sc.T, err)
}

func TestSlackSockets(t *testing.T) {
	t.Parallel()

	t.Run("a worker takes the socket's lease and gives it up when the socket ends", func(t *testing.T) {
		t.Parallel()
		sc := newSlackCase(t)
		sc.ActingAsOwner().PostJSON("/api/v1/channels/slack", map[string]any{
			"bot_token": slackBot, "signing_secret": slackSigning, "inbound_transport": "socket",
			"app_token": "xapp-fixture",
		}).AssertCreated()
		sc.Drain()
		sc.slack.answer("apps.connections.open", slackAnswer{status: 200, body: refused("invalid_auth")})

		sockets := sc.App.Actions.Slack.Sockets
		sockets.Tick(sc.T.Context())

		require.Eventually(t, func() bool {
			var owner *string
			err := sc.Operator().QueryRow(sc.T.Context(),
				"SELECT owner FROM channel_socket_sessions WHERE kind = 'slack_socket'").Scan(&owner)

			return err == nil && owner == nil
		}, 5*time.Second, 20*time.Millisecond)

		var lastError *string
		require.NoError(t, sc.Operator().QueryRow(sc.T.Context(),
			"SELECT last_error FROM channel_socket_sessions WHERE kind = 'slack_socket'").Scan(&lastError))
		require.NotNil(t, lastError)
	})
}
