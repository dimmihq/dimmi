/*
|--------------------------------------------------------------------------
| Connecting Slack
|--------------------------------------------------------------------------
|
| A workspace connects Slack in one of two ways. Through the install's
| shared app, whose installation the integrations hold: once it is
| connected for channels, a Slack connection is made for it and its
| channels are read. Or with an app of its own, created from the
| manifest the connection hands out: the connection waits until its bot
| token and signing secret are pasted, then proves them with Slack
| before it goes live. An app on a socket also needs its app token, and
| Slack must accept it for a socket.
|
| One Slack app in one team feeds one workspace. Connecting it again in
| the same workspace says so; connecting one another workspace holds is
| refused without naming it. Everything Slack says about the app is
| checked before anything is kept, so a refused token leaves nothing.
|
*/

package channels

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConnectSlackInput is a workspace's own Slack app to connect: a name,
the inbox its conversations land in, the default inbox when nil, how it
receives, and, when already at hand, its secrets.
*/
type ConnectSlackInput struct {
	Membership authz.Membership
	Name       string
	InboxID    *ids.UUID
	Transport  string
	Secrets    Secrets
}

/*
Connected is a Slack connection with what setting its app up needs.
*/
type Connected struct {
	Connection models.ChannelConnection
	Setup      Setup
}

/*
Setup is how to create the app: its manifest, the address that creates
it in Slack, and the addresses Slack delivers to.
*/
type Setup struct {
	Manifest        map[string]any
	CreateAppURL    string
	EventsURL       string
	InteractionsURL string
}

/*
Connect connects a workspace's own Slack app, waiting for its secrets
when none were given.

Rules:
  - The caller needs channels:manage.
  - Secrets are proven with Slack before anything is kept.

Errors: [exceptions.ErrForbidden], [exceptions.ErrSlackTokenRejected],
[exceptions.ErrSlackScopesMissing], [exceptions.ErrSlackSocketModeDisabled],
[exceptions.ErrChannelAlreadyConnected],
[exceptions.ErrChannelConnectedElsewhere].
*/
func (s *Slack) Connect(ctx context.Context, in ConnectSlackInput) (Connected, error) {
	if !in.Membership.Can("channels:manage") {
		return Connected{}, exceptions.ErrForbidden
	}

	ws := in.Membership.Workspace.ID
	ctx = tenancy.With(ctx, ws.String())

	transport := TransportWebhook
	if in.Transport == TransportSocket {
		transport = TransportSocket
	}

	conn, err := s.create(ctx, ws, in, transport)
	if err != nil {
		return Connected{}, err
	}

	if in.Secrets.BotToken != "" {
		if conn, err = s.prove(ctx, conn, in.Secrets); err != nil {
			return Connected{}, err
		}
	}

	return Connected{Connection: conn, Setup: s.SetupOf(conn)}, nil
}

/*
create stores the connection, waiting for its secrets.
*/
func (s *Slack) create(ctx context.Context, ws ids.UUID, in ConnectSlackInput, transport string) (
	models.ChannelConnection, error,
) {
	config, err := json.Marshal(ConnectionFacts{Mode: ModeCustom, InboundTransport: transport})
	if err != nil {
		return models.ChannelConnection{}, fmt.Errorf("encode slack config: %w", err)
	}

	var conn models.ChannelConnection

	err = s.read(ctx, func(q *models.Queries) error {
		inbox, err := s.inboxOf(ctx, q, ws, in.InboxID)
		if err != nil {
			return err
		}

		conn, err = q.CreateSlackConnection(ctx, models.CreateSlackConnectionParams{
			WorkspaceID: ws, InboxID: inbox, Name: cmp.Or(in.Name, "Slack"), Status: "pending", Config: config,
			Now: s.Clock.Now(),
		})

		return err
	})
	if err != nil {
		return conn, fmt.Errorf("create slack connection: %w", err)
	}

	return conn, nil
}

