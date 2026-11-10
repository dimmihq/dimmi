/*
|--------------------------------------------------------------------------
| Slack Channel
|--------------------------------------------------------------------------
|
| Everything the Slack channel does besides posting replies, built once
| with the services it uses. Deliveries are kept as they came and then
| processed by a job, so Slack hears back at once and nothing is lost.
| Processing hands each event and interaction to the router, where this
| channel and other features each claim what they handle.
|
| A connection's own facts, such as the team it is installed in and its
| bot's user, live in its config; its tokens live sealed, and only the
| token service opens them.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/integrations/chatspace"
	"github.com/dimmihq/dimmi/app/integrations/oauth"
	"github.com/dimmihq/dimmi/app/models"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
SharedApp is the install's own Slack app, the one every cloud workspace
installs: its id and signing secret. Empty means the install has no
shared app, so only workspaces' own apps connect.
*/
type SharedApp struct {
	AppID         string
	SigningSecret string
}

/*
Slack runs the Slack channel. Shared is the install's shared app,
Classifier the request classifier rooms with the ai trigger use, nil
when none is set, and Judge the comparison of drafts with sent replies,
nil when none is set. Articles drafts help articles from conversations
the team solved. Revocations reports an uninstall or revoked tokens of
a connection made through an installation to the integrations, and
Scopes is the integrations' scope registry the manifest asks for. AppURL
is the install's address, for links back.
*/
type Slack struct {
	DB           *database.DB
	Clock        clock.Clock
	Queue        *queue.Queue
	Cache        *cache.Store
	Events       *events.Dispatcher
	IDs          *ids.Registry
	Log          log.Logger
	Driver       *slackdriver.Driver
	Secrets      *SlackSecrets
	Receiver     contracts.InboundReceiver
	Edits        *conversations.InboundEdits
	Pleasantries *chatspace.Pleasantries
	Router       *Router
	Config       appconfig.SlackChannelConfig
	Shared       *SharedApp
	Classifier   RequestClassifier
	Judge        DraftJudge
	Articles     *helpcenters.Articles
	Revocations  *integrations.ApplyProviderRevocation
	Scopes       *oauth.Registry
	AppURL       string
}

/*
ConnectionFacts are what a Slack connection knows about its install:
how it was connected, how it receives, the team, the app and the bot.
*/
type ConnectionFacts struct {
	Mode             string `json:"mode,omitempty"`
	InboundTransport string `json:"inbound_transport,omitempty"`
	TeamID           string `json:"team_id,omitempty"`
	TeamName         string `json:"external_name,omitempty"`
	AppID            string `json:"app_id,omitempty"`
	BotUserID        string `json:"bot_user_id,omitempty"`
	BotID            string `json:"bot_id,omitempty"`
}

/*
The ways a connection receives, and is connected.
*/
const (
	TransportWebhook = "webhook"
	TransportSocket  = "socket"
	ModeCustom       = "custom"
	ModeShared       = "shared"
)

/*
FactsOf reads a connection's facts from its config.
*/
func FactsOf(conn models.ChannelConnection) ConnectionFacts {
	var facts ConnectionFacts
	_ = json.Unmarshal(conn.Config, &facts)

	return facts
}

/*
read runs the queries in a transaction scoped to the context's
workspace, the only way rows are visible under row level security.
*/
func (s *Slack) read(ctx context.Context, fn func(q *models.Queries) error) error {
	return s.DB.Transaction(ctx, func(_ context.Context, tx *database.Tx) error {
		return fn(models.New(tx))
	})
}

/*
connection returns the live Slack connection with the id.
*/
func (s *Slack) connection(ctx context.Context, workspaceID, id ids.UUID) (models.ChannelConnection, error) {
	var conn models.ChannelConnection

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		conn, err = q.FindChannelConnection(ctx, models.FindChannelConnectionParams{WorkspaceID: workspaceID, ID: id})

		return err
	})
	if err != nil {
		return conn, fmt.Errorf("find slack connection: %w", err)
	}

	return conn, nil
}

/*
api returns the Web API with the connection's bot token.
*/
func (s *Slack) api(ctx context.Context, conn models.ChannelConnection) (*slackdriver.API, error) {
	token, err := s.Driver.Credentials.BotToken(ctx, conn)
	if err != nil {
		return nil, err
	}

	return s.Driver.API(token), nil
}

/*
conversationLink is the inbox address of a conversation.
*/
func (s *Slack) conversationLink(conversationID ids.UUID) string {
	return s.AppURL + "/d/inbox/all/" + models.PublicID(models.PrefixConversation, conversationID)
}
