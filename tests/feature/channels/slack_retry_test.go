package channels_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
)

/*
retryNow makes every job waiting to be tried again due now and drains
the queue.
*/
func (sc *slackCase) retryNow() {
	sc.T.Helper()

	_, err := sc.Operator().Exec(sc.T.Context(),
		"UPDATE river_job SET state = 'available', scheduled_at = now() WHERE state IN ('retryable', 'available')")
	require.NoError(sc.T, err)
	sc.Drain()
}

func TestSlackSendRetries(t *testing.T) {
	t.Parallel()

	t.Run("slack's own trouble on posting is tried again and the reply is sent", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		sc.slack.answer("chat.postMessage", slackAnswer{status: 200, body: `{"ok": false, "error": "internal_error"}`})

		sc.replyAsOwner(conv, "On it")

		assert.Equal(t, string(enums.DeliveryStatusPending), *sc.lastReply().DeliveryStatus)

		sc.retryNow()

		assert.Equal(t, string(enums.DeliveryStatusSent), *sc.lastReply().DeliveryStatus)
		assert.Len(t, sc.slack.called("chat.postMessage"), 2)
	})

	t.Run("slack answering rate limited in its body postpones the reply", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		sc.slack.answer("chat.postMessage", slackAnswer{status: 200, body: `{"ok": false, "error": "ratelimited"}`})

		sc.replyAsOwner(conv, "On it")

		assert.Equal(t, string(enums.DeliveryStatusPending), *sc.lastReply().DeliveryStatus)

		sc.DrainLater()

		assert.Equal(t, string(enums.DeliveryStatusSent), *sc.lastReply().DeliveryStatus)
	})

	t.Run("a channel gone fails the reply without trying again", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		sc.slack.answer("chat.postMessage",
			slackAnswer{status: 200, body: `{"ok": false, "error": "channel_not_found"}`})

		sc.replyAsOwner(conv, "On it")

		assert.Equal(t, string(enums.DeliveryStatusFailed), *sc.lastReply().DeliveryStatus)

		sc.retryNow()

		assert.Len(t, sc.slack.called("chat.postMessage"), 1)
	})

	t.Run("files failing after the text posted share only the files on the next try", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		driver, err := sc.App.Channels.Driver(slackdriver.Type)
		require.NoError(t, err)
		sc.slack.answer("files.completeUploadExternal",
			slackAnswer{status: 200, body: `{"ok": false, "error": "service_unavailable"}`})

		ctx := tenancy.With(sc.T.Context(), sc.Workspace.Workspace.ID.String())
		msg := contracts.OutboundMessage{
			MessageID: sc.messages(conv)[0].ID, ExternalThreadKey: *conv.ExternalThreadKey,
			BodyMarkdown: "See attached",
			Attachments: []contracts.OutboundAttachment{{
				Filename: "report.txt", SizeBytes: 5,
				Open: func(context.Context) (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("hello")), nil
				},
			}},
		}

		_, err = driver.Send(ctx, sc.conn, msg)
		require.Error(t, err)
		require.NotErrorIs(t, err, contracts.ErrPermanent)

		result, err := driver.Send(ctx, sc.conn, msg)
		require.NoError(t, err)

		assert.Len(t, sc.slack.called("chat.postMessage"), 1)
		assert.Len(t, sc.slack.called("files.completeUploadExternal"), 2)
		assert.Equal(t, supportRoom+":1730000500.000200", result.ChannelMessageID)
		assert.Equal(t, []string{supportRoom + ":1730000500.000200", "file:F0FILE"}, result.Metadata["parts"])
	})
}
