/*
|--------------------------------------------------------------------------
| Who A Slack Delivery Belongs To
|--------------------------------------------------------------------------
|
| Most deliveries belong to a channel connection. An installation of the
| shared app made only for notifications or Operator has none, and its
| deliveries still count: a press on a notification, a mention of
| Operator. Those are kept for the installation and handed to the
| router with what the installation knows, its workspace, team, app and
| bot, so the features that claim them can answer, while the channel's
| own handlers, which need rooms and an inbox, leave them alone.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/models"
)

/*
routeInstallation adds the live installation of the team on the shared
app, when there is one, as a delivery's target of its own.
*/
func (s *Slack) routeInstallation(ctx context.Context, out []routed, appID, team string) ([]routed, error) {
	found, err := models.New(s.DB).RouteIntegrationInstallation(ctx, models.RouteIntegrationInstallationParams{
		Provider: slackdriver.Type, AppID: appID, AccountID: team,
	})
	if errors.Is(err, database.ErrNotFound) {
		return out, nil
	}

	if err != nil {
		return out, fmt.Errorf("route slack installation: %w", err)
	}

	return append(out, routed{workspaceID: found.WorkspaceID, installationID: &found.IntegrationInstallationID}), nil
}

/*
connectionOf is the connection a delivery is kept for, nil for one kept
for its installation.
*/
func connectionOf(r routed) *ids.UUID {
	if r.connectionID == ids.Nil() {
		return nil
	}

	return &r.connectionID
}

/*
ownerOf names what a kept delivery belongs to.
*/
func ownerOf(row models.ChannelInboundEvent) string {
	if row.ChannelConnectionID != nil {
		return row.ChannelConnectionID.String()
	}

	if row.IntegrationInstallationID != nil {
		return row.IntegrationInstallationID.String()
	}

	return row.WorkspaceID.String()
}

/*
ownerConnection returns the live connection a delivery belongs to, or
for an installation's delivery the connection the installation stands
for, reporting false when either is gone.
*/
func (s *Slack) ownerConnection(ctx context.Context, row models.ChannelInboundEvent) (
	models.ChannelConnection, bool, error,
) {
	if row.ChannelConnectionID == nil {
		return s.installationConnection(ctx, row.WorkspaceID, row.IntegrationInstallationID)
	}

	conn, err := s.connection(ctx, row.WorkspaceID, *row.ChannelConnectionID)
	if err != nil {
		return conn, false, err
	}

	return conn, conn.DeletedAt == nil && conn.Status != "disconnected", nil
}

/*
installationConnection is an installation without a channel connection
as the router's handlers read a connection: no id, its workspace, the
installation its token opens through, and its team, app and bot.
*/
func (s *Slack) installationConnection(ctx context.Context, workspaceID ids.UUID, installationID *ids.UUID) (
	models.ChannelConnection, bool, error,
) {
	if installationID == nil {
		return models.ChannelConnection{}, false, nil
	}

	ctx = tenancy.With(ctx, workspaceID.String())

	var install models.IntegrationInstallation

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		install, err = q.FindIntegrationInstallation(ctx, models.FindIntegrationInstallationParams{
			WorkspaceID: workspaceID, ID: *installationID,
		})

		return err
	})
	if err != nil {
		return models.ChannelConnection{}, false, ignoreMissing(err)
	}

	var meta struct {
		BotUserID string `json:"bot_user_id"`
		BotID     string `json:"bot_id"`
	}

	_ = json.Unmarshal(install.Metadata, &meta)

	config, err := json.Marshal(ConnectionFacts{
		Mode: ModeShared, InboundTransport: TransportWebhook, TeamID: deref(install.ExternalAccountID),
		TeamName: install.Name, AppID: deref(install.ExternalAppID), BotUserID: meta.BotUserID, BotID: meta.BotID,
	})
	if err != nil {
		return models.ChannelConnection{}, false, fmt.Errorf("encode installation facts: %w", err)
	}

	return models.ChannelConnection{
		WorkspaceID: workspaceID, Type: slackdriver.Type, Status: "active", Config: config,
		IntegrationInstallationID: &install.ID,
	}, install.Status != "disconnected" && install.DeletedAt == nil, nil
}

/*
onConnection keeps an event handler of the channel to deliveries on a
channel connection.
*/
func onConnection(h EventHandler) EventHandler {
	return func(ctx context.Context, ev SlackEvent) (bool, error) {
		if !ev.Connected() {
			return false, nil
		}

		return h(ctx, ev)
	}
}

/*
pressOnConnection keeps an interaction handler of the channel to
deliveries on a channel connection.
*/
func pressOnConnection(h InteractionHandler) InteractionHandler {
	return func(ctx context.Context, in SlackInteraction) (bool, error) {
		if !in.Connected() {
			return false, nil
		}

		return h(ctx, in)
	}
}
