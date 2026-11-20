/*
|--------------------------------------------------------------------------
| WhatsApp Channel
|--------------------------------------------------------------------------
|
| Everything the WhatsApp channel does besides sending replies, built
| once with the services it uses. Meta's deliveries are checked, kept as
| they came and processed by a job, so Meta hears back at once and
| nothing is lost. Processing turns customers' messages into inbound
| messages for the inbox, moves each customer's 24-hour window, records
| how far the business's messages got, and keeps templates current.
|
| A connection's facts, such as its number and WhatsApp account, live in
| its config; its token lives sealed in its row, bound to it, and only
| this channel opens it.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/contracts"
	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
The ways a WhatsApp number is connected: through the install's own
Meta app, by Embedded Signup or a token of it, or through a workspace's
own Meta app, whose secret signs its deliveries.
*/
const (
	WhatsAppModeShared = "shared"
	WhatsAppModeOwn    = "own"
)

/*
WhatsApp runs the WhatsApp channel. Service is the install's own Meta
app, empty when it has none. AppURL is the install's address, for the
webhook address a workspace's own app is given.
*/
type WhatsApp struct {
	DB       *database.DB
	Clock    clock.Clock
	Queue    *queue.Queue
	Cache    *cache.Store
	Events   *events.Dispatcher
	IDs      *ids.Registry
	Log      log.Logger
	Keys     *encryption.Keys
	Driver   *wadriver.Driver
	Receiver contracts.InboundReceiver
	Config   appconfig.WhatsAppChannelConfig
	Service  appconfig.WhatsAppServiceConfig
	AppURL   string
}

/*
WhatsAppFactsOf reads a WhatsApp connection's facts from its config.
*/
func WhatsAppFactsOf(conn models.ChannelConnection) wadriver.Facts {
	var facts wadriver.Facts
	_ = json.Unmarshal(conn.Config, &facts)

	return facts
}

/*
WhatsAppSecrets are what a WhatsApp connection calls and checks Meta
with: its token, and for a workspace's own app the app's secret and the
token Meta echoes when it checks the webhook address.
*/
type WhatsAppSecrets struct {
	AccessToken string `json:"access_token,omitempty"`
	AppSecret   string `json:"app_secret,omitempty"`
	VerifyToken string `json:"verify_token,omitempty"`
	PIN         string `json:"pin,omitempty"`
}

/*
seal seals the secrets for the connection's row.
*/
func (w *WhatsApp) seal(ctx context.Context, conn models.ChannelConnection, secrets WhatsAppSecrets) (
	encryption.Sealed, error,
) {
	sealed, err := encryption.Seal(ctx, w.Keys, encryption.Slot{
		Scope: encryption.Scope{Kind: "workspace", ID: conn.WorkspaceID.String()}, AAD: secretsAAD(conn.ID),
	}, secrets)
	if err != nil {
		return nil, fmt.Errorf("seal whatsapp secrets: %w", err)
	}

	return sealed, nil
}

/*
open opens the connection's secrets, empty when it has none.
*/
func (w *WhatsApp) open(ctx context.Context, conn models.ChannelConnection) (WhatsAppSecrets, error) {
	if len(conn.CredentialsEncrypted) == 0 {
		return WhatsAppSecrets{}, nil
	}

	secrets, err := encryption.Open[WhatsAppSecrets](ctx, w.Keys, conn.CredentialsEncrypted, secretsAAD(conn.ID))
	if err != nil {
		return WhatsAppSecrets{}, fmt.Errorf("open whatsapp secrets: %w", err)
	}

	return secrets, nil
}

/*
Token returns the token the connection calls Meta with. A connection
without one is broken until it is connected again.
*/
func (w *WhatsApp) Token(ctx context.Context, conn models.ChannelConnection) (string, error) {
	secrets, err := w.open(ctx, conn)
	if err != nil {
		return "", err
	}

	if secrets.AccessToken == "" {
		return "", &contracts.SendError{
			Code: wadriver.CodeConnectionBroken, Message: "The WhatsApp connection has no token. Connect it again.",
			Err: contracts.ErrConnectionBroken,
		}
	}

	return secrets.AccessToken, nil
}

