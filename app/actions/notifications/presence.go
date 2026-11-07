/*
|--------------------------------------------------------------------------
| Focus And Test Sends
|--------------------------------------------------------------------------
|
| A dashboard tab that is visible says so every half minute, with the
| conversation it shows; while it does, that member gets no email or
| push about what they are already looking at. The beat expires on its
| own, so a closed laptop never silences anyone for long. A member can
| also send themselves a test on any channel to see it arrive.
|
*/

package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	fnotify "github.com/dimmihq/dimmi/framework/notifications"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
focusTTL is how long one heartbeat counts.
*/
const focusTTL = 75 * time.Second

/*
Presence records focus heartbeats and sends test notifications.
*/
type Presence struct {
	DB         *database.DB
	Clock      interface{ Now() time.Time }
	Cache      *cache.Store
	Notify     *fnotify.Manager
	Recipients *Recipients
	Links      Links
}

/*
Blur records that the member's dashboard is hidden.
*/
func (p *Presence) Blur(ctx context.Context, m authz.Membership) error {
	return p.Cache.Delete(ctx, FocusKey(m.Workspace.ID, m.Member.ID))
}

/*
Focus records that the member's dashboard is visible, on a conversation
or not.
*/
func (p *Presence) Focus(ctx context.Context, m authz.Membership, conversation *ids.UUID) error {
	key := FocusKey(m.Workspace.ID, m.Member.ID)
	value := "*"
	if conversation != nil {
		value = conversation.String()
	}
	return p.Cache.Set(ctx, key, []byte(value), focusTTL)
}

/*
ErrNothingToReach refuses a test on a channel that cannot reach the
member: no verified address, or no browser with push on.
*/
var ErrNothingToReach = fault.New(fault.Invalid, "channel_unreachable",
	"Nothing can receive that yet: verify your email, or enable push on this browser first.")

/*
Test sends the member a test on the channel now.
*/
func (p *Presence) Test(ctx context.Context, m authz.Membership, channel string) error {
	var r []Recipient
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		r, err = p.Recipients.Load(ctx, models.New(tx), m.Workspace.ID, []ids.UUID{m.Member.ID})
		return err
	})
	if err != nil || len(r) == 0 {
		return err
	}
	name := map[string]string{ChannelInApp: "database", ChannelEmail: "mail", ChannelPush: "webpush"}[channel]
	if name == "" {
		return fault.New(fault.Invalid, "channel_invalid", "Pick in_app, email or push.").OnField("channel")
	}
	if (name == "mail" && r[0].RouteFor(name) == nil) || (name == "webpush" && len(r[0].Push) == 0) {
		return ErrNothingToReach
	}
	a := &Alert{
		Trigger: "test", WorkspaceID: m.Workspace.ID, Workspace: m.Workspace.Name, TitleKey: "test",
		Link: p.Links.Settings(m.Workspace.Slug), ManageLink: p.Links.Settings(m.Workspace.Slug),
		OccurredAt: p.Clock.Now(), Channels: []string{name},
	}
	if err := p.Notify.Send(ctx, a, r[0]); err != nil {
		return fmt.Errorf("send test notification: %w", err)
	}
	return nil
}
