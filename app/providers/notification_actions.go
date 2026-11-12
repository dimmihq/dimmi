/*
|--------------------------------------------------------------------------
| Notification Wiring
|--------------------------------------------------------------------------
|
| Teammate notifications: the events that trigger them, the job that
| dispatches them, the alert type queued email and push decode into, how
| a queued delivery finds its recipient again, and the listeners that
| keep the bell in step when a member reads a conversation or someone
| decides an action call.
|
*/

package providers

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	fnotify "github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/notifications/channels/webpush"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/notifications"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
bootNotifications builds the notification actions, registers the alert
type, its recipients and the dispatch job, and listens for the events
that notify.
*/
func bootNotifications(a *app.App) error {
	fanout := &appbroadcasting.Fanout{DB: a.DB, Cache: a.Cache}
	recipients := &notifications.Recipients{DB: a.DB, Keys: a.Keys}
	checker := &notifications.Checker{DB: a.DB, Cache: a.Cache, Clock: a.Clock, Fanout: fanout}
	links := notifications.Links{Base: a.Application.Config.App.URL, Key: a.DerivedKey(unsubscribePurpose)}
	log := &notifications.DeliveryLog{DB: a.DB, Clock: a.Clock, Lang: a.Lang}
	publisher := notifications.BroadcastPublisher{Broadcaster: a.Broadcast}

	fnotify.RegisterWith(a.Notify, func() *notifications.Alert { return notifications.NewAlert(checker) })
	a.Notify.Resolve(notifications.RecipientType, recipients.Resolve)
	a.Notify.Observe(log.Observe)

	a.Actions.NotificationCenter = &notifications.Center{DB: a.DB, Clock: a.Clock, Publisher: publisher}
	a.Actions.NotificationUnsubscribes = &notifications.Unsubscribes{
		DB: a.DB, Clock: a.Clock, Key: links.Key, Links: links,
	}
	a.Actions.NotificationPresence = &notifications.Presence{
		DB: a.DB, Clock: a.Clock, Cache: a.Cache, Notify: a.Notify, Recipients: recipients, Links: links,
	}
	public, err := vapidPublicKey(a)
	if err != nil {
		return err
	}
	a.Actions.PushDevices = &notifications.Devices{DB: a.DB, Clock: a.Clock, Keys: a.Keys, PublicKey: public}

	dispatcher := &notifications.Dispatcher{
		DB: a.DB, Clock: a.Clock, Notify: a.Notify, Recipients: recipients, Checker: checker, Fanout: fanout,
		Links: links, Log: log,
	}
	if err := queue.Register(a.Queue, dispatcher); err != nil {
		return fmt.Errorf("notification dispatch job: %w", err)
	}
	if err := registerDeliveryUpkeep(a, log, &notifications.QuietSummaries{
		DB: a.DB, Clock: a.Clock, Notify: a.Notify, Recipients: recipients, Links: links,
	}); err != nil {
		return err
	}
	listenForNotifications(a)
	listenForReads(a, publisher)
	return nil
}

/*
unsubscribePurpose derives the key unsubscribe links in teammate
emails are signed with.
*/
const unsubscribePurpose = "notifications.unsubscribe"

/*
registerDeliveryUpkeep registers the delivery log's prune and the
quiet hours summary sweep.
*/
func registerDeliveryUpkeep(a *app.App, log *notifications.DeliveryLog, summaries *notifications.QuietSummaries,
) error {
	if err := queue.Register(a.Queue, &jobs.PruneNotificationDeliveries{Log: log}); err != nil {
		return fmt.Errorf("notification delivery prune job: %w", err)
	}
	if err := queue.Register(a.Queue, &jobs.SendQuietHoursSummaries{Summaries: summaries}); err != nil {
		return fmt.Errorf("quiet hours summary job: %w", err)
	}
	return nil
}

/*
vapidPublicKey is the key browsers subscribe with: the configured one,
or the one derived from the install's key.
*/
func vapidPublicKey(a *app.App) (string, error) {
	if section, ok := a.Application.Config.Notifications.Channels["webpush"]; ok {
		if key := section.String("vapid_public_key"); key != "" {
			return key, nil
		}
	}
	_, public, err := webpush.DeriveKeys(a.DerivedKey("notifications.webpush.vapid"))
	if err != nil {
		return "", fmt.Errorf("push keys: %w", err)
	}
	return public, nil
}

/*
listenForNotifications queues a dispatch for every event that notifies.
*/
func listenForNotifications(a *app.App) {
	d := a.Events
	optional := func(ev notifications.Event, ok bool) queue.Args {
		if !ok {
			return nil
		}
		return ev
	}
	events.ListenQueued(d, func(e appevents.MessageCreated) queue.Args {
		return optional(notifications.FromMessageCreated(e))
	})
	events.ListenQueued(d, func(e appevents.MessageUpdated) queue.Args {
		return optional(notifications.FromMessageUpdated(e))
	})
	events.ListenQueued(d, func(e appevents.ConversationAssigned) queue.Args {
		return optional(notifications.FromAssigned(e))
	})
	events.ListenQueued(d, func(e appevents.ConversationUnsnoozed) queue.Args {
		return optional(notifications.FromUnsnoozed(e))
	})
	events.ListenQueued(d, func(e appevents.AiHandoffRequested) queue.Args { return notifications.FromHandoff(e) })
	events.ListenQueued(d, func(e appevents.AiActionCallApprovalRequested) queue.Args {
		return notifications.FromApprovalRequested(e)
	})
	events.ListenQueued(d, func(e appevents.TeamMemberAdded) queue.Args {
		return optional(notifications.FromTeamMemberAdded(e, a.Clock.Now()))
	})
}

/*
listenForReads keeps the bell in step: reading a conversation reads its
notifications, and a decided action call takes its buttons away.
*/
func listenForReads(a *app.App, publisher notifications.Publisher) {
	events.Listen(a.Events, events.InTx, func(ctx context.Context, e appevents.ConversationRead) error {
		if e.Unread {
			return nil
		}
		return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			return readConversationNotifications(ctx, tx, publisher, notifications.Read{
				WorkspaceID: e.WorkspaceID, MemberID: e.MemberID, ConversationID: e.ConversationID, At: a.Clock.Now(),
			})
		})
	})
	events.Listen(a.Events, events.InTx, func(ctx context.Context, e appevents.AiActionCallUpdated) error {
		if e.State == "pending_approval" {
			return nil
		}
		return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			return notifications.ResolveApproval(ctx, models.New(tx), notifications.Decision{
				WorkspaceID: e.WorkspaceID, CallID: e.CallID, State: e.State, At: a.Clock.Now(),
			})
		})
	})
}

/*
readConversationNotifications marks a member's notifications about the
conversation they read, and tells their tabs once it commits.
*/
func readConversationNotifications(ctx context.Context, tx *database.Tx, publisher notifications.Publisher,
	r notifications.Read,
) error {
	q := models.New(tx)
	changed, err := notifications.ConversationRead(ctx, q, r)
	if err != nil || !changed {
		return err
	}
	counts, err := notifications.MemberCounts(ctx, q, r.WorkspaceID, r.MemberID)
	if err != nil {
		return err
	}
	tx.AfterCommit(func(ctx context.Context) {
		_ = publisher.PublishToMember(ctx, r.MemberID, notifications.EventRead, map[string]int64{
			"unread_count": counts.Unread, "unseen_count": counts.Unseen,
		})
	})
	return nil
}