/*
graph returns the Graph API with the token.
*/
func (w *WhatsApp) graph(token string) *wadriver.Graph {
	return w.Driver.Graph(token)
}

/*
read runs the queries in a transaction scoped to the context's
workspace, the only way rows are visible under row level security.
*/
func (w *WhatsApp) read(ctx context.Context, fn func(q *models.Queries) error) error {
	return w.DB.Transaction(ctx, func(_ context.Context, tx *database.Tx) error {
		return fn(models.New(tx))
	})
}

/*
connection returns the workspace's live WhatsApp connection with the id.
*/
func (w *WhatsApp) connection(ctx context.Context, workspaceID, id ids.UUID) (models.ChannelConnection, error) {
	var conn models.ChannelConnection

	err := w.read(ctx, func(q *models.Queries) error {
		var err error
		conn, err = q.FindChannelConnection(ctx, models.FindChannelConnectionParams{WorkspaceID: workspaceID, ID: id})

		return err
	})

	gone := err == nil && (conn.Type != wadriver.Type || conn.DeletedAt != nil)
	if errors.Is(err, database.ErrNotFound) || gone {
		return conn, exceptions.ErrNotFound
	}

	return conn, err
}

/*
WhatsAppPlaces hands the driver what it needs from the inbox: the parts
a reply became, the customer's window and the connection's templates.
*/
type WhatsAppPlaces struct {
	DB    *database.DB
	Clock clock.Clock
	// Sync copies a connection's templates from Meta.
	Sync func(ctx context.Context, conn models.ChannelConnection) error
}

/*
read runs the queries in a transaction scoped to the context's
workspace.
*/
func (p *WhatsAppPlaces) read(ctx context.Context, fn func(q *models.Queries) error) error {
	return p.DB.Transaction(ctx, func(_ context.Context, tx *database.Tx) error {
		return fn(models.New(tx))
	})
}

