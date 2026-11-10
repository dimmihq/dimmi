/*
|--------------------------------------------------------------------------
| Slack Through An Installation
|--------------------------------------------------------------------------
|
| One Slack installation serves every feature that uses Slack. When it
| connects for channels, the workspace gets its Slack connection: it
| points at the installation and never copies a token, lands in the
| default inbox, and reads the channels its app is already in. Slack
| connecting again for the same installation brings its connection back
| instead of making a second one. When the installation is disconnected,
| so is the channel.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/tenancy"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
featureChannels is the integrations feature the Slack channel is.
*/
const featureChannels = "channels"

/*
ListenToInstallations follows the life of Slack installations, in the
transaction that changed them.
*/
func (s *Slack) ListenToInstallations(d *events.Dispatcher) {
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.IntegrationConnected) error {
		if e.Provider != slackdriver.Type || !slices.Contains(e.Features, featureChannels) {
			return nil
		}

		return s.Installed(ctx, e)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.IntegrationDisconnected) error {
		if e.Provider != slackdriver.Type {
			return nil
		}

		return s.Uninstalled(ctx, e)
	})
}

/*
Installed makes or brings back the Slack connection of an installation
that connected for channels, then reads its rooms and checks its
channels against the auto-join rules.
*/
func (s *Slack) Installed(ctx context.Context, e appevents.IntegrationConnected) error {
	ctx = tenancy.With(ctx, e.WorkspaceID.String())

	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		install, err := q.FindIntegrationInstallation(ctx, models.FindIntegrationInstallationParams{
			WorkspaceID: e.WorkspaceID, ID: e.InstallationID,
		})
		if err != nil {
			return fmt.Errorf("find installation: %w", err)
		}

		config, err := json.Marshal(installedFacts(install, e))
		if err != nil {
			return fmt.Errorf("encode slack config: %w", err)
		}

		conn, err := s.installedConnection(ctx, q, install, config)
		if err != nil {
			return err
		}

		if err := s.Queue.PushTx(ctx, tx, jobs.SyncSlackRoomsArgs{
			WorkspaceID: conn.WorkspaceID, ConnectionID: conn.ID,
		}); err != nil {
			return err
		}

		return s.Queue.PushTx(ctx, tx, jobs.JoinSlackChannelsArgs{WorkspaceID: conn.WorkspaceID, ConnectionID: conn.ID})
	})
}

/*
installedFacts are what the connection knows of its installation.
*/
func installedFacts(install models.IntegrationInstallation, e appevents.IntegrationConnected) ConnectionFacts {
	var meta struct {
		BotUserID string `json:"bot_user_id"`
		BotID     string `json:"bot_id"`
	}

	_ = json.Unmarshal(install.Metadata, &meta)

	return ConnectionFacts{
		Mode: ModeShared, InboundTransport: TransportWebhook, TeamID: e.ExternalAccountID, TeamName: install.Name,
		AppID: e.ExternalAppID, BotUserID: meta.BotUserID, BotID: meta.BotID,
	}
}

/*
installedConnection returns the installation's connection, live again:
its own, the workspace's disconnected one of the same team, which an
installation made again after a removal carries on, or a new one in
the default inbox.
*/
func (s *Slack) installedConnection(ctx context.Context, q *models.Queries, install models.IntegrationInstallation,
	config []byte,
) (models.ChannelConnection, error) {
	external := slackdriver.ExternalID(deref(install.ExternalAppID), deref(install.ExternalAccountID))

	conn, found, err := s.ownedConnection(ctx, q, install)
	if err != nil {
		return conn, err
	}

	if found {
		return q.ConnectChannelConnection(ctx, models.ConnectChannelConnectionParams{
			ExternalID: &external, Config: config, Now: s.Clock.Now(), WorkspaceID: conn.WorkspaceID, ID: conn.ID,
		})
	}

	inbox, err := q.FindDefaultInbox(ctx, install.WorkspaceID)
	if err != nil {
		return conn, fmt.Errorf("find default inbox: %w", err)
	}

	return q.CreateSlackConnection(ctx, models.CreateSlackConnectionParams{
		WorkspaceID: install.WorkspaceID, InboxID: inbox.ID, Name: install.Name, Status: "active",
		ExternalID: &external, Config: config, IntegrationInstallationID: &install.ID, Now: s.Clock.Now(),
	})
}

/*
ownedConnection finds the connection the installation feeds, adopting
the workspace's disconnected one of the same team when it has none.
*/
func (s *Slack) ownedConnection(ctx context.Context, q *models.Queries, install models.IntegrationInstallation,
) (models.ChannelConnection, bool, error) {
	conn, err := q.FindInstallationChannelConnection(ctx, models.FindInstallationChannelConnectionParams{
		WorkspaceID: install.WorkspaceID, IntegrationInstallationID: &install.ID,
	})
	if err == nil {
		return conn, true, nil
	}

	if !errors.Is(err, database.ErrNotFound) {
		return conn, false, fmt.Errorf("find slack connection: %w", err)
	}

	conn, err = q.FindAdoptableSlackConnection(ctx, models.FindAdoptableSlackConnectionParams{
		WorkspaceID: install.WorkspaceID, TeamID: deref(install.ExternalAccountID),
	})
	if errors.Is(err, database.ErrNotFound) {
		return conn, false, nil
	}

	if err != nil {
		return conn, false, fmt.Errorf("find earlier slack connection: %w", err)
	}

	err = q.AdoptChannelConnection(ctx, models.AdoptChannelConnectionParams{
		IntegrationInstallationID: &install.ID, Now: s.Clock.Now(), WorkspaceID: conn.WorkspaceID, ID: conn.ID,
	})
	if err != nil {
		return conn, false, fmt.Errorf("adopt slack connection: %w", err)
	}

	return conn, true, nil
}

/*
Uninstalled disconnects the Slack connection of a disconnected
installation and frees its address, so the team can be connected again.
*/
func (s *Slack) Uninstalled(ctx context.Context, e appevents.IntegrationDisconnected) error {
	ctx = tenancy.With(ctx, e.WorkspaceID.String())

	var conn models.ChannelConnection

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		conn, err = q.FindInstallationChannelConnection(ctx, models.FindInstallationChannelConnectionParams{
			WorkspaceID: e.WorkspaceID, IntegrationInstallationID: &e.InstallationID,
		})

		return err
	})
	if err != nil {
		return ignoreMissing(err)
	}

	if err := s.disconnect(ctx, conn, "Slack was disconnected."); err != nil {
		return err
	}

	return s.read(ctx, func(q *models.Queries) error {
		err := q.ReleaseChannelConnectionAddress(ctx, models.ReleaseChannelConnectionAddressParams{
			Now: s.Clock.Now(), WorkspaceID: conn.WorkspaceID, ID: conn.ID,
		})
		if err != nil {
			return fmt.Errorf("release slack address: %w", err)
		}

		return nil
	})
}

/*
deref returns the text, or empty for none.
*/
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
