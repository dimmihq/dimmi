/*
|--------------------------------------------------------------------------
| Broadcasts
|--------------------------------------------------------------------------
|
| Broadcasts' wiring: a sender per channel, the signed public links in
| what they send, the jobs behind each send, the hooks that turn answers
| on email, the Messenger and Slack into conversations, and the live
| report on open screens.
|
*/

package providers

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/broadcasts"
	"github.com/dimmihq/dimmi/app/actions/email"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
broadcastLinksPurpose names the key broadcast links are signed with.
*/
const broadcastLinksPurpose = "broadcast-links"

/*
bootBroadcasts builds broadcasts on top of the channels and registers
their jobs.
*/
func bootBroadcasts(a *app.App) error {
	links := broadcasts.Links{BaseURL: a.Application.Config.App.URL, Key: a.DerivedKey(broadcastLinksPurpose)}
	cfg := a.Config.Channels.Email
	messenger := &broadcasts.MessengerSender{DB: a.DB, Clock: a.Clock, Events: a.Events}

	b := &broadcasts.Broadcasts{
		DB: a.DB, Clock: a.Clock, Queue: a.Queue, Events: a.Events, Audiences: a.Actions.Audiences,
		Config: a.Config.Broadcasts, Links: links,
		Senders: map[string]broadcasts.Sender{
			broadcasts.ChannelEmail: &broadcasts.EmailSender{Links: links, Pacer: &broadcasts.Pacer{
				DB: a.DB, Limiter: a.Limiter, Clock: a.Clock, Limits: cfg.Sending.Limits, Config: a.Config.Broadcasts,
			}, Mailer: &email.SendBroadcastEmail{
				DB: a.DB, Clock: a.Clock, Mail: a.Mail, Config: cfg, Senders: a.EmailDomains,
				Stamps: inbound.NewStamps(a.DerivedKey(stampsKeyPurpose)), Transport: customerTransport(a),
			}},
			broadcasts.ChannelChat: messenger,
			broadcasts.ChannelPost: messenger,
		},
	}

	if a.Actions.Slack != nil && a.Actions.Slack.Channel != nil {
		b.Senders[broadcasts.ChannelSlack] = &broadcasts.SlackSender{DB: a.DB, Driver: a.Actions.Slack.Channel.Driver}
		a.Actions.Slack.Channel.Broadcasts = b
	}

	a.Actions.Broadcasts = b
	a.Actions.ProcessInboundEmail.Replies = b

	listenForBroadcastReplies(a, b)
	(&appbroadcasting.BroadcastReports{Server: a.Broadcast}).Listen(a.Events)

	return errors.Join(
		queue.Register(a.Queue, &broadcasts.StartWorker{Broadcasts: b}),
		queue.Register(a.Queue, &broadcasts.DispatchWorker{Broadcasts: b}),
		queue.Register(a.Queue, &broadcasts.SendWorker{Broadcasts: b}),
		queue.Register(a.Queue, &broadcasts.TallyWorker{Broadcasts: b}),
		queue.Register(a.Queue, &broadcasts.SweepWorker{Broadcasts: b}),
	)
}

/*
listenForBroadcastReplies counts answers in Messenger conversations a
broadcast started, and recounts a broadcast whenever something outside
its jobs changes a recipient.
*/
func listenForBroadcastReplies(a *app.App, b *broadcasts.Broadcasts) {
	events.Listen(a.Events, events.InTx, func(ctx context.Context, e appevents.MessageCreated) error {
		if e.AuthorType != string(enums.AuthorTypeContact) || e.Type != string(enums.MessageTypeMessage) {
			return nil
		}

		scoped := tenancy.With(ctx, e.WorkspaceID.String())

		return a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
			c, err := models.New(tx).FindConversation(ctx, models.FindConversationParams{
				WorkspaceID: e.WorkspaceID, ID: e.ConversationID,
			})
			if err != nil {
				return fmt.Errorf("find replied conversation: %w", err)
			}

			return b.MessengerReplied(ctx, tx, c)
		})
	})

	events.Listen(a.Events, events.AfterCommit, func(ctx context.Context, e appevents.BroadcastRecipientChanged) error {
		return a.Queue.Push(context.WithoutCancel(ctx), jobs.TallyBroadcastArgs{
			WorkspaceID: e.WorkspaceID, BroadcastID: e.BroadcastID,
		})
	})
}