/*
SentParts returns the WhatsApp messages the reply already became.
*/
func (p *WhatsAppPlaces) SentParts(ctx context.Context, conn models.ChannelConnection, messageID ids.UUID) (
	[]wadriver.Part, error,
) {
	var out []wadriver.Part

	err := p.read(ctx, func(q *models.Queries) error {
		rows, err := q.ListChannelMessageRefs(ctx, models.ListChannelMessageRefsParams{
			WorkspaceID: conn.WorkspaceID, MessageID: messageID,
		})
		for _, row := range rows {
			if row.ChannelConnectionID == conn.ID {
				out = append(out, wadriver.Part{
					ExternalID: row.ExternalMessageID, Index: int(row.PartIndex), Kind: row.Kind,
				})
			}
		}

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list whatsapp parts: %w", err)
	}

	return out, nil
}

/*
RecordParts remembers the WhatsApp messages sent for the reply, so a
status report for any of them finds it.
*/
func (p *WhatsAppPlaces) RecordParts(
	ctx context.Context, conn models.ChannelConnection, messageID ids.UUID, parts []wadriver.Part,
) error {
	return p.read(ctx, func(q *models.Queries) error {
		for _, part := range parts {
			err := q.CreateChannelMessageRef(ctx, models.CreateChannelMessageRefParams{
				WorkspaceID: conn.WorkspaceID, MessageID: messageID, ChannelConnectionID: conn.ID,
				ExternalMessageID: part.ExternalID, PartIndex: int64(part.Index), Kind: part.Kind, Now: p.Clock.Now(),
			})
			if err != nil {
				return fmt.Errorf("record whatsapp part: %w", err)
			}
		}

		return nil
	})
}

/*
Window returns the customer's window on the connection, closed when
they never wrote.
*/
func (p *WhatsAppPlaces) Window(ctx context.Context, conn models.ChannelConnection, contactID ids.UUID) (
	wadriver.Window, error,
) {
	state, err := windowOf(ctx, p.DB, conn, contactID)

	return state, err
}

/*
windowOf reads the customer's window on the connection.
*/
func windowOf(ctx context.Context, db *database.DB, conn models.ChannelConnection, contactID ids.UUID) (
	wadriver.Window, error,
) {
	var out wadriver.Window

	err := db.Transaction(ctx, func(_ context.Context, tx *database.Tx) error {
		state, err := models.New(tx).FindChannelContactState(ctx, models.FindChannelContactStateParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, ContactID: contactID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err == nil && state.WindowExpiresAt != nil {
			out.ExpiresAt = *state.WindowExpiresAt
		}

		if err == nil && state.LastInboundMessageID != nil {
			out.LastInboundID = *state.LastInboundMessageID
		}

		return err
	})
	if err != nil {
		return out, fmt.Errorf("find whatsapp window: %w", err)
	}

	return out, nil
}

/*
RefreshTemplates copies the connection's templates from Meta again. When
Meta cannot be asked now, the copy is marked out of date instead, so the
next look at the templates asks Meta.
*/
func (p *WhatsAppPlaces) RefreshTemplates(ctx context.Context, conn models.ChannelConnection) error {
	if p.Sync != nil {
		if err := p.Sync(ctx, conn); err == nil {
			return nil
		}
	}

	if err := p.expire(ctx, conn); err != nil {
		return err
	}

	return errTemplatesStale
}

/*
expire marks the connection's copied templates out of date.
*/
func (p *WhatsAppPlaces) expire(ctx context.Context, conn models.ChannelConnection) error {
	return p.read(ctx, func(q *models.Queries) error {
		return q.ExpireWhatsAppTemplates(ctx, models.ExpireWhatsAppTemplatesParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID,
		})
	})
}

/*
Phone returns the customer's phone as WhatsApp proved it, its digits,
empty when WhatsApp never shared it.
*/
func (p *WhatsAppPlaces) Phone(ctx context.Context, conn models.ChannelConnection, contactID ids.UUID) (
	string, error,
) {
	var out string

	err := p.read(ctx, func(q *models.Queries) error {
		rows, err := q.ListContactIdentities(ctx, models.ListContactIdentitiesParams{
			WorkspaceID: conn.WorkspaceID, ContactID: contactID,
		})
		for _, row := range rows {
			if row.Type == string(enums.IdentityTypePhone) && row.Verified {
				out = strings.TrimPrefix(row.Value, "+")
			}
		}

		return err
	})
	if err != nil {
		return "", fmt.Errorf("find whatsapp phone: %w", err)
	}

	return out, nil
}

/*
Template returns the connection's template as Meta last described it.
*/
func (p *WhatsAppPlaces) Template(ctx context.Context, conn models.ChannelConnection, name, language string) (
	wadriver.Template, bool, error,
) {
	var (
		out   wadriver.Template
		found bool
	)

	err := p.read(ctx, func(q *models.Queries) error {
		row, err := q.FindWhatsAppTemplate(ctx, models.FindWhatsAppTemplateParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, Name: name, Language: language,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return err
		}

		out, found = templateOf(row), true

		return nil
	})

	return out, found, err
}

/*
templateOf reads a stored template back into Meta's shape.
*/
func templateOf(row models.WhatsappTemplate) wadriver.Template {
	var stored storedTemplate
	_ = json.Unmarshal(row.Components, &stored)

	return wadriver.Template{
		ID: row.ExternalID, Name: row.Name, Language: row.Language, Status: strings.ToUpper(row.Status),
		Category: strings.ToUpper(row.Category), ParameterFormat: stored.ParameterFormat,
		Components: stored.Components,
	}
}

/*
storedTemplate is what a template row keeps of Meta's description: how
its variables are named and its parts.
*/
type storedTemplate struct {
	ParameterFormat string               `json:"parameter_format,omitempty"`
	Components      []wadriver.Component `json:"components"`
}
