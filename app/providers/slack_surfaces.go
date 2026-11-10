/*
|--------------------------------------------------------------------------
| Slack For Teammates
|--------------------------------------------------------------------------
|
| What teammates get from the workspace's Slack installation besides the
| customer channel: alerts posted to the channels the team picked and to
| their own direct messages, the buttons on those alerts, the daily and
| weekly digests, and Operator answering in internal rooms and direct
| messages. Each claims its own share of Slack's deliveries on the
| channel's router, before the channel's own claims see what is left.
| When the installation is removed, its destinations stop at once.
|
*/

package providers

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/actions/notifications"
	"github.com/dimmihq/dimmi/app/actions/operatorslack"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
bootSlackSurfaces wires Slack notifications, digests and Operator in
Slack.
*/
func bootSlackSurfaces(a *app.App) error {
	if a.Actions.Slack == nil || a.Actions.Integrations == nil || a.Actions.OperatorAgent == nil {
		return nil
	}

	driver := a.Actions.Slack.Channel.Driver
	links := notifications.Links{Base: a.Application.Config.App.URL}
	channel := &notifications.SlackChannel{
		DB: a.DB, Clock: a.Clock, Tokens: a.Actions.Integrations.Tokens, API: driver.API,
	}

	a.Notify.Add(notifications.SlackChannelName, channel)
	a.Notify.Resolve(notifications.DestinationType, (&notifications.Destinations{DB: a.DB}).Resolve)

	a.Actions.NotificationDestinations = &notifications.ManageDestinations{
		DB: a.DB, Clock: a.Clock, Tokens: a.Actions.Integrations.Tokens, API: driver.API, Channel: channel,
	}

	claimSlack(a, slackBot(driver), links)
	listenForUninstalls(a)

	digests := &notifications.Digests{DB: a.DB, Clock: a.Clock, Channel: channel, Links: links}

	return queue.Register(a.Queue, &jobs.SendNotificationDigests{Digests: digests})
}

/*
slackBot opens Slack with a connection's bot token.
*/
func slackBot(driver *slackdriver.Driver) func(ctx context.Context, conn models.ChannelConnection,
) (*slackdriver.API, error) {
	return func(ctx context.Context, conn models.ChannelConnection) (*slackdriver.API, error) {
		token, err := driver.Credentials.BotToken(ctx, conn)
		if err != nil {
			return nil, fmt.Errorf("slack bot token: %w", err)
		}

		return driver.API(token), nil
	}
}

/*
claimSlack claims the alert buttons and Operator's share of Slack's
deliveries.
*/
func claimSlack(a *app.App, bot func(context.Context, models.ChannelConnection) (*slackdriver.API, error),
	links notifications.Links,
) {
	appURL := a.Application.Config.App.URL
	router := a.Actions.Slack.Router

	buttons := &notifications.SlackButtons{
		DB: a.DB, Clock: a.Clock, IDs: a.IDs, Changes: a.Actions.Inbox.ApplyChanges, Bot: bot, Links: links,
		AppURL: appURL,
	}
	pressAlert := func(ctx context.Context, in channels.SlackInteraction) (bool, error) {
		return buttons.Handle(ctx, notifications.SlackPress{
			Connection: in.Connection, Interaction: in.Interaction, Action: in.Action,
		})
	}
	router.OnAction(notifications.SlackActionPrefix, pressAlert)

	op := &operatorslack.Operator{
		DB: a.DB, Clock: a.Clock, IDs: a.IDs, Threads: a.Actions.OperatorAgent.Threads,
		Changesets: a.Actions.OperatorAgent.Changesets, Bot: bot, AppURL: appURL,
	}
	router.OnEvent("app_mention", func(ctx context.Context, ev channels.SlackEvent) (bool, error) {
		return op.OnMention(ctx, operatorslack.Asked{Connection: ev.Connection, Event: ev.Event})
	})
	router.OnEvent("message.im", func(ctx context.Context, ev channels.SlackEvent) (bool, error) {
		return op.OnDirect(ctx, operatorslack.Asked{Connection: ev.Connection, Event: ev.Event})
	})
	pressChangeset := func(ctx context.Context, in channels.SlackInteraction) (bool, error) {
		return op.OnAction(ctx, operatorslack.Pressed{
			Connection: in.Connection, Interaction: in.Interaction, Action: in.Action,
		})
	}
	router.OnAction(operatorslack.ActionPrefix, pressChangeset)
}

/*
listenForUninstalls stops an installation's destinations in the
transaction that disconnected it.
*/
func listenForUninstalls(a *app.App) {
	events.Listen(a.Events, events.InTx, func(ctx context.Context, e appevents.IntegrationDisconnected) error {
		if e.Provider != slackdriver.Type {
			return nil
		}

		scoped := tenancy.With(ctx, e.WorkspaceID.String())

		return a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
			return notifications.Uninstalled(ctx, models.New(tx), notifications.Removal{
				WorkspaceID: e.WorkspaceID, InstallationID: e.InstallationID, At: a.Clock.Now(),
			})
		})
	})
}
