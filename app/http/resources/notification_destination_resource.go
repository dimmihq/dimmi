/*
|--------------------------------------------------------------------------
| Notification Destination Resources
|--------------------------------------------------------------------------
|
| Slack destinations as the settings page shows them: where they post,
| what they hear about, their digests and whether they work. No token is
| ever part of one; a destination only names its installation.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/notifications"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/models"
)

/*
NotificationDestination is one Slack destination. SlackChannelID and
SlackChannelName say where a channel destination posts; a direct
message has neither.
*/
type NotificationDestination struct {
	ID               string     `json:"id"`
	Object           string     `json:"object"`
	Scope            string     `json:"scope"`
	Type             string     `json:"type"`
	Name             string     `json:"name"`
	IntegrationID    string     `json:"integration_id"`
	SlackChannelID   *string    `json:"slack_channel_id"`
	SlackChannelName *string    `json:"slack_channel_name"`
	Triggers         []string   `json:"triggers"`
	InboxIDs         []string   `json:"inbox_ids"`
	IncludeAIHandled bool       `json:"include_ai_handled"`
	ShowPreviews     bool       `json:"show_previews"`
	Digests          []string   `json:"digests"`
	DigestTime       string     `json:"digest_time"`
	NextDigestAt     *time.Time `json:"next_digest_at"`
	Status           string     `json:"status"`
	LastErrorCode    *string    `json:"last_error_code"`
	LastError        *string    `json:"last_error"`
	LastDeliveredAt  *time.Time `json:"last_delivered_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

/*
NewNotificationDestination shapes a destination for the API.
*/
func NewNotificationDestination(row models.NotificationDestination) NotificationDestination {
	out := NotificationDestination{
		ID: models.PublicID(models.PrefixDestination, row.ID), Object: "notification_destination", Scope: row.Scope,
		Type: row.Type, Name: row.Name,
		IntegrationID: models.PublicID(models.PrefixIntegrationInstallation, row.IntegrationInstallationID),
		Triggers:      nonNilList(row.Triggers), IncludeAIHandled: row.IncludeAiHandled,
		ShowPreviews: row.ShowPreviews, Digests: nonNilList(row.Digests), DigestTime: row.DigestTime,
		NextDigestAt: row.NextDigestAt, Status: row.Status, LastErrorCode: row.LastErrorCode,
		LastError: row.LastError, LastDeliveredAt: row.LastDeliveredAt, CreatedAt: row.CreatedAt,
	}

	if row.InboxIds != nil {
		out.InboxIDs = inboxIDs(row.InboxIds)
	}

	if c := notifications.ConfigOf(row); c.ChannelID != "" {
		out.SlackChannelID, out.SlackChannelName = &c.ChannelID, &c.ChannelName
	}

	return out
}

/*
NewNotificationDestinations shapes destinations for the API.
*/
func NewNotificationDestinations(rows []models.NotificationDestination) []NotificationDestination {
	out := make([]NotificationDestination, 0, len(rows))
	for _, row := range rows {
		out = append(out, NewNotificationDestination(row))
	}

	return out
}

/*
inboxIDs writes inbox ids as public ids.
*/
func inboxIDs(list []ids.UUID) []string {
	out := make([]string, 0, len(list))
	for _, id := range list {
		out = append(out, models.PublicID(models.PrefixInbox, id))
	}

	return out
}

/*
NotificationDestinationTest is what Slack answered a test post.
*/
type NotificationDestinationTest struct {
	Object        string  `json:"object"`
	DestinationID string  `json:"destination_id"`
	Status        string  `json:"status"`
	ErrorCode     *string `json:"error_code"`
	Error         *string `json:"error"`
}

/*
NewNotificationDestinationTest shapes a test's outcome for the API.
*/
func NewNotificationDestinationTest(id ids.UUID, r notifications.TestResult) NotificationDestinationTest {
	out := NotificationDestinationTest{
		Object: "notification_destination_test", DestinationID: models.PublicID(models.PrefixDestination, id),
		Status: r.Status,
	}

	if r.Code != "" {
		out.ErrorCode, out.Error = &r.Code, &r.Message
	}

	return out
}

/*
SlackPostable is a Slack channel a destination can post in.
*/
type SlackPostable struct {
	ID        string `json:"id"`
	Object    string `json:"object"`
	Name      string `json:"name"`
	IsPrivate bool   `json:"is_private"`
}

/*
NewSlackPostables shapes Slack channels for the API.
*/
func NewSlackPostables(list []slackdriver.Channel) []SlackPostable {
	out := make([]SlackPostable, 0, len(list))
	for _, c := range list {
		out = append(out, SlackPostable{ID: c.ID, Object: "slack_channel", Name: c.Name, IsPrivate: c.IsPrivate})
	}

	return out
}
