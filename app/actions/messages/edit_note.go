/*
|--------------------------------------------------------------------------
| Edit Note
|--------------------------------------------------------------------------
|
| A teammate fixes their own note at any time; the note says it was
| edited and by whom. Replies have reached the customer and are not
| edited, and nobody edits another teammate's words.
|
*/

package messages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EditInput is who rewrites which of their notes, and the new text.
*/
type EditInput struct {
	Membership authz.Membership
	Ref        conversations.Ref
	MessageID  ids.UUID
	Body       string
	MutationID string
}

/*
EditNote replaces the text of one of the caller's notes.

Rules:
  - The caller needs conversations:write.
  - Only their own live note; any other entry is not editable.

Events: [appevents.MessageUpdated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrMessageNotEditable], or a wrapped database error.
*/
type EditNote struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
	IDs    *ids.Registry
}

/*
Handle edits the note and returns it.
*/
func (a *EditNote) Handle(ctx context.Context, in EditInput) (Written, error) {
	if err := conversations.CanWrite(in.Membership); err != nil {
		return Written{}, err
	}

	var out Written

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		s, err := open(ctx, tx, conversations.Services{Events: a.Events, Clock: a.Clock}, target{
			in.Membership, in.Ref, in.MutationID,
		})
		if err != nil {
			return err
		}

		if err := s.edit(ctx, a.IDs, in.MessageID, in.Body); err != nil {
			return err
		}

		out, err = s.written(ctx)

		return err
	})

	return out, err
}

/*
edit finds the caller's own live note and rewrites it.
*/
func (s *session) edit(ctx context.Context, reg *ids.Registry, id ids.UUID, body string) error {
	msg, err := s.w.Q.LockMessage(ctx, models.LockMessageParams{
		WorkspaceID: s.c.WorkspaceID, ConversationID: s.c.ID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && msg.DeletedAt != nil) {
		return exceptions.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("find note: %w", err)
	}

	own := msg.AuthorType == string(enums.AuthorTypeMember) && msg.AuthorID != nil && *msg.AuthorID == s.m.Member.ID
	if msg.Type != string(enums.MessageTypeNote) || !own {
		return exceptions.ErrMessageNotEditable
	}

	return s.rewrite(ctx, reg, msg.ID, body)
}

/*
rewrite stores the note's new text under the workspace's redaction
rules, replacing which rules hit the earlier text, keeps its mentions in
step with what the redacted text names, and announces it with the
teammates it newly mentions.
*/
func (s *session) rewrite(ctx context.Context, reg *ids.Registry, id ids.UUID, body string) error {
	hits, err := s.w.RedactEdit(ctx, s.c.WorkspaceID, &body)
	if err != nil {
		return fmt.Errorf("redact note: %w", err)
	}

	m, err := s.resolve(ctx, reg, body)
	if err != nil {
		return err
	}

	raw, err := json.Marshal(map[string]any{
		"edited_by": models.PublicID(models.PrefixMember, s.w.Actor.ID), "mentions": m.tokens, "redactions": hits,
	})
	if err != nil {
		return fmt.Errorf("encode edit: %w", err)
	}

	if s.msg, err = s.w.Q.EditMessage(ctx, models.EditMessageParams{
		Body: &body, Now: s.w.Now, Metadata: raw, WorkspaceID: s.c.WorkspaceID, ID: id,
	}); err != nil {
		return fmt.Errorf("edit note: %w", err)
	}

	if err := s.w.AnnounceRedaction(ctx, s.c, s.msg, hits); err != nil {
		return err
	}

	added, err := s.record(ctx, m)
	if err != nil {
		return err
	}

	r := appevents.MessageRef{ConversationRef: appevents.ConversationRef{ConversationID: s.c.ID}, MessageID: id}

	return s.w.Dispatch(ctx, appevents.MessageUpdated{
		Base: s.w.Base(s.c.WorkspaceID), MessageRef: r, AddedMentionedMemberIDs: added,
	})
}
