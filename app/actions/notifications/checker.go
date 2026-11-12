/*
|--------------------------------------------------------------------------
| Does It Still Matter
|--------------------------------------------------------------------------
|
| An email or a push that waited asks once more before it goes. Not if
| the member turned that channel off meanwhile, paused, or is in quiet
| hours. Not if the conversation is gone, marked spam, or out of their
| sight. Not if someone already did what it asks: picked up the new
| conversation, answered the customer, decided the action. Not if the
| member read it already, or is looking at it right now. And a customer
| who keeps writing gets one push for the burst, not one per message.
| The bell is never held back: it is the record of what happened.
|
| A delivery held back says why, so the delivery log can tell the
| member.
|
*/

package notifications

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	fnotify "github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/models"
)

/*
pushCooldown is how long a push about the same thing holds back the
next one.
*/
const pushCooldown = 30 * time.Second

/*
Checker decides whether a waiting alert still goes out. Fanout knows
who may see what; Cache holds the focus heartbeats and push cooldowns.
*/
type Checker struct {
	DB     *database.DB
	Cache  *cache.Store
	Clock  clock.Clock
	Fanout *appbroadcasting.Fanout
}

/*
Allow reports whether the delivery goes out. One held back for a known
reason comes back as a skip naming it.
*/
func (c *Checker) Allow(ctx context.Context, a *Alert, d fnotify.Delivery) (bool, error) {
	reason, err := c.reason(ctx, a, d)
	if err != nil {
		return false, err
	}
	if reason != "" {
		return false, fnotify.Skip(reason)
	}
	return true, nil
}

/*
reason is why the delivery is held back, "" when it goes out.
*/
func (c *Checker) reason(ctx context.Context, a *Alert, d fnotify.Delivery) (string, error) {
	channel := appChannel(d.Channel)
	if channel == ChannelInApp {
		return "", nil
	}
	if _, ok := d.Notifiable.(Destination); ok {
		still, err := c.destinationStill(ctx, a)
		if err != nil || still {
			return "", err
		}
		return ReasonNotNeeded, nil
	}
	r, ok := d.Notifiable.(Recipient)
	if !ok {
		return ReasonNoRoute, nil
	}
	reason := ""
	target := delivery{alert: a, to: r, channel: channel}
	scoped := tenancy.With(ctx, a.WorkspaceID.String())
	err := c.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		var err error
		reason, err = c.still(ctx, models.New(tx), target)
		return err
	})
	if err != nil || reason != "" {
		return reason, err
	}
	return c.unwatched(ctx, a, r, channel)
}

/*
delivery is one alert on its way to one member on one channel.
*/
type delivery struct {
	alert   *Alert
	to      Recipient
	channel string
}

/*
still checks what the database says: the member's choices, whether
their address takes mail, the conversation and the action call. It
returns why the delivery is held back, "" when it goes out.
*/
func (c *Checker) still(ctx context.Context, q *models.Queries, d delivery) (string, error) {
	a, r, channel := d.alert, d.to, d.channel
	t, ok := FindTrigger(a.Trigger)
	if !ok {
		return ReasonNotNeeded, nil
	}
	choices, err := LoadChoices(ctx, q, a.WorkspaceID, r.MemberID)
	if err != nil {
		return "", err
	}
	if !choices.Enabled(t, channel) {
		return ReasonPreferenceOff, nil
	}
	if quiet := choices.QuietReason(c.Clock.Now(), t.Key, r.Timezone); quiet != "" {
		return quiet, nil
	}
	if channel == ChannelEmail {
		if suppressed, err := c.suppressed(ctx, q, a, r); err != nil || suppressed {
			return ReasonSuppressed, err
		}
	}
	allowed, reason := true, ReasonNotNeeded
	switch {
	case a.CallID != nil:
		allowed, err = c.callPending(ctx, q, a)
	case a.ConversationID != nil:
		allowed, reason, err = c.conversationStill(ctx, q, d, choices)
	}
	if err != nil || allowed {
		return "", err
	}
	return reason, nil
}

/*
suppressed reports whether the member's address may not be mailed,
after a bounce or a complaint.
*/
func (c *Checker) suppressed(ctx context.Context, q *models.Queries, a *Alert, r Recipient) (bool, error) {
	if r.Email == "" {
		return false, nil
	}
	rows, err := q.ListSuppressedEmails(ctx, models.ListSuppressedEmailsParams{
		WorkspaceID: a.WorkspaceID, Emails: []string{r.Email}, Now: c.Clock.Now(),
	})
	if err != nil {
		return false, fmt.Errorf("check suppressed address: %w", err)
	}
	return len(rows) > 0, nil
}

/*
callPending reports whether the action call still waits for a decision.
*/
func (c *Checker) callPending(ctx context.Context, q *models.Queries, a *Alert) (bool, error) {
	call, err := q.FindActionCall(ctx, models.FindActionCallParams{WorkspaceID: a.WorkspaceID, ID: *a.CallID})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load action call: %w", err)
	}
	return call.State == "pending_approval", nil
}

