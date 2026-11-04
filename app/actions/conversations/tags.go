/*
|--------------------------------------------------------------------------
| Conversation Tags
|--------------------------------------------------------------------------
|
| Tagging is how a team sorts its work. A teammate names tags by id or
| types a name, and a new name becomes a tag. Tagging from a message
| remembers which message prompted it, but the tag belongs to the
| conversation, so there is one vocabulary and one way to filter. A tag
| already on the conversation, or not on it when removing, changes
| nothing and says nothing.
|
*/

package conversations

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AddTags puts the tags named on the conversation, noting the message
that prompted them when one is given, and reports whether any was new.
*/
func (w *Writer) AddTags(
	ctx context.Context, c models.Conversation, refs tags.Refs, messageID *ids.UUID,
) (bool, error) {
	if refs.Empty() {
		return false, nil
	}

	if err := w.checkTagMessage(ctx, c, messageID); err != nil {
		return false, err
	}

	tagIDs, err := tags.Resolve(ctx, w.Q, tags.Scope{WorkspaceID: c.WorkspaceID, Now: w.Now}, refs)
	if err != nil {
		return false, err
	}

	added, err := w.Q.TagConversation(ctx, models.TagConversationParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, TagIds: tagIDs, CreatedByType: w.actorType(),
		CreatedByID: w.authorID(), MessageID: messageID, Now: w.Now,
	})
	if err != nil {
		return false, fmt.Errorf("tag conversation: %w", err)
	}

	if len(added) == 0 {
		return false, nil
	}

	count, err := w.Q.CountConversationTags(ctx, models.CountConversationTagsParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID,
	})
	if err != nil {
		return false, fmt.Errorf("count conversation tags: %w", err)
	}

	if count > tags.MaxOnRecord {
		return false, exceptions.ErrTagLimitReached
	}

	return true, w.tagEvent(ctx, c, "conversation.tagged", added)
}

/*
RemoveTags takes the tags off the conversation and reports whether any
was on it.
*/
func (w *Writer) RemoveTags(ctx context.Context, c models.Conversation, tagIDs []ids.UUID) (bool, error) {
	if len(tagIDs) == 0 {
		return false, nil
	}

	removed, err := w.Q.UntagConversationMany(ctx, models.UntagConversationManyParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, TagIds: tagIDs,
	})
	if err != nil {
		return false, fmt.Errorf("untag conversation: %w", err)
	}

	if len(removed) == 0 {
		return false, nil
	}

	return true, w.tagEvent(ctx, c, "conversation.untagged", removed)
}

/*
checkTagMessage refuses a message of another conversation as what
prompted a tag.
*/
func (w *Writer) checkTagMessage(ctx context.Context, c models.Conversation, messageID *ids.UUID) error {
	if messageID == nil {
		return nil
	}

	_, err := w.Q.FindMessage(ctx, models.FindMessageParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, ID: *messageID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrTagMessageNotInConversation
	}

	if err != nil {
		return fmt.Errorf("find message: %w", err)
	}

	return nil
}

/*
tagEvent touches the conversation, records the change on its timeline
and dispatches its event.
*/
func (w *Writer) tagEvent(ctx context.Context, c models.Conversation, name string, tagIDs []ids.UUID) error {
	if err := w.Q.TouchConversation(ctx, models.TouchConversationParams{
		Now: w.Now, WorkspaceID: c.WorkspaceID, ID: c.ID,
	}); err != nil {
		return fmt.Errorf("touch conversation: %w", err)
	}

	rows, err := w.Q.TagsByIDs(ctx, models.TagsByIDsParams{WorkspaceID: c.WorkspaceID, Ids: tagIDs})
	if err != nil {
		return fmt.Errorf("read tags: %w", err)
	}

	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}

	var event any = appevents.ConversationTagged{Base: w.Base(c.WorkspaceID), ConversationRef: ref(c), TagIDs: tagIDs}
	if name == "conversation.untagged" {
		event = appevents.ConversationUntagged{Base: w.Base(c.WorkspaceID), ConversationRef: ref(c), TagIDs: tagIDs}
	}

	return w.emit(ctx, c, entry{name, map[string]any{"tags": names}}, event)
}

/*
TagConversationInput is who tags which conversation with which tags,
and the message that prompted it, nil when none did. MutationID is the
id the client gave the change, empty when it gave none.
*/
type TagConversationInput struct {
	Membership authz.Membership
	Ref        Ref
	Tags       tags.Refs
	MessageID  *ids.UUID
	MutationID string
}

/*
TagConversation adds tags to a conversation.

Rules:
  - The caller needs conversations:write.
  - Names no tag has yet become tags.
  - A conversation holds at most 50 tags.

Events: [appevents.ConversationTagged] when any tag was new to it.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrTagLimitReached],
[exceptions.ErrTagMessageNotInConversation], or a wrapped database
error.
*/
type TagConversation struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle adds the tags and returns the conversation as it now is.
*/
func (a *TagConversation) Handle(ctx context.Context, in TagConversationInput) (Loaded, error) {
	change := func(ctx context.Context, w *Writer, c models.Conversation) error {
		_, err := w.AddTags(ctx, c, in.Tags, in.MessageID)

		return err
	}

	return changeTags(ctx, tagChange{
		db: a.DB, services: Services{Events: a.Events, Clock: a.Clock}, m: in.Membership, ref: in.Ref,
		mutationID: in.MutationID, change: change,
	})
}

/*
UntagConversationInput is who takes which tag off which conversation.
*/
type UntagConversationInput struct {
	Membership authz.Membership
	Ref        Ref
	TagID      ids.UUID
	MutationID string
}

/*
UntagConversation removes a tag from a conversation.

Rules:
  - The caller needs conversations:write.
  - A tag not on the conversation changes nothing.

Events: [appevents.ConversationUntagged] when the tag was on it.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type UntagConversation struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle removes the tag and returns the conversation as it now is.
*/
func (a *UntagConversation) Handle(ctx context.Context, in UntagConversationInput) (Loaded, error) {
	change := func(ctx context.Context, w *Writer, c models.Conversation) error {
		_, err := w.RemoveTags(ctx, c, []ids.UUID{in.TagID})

		return err
	}

	return changeTags(ctx, tagChange{
		db: a.DB, services: Services{Events: a.Events, Clock: a.Clock}, m: in.Membership, ref: in.Ref,
		mutationID: in.MutationID, change: change,
	})
}

/*
tagChange is one change of a conversation's tags in its own transaction.
*/
type tagChange struct {
	db         *database.DB
	services   Services
	m          authz.Membership
	ref        Ref
	mutationID string
	change     func(context.Context, *Writer, models.Conversation) error
}

/*
changeTags runs the change under the conversation's lock and loads the
conversation as it now is.
*/
func changeTags(ctx context.Context, t tagChange) (Loaded, error) {
	if err := CanWrite(t.m); err != nil {
		return Loaded{}, err
	}

	var out Loaded

	err := t.db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := NewWriter(ctx, tx, t.services, t.m)
		if err != nil {
			return err
		}

		w.MutationID = t.mutationID

		c, err := Lock(ctx, w.Q, t.m.Workspace.ID, t.ref)
		if err != nil {
			return err
		}

		if err := t.change(ctx, w, c); err != nil {
			return err
		}

		out, err = LoadOne(ctx, w.Q, t.m.Member.ID, c)

		return err
	})

	return out, err
}
