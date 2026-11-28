/*
|--------------------------------------------------------------------------
| Messenger And Instagram Channels
|--------------------------------------------------------------------------
|
| Everything the two channels do besides sending replies, built once for
| each platform with the services it uses. Meta's deliveries are checked,
| kept as they came and processed by a job, so Meta hears back at once
| and nothing is lost. Processing turns customers' messages into inbound
| messages for the inbox and records when each customer last wrote,
| which decides who may still answer them.
|
| A connection's facts, such as its Page or Instagram account, live in
| its config; its tokens live sealed in its row, bound to it, and only
| this channel opens them.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/channel/meta"
	"github.com/dimmihq/dimmi/app/drivers/channel/social"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
Social runs one platform's channel, Messenger or Instagram. App is the
install's Meta app and Login the platform's login on it, both empty
when the install has none. AppURL is the install's address, which the
address Meta's sign-in returns to must be under.
*/
type Social struct {
	Platform social.Platform
	DB       *database.DB
	Clock    clock.Clock
	Queue    *queue.Queue
	Cache    *cache.Store
	IDs      *ids.Registry
	Log      log.Logger
	Keys     *encryption.Keys
	Driver   *social.Driver
	Receiver contracts.InboundReceiver
	Edits    *conversations.InboundEdits
	Config   appconfig.SocialChannelConfig
	App      appconfig.MetaServiceConfig
	Login    appconfig.SocialServiceConfig
	AppURL   string
	// Pictures is where connected accounts' pictures are kept.
	Pictures filesystem.Filesystem
}

/*
SocialFactsOf reads a Messenger or Instagram connection's facts from
its config.
*/
func SocialFactsOf(conn models.ChannelConnection) social.Facts {
	var facts social.Facts

	_ = json.Unmarshal(conn.Config, &facts)

	return facts
}

/*
SocialSecrets are the tokens a connection calls Meta with. AccessToken
sends and reads: a Page's own token on Messenger, the business's on
Instagram. BusinessToken is the business's token a Page's is drawn from
again when Meta stops taking it.
*/
type SocialSecrets struct {
	AccessToken   string `json:"access_token,omitempty"`
	BusinessToken string `json:"business_token,omitempty"`
}

/*
seal seals the secrets for the connection's row.
*/
func (s *Social) seal(ctx context.Context, conn models.ChannelConnection, secrets SocialSecrets) (
	encryption.Sealed, error,
) {
	sealed, err := encryption.Seal(ctx, s.Keys, encryption.Slot{
		Scope: encryption.Scope{Kind: "workspace", ID: conn.WorkspaceID.String()}, AAD: secretsAAD(conn.ID),
	}, secrets)
	if err != nil {
		return nil, fmt.Errorf("seal %s secrets: %w", s.Platform, err)
	}

	return sealed, nil
}

/*
open opens the connection's secrets, empty when it has none.
*/
func (s *Social) open(ctx context.Context, conn models.ChannelConnection) (SocialSecrets, error) {
	if len(conn.CredentialsEncrypted) == 0 {
		return SocialSecrets{}, nil
	}

	secrets, err := encryption.Open[SocialSecrets](ctx, s.Keys, conn.CredentialsEncrypted, secretsAAD(conn.ID))
	if err != nil {
		return SocialSecrets{}, fmt.Errorf("open %s secrets: %w", s.Platform, err)
	}

	return secrets, nil
}

/*
Token returns the token the connection calls Meta with. A connection
without one is broken until it is connected again.
*/
func (s *Social) Token(ctx context.Context, conn models.ChannelConnection) (string, error) {
	secrets, err := s.open(ctx, conn)
	if err != nil {
		return "", err
	}

	if secrets.AccessToken == "" {
		return "", &contracts.SendError{
			Code:    social.CodeConnectionBroken,
			Message: "The " + s.Platform.Name() + " connection has no token. Connect it again.",
			Err:     contracts.ErrConnectionBroken,
		}
	}

	return secrets.AccessToken, nil
}

