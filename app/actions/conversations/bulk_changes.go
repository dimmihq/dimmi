/*
|--------------------------------------------------------------------------
| Bulk Conversation Changes
|--------------------------------------------------------------------------
|
| A teammate clearing a list picks many conversations and closes, snoozes,
| assigns, tags or trashes them in one go. Each conversation is changed
| in a transaction of its own, the same way a change to one would be, so
| one the caller may not touch or that cannot change fails alone and the
| rest still go through. The answer says how each one went.
|
*/

package conversations

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
MaxBulkConversations is the most conversations one bulk change takes.
*/
const MaxBulkConversations = 200

/*
BulkChangesInput is who changes which conversations, and how: the
changes of an update, or a move to the trash.
*/
type BulkChangesInput struct {
	Membership authz.Membership
	IDs        []ids.UUID
	Changes    Changes
	Trash      bool
	MutationID string
}

/*
BulkResult is how one conversation of a bulk change went: the fields
that changed, or the code of the error that stopped it.
*/
type BulkResult struct {
	ID      ids.UUID
	Changed []string
	Error   string
}

/*
BulkChangeConversations changes many conversations at once.

Rules:
  - The caller needs conversations:write, and conversations:delete to
    trash.
  - One to MaxBulkConversations conversations, each changed on its own
    by [ApplyConversationChanges] or [TrashConversation].
  - A conversation that fails is reported and skipped; the others stay
    changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrBulkTooMany], or a
wrapped database error, which stops the batch.
*/
type BulkChangeConversations struct {
	Apply *ApplyConversationChanges
	Trash *TrashConversation
}

/*
Handle changes each conversation and returns how each went, in the
order named.
*/
func (a *BulkChangeConversations) Handle(ctx context.Context, in BulkChangesInput) ([]BulkResult, error) {
	if err := CanWrite(in.Membership); err != nil {
		return nil, err
	}

	if in.Trash && !in.Membership.Can("conversations:delete") {
		return nil, exceptions.ErrForbidden
	}

	if len(in.IDs) == 0 || len(in.IDs) > MaxBulkConversations {
		return nil, exceptions.ErrBulkTooMany
	}

	out := make([]BulkResult, 0, len(in.IDs))
	seen := map[ids.UUID]bool{}

	for _, id := range in.IDs {
		if seen[id] {
			continue
		}

		seen[id] = true

		result, err := a.one(ctx, in, id)
		if err != nil {
			return out, err
		}

		out = append(out, result)
	}

	return out, nil
}

/*
one changes one conversation. A refusal becomes the result's error; a
failure of the database stops the batch.
*/
func (a *BulkChangeConversations) one(ctx context.Context, in BulkChangesInput, id ids.UUID) (BulkResult, error) {
	result := BulkResult{ID: id, Changed: []string{}}
	ref := Ref{ID: id}

	var err error

	if in.Trash {
		err = a.Trash.Handle(ctx, TrashInput{Membership: in.Membership, Ref: ref, MutationID: in.MutationID})
		if err == nil {
			result.Changed = append(result.Changed, "deleted")
		}
	} else {
		var applied Applied

		applied, err = a.Apply.Handle(ctx, ApplyChangesInput{
			Membership: in.Membership, Ref: ref, Changes: in.Changes, MutationID: in.MutationID,
		})
		if err == nil {
			result.Changed = append(result.Changed, applied.Changed...)
		}
	}

	if err == nil {
		return result, nil
	}

	if refusal, ok := errors.AsType[*fault.Error](err); ok {
		result.Error = refusal.Code()

		return result, nil
	}

	return result, err
}