/*
inboxOf returns the inbox named, which must be the workspace's, or the
default one.
*/
func (s *Slack) inboxOf(ctx context.Context, q *models.Queries, ws ids.UUID, named *ids.UUID) (ids.UUID, error) {
	if named == nil {
		inbox, err := q.FindDefaultInbox(ctx, ws)

		return inbox.ID, err
	}

	inbox, err := q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: ws, ID: *named})
	if errors.Is(err, database.ErrNotFound) || (err == nil && inbox.DeletedAt != nil) {
		return ids.Nil(), exceptions.ErrNotFound.OnField("inbox_id")
	}

	return inbox.ID, err
}

/*
SetSecrets proves and stores new secrets for the workspace's Slack
connection.

Errors: as [Slack.Connect], and [exceptions.ErrNotFound] for another
workspace's or another type's connection.
*/
func (s *Slack) SetSecrets(ctx context.Context, m authz.Membership, connectionID ids.UUID, secrets Secrets) (
	models.ChannelConnection, error,
) {
	if !m.Can("channels:manage") {
		return models.ChannelConnection{}, exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	conn, err := s.connection(ctx, m.Workspace.ID, connectionID)
	gone := err == nil && (conn.Type != slackdriver.Type || conn.DeletedAt != nil)
	if errors.Is(err, database.ErrNotFound) || gone {
		return conn, exceptions.ErrNotFound
	}

	if err != nil {
		return conn, err
	}

	return s.prove(ctx, conn, secrets)
}

/*
prove checks the secrets with Slack and, when Slack accepts them, keeps
them and makes the connection live, then reads its channels.
*/
func (s *Slack) prove(ctx context.Context, conn models.ChannelConnection, secrets Secrets) (
	models.ChannelConnection, error,
) {
	facts := FactsOf(conn)

	if err := shapeOf(secrets); err != nil {
		return conn, err
	}

	identity, err := s.identify(ctx, facts, secrets)
	if err != nil {
		return conn, err
	}

	facts.TeamID, facts.TeamName, facts.AppID = identity.TeamID, identity.TeamName, identity.AppID
	facts.BotUserID, facts.BotID = identity.UserID, identity.BotID

	external := slackdriver.ExternalID(identity.AppID, identity.TeamID)
	if err := s.unclaimed(ctx, conn, external); err != nil {
		return conn, err
	}

	return s.goLive(ctx, conn, facts, secrets)
}

/*
shapeOf refuses secrets that cannot be a Slack app's: a bot token of
another kind, a missing signing secret, or an app token of another kind.
*/
func shapeOf(secrets Secrets) error {
	switch {
	case !strings.HasPrefix(secrets.BotToken, "xoxb-"):
		return exceptions.ErrSlackTokenRejected
	case secrets.SigningSecret == "":
		return exceptions.ErrSlackSigningSecretRequired
	case secrets.AppToken != "" && !strings.HasPrefix(secrets.AppToken, "xapp-"):
		return exceptions.ErrSlackSocketModeDisabled
	default:
		return nil
	}
}

/*
identify asks Slack who the bot token is, checks it holds every scope
the channel needs, and that the app token opens a socket when one is
needed.
*/
func (s *Slack) identify(ctx context.Context, facts ConnectionFacts, secrets Secrets) (slackdriver.Identity, error) {
	api := s.Driver.API(secrets.BotToken)

	identity, err := api.AuthTest(ctx)
	if errors.Is(err, contracts.ErrConnectionBroken) || errors.Is(err, contracts.ErrPermanent) {
		return identity, exceptions.ErrSlackTokenRejected
	}

	if err != nil {
		return identity, exceptions.ErrChannelUnavailable
	}

	if missing := missingScopes(identity.Scopes); len(missing) > 0 {
		return identity, exceptions.ErrSlackScopesMissing.With("missing", missing)
	}

	if identity.AppID, err = api.BotAppID(ctx, identity.BotID); err != nil {
		return identity, exceptions.ErrSlackTokenRejected
	}

	if facts.InboundTransport != TransportSocket {
		return identity, nil
	}

	err = slackdriver.CheckAppToken(ctx, slackdriver.SocketOptions{
		AppToken: secrets.AppToken, BotToken: secrets.BotToken, HTTP: s.Driver.HTTP, APIURL: s.Driver.APIURL,
	})
	if secrets.AppToken == "" || err != nil {
		return identity, exceptions.ErrSlackSocketModeDisabled
	}

	return identity, nil
}

/*
missingScopes returns the scopes the channel needs that the token
lacks. A token whose scopes Slack does not list is taken as complete.
*/
func missingScopes(granted []string) []string {
	if len(granted) == 0 {
		return nil
	}

	var missing []string

	for _, scope := range slackdriver.BotScopes() {
		if !slices.Contains(granted, scope) {
			missing = append(missing, scope)
		}
	}

	return missing
}

/*
unclaimed refuses an app and team another live connection holds.
*/
func (s *Slack) unclaimed(ctx context.Context, conn models.ChannelConnection, external string) error {
	return s.read(ctx, func(q *models.Queries) error {
		found, err := q.FindLiveChannelConnectionByExternalID(ctx, models.FindLiveChannelConnectionByExternalIDParams{
			WorkspaceID: conn.WorkspaceID, Type: slackdriver.Type, ExternalID: &external,
		})
		if err == nil && found.ID != conn.ID {
			return exceptions.ErrChannelAlreadyConnected
		}

		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find slack connection: %w", err)
		}

		_, err = q.RouteChannelConnection(ctx, models.RouteChannelConnectionParams{
			ConnectionType: slackdriver.Type, ProviderKey: external,
		})
		if err == nil && found.ID != conn.ID {
			return exceptions.ErrChannelConnectedElsewhere
		}

		return ignoreMissing(err)
	})
}

