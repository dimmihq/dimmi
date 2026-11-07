/*
|--------------------------------------------------------------------------
| Notification Center
|--------------------------------------------------------------------------
|
| A member's own bell: their notifications, newest change first, what is
| unread and what they have not even glanced at yet. Opening the bell
| clears the badge; opening a notification, or the conversation it is
| about, marks it read. Only the acting member's own rows are ever read
| or changed. Every change tells their other open tabs at once.
|
*/

package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EventRead is the live update a member's tabs hear when notifications
are read or seen.
*/
const EventRead = "notifications.read"

/*
Publisher sends live updates to a member's own channel.
*/
type Publisher interface {
	/*
		PublishToMember sends the event to the member's tabs.
	*/
	PublishToMember(ctx context.Context, member ids.UUID, event string, data any) error
}

/*
Center serves a member's notifications.
*/
type Center struct {
	DB        *database.DB
	Clock     clock.Clock
	Publisher Publisher
}

/*
ListInput filters a member's notifications: unread only, one trigger,
and the page after a cursor.
*/
type ListInput struct {
	UnreadOnly bool
	Trigger    string
	BeforeAt   *time.Time
	BeforeID   *ids.UUID
	Limit      int32
}

/*
Page is one page of notifications with the bell's counts.
*/
type Page struct {
	Items  []models.Notification
	Unread int64
	Unseen int64
}

/*
Counts are the bell's counts.
*/
type Counts struct {
	Unread int64
	Unseen int64
}

/*
requireRead checks the caller may read their notifications.
*/
func requireRead(m authz.Membership) error {
	if !m.Can("notifications:read") {
		return exceptions.ErrForbidden
	}
	return nil
}

/*
requireWrite checks the caller may change their notifications.
*/
func requireWrite(m authz.Membership) error {
	if !m.Can("notifications:write") {
		return exceptions.ErrForbidden
	}
	return nil
}

/*
List returns a page of the member's notifications.
*/
func (c *Center) List(ctx context.Context, m authz.Membership, in ListInput) (Page, error) {
	if err := requireRead(m); err != nil {
		return Page{}, err
	}
	var out Page
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		params := models.ListMemberNotificationsParams{
			TenantID: tenantOf(m), NotifiableID: keyOf(m), UnreadOnly: in.UnreadOnly, BeforeAt: in.BeforeAt,
			BeforeID: in.BeforeID, RowLimit: max(min(in.Limit, 100), 1),
		}
		if in.Trigger != "" {
			params.Trigger = &in.Trigger
		}
		var err error
		if out.Items, err = q.ListMemberNotifications(ctx, params); err != nil {
			return fmt.Errorf("list notifications: %w", err)
		}
		counts, err := count(ctx, q, m)
		out.Unread, out.Unseen = counts.Unread, counts.Unseen
		return err
	})
	return out, err
}

/*
MarkRead marks the named notifications read, or every unread one when
none are named.
*/
func (c *Center) MarkRead(ctx context.Context, m authz.Membership, picked []ids.UUID) (Counts, error) {
	if err := requireWrite(m); err != nil {
		return Counts{}, err
	}
	return c.change(ctx, m, func(ctx context.Context, q *models.Queries) error {
		_, err := q.MarkMemberNotificationsRead(ctx, models.MarkMemberNotificationsReadParams{
			Now: new(c.Clock.Now()), TenantID: tenantOf(m), NotifiableID: keyOf(m), Ids: nonNil(picked),
		})
		return err
	})
}

/*
MarkUnread marks the named notifications unread.
*/
func (c *Center) MarkUnread(ctx context.Context, m authz.Membership, picked []ids.UUID) (Counts, error) {
	if err := requireWrite(m); err != nil {
		return Counts{}, err
	}
	return c.change(ctx, m, func(ctx context.Context, q *models.Queries) error {
		_, err := q.MarkMemberNotificationsUnread(ctx, models.MarkMemberNotificationsUnreadParams{
			TenantID: tenantOf(m), NotifiableID: keyOf(m), Ids: nonNil(picked),
		})
		return err
	})
}

/*
MarkSeen clears the bell's badge.
*/
func (c *Center) MarkSeen(ctx context.Context, m authz.Membership) (Counts, error) {
	if err := requireWrite(m); err != nil {
		return Counts{}, err
	}
	return c.change(ctx, m, func(ctx context.Context, q *models.Queries) error {
		return q.MarkMemberNotificationsSeen(ctx, models.MarkMemberNotificationsSeenParams{
			Now: new(c.Clock.Now()), TenantID: tenantOf(m), NotifiableID: keyOf(m),
		})
	})
}

/*
change runs a write, returns the counts after it and tells the
member's tabs.
*/
func (c *Center) change(ctx context.Context, m authz.Membership,
	write func(context.Context, *models.Queries) error,
) (Counts, error) {
	var out Counts
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if err := write(ctx, q); err != nil {
			return fmt.Errorf("update notifications: %w", err)
		}
		var err error
		out, err = count(ctx, q, m)
		return err
	})
	if err != nil {
		return out, err
	}
	c.publish(ctx, m.Member.ID, out)
	return out, nil
}

/*
publish tells the member's tabs their counts changed. A failure only
costs the live update.
*/
func (c *Center) publish(ctx context.Context, member ids.UUID, counts Counts) {
	if c.Publisher == nil {
		return
	}
	_ = c.Publisher.PublishToMember(ctx, member, EventRead, map[string]int64{
		"unread_count": counts.Unread, "unseen_count": counts.Unseen,
	})
}

/*
count returns the member's bell counts.
*/
func count(ctx context.Context, q *models.Queries, m authz.Membership) (Counts, error) {
	row, err := q.CountMemberNotifications(ctx, models.CountMemberNotificationsParams{
		TenantID: tenantOf(m), NotifiableID: keyOf(m),
	})
	if err != nil {
		return Counts{}, fmt.Errorf("count notifications: %w", err)
	}
	return Counts{Unread: row.Unread, Unseen: row.Unseen}, nil
}

func tenantOf(m authz.Membership) *string {
	id := m.Workspace.ID.String()
	return &id
}

func keyOf(m authz.Membership) string {
	return RecipientKey(m.Workspace.ID, m.Member.ID)
}

func nonNil(in []ids.UUID) []ids.UUID {
	if in == nil {
		return []ids.UUID{}
	}
	return in
}
