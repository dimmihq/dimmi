/*
|--------------------------------------------------------------------------
| Slack Places
|--------------------------------------------------------------------------
|
| What the driver needs to know from the database while it posts: which
| Slack channel a room is today, since a channel's id can change while
| the room stays, and which parts of a reply were already posted, so a
| retried send never posts twice. Every part's id is filed under its
| message, so a click, an edit or a deletion on any part finds it.
|
*/

package channels

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SlackPlaces answers the driver from the database.
*/
type SlackPlaces struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
RoomChannel returns the Slack channel of the room; a room no longer
watched fails the reply for good.
*/
func (p *SlackPlaces) RoomChannel(ctx context.Context, conn models.ChannelConnection, roomID ids.UUID) (
	string, error,
) {
	var room models.ChannelRoom

	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		room, err = models.New(tx).FindChannelRoom(ctx, models.FindChannelRoomParams{
			WorkspaceID: conn.WorkspaceID, ID: roomID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && room.Status == string(enums.RoomStatusRemoved)) {
		return "", &contracts.SendError{
			Code: slackdriver.CodeChannelGone, Message: "The Slack channel is no longer watched.",
			Err: contracts.ErrPermanent,
		}
	}

	if err != nil {
		return "", fmt.Errorf("find room: %w", err)
	}

	return room.ExternalID, nil
}

/*
SentParts returns the parts already posted for the message.
*/
func (p *SlackPlaces) SentParts(ctx context.Context, conn models.ChannelConnection, messageID ids.UUID) (
	[]slackdriver.Part, error,
) {
	var refs []models.ChannelMessageRef

	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		refs, err = models.New(tx).ListChannelMessageRefs(ctx, models.ListChannelMessageRefsParams{
			WorkspaceID: conn.WorkspaceID, MessageID: messageID,
		})

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list sent parts: %w", err)
	}

	parts := make([]slackdriver.Part, 0, len(refs))
	for _, ref := range refs {
		parts = append(parts, slackdriver.Part{
			ExternalID: ref.ExternalMessageID, Index: int(ref.PartIndex), Kind: ref.Kind,
		})
	}

	return parts, nil
}

/*
RecordParts files the posted parts under the message.
*/
func (p *SlackPlaces) RecordParts(
	ctx context.Context, conn models.ChannelConnection, messageID ids.UUID, parts []slackdriver.Part,
) error {
	owner := partsOwner{conn: conn, messageID: messageID, at: p.Clock.Now()}

	return p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return recordParts(ctx, models.New(tx), owner, parts)
	})
}

/*
partsOwner is the message parts are filed under, on its connection, at
the time they are filed.
*/
type partsOwner struct {
	conn      models.ChannelConnection
	messageID ids.UUID
	at        time.Time
}

/*
recordParts files parts under the message in the caller's transaction.
*/
func recordParts(ctx context.Context, q *models.Queries, owner partsOwner, parts []slackdriver.Part) error {
	for _, part := range parts {
		err := q.CreateChannelMessageRef(ctx, models.CreateChannelMessageRefParams{
			WorkspaceID: owner.conn.WorkspaceID, MessageID: owner.messageID, ChannelConnectionID: owner.conn.ID,
			ExternalMessageID: part.ExternalID, PartIndex: int64(part.Index), Kind: part.Kind, Now: owner.at,
		})
		if err != nil {
			return fmt.Errorf("record sent part: %w", err)
		}
	}

	return nil
}
