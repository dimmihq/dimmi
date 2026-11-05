/*
|--------------------------------------------------------------------------
| Seen Receipts
|--------------------------------------------------------------------------
|
| A customer chatting in the messenger sees "Seen" once someone on the
| team read their latest message, so they know it reached a person. The
| receipt tells them when, never which member: a teammate's name and
| picture travel only when the workspace chose to show them, and member
| ids never leave the team. Read markers themselves stay with the team.
|
| Teammates reading the same conversation at once would repeat the
| receipt, so it goes out at most once per conversation every five
| seconds. Only widget conversations get one; mail has no such signal.
|
*/

package broadcasting

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SeenThrottle is the shortest time between two receipts of one
conversation.
*/
const SeenThrottle = 5 * time.Second

/*
SeenReceipts relays a teammate reading a widget conversation to the
customer's widget.
*/
type SeenReceipts struct {
	DB     *database.DB
	Cache  *cache.Store
	Server Publisher
}

/*
Listen makes the relay hear read markers once they are saved.
*/
func (s *SeenReceipts) Listen(d *events.Dispatcher) {
	events.Listen(d, events.AfterCommit, s.read)
}

/*
seenBy is the teammate a receipt names, when the workspace shows names.
*/
type seenBy struct {
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

/*
seenReceipt is what the customer's widget learns.
*/
type seenReceipt struct {
	ConversationID string    `json:"conversation_id"`
	SeenAt         time.Time `json:"seen_at"`
	SeenBy         *seenBy   `json:"seen_by,omitempty"`
}

/*
seenTarget is a read that answers the customer's latest message.
*/
type seenTarget struct {
	contact ids.UUID
	by      *seenBy
}

/*
read sends the receipt when the marker covers the customer's latest
message on a widget conversation.
*/
func (s *SeenReceipts) read(ctx context.Context, e appevents.ConversationRead) error {
	if e.Unread || e.LastReadMessageID == nil {
		return nil
	}

	target, ok, err := s.target(ctx, e)
	if err != nil || !ok {
		return err
	}

	first, err := s.Cache.Add(ctx, "seen:"+e.ConversationID.String(), []byte{1}, SeenThrottle)
	if err != nil || !first {
		return err
	}

	m := fbroadcasting.Message{
		ID: ids.Encode("evt", ids.New()), Event: "conversation.seen_by_team",
		Data: seenReceipt{
			ConversationID: models.PublicID(models.PrefixConversation, e.ConversationID),
			SeenAt:         e.OccurredAt, SeenBy: target.by,
		},
	}

	if err := s.Server.Publish(ctx, []string{WidgetContactChannel(target.contact)}, m); err != nil {
		return fmt.Errorf("relay seen receipt: %w", err)
	}

	return nil
}

/*
target finds the conversation's customer, and reports false when the
read does not answer a widget customer's latest message.
*/
func (s *SeenReceipts) target(ctx context.Context, e appevents.ConversationRead) (seenTarget, bool, error) {
	var out seenTarget

	found := false

	scoped := tenancy.With(ctx, e.WorkspaceID.String())

	err := s.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: e.WorkspaceID, ID: e.ConversationID,
		})
		if err != nil {
			return ignoreMissing(err)
		}

		if found, err = answersCustomer(ctx, q, c, *e.LastReadMessageID); err != nil || !found {
			return err
		}

		out.contact = c.ContactID
		out.by, err = namedReader(ctx, q, e)

		return err
	})

	return out, found, err
}

/*
answersCustomer reports whether the conversation is a widget one and
the marker reaches the customer's latest message.
*/
func answersCustomer(ctx context.Context, q *models.Queries, c models.Conversation, marker ids.UUID) (bool, error) {
	channel, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: c.WorkspaceID, ID: c.ChannelConnectionID,
	})
	if err != nil || channel.Type != "widget" {
		return false, ignoreMissing(err)
	}

	latest, err := q.LatestContactMessage(ctx, models.LatestContactMessageParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID,
	})
	if err != nil {
		return false, ignoreMissing(err)
	}

	return bytes.Compare(marker[:], latest.ID[:]) >= 0, nil
}

/*
namedReader returns the teammate who read, when the workspace shows
names on receipts, nil otherwise.
*/
func namedReader(ctx context.Context, q *models.Queries, e appevents.ConversationRead) (*seenBy, error) {
	w, err := q.FindWorkspace(ctx, e.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("find workspace: %w", err)
	}

	if !w.SeenReceiptShowsName() {
		return nil, nil
	}

	rows, err := q.MembersByIDs(ctx, models.MembersByIDsParams{WorkspaceID: e.WorkspaceID, Ids: []ids.UUID{e.MemberID}})
	if err != nil || len(rows) == 0 {
		return nil, err
	}

	return &seenBy{Name: rows[0].User.Name}, nil
}

/*
ignoreMissing treats a row that is gone as nothing to relay.
*/
func ignoreMissing(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	return err
}
