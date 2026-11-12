/*
|--------------------------------------------------------------------------
| Slack Channel Provider
|--------------------------------------------------------------------------
|
| Wires the Slack channel: its driver registered with the inbox, the
| router every feature claims Slack's deliveries on, the channel's own
| claims, the jobs that process deliveries, follow inbox changes and run
| auto-join scans, and the listeners on the inbox and on the Slack
| installation. Replies keep their files on the default disk. A worker
| process also holds the sockets of apps without a public URL, under
| leases shared with every other worker.
|
| The shared app is the install's own Slack app from the services
| configuration; without it only workspaces' own apps connect. Tokens of
| connections made through an installation come from the integrations'
| token service, and an uninstall is reported back to them.
|
*/

package providers

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/integrations/chatspace"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/lang"
)

/*
bootSlack builds the Slack channel, registers its driver and jobs, and
starts its sockets on a worker.
*/
func bootSlack(ctx context.Context, a *app.App) error {
	cfg := a.Config.Channels.Slack
	secrets := &channels.SlackSecrets{
		Keys: a.Keys, Installations: installationTokens{tokens: a.Actions.Integrations.Tokens},
	}
	driver := &slackdriver.Driver{
		HTTP: a.HTTP, APIURL: cfg.APIURL, Credentials: secrets, Limiter: a.Limiter,
		Places:  &channels.SlackPlaces{DB: a.DB, Clock: a.Clock},
		Markers: slackdriver.Markers{Closed: cfg.ClosedMarker, Working: cfg.WorkingMarker},
	}

	if err := a.Channels.Register(driver); err != nil {
		return err
	}

	slack, err := newSlack(a, driver, secrets)
	if err != nil {
		return err
	}

	slack.Claim(slack.Router)
	slack.Listen(a.Events)
	slack.ListenToInstallations(a.Events)

	a.Actions.Slack = &app.SlackActions{
		Channel: slack, Router: slack.Router, Edits: slack.Edits,
		Sockets: &channels.SocketSupervisor{Slack: slack, Owner: "replica:" + ids.New().String()},
	}

	startSockets(ctx, a)

	return errors.Join(
		queue.Register(a.Queue, &jobs.ProcessChannelInboundEvent{Processor: slack}),
		queue.Register(a.Queue, &jobs.ApplySlackEffect{Effects: slack}),
		queue.Register(a.Queue, &jobs.SyncSlackRooms{Syncer: slack}),
		queue.Register(a.Queue, &jobs.JoinSlackChannels{Joiner: slack}),
	)
}

/*
newSlack builds the Slack channel with the services it uses.
*/
func newSlack(a *app.App, driver *slackdriver.Driver, secrets *channels.SlackSecrets) (*channels.Slack, error) {
	cfg := a.Config.Channels.Slack
	diskName := a.Application.Config.Filesystems.Default

	disk, err := a.Storage.Disk(diskName)
	if err != nil {
		return nil, fmt.Errorf("slack disk: %w", err)
	}

	pleasantries, err := chatspace.LoadPleasantries(lang.Files(), cfg.PleasantryMaxChars)
	if err != nil {
		return nil, err
	}

	edits := &conversations.InboundEdits{DB: a.DB, Events: a.Events, Clock: a.Clock}
	services := a.Config.Services.Slack

	return &channels.Slack{
		DB: a.DB, Clock: a.Clock, Queue: a.Queue, Cache: a.Cache, Events: a.Events, IDs: a.IDs, Log: a.Log,
		Driver: driver, Secrets: secrets, Edits: edits, Pleasantries: pleasantries, Router: channels.NewRouter(),
		Config: cfg, AppURL: a.Application.Config.App.URL, Revocations: a.Actions.Integrations.ApplyRevocation,
		Scopes: a.Actions.Integrations.Registry, Articles: a.Actions.Articles,
		Shared: &channels.SharedApp{AppID: services.AppID, SigningSecret: services.SigningSecret},
		Receiver: &conversations.ReceiveInboundMessage{
			DB: a.DB, Events: a.Events, Clock: a.Clock, Disk: disk, DiskName: diskName, Queue: a.Queue,
			ReopenWindow: a.Config.Channels.Email.Inbound.ReopenWindow,
		},
	}, nil
}

/*
startSockets holds the Slack sockets this worker leases until the
application shuts down.
*/
func startSockets(ctx context.Context, a *app.App) {
	if !a.Roles.Has(foundation.RoleWorker) {
		return
	}

	sockets, stop := context.WithCancel(context.WithoutCancel(ctx))
	a.OnShutdown("slack sockets", func(context.Context) error {
		stop()

		return nil
	})

	go a.Actions.Slack.Sockets.Run(sockets)
}

/*
installationTokens hands the Slack channel the tokens of installations
from the integrations' token service. An installation gone or broken
breaks the connection; a teammate without a grant of their own simply
has none, so replies go as the bot.
*/
type installationTokens struct {
	tokens *integrations.Tokens
}

/*
BotToken returns the installation's bot token.
*/
func (t installationTokens) BotToken(ctx context.Context, workspaceID, installationID ids.UUID) (string, error) {
	token, err := t.tokens.Bot(tenancy.With(ctx, workspaceID.String()), installationID)
	if errors.Is(err, integrations.ErrIntegrationDisconnected) || errors.Is(err, integrations.ErrIntegrationBroken) {
		return "", &contracts.SendError{
			Code: slackdriver.CodeAuthFailed, Message: "The Slack installation must be reconnected.",
			Err: contracts.ErrConnectionBroken,
		}
	}

	return token, err
}

/*
UserToken returns the teammate's own Slack token on the installation.
*/
func (t installationTokens) UserToken(ctx context.Context, workspaceID, installationID, memberID ids.UUID) (
	channels.UserGrant, bool, error,
) {
	token, user, err := t.tokens.User(tenancy.With(ctx, workspaceID.String()), installationID, memberID)
	if errors.Is(err, integrations.ErrNoMemberGrant) {
		return channels.UserGrant{}, false, nil
	}

	if err != nil {
		return channels.UserGrant{}, false, err
	}

	return channels.UserGrant{Token: token, UserID: user}, true, nil
}
