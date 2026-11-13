/*
|--------------------------------------------------------------------------
| Broadcast Payloads
|--------------------------------------------------------------------------
|
| What a realtime broadcast carries about a conversation or a message is
| exactly what the API answers with, so an open screen patches what it
| fetched with what it hears and never translates. The one difference is
| that a broadcast reaches every teammate at once: whether a
| conversation is unread is each reader's own, so a broadcast leaves it
| out and a screen keeps the value it had. A conversation in the trash
| is still broadcast, with when it was deleted, so screens drop it.
|
*/

package resources

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConversationBroadcast is a conversation as a broadcast carries it: the
API's conversation without the reader's unread flag, and when it was
deleted, nil while it is live.
*/
type ConversationBroadcast struct {
	Conversation

	Unread    *bool      `json:"unread,omitempty"`
	DeletedAt *time.Time `json:"deleted_at"`
}

/*
LastUpdated returns when the conversation last changed.
*/
func (c ConversationBroadcast) LastUpdated() time.Time {
	return c.UpdatedAt
}

/*
LastUpdated returns when the entry last changed.
*/
func (e TimelineEntry) LastUpdated() time.Time {
	return e.UpdatedAt
}

/*
BroadcastPayloads renders the subjects of broadcasts from the stored
records, in the API's shapes.
*/
type BroadcastPayloads struct {
	DB    *database.DB
	Disks messages.Disks
}

/*
ConversationRow renders the conversation as lists show it, which is the
same as the conversation itself.
*/
func (p BroadcastPayloads) ConversationRow(ctx context.Context, workspaceID, id ids.UUID) (any, bool, error) {
	return p.Conversation(ctx, workspaceID, id)
}

/*
Conversation renders the whole conversation, deleted or not.
*/
func (p BroadcastPayloads) Conversation(ctx context.Context, workspaceID, id ids.UUID) (any, bool, error) {
	var out ConversationBroadcast

	found, err := p.read(ctx, workspaceID, func(ctx context.Context, q *models.Queries) error {
		c, err := q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: workspaceID, ID: id})
		if err != nil {
			return err
		}

		loaded, err := conversations.LoadOne(ctx, q, ids.Nil(), c)
		out = ConversationBroadcast{Conversation: NewConversation(loaded), DeletedAt: c.DeletedAt}

		return err
	})
	if err != nil || !found {
		return nil, found, err
	}

	return out, true, nil
}

/*
Message renders the timeline entry, deleted or not, with its author.
*/
func (p BroadcastPayloads) Message(ctx context.Context, workspaceID, id ids.UUID) (any, bool, error) {
	var out TimelineEntry

	found, err := p.read(ctx, workspaceID, func(ctx context.Context, q *models.Queries) error {
		m, err := q.FindBroadcastMessage(ctx, models.FindBroadcastMessageParams{WorkspaceID: workspaceID, ID: id})
		if err != nil {
			return err
		}

		c, err := q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: workspaceID, ID: m.ConversationID})
		if err != nil {
			return err
		}

		contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: workspaceID, ID: c.ContactID})
		if err != nil {
			return err
		}

		members, err := conversations.Members(ctx, q, workspaceID, authorIDs(m))
		if err != nil {
			return err
		}

		// The entry carries its files as the timeline does, so a client
		// that puts it in place of the one it holds keeps them.
		files, err := p.files(ctx, q, m)
		out = NewTimelineEntries([]models.Message{m}, contact, members, files)[0]

		return err
	})
	if err != nil || !found {
		return nil, found, err
	}

	return out, true, nil
}

/*
files reads the message's files with their links.
*/
func (p BroadcastPayloads) files(
	ctx context.Context, q *models.Queries, m models.Message,
) (map[ids.UUID][]messages.File, error) {
	rows, err := q.ListMessageAttachments(ctx, models.ListMessageAttachmentsParams{
		WorkspaceID: m.WorkspaceID, MessageIds: []ids.UUID{m.ID},
	})
	if err != nil {
		return nil, err
	}

	out := map[ids.UUID][]messages.File{}
	for _, row := range rows {
		out[m.ID] = append(out[m.ID], messages.PresentFile(ctx, p.Disks, row))
	}

	return out, nil
}

/*
read runs the lookup inside the workspace and reports whether it found
its record.
*/
func (p BroadcastPayloads) read(
	ctx context.Context, workspaceID ids.UUID, find func(ctx context.Context, q *models.Queries) error,
) (bool, error) {
	err := p.DB.Transaction(tenancy.With(ctx, workspaceID.String()), func(ctx context.Context, tx *database.Tx) error {
		return find(ctx, models.New(tx))
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("read broadcast subject: %w", err)
	}

	return true, nil
}

/*
authorIDs returns the entry's author as a member id to look up, none
when nobody or no member wrote it.
*/
func authorIDs(m models.Message) []ids.UUID {
	if m.AuthorID == nil {
		return nil
	}

	return []ids.UUID{*m.AuthorID}
}