/*
goLive keeps the secrets and the facts, makes the connection live and
queues reading its channels.
*/
func (s *Slack) goLive(ctx context.Context, conn models.ChannelConnection, facts ConnectionFacts,
	secrets Secrets,
) (models.ChannelConnection, error) {
	sealed, err := s.Secrets.Seal(ctx, conn, secrets)
	if err != nil {
		return conn, err
	}

	config, err := json.Marshal(facts)
	if err != nil {
		return conn, fmt.Errorf("encode slack config: %w", err)
	}

	external := slackdriver.ExternalID(facts.AppID, facts.TeamID)

	err = s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		conn, err = models.New(tx).ConnectChannelConnection(ctx, models.ConnectChannelConnectionParams{
			ExternalID: &external, Config: config, CredentialsEncrypted: sealed, Now: s.Clock.Now(),
			WorkspaceID: conn.WorkspaceID, ID: conn.ID,
		})
		if database.IsUniqueViolation(err, "") {
			return exceptions.ErrChannelConnectedElsewhere
		}

		if err != nil {
			return fmt.Errorf("connect slack: %w", err)
		}

		return s.Queue.PushTx(ctx, tx, jobs.SyncSlackRoomsArgs{WorkspaceID: conn.WorkspaceID, ConnectionID: conn.ID})
	})

	return conn, err
}

/*
SetupOf returns how to create the connection's app in Slack.
*/
func (s *Slack) SetupOf(conn models.ChannelConnection) Setup {
	public := models.PublicID(models.PrefixChannelConnection, conn.ID)
	opts := slackdriver.ManifestOptions{
		Name: conn.Name, AppURL: s.AppURL, ConnectionID: public,
		Socket: FactsOf(conn).InboundTransport == TransportSocket,
	}

	if s.Scopes != nil {
		opts.BotScopes = s.Scopes.BotScopes(slackdriver.Type, nil)
		opts.UserScopes = s.Scopes.UserScopes(slackdriver.Type)
	}

	manifest := slackdriver.Manifest(opts)
	create, _ := slackdriver.CreateAppURL(manifest)

	return Setup{
		Manifest: manifest, CreateAppURL: create, EventsURL: s.AppURL + "/webhooks/slack/events/" + public,
		InteractionsURL: s.AppURL + "/webhooks/slack/interactions/" + public,
	}
}
