/*
|--------------------------------------------------------------------------
| Notification Destinations
|--------------------------------------------------------------------------
|
| A destination is a place in Slack alerts go besides a member's own
| channels: a channel the team picked, which hears about the triggers it
| listed, or a teammate's direct message with the bot. Each names the
| Slack installation it posts through and keeps only facts about Slack,
| never a token. A channel destination is a recipient of its own, so its
| posts are queued and retried like any member's alert.
|
*/

package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	fnotify "github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
DestinationType names channel destinations as notification recipients.
*/
const DestinationType = "notification_destination"

/*
The kinds, scopes and statuses of destinations.
*/
const (
	DestinationSlackChannel = "slack_channel"
	DestinationSlackDM      = "slack_dm"
	ScopeWorkspace          = "workspace"
	ScopeMember             = "member"
	StatusActive            = "active"
	StatusError             = "error"
)

/*
DestinationConfig are the Slack facts a destination keeps: the channel
it posts in and its name, or the teammate's Slack user and the channel
of their direct message once it was opened.
*/
type DestinationConfig struct {
	ChannelID   string `json:"slack_channel_id,omitempty"`
	ChannelName string `json:"slack_channel_name,omitempty"`
	UserID      string `json:"slack_user_id,omitempty"`
	DMChannelID string `json:"slack_dm_channel_id,omitempty"`
}

/*
ConfigOf reads a destination's Slack facts.
*/
func ConfigOf(row models.NotificationDestination) DestinationConfig {
	var c DestinationConfig
	_ = json.Unmarshal(row.Config, &c)

	return c
}

/*
RouteOf is where the destination's posts go.
*/
func RouteOf(row models.NotificationDestination) *SlackRoute {
	c := ConfigOf(row)
	channel := c.ChannelID
	if row.Type == DestinationSlackDM {
		channel = c.DMChannelID
	}

	return &SlackRoute{
		WorkspaceID: row.WorkspaceID, DestinationID: row.ID, InstallationID: row.IntegrationInstallationID,
		Channel: channel, UserID: c.UserID,
	}
}

/*
Destination is a Slack channel destination receiving alerts.
*/
type Destination struct {
	Row models.NotificationDestination
}

/*
NotifiableType names the recipient type.
*/
func (Destination) NotifiableType() string { return DestinationType }

/*
NotifiableID names the destination inside its workspace.
*/
func (d Destination) NotifiableID() string { return RecipientKey(d.Row.WorkspaceID, d.Row.ID) }

/*
RouteFor reaches the destination on Slack alone.
*/
func (d Destination) RouteFor(channel string) any {
	if channel != SlackChannelName {
		return nil
	}

	return RouteOf(d.Row)
}

/*
Destinations loads destinations as recipients.
*/
type Destinations struct {
	DB *database.DB
}

/*
Resolve loads the working destination a key names, for a queued
delivery.
*/
func (r *Destinations) Resolve(ctx context.Context, key string) (fnotify.Notifiable, error) {
	ws, id, err := parseKey(key)
	if err != nil {
		return nil, err
	}

	var row models.NotificationDestination

	err = r.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		row, err = models.New(tx).FindNotificationDestination(ctx, models.FindNotificationDestinationParams{
			WorkspaceID: ws, ID: id,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && row.Status != StatusActive) {
		return nil, fnotify.ErrNotifiableGone
	}

	if err != nil {
		return nil, fmt.Errorf("load notification destination: %w", err)
	}

	return Destination{Row: row}, nil
}

/*
memberSlack returns the member's working Slack direct message route,
reporting false when they have none.
*/
func memberSlack(ctx context.Context, q *models.Queries, ws, member ids.UUID) (*SlackRoute, bool, error) {
	row, err := q.FindMemberSlackDestination(ctx, models.FindMemberSlackDestinationParams{
		WorkspaceID: ws, MemberID: member,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, fmt.Errorf("load slack destination: %w", err)
	}

	return RouteOf(row), true, nil
}
