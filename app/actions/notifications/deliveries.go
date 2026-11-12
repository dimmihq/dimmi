/*
|--------------------------------------------------------------------------
| Delivery Log
|--------------------------------------------------------------------------
|
| Every teammate alert leaves a line per channel: in the app, email,
| push or Slack, sent, failed and why, or skipped and why. Most lines
| come from the notification manager as each delivery ends; a channel
| the member turned off, or has no way to reach them on, never gets
| that far, so the dispatch writes those lines itself under the same
| delivery id. The log answers "why did I not get that?" and is kept
| for thirty days.
|
*/

package notifications

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/ids"
	fnotify "github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Why a delivery was skipped or failed.
*/
const (
	ReasonPreferenceOff = "preference_off"
	ReasonNoRoute       = "no_route"
	ReasonQuietHours    = "quiet_hours"
	ReasonPaused        = "paused"
	ReasonSuppressed    = "suppressed"
	ReasonAIHandled     = "ai_handled"
	ReasonAlreadySeen   = "already_seen"
	ReasonNotNeeded     = "no_longer_needed"
	ReasonNotVisible    = "not_visible"
	ReasonCooldown      = "cooldown"
	ReasonRejected      = "rejected"
	ReasonError         = "error"
)

/*
Log limits: how long lines are kept, how many one prune pass deletes,
and how long a failure's detail may be.
*/
const (
	deliveryRetention = 30 * 24 * time.Hour
	pruneBatch        = 5000
	detailLength      = 300
)

/*
DeliveryLog writes the delivery log. Lang renders titles for the lines
the dispatch writes itself.
*/
type DeliveryLog struct {
	DB    *database.DB
	Clock clock.Clock
	Lang  *i18n.Translator
}

/*
Line is one delivery of an alert on one channel, to a member or a Slack
channel.
*/
type Line struct {
	Alert       *Alert
	DeliveryID  string
	Member      *ids.UUID
	Destination *ids.UUID
	Channel     string
	Status      string
	Reason      string
	Detail      string
	Title       string
}

/*
Observe records how a delivery the manager ran ended. Only alerts are
logged, and only on the channels a member chooses; the live update
that refreshes an open dashboard is not one.
*/
func (l *DeliveryLog) Observe(ctx context.Context, o fnotify.Outcome) {
	a, ok := o.Notification.(*Alert)
	channel := logChannel(o.Delivery.Channel)
	if !ok || channel == "" {
		return
	}
	line := Line{
		Alert: a, DeliveryID: o.Delivery.ID, Channel: channel, Status: o.Status, Reason: o.Reason,
		Title: a.title(ctx),
	}
	switch to := o.Delivery.Notifiable.(type) {
	case Recipient:
		line.Member = &to.MemberID
	case Destination:
		line.Destination = &to.Row.ID
	default:
		return
	}
	if o.Status == fnotify.StatusSkipped && line.Reason == "" {
		line.Reason = ReasonNotNeeded
	}
	if o.Err != nil {
		line.Reason, line.Detail = failure(o.Err)
	}
	_ = l.Record(ctx, line)
}

/*
told is one member's alert as the dispatch decides it: the alert, its
trigger, the member's choices, the member and the delivery id.
*/
type told struct {
	alert   *Alert
	trigger Trigger
	choices Choices
	to      Recipient
	id      string
}

/*
skipped records a channel the dispatch never queued for the member,
its title written in the member's language.
*/
func (l *DeliveryLog) skipped(ctx context.Context, t told, channel, reason string) {
	tctx := i18n.WithLocale(ctx, t.to.Language)
	if l.Lang != nil {
		tctx = i18n.WithTranslator(tctx, l.Lang)
	}
	_ = l.Record(ctx, Line{
		Alert: t.alert, DeliveryID: t.id, Member: &t.to.MemberID, Channel: channel, Status: fnotify.StatusSkipped,
		Reason: reason, Title: t.alert.title(tctx),
	})
}