/*
conversationStill checks the conversation the alert is about.
*/
func (c *Checker) conversationStill(ctx context.Context, q *models.Queries, d delivery, choices Choices,
) (bool, string, error) {
	a, r, channel := d.alert, d.to, d.channel
	s, err := q.NotificationConversationState(ctx, models.NotificationConversationStateParams{
		WorkspaceID: a.WorkspaceID, ConversationID: *a.ConversationID, MemberID: r.MemberID, Since: a.OccurredAt,
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, ReasonNotNeeded, nil
	}
	if err != nil {
		return false, "", fmt.Errorf("load conversation state: %w", err)
	}
	if s.DeletedAt != nil || s.Spam {
		return false, ReasonNotNeeded, nil
	}
	if channel == ChannelEmail && !s.LastReadAt.Before(a.OccurredAt) {
		return false, ReasonAlreadySeen, nil
	}
	if !matters(a.Trigger, s, r, choices.Settings.IncludeAiHandled) {
		return false, ReasonNotNeeded, nil
	}
	held := appbroadcasting.Holding{Inbox: s.InboxID, Member: s.AssigneeMemberID, Team: s.AssigneeTeamID}
	sees, err := c.sees(ctx, a, r, held)
	return sees, ReasonNotVisible, err
}

/*
matters applies each trigger's own condition: whether what it asks is
still undone.
*/
func matters(trigger string, s models.NotificationConversationStateRow, r Recipient, includeAI bool) bool {
	aiBusy := s.AiState == "active" && !includeAI
	switch trigger {
	case TriggerNewConversation:
		return s.AssigneeMemberID == nil && !s.AnsweredSince && !aiBusy
	case TriggerCustomerReplied:
		return !s.CustomerWroteSince && !s.AnsweredSince && !aiBusy
	case TriggerAssigned:
		return s.AssigneeMemberID != nil && *s.AssigneeMemberID == r.MemberID
	case TriggerHandoff:
		return !s.AnsweredSince
	case TriggerSnoozeExpired:
		return s.Status == "open"
	}
	return true
}

/*
sees reports whether the member may see the conversation as it is held
now. Most members see everything; only restricted ones are checked.
*/
func (c *Checker) sees(ctx context.Context, a *Alert, r Recipient, h appbroadcasting.Holding) (bool, error) {
	if c.Fanout == nil {
		return true, nil
	}
	restricted, err := c.Fanout.Restricted(ctx, a.WorkspaceID)
	if err != nil {
		return false, fmt.Errorf("load restricted members: %w", err)
	}
	i := slices.IndexFunc(restricted, func(m appbroadcasting.RestrictedMember) bool { return m.MemberID == r.MemberID })
	return i < 0 || restricted[i].Sees(h), nil
}

/*
unwatched checks what only the cache knows: whether the member is
looking at the conversation now, and whether a push about the same
thing just went out.
*/
func (c *Checker) unwatched(ctx context.Context, a *Alert, r Recipient, channel string) (string, error) {
	if c.Cache == nil {
		return "", nil
	}
	focus, err := c.Cache.Get(ctx, FocusKey(r.WorkspaceID, r.MemberID))
	if err == nil && watching(a, string(focus)) {
		return ReasonAlreadySeen, nil
	}
	if channel != ChannelPush && channel != ChannelSlack {
		return "", nil
	}
	key := "ntf:cooldown:" + r.MemberID.String() + ":" + a.GroupKey(fnotify.Delivery{}) + ":" + a.TitleKey + ":" +
		channel
	added, err := c.Cache.Add(ctx, key, []byte("1"), pushCooldown)
	if first := added || err != nil; first {
		return "", nil
	}
	return ReasonCooldown, nil
}

/*
watching reports whether what the member's dashboard shows covers the
alert: the conversation itself, or, for a new conversation, the
dashboard at all.
*/
func watching(a *Alert, focus string) bool {
	if focus == "" {
		return false
	}
	if a.Trigger == TriggerNewConversation {
		return true
	}
	return a.ConversationID != nil && focus == a.ConversationID.String()
}

/*
FocusKey is where a member's focus heartbeat is kept.
*/
func FocusKey(ws, member fmtStringer) string {
	return "ntf:focus:" + ws.String() + ":" + member.String()
}

/*
fmtStringer is anything with a String method, such as an id.
*/
type fmtStringer interface{ String() string }

/*
appChannel maps a framework channel to the member's channel.
*/
func appChannel(channel string) string {
	switch channel {
	case "mail":
		return ChannelEmail
	case "webpush":
		return ChannelPush
	case SlackChannelName:
		return ChannelSlack
	}
	return ChannelInApp
}

/*
destinationStill reports whether a post to a Slack channel still goes
out: only a conversation gone or marked spam holds it back, since the
channel keeps a log of what happened.
*/
func (c *Checker) destinationStill(ctx context.Context, a *Alert) (bool, error) {
	if a.ConversationID == nil {
		return true, nil
	}
	allowed := false
	scoped := tenancy.With(ctx, a.WorkspaceID.String())
	err := c.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		conv, err := models.New(tx).FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: a.WorkspaceID, ID: *a.ConversationID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("load conversation: %w", err)
		}
		allowed = conv.DeletedAt == nil && !conv.Spam
		return nil
	})
	return allowed, err
}
