/*
|--------------------------------------------------------------------------
| Slack Notification Channel
|--------------------------------------------------------------------------
|
| Alerts reach Slack through the workspace's one Slack installation, as
| its bot: in a channel the team picked, or in a direct message with a
| teammate who linked their Slack user. The channel never holds a token;
| it asks the token service for the installation's each time, so an
| installation that is gone or broken stops every destination of it at
| once. A direct message is opened the first time and its channel kept.
|
| Slack refusing for good, such as a channel archived or a teammate who
| left, stops that destination with the reason so the settings show it;
| a rate limit or a passing failure is tried again by the queue.
|
*/

package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	fnotify "github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SlackChannelName is the notification channel that posts to Slack.
*/
const SlackChannelName = "slack"

/*
The codes a Slack destination stops with when its installation is gone
or Slack refused it for good.
*/
const (
	CodeIntegrationGone = "integration_disconnected"
	CodeSlackRefused    = "slack_refused"
)

/*
SlackRoute is where a Slack alert goes: the destination and its
installation, the Slack channel, empty for a direct message not opened
yet, and the Slack user of a direct message.
*/
type SlackRoute struct {
	WorkspaceID    ids.UUID
	DestinationID  ids.UUID
	InstallationID ids.UUID
	Channel        string
	UserID         string
}

/*
SlackMessage is a notification that can be shown in Slack as a card.
*/
type SlackMessage interface {
	/*
		ToSlack returns the card for the delivery to the route.
	*/
	ToSlack(ctx context.Context, d fnotify.Delivery, to SlackRoute) (slackdriver.Card, error)
}

/*
SlackTokens hands out an installation's bot token; the context carries
the installation's workspace.
*/
type SlackTokens interface {
	/*
		Bot returns the token acting as the app on the installation.
	*/
	Bot(ctx context.Context, installationID ids.UUID) (string, error)
}

/*
SlackChannel posts alerts to Slack. API opens Slack's Web API with a
token.
*/
type SlackChannel struct {
	DB     *database.DB
	Clock  clock.Clock
	Tokens SlackTokens
	API    func(token string) *slackdriver.API
}

/*
Send posts the notification to the recipient's Slack route, if they
have one.
*/
func (c *SlackChannel) Send(ctx context.Context, d fnotify.Delivery, n fnotify.Notification) error {
	route, ok := d.Notifiable.RouteFor(SlackChannelName).(*SlackRoute)
	if !ok || route == nil {
		return nil
	}

	m, ok := n.(SlackMessage)
	if !ok {
		return fnotify.ErrUnsupported
	}

	card, err := m.ToSlack(ctx, d, *route)
	if err != nil {
		return err
	}

	_, err = c.Post(ctx, *route, card)

	return err
}

/*
Post posts the card to the route and returns where it landed: its Slack
channel and timestamp. A refusal for good stops the destination and
comes back marked permanent.
*/
func (c *SlackChannel) Post(ctx context.Context, route SlackRoute, card slackdriver.Card) (Posted, error) {
	ctx = tenancy.With(ctx, route.WorkspaceID.String())

	api, err := c.open(ctx, route)
	if err != nil {
		return Posted{}, err
	}

	channel, err := c.target(ctx, api, route)
	if err != nil {
		return Posted{}, c.refused(ctx, route, err)
	}

	ts, err := api.PostCard(ctx, channel, "", card)
	if err != nil {
		return Posted{}, c.refused(ctx, route, err)
	}

	return Posted{Channel: channel, TS: ts}, c.write(ctx, route.WorkspaceID,
		func(ctx context.Context, q *models.Queries) error {
			now := c.Clock.Now()

			return q.RecordNotificationDestinationDelivery(ctx, models.RecordNotificationDestinationDeliveryParams{
				Now: now, WorkspaceID: route.WorkspaceID, ID: route.DestinationID,
			})
		})
}

/*
Posted is where a card landed in Slack.
*/
type Posted struct {
	Channel string
	TS      string
}

/*
open returns the Web API with the installation's bot token, stopping
the destination when the installation no longer works.
*/
func (c *SlackChannel) open(ctx context.Context, route SlackRoute) (*slackdriver.API, error) {
	token, err := c.Tokens.Bot(ctx, route.InstallationID)
	if gone(err) {
		failed := c.fail(ctx, route, CodeIntegrationGone, "The Slack installation must be reconnected.")

		return nil, errors.Join(fnotify.ErrPermanent, err, failed)
	}

	if err != nil {
		return nil, fmt.Errorf("slack token: %w", err)
	}

	return c.API(token), nil
}

/*
target returns the Slack channel the route posts in, opening and
keeping a direct message's channel the first time.
*/
func (c *SlackChannel) target(ctx context.Context, api *slackdriver.API, route SlackRoute) (string, error) {
	if route.Channel != "" {
		return route.Channel, nil
	}

	channel, err := api.OpenDirect(ctx, route.UserID)
	if err != nil {
		return "", err
	}

	config, err := json.Marshal(DestinationConfig{DMChannelID: channel})
	if err != nil {
		return "", fmt.Errorf("encode destination config: %w", err)
	}

	return channel, c.write(ctx, route.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		return q.SetNotificationDestinationConfig(ctx, models.SetNotificationDestinationConfigParams{
			Config: config, Now: c.Clock.Now(), WorkspaceID: route.WorkspaceID, ID: route.DestinationID,
		})
	})
}

/*
refused sorts Slack's refusal: a rate limit or a passing failure is
returned to be tried again, anything else stops the destination.
*/
func (c *SlackChannel) refused(ctx context.Context, route SlackRoute, err error) error {
	if _, limited := errors.AsType[*contracts.RateLimitedError](err); limited {
		return err
	}

	sendErr, ok := errors.AsType[*contracts.SendError](err)
	if !ok {
		return err
	}

	failed := c.fail(ctx, route, sendErr.Code, sendErr.Message)

	return errors.Join(fnotify.ErrPermanent, err, failed)
}

/*
fail stops the destination with the reason.
*/
func (c *SlackChannel) fail(ctx context.Context, route SlackRoute, code, message string) error {
	return c.write(ctx, route.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		return q.FailNotificationDestination(ctx, models.FailNotificationDestinationParams{
			Code: code, Message: message, Now: c.Clock.Now(), WorkspaceID: route.WorkspaceID, ID: route.DestinationID,
		})
	})
}

/*
write runs the queries in a transaction of the workspace.
*/
func (c *SlackChannel) write(ctx context.Context, ws ids.UUID, fn func(ctx context.Context, q *models.Queries) error,
) error {
	return c.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		if err := fn(ctx, models.New(tx)); err != nil {
			return fmt.Errorf("record slack destination: %w", err)
		}

		return nil
	})
}