/*
Record writes one line, or updates it when the same delivery was
retried.
*/
func (l *DeliveryLog) Record(ctx context.Context, line Line) error {
	id, err := ids.Parse(line.DeliveryID)
	if err != nil {
		return fmt.Errorf("record notification delivery: %w", err)
	}
	a := line.Alert
	scoped := tenancy.With(ctx, a.WorkspaceID.String())
	err = l.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).RecordNotificationDelivery(ctx, models.RecordNotificationDeliveryParams{
			WorkspaceID: a.WorkspaceID, WorkspaceMemberID: line.Member, NotificationDestinationID: line.Destination,
			DeliveryID: id, Trigger: a.Trigger, Channel: line.Channel, Status: line.Status,
			Reason: optional(line.Reason), Detail: optional(line.Detail), Title: line.Title, Link: a.Link,
			OccurredAt: a.OccurredAt, Now: l.Clock.Now(),
		})
	})
	if err != nil {
		return fmt.Errorf("record notification delivery: %w", err)
	}
	return nil
}

/*
Prune deletes the lines older than thirty days, a batch at a time.
*/
func (l *DeliveryLog) Prune(ctx context.Context) error {
	before := l.Clock.Now().Add(-deliveryRetention)
	for {
		n, err := models.New(l.DB).PruneNotificationDeliveries(ctx, models.PruneNotificationDeliveriesParams{
			Before: before, MaxRows: pruneBatch,
		})
		if err != nil {
			return fmt.Errorf("prune notification deliveries: %w", err)
		}
		if n < pruneBatch {
			return nil
		}
	}
}

/*
logChannel names a framework channel as the log does, "" for one the
log leaves out.
*/
func logChannel(channel string) string {
	switch channel {
	case "database":
		return ChannelInApp
	case "mail":
		return ChannelEmail
	case "webpush":
		return ChannelPush
	case SlackChannelName:
		return ChannelSlack
	}
	return ""
}

/*
failure reads a failed delivery's reason and a short detail.
*/
func failure(err error) (string, string) {
	reason := ReasonError
	if errors.Is(err, fnotify.ErrPermanent) || errors.Is(err, fnotify.ErrInvalidRoute) {
		reason = ReasonRejected
	}
	return reason, truncate(err.Error(), detailLength)
}

/*
optional is nil for an empty string.
*/
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

/*
skipChannels records, for the member, every channel the alert will not
be queued on and why: turned off, or no way to reach them. Push and
Slack are only logged for members who set them up, so a member who
never will is not told so on every alert. A line that cannot be
written never holds the alert back.
*/
func (d *Dispatcher) skipChannels(ctx context.Context, tl told) {
	if d.Log == nil {
		return
	}
	t, c, r := tl.trigger, tl.choices, tl.to
	reasons := map[string]string{}
	for _, ch := range Channels() {
		switch {
		case !c.Enabled(t, ch):
			reasons[ch] = ReasonPreferenceOff
		case ch == ChannelEmail && !r.EmailVerified:
			reasons[ch] = ReasonNoRoute
		}
	}
	if len(r.Push) == 0 {
		delete(reasons, ChannelPush)
	}
	if r.Slack == nil {
		delete(reasons, ChannelSlack)
	}
	for _, ch := range Channels() {
		if reason, ok := reasons[ch]; ok {
			d.Log.skipped(ctx, tl, ch, reason)
		}
	}
}

/*
skipAll records every channel the member keeps on as skipped for one
reason, such as the agent answering the conversation.
*/
func (d *Dispatcher) skipAll(ctx context.Context, tl told, reason string) {
	if d.Log == nil {
		return
	}
	tl.id = ids.New().String()
	for _, ch := range []string{ChannelInApp, ChannelEmail} {
		if tl.choices.Enabled(tl.trigger, ch) {
			d.Log.skipped(ctx, tl, ch, reason)
		}
	}
}