/*
graph returns the Graph API with the token.
*/
func (s *Social) graph(token string) *meta.Graph {
	return s.Driver.Graph(token)
}

/*
read runs the queries in a transaction scoped to the context's
workspace, the only way rows are visible under row level security.
*/
func (s *Social) read(ctx context.Context, fn func(q *models.Queries) error) error {
	return s.DB.Transaction(ctx, func(_ context.Context, tx *database.Tx) error {
		return fn(models.New(tx))
	})
}

/*
connection returns the workspace's live connection of the platform with
the id.
*/
func (s *Social) connection(ctx context.Context, workspaceID, id ids.UUID) (models.ChannelConnection, error) {
	var conn models.ChannelConnection

	err := s.read(ctx, func(q *models.Queries) error {
		var err error
		conn, err = q.FindChannelConnection(ctx, models.FindChannelConnectionParams{WorkspaceID: workspaceID, ID: id})

		return err
	})

	gone := err == nil && (conn.Type != string(s.Platform) || conn.DeletedAt != nil)
	if errors.Is(err, database.ErrNotFound) || gone {
		return conn, exceptions.ErrNotFound
	}

	return conn, err
}

/*
SocialPlaces hands the driver what it needs from the inbox: the parts a
reply became and when the customer last wrote.
*/
type SocialPlaces struct {
	DB    *database.DB
	Clock clock.Clock
}

func (p *SocialPlaces) read(ctx context.Context, fn func(q *models.Queries) error) error {
	return p.DB.Transaction(ctx, func(_ context.Context, tx *database.Tx) error {
		return fn(models.New(tx))
	})
}

/*
SentParts returns the messages on the platform the reply already
became.
*/
func (p *SocialPlaces) SentParts(ctx context.Context, conn models.ChannelConnection, messageID ids.UUID) (
	[]social.Part, error,
) {
	var out []social.Part

	err := p.read(ctx, func(q *models.Queries) error {
		rows, err := q.ListChannelMessageRefs(ctx, models.ListChannelMessageRefsParams{
			WorkspaceID: conn.WorkspaceID, MessageID: messageID,
		})
		for _, row := range rows {
			if row.ChannelConnectionID == conn.ID {
				out = append(out, social.Part{
					ExternalID: row.ExternalMessageID, Index: int(row.PartIndex), Kind: row.Kind,
				})
			}
		}

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list sent parts: %w", err)
	}

	return out, nil
}

/*
RecordParts remembers the messages on the platform the reply became.
*/
func (p *SocialPlaces) RecordParts(
	ctx context.Context, conn models.ChannelConnection, messageID ids.UUID, parts []social.Part,
) error {
	return p.read(ctx, func(q *models.Queries) error {
		for _, part := range parts {
			err := q.CreateChannelMessageRef(ctx, models.CreateChannelMessageRefParams{
				WorkspaceID: conn.WorkspaceID, MessageID: messageID, ChannelConnectionID: conn.ID,
				ExternalMessageID: part.ExternalID, PartIndex: int64(part.Index), Kind: part.Kind, Now: p.Clock.Now(),
			})
			if err != nil {
				return fmt.Errorf("record sent part: %w", err)
			}
		}

		return nil
	})
}

/*
Window returns when the customer last wrote on the connection, zero
when they never did.
*/
func (p *SocialPlaces) Window(ctx context.Context, conn models.ChannelConnection, contactID ids.UUID) (
	social.Window, error,
) {
	var out social.Window

	err := p.read(ctx, func(q *models.Queries) error {
		state, err := q.FindChannelContactState(ctx, models.FindChannelContactStateParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, ContactID: contactID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find contact state: %w", err)
		}

		if state.LastInboundAt != nil {
			out.LastInboundAt = *state.LastInboundAt
		}

		if state.LastInboundMessageID != nil {
			out.LastInboundID = *state.LastInboundMessageID
		}

		return nil
	})

	return out, err
}
