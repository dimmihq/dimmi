/*
|--------------------------------------------------------------------------
| Notification Controller
|--------------------------------------------------------------------------
|
| A member's own bell and notification setup: the list and its read
| state, the settings matrix, settings, the focus heartbeat and test
| sends. Every endpoint acts on the calling member alone.
|
*/

package v1

import (
	"context"
	"strconv"
	"strings"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/notifications"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
NotificationController serves a member's notifications.
*/
type NotificationController struct {
	Center   *notifications.Center
	Presence *notifications.Presence
	IDs      *ids.Registry
}

/*
NotificationsOutput is a page of notifications with the bell's counts.
*/
type NotificationsOutput struct {
	Body struct {
		Data []resources.Notification `json:"data"`
		Meta struct {
			resources.NotificationCounts
			NextCursor *string `json:"next_cursor"`
		} `json:"meta"`
	}
}

/*
NotificationCountsOutput is the bell's counts after a change.
*/
type NotificationCountsOutput struct {
	Body struct {
		Meta resources.NotificationCounts `json:"meta"`
	}
}

/*
NotificationPreferencesOutput is a member's matrix and settings.
*/
type NotificationPreferencesOutput struct {
	Body fhttp.Resource[resources.NotificationPreferences]
}

/*
NotificationSettingsOutput is a member's settings.
*/
type NotificationSettingsOutput struct {
	Body fhttp.Resource[resources.NotificationSettings]
}

/*
Index lists the member's notifications.
*/
func (c *NotificationController) Index(ctx context.Context, in *requests.IndexNotificationsRequest,
) (*NotificationsOutput, error) {
	limit := in.Limit
	if limit == 0 {
		limit = 20
	}
	list := notifications.ListInput{UnreadOnly: in.Status == "unread", Trigger: in.Trigger, Limit: limit + 1}
	keep := int(limit)
	list.BeforeAt, list.BeforeID = readCursor(in.Cursor)
	page, err := c.Center.List(ctx, membership(ctx), list)
	if err != nil {
		return nil, err
	}
	out := &NotificationsOutput{}
	items := page.Items
	if len(items) > keep {
		items = items[:keep]
		last := items[len(items)-1]
		next := strconv.FormatInt(last.UpdatedAt.UnixMicro(), 10) + "_" + last.ID.String()
		out.Body.Meta.NextCursor = &next
	}
	out.Body.Data = resources.NewNotifications(items)
	out.Body.Meta.UnreadCount, out.Body.Meta.UnseenCount = page.Unread, page.Unseen
	return out, nil
}

/*
readCursor reads a page cursor, nothing for none or a bad one.
*/
func readCursor(cursor string) (*time.Time, *ids.UUID) {
	at, id, ok := strings.Cut(cursor, "_")
	if !ok {
		return nil, nil
	}
	micros, err := strconv.ParseInt(at, 10, 64)
	if err != nil {
		return nil, nil
	}
	parsed, err := ids.Parse(id)
	if err != nil {
		return nil, nil
	}
	t := time.UnixMicro(micros)
	return &t, &parsed
}

/*
Update marks notifications read, unread or seen.
*/
func (c *NotificationController) Update(ctx context.Context, in *requests.UpdateNotificationsRequest,
) (*NotificationCountsOutput, error) {
	m, b := membership(ctx), in.Body
	var counts notifications.Counts
	var err error
	switch {
	case b.Seen != nil && *b.Seen:
		counts, err = c.Center.MarkSeen(ctx, m)
	case b.Read != nil && *b.Read:
		counts, err = c.Center.MarkRead(ctx, m, c.notificationIDs(b.IDs))
	case b.Read != nil:
		counts, err = c.Center.MarkUnread(ctx, m, c.notificationIDs(b.IDs))
	default:
		counts, err = c.Center.MarkSeen(ctx, m)
	}
	if err != nil {
		return nil, err
	}
	out := &NotificationCountsOutput{}
	out.Body.Meta = resources.NotificationCounts{UnreadCount: counts.Unread, UnseenCount: counts.Unseen}
	return out, nil
}

/*
notificationIDs decodes public ids, skipping ones that are not
notifications.
*/
func (c *NotificationController) notificationIDs(public []string) []ids.UUID {
	out := make([]ids.UUID, 0, len(public))
	for _, p := range public {
		if id, err := c.IDs.Decode(string(models.PrefixNotification), p); err == nil {
			out = append(out, id)
		}
	}
	return out
}

/*
Preferences returns the member's matrix and settings.
*/
func (c *NotificationController) Preferences(ctx context.Context, _ *requests.NoInput,
) (*NotificationPreferencesOutput, error) {
	matrix, err := c.Center.Preferences(ctx, membership(ctx))
	return preferencesOutput(matrix, err)
}

/*
UpdatePreferences changes cells of the matrix.
*/
func (c *NotificationController) UpdatePreferences(ctx context.Context,
	in *requests.UpdateNotificationPreferencesRequest,
) (*NotificationPreferencesOutput, error) {
	changes := make([]notifications.CellChange, len(in.Body.Preferences))
	for i, p := range in.Body.Preferences {
		changes[i] = notifications.CellChange(p)
	}
	matrix, err := c.Center.UpdatePreferences(ctx, membership(ctx), changes)
	return preferencesOutput(matrix, err)
}

/*
ResetPreferences puts the matrix back to its defaults.
*/
func (c *NotificationController) ResetPreferences(ctx context.Context, _ *requests.NoInput,
) (*NotificationPreferencesOutput, error) {
	matrix, err := c.Center.ResetPreferences(ctx, membership(ctx))
	return preferencesOutput(matrix, err)
}

func preferencesOutput(m notifications.Matrix, err error) (*NotificationPreferencesOutput, error) {
	if err != nil {
		return nil, err
	}
	return &NotificationPreferencesOutput{Body: fhttp.NewResource(resources.NewNotificationPreferences(m))}, nil
}

/*
UpdateSettings changes the member's settings.
*/
func (c *NotificationController) UpdateSettings(ctx context.Context, in *requests.UpdateNotificationSettingsRequest,
) (*NotificationSettingsOutput, error) {
	b := in.Body
	s, err := c.Center.UpdateSettings(ctx, membership(ctx), notifications.SettingsChange{
		Timezone: b.Timezone, QuietHoursEnabled: b.QuietHoursEnabled, QuietHoursStart: b.QuietHoursStart,
		QuietHoursEnd: b.QuietHoursEnd, QuietHoursDays: b.QuietHoursDays, QuietHoursBypass: b.QuietHoursBypass,
		PausedUntil: b.PausedUntil, ClearPause: b.ClearPause, ShowPreviews: b.ShowPreviews,
		IncludeAIHandled: b.IncludeAIHandled,
	})
	if err != nil {
		return nil, err
	}
	return &NotificationSettingsOutput{Body: fhttp.NewResource(resources.NewNotificationSettings(s))}, nil
}

/*
Focus records the dashboard's heartbeat.
*/
func (c *NotificationController) Focus(ctx context.Context, in *requests.NotificationFocusRequest,
) (*EmptyOutput, error) {
	if !in.Body.Visible {
		return &EmptyOutput{}, c.Presence.Blur(ctx, membership(ctx))
	}
	var conversation *ids.UUID
	if id, err := c.IDs.Decode(string(models.PrefixConversation), in.Body.ConversationID); err == nil {
		conversation = &id
	}
	return &EmptyOutput{}, c.Presence.Focus(ctx, membership(ctx), conversation)
}

/*
Test sends the member a test notification.
*/
func (c *NotificationController) Test(ctx context.Context, in *requests.NotificationTestRequest,
) (*EmptyOutput, error) {
	return &EmptyOutput{}, c.Presence.Test(ctx, membership(ctx), in.Body.Channel)
}
