/*
|--------------------------------------------------------------------------
| Webhook Snapshots
|--------------------------------------------------------------------------
|
| What an outbound webhook carries about a conversation, a message, a
| contact or a tag is exactly what the API answers with, so a receiver
| reads one shape whether it polls or listens. A record that no longer
| exists renders as nothing and the payload shows a tombstone in its
| place. Event entries of a timeline never leave.
|
*/

package resources

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/webhooks"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WebhookSnapshots renders the objects webhook payloads carry from the
stored records.
*/
type WebhookSnapshots struct {
	DB *database.DB
}

/*
Snapshot renders the object of the kind, an empty snapshot when it is
gone.
*/
func (s WebhookSnapshots) Snapshot(ctx context.Context, ws ids.UUID, kind string, id ids.UUID,
) (webhooks.Snapshot, error) {
	var out webhooks.Snapshot

	err := s.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = snapshotOf(ctx, models.New(tx), ws, objectRef{kind: kind, id: id})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return webhooks.Snapshot{}, nil
	}

	if err != nil {
		return out, fmt.Errorf("render webhook %s: %w", kind, err)
	}

	return out, nil
}

/*
objectRef names one object by its kind and id.
*/
type objectRef struct {
	kind string
	id   ids.UUID
}

/*
snapshotOf reads and shapes one object of the kind.
*/
func snapshotOf(ctx context.Context, q *models.Queries, ws ids.UUID, ref objectRef) (webhooks.Snapshot, error) {
	kind, id := ref.kind, ref.id
	switch kind {
	case "conversation":
		return conversationSnapshot(ctx, q, ws, id)
	case "message":
		return messageSnapshot(ctx, q, ws, id)
	case "contact":
		c, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: id})

		return webhooks.Snapshot{Object: NewContact(c), ContactID: &c.ID}, err
	case "tag":
		t, err := q.FindTag(ctx, models.FindTagParams{WorkspaceID: ws, ID: id})

		return webhooks.Snapshot{Object: NewTag(t)}, err
	}

	return webhooks.Snapshot{}, nil
}

/*
conversationSnapshot shapes a conversation, in the trash or not.
*/
func conversationSnapshot(ctx context.Context, q *models.Queries, ws, id ids.UUID) (webhooks.Snapshot, error) {
	c, err := q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: id})
	if err != nil {
		return webhooks.Snapshot{}, err
	}

	loaded, err := conversations.LoadOne(ctx, q, ids.Nil(), c)
	if err != nil {
		return webhooks.Snapshot{}, err
	}

	shaped := ConversationBroadcast{Conversation: NewConversation(loaded), DeletedAt: c.DeletedAt}

	return webhooks.Snapshot{Object: shaped, ContactID: &c.ContactID, InboxID: &c.InboxID}, nil
}

/*
messageSnapshot shapes a timeline entry with its author, marking notes
and skipping event entries.
*/
func messageSnapshot(ctx context.Context, q *models.Queries, ws, id ids.UUID) (webhooks.Snapshot, error) {
	convID, err := q.FindMessageConversation(ctx, models.FindMessageConversationParams{WorkspaceID: ws, ID: id})
	if err != nil {
		return webhooks.Snapshot{}, err
	}

	m, err := q.FindMessage(ctx, models.FindMessageParams{WorkspaceID: ws, ConversationID: convID, ID: id})
	if err != nil {
		return webhooks.Snapshot{}, err
	}

	if m.Type == string(enums.MessageTypeEvent) {
		return webhooks.Snapshot{Skip: true}, nil
	}

	c, err := q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: convID})
	if err != nil {
		return webhooks.Snapshot{}, err
	}

	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: c.ContactID})
	if err != nil {
		return webhooks.Snapshot{}, err
	}

	members, err := conversations.Members(ctx, q, ws, authorIDs(m))
	if err != nil {
		return webhooks.Snapshot{}, err
	}

	return webhooks.Snapshot{
		Object: NewTimelineEntry(m, contact, members), ContactID: &c.ContactID, InboxID: &c.InboxID,
		Note: m.Type == string(enums.MessageTypeNote),
	}, nil
}
