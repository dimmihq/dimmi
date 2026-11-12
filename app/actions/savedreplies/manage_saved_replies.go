/*
|--------------------------------------------------------------------------
| Manage Saved Replies
|--------------------------------------------------------------------------
|
| Adding, changing and removing replies. A reply moves between personal
| and shared on an update; sharing one, or touching one already shared,
| takes saved_replies:manage_shared. Shortcuts stay unique per owner and
| among shared replies, which the table's index enforces.
|
*/

package savedreplies

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CreateSavedReplyInput is who adds which reply.
*/
type CreateSavedReplyInput struct {
	Membership authz.Membership
	Fields     Fields
}

/*
CreateSavedReply adds a reply, the caller's own unless it is shared.

Rules:
  - The caller needs conversations:write, and saved_replies:manage_shared
    for a shared reply.
  - A title and text are required; a workspace keeps at most 2,000.

Errors: [exceptions.ErrForbidden], [exceptions.ErrSavedReplyTitleRequired],
[exceptions.ErrSavedReplyBodyRequired], [exceptions.ErrSavedReplyShortcutTaken],
[exceptions.ErrSavedReplyLimitReached], [exceptions.ErrInvalidMacro], or a
wrapped database error.
*/
type CreateSavedReply struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle adds the reply and returns it.
*/
func (a *CreateSavedReply) Handle(ctx context.Context, in CreateSavedReplyInput) (models.SavedReply, error) {
	var out models.SavedReply

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		created, err := create(ctx, models.New(tx), in, a.Clock.Now())
		out = created

		return err
	})

	return out, err
}

/*
create checks and stores one reply inside the caller's transaction.
*/
func create(
	ctx context.Context, q *models.Queries, in CreateSavedReplyInput, now time.Time,
) (models.SavedReply, error) {
	m, f := in.Membership, in.Fields

	shared := f.Shared != nil && *f.Shared
	if !canChange(m, shared) {
		return models.SavedReply{}, exceptions.ErrForbidden
	}

	params := models.CreateSavedReplyParams{
		WorkspaceID: m.Workspace.ID, Shortcut: cleanShortcut(f.Shortcut), Now: now,
	}

	if !shared {
		params.OwnerMemberID = &m.Member.ID
	}

	var err error
	if params.Title, err = cleanTitle(deref(f.Title)); err != nil {
		return models.SavedReply{}, err
	}

	if params.Body, err = cleanBody(deref(f.Body)); err != nil {
		return models.SavedReply{}, err
	}

	if f.Macro != nil {
		if params.Actions, err = encodeMacro(ctx, q, m.Workspace.ID, *f.Macro); err != nil {
			return models.SavedReply{}, err
		}
	}

	if err := checkRoom(ctx, q, m.Workspace.ID); err != nil {
		return models.SavedReply{}, err
	}

	out, err := q.CreateSavedReply(ctx, params)
	if database.IsUniqueViolation(err, shortcutConstraint) {
		return out, exceptions.ErrSavedReplyShortcutTaken
	}

	if err != nil {
		return out, fmt.Errorf("create saved reply: %w", err)
	}

	return out, nil
}

/*
checkRoom refuses another reply once the workspace holds the most it
may keep.
*/
func checkRoom(ctx context.Context, q *models.Queries, workspace ids.UUID) error {
	count, err := q.CountSavedReplies(ctx, workspace)
	if err != nil {
		return fmt.Errorf("count saved replies: %w", err)
	}

	if count >= MaxInWorkspace {
		return exceptions.ErrSavedReplyLimitReached
	}

	return nil
}

/*
UpdateSavedReplyInput is who changes which reply and how.
*/
type UpdateSavedReplyInput struct {
	Membership authz.Membership
	ID         ids.UUID
	Fields     Fields
}

/*
UpdateSavedReply changes a reply's title, text, shortcut, actions or
whether it is shared.

Rules:
  - The caller may change their own replies, and shared ones with
    saved_replies:manage_shared; sharing a reply takes it too.
  - Another member's personal reply is not found.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], and those of
[CreateSavedReply] but the limit.
*/
type UpdateSavedReply struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle changes the reply and returns it as it now is.
*/
func (a *UpdateSavedReply) Handle(ctx context.Context, in UpdateSavedReplyInput) (models.SavedReply, error) {
	var out models.SavedReply

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		current, err := find(ctx, q, in.Membership, in.ID)
		if err != nil {
			return err
		}

		params, err := a.next(ctx, q, current, in)
		if err != nil {
			return err
		}

		out, err = q.UpdateSavedReply(ctx, params)
		if database.IsUniqueViolation(err, shortcutConstraint) {
			return exceptions.ErrSavedReplyShortcutTaken
		}

		if err != nil {
			return fmt.Errorf("update saved reply: %w", err)
		}

		return nil
	})

	return out, err
}

/*
next works out what the reply becomes, refusing a change the caller may
not make.
*/
func (a *UpdateSavedReply) next(
	ctx context.Context, q *models.Queries, current models.SavedReply, in UpdateSavedReplyInput,
) (models.UpdateSavedReplyParams, error) {
	m, f := in.Membership, in.Fields
	params := models.UpdateSavedReplyParams{
		Title: current.Title, Body: current.Body, Shortcut: current.Shortcut, Now: a.Clock.Now(),
		WorkspaceID: current.WorkspaceID, ID: current.ID,
	}

	if err := moveOwner(&params, m, current, f); err != nil {
		return params, err
	}

	if err := changeText(&params, f); err != nil {
		return params, err
	}

	var err error
	if f.Macro != nil {
		params.Actions, err = encodeMacro(ctx, q, m.Workspace.ID, *f.Macro)
	}

	return params, err
}

/*
moveOwner refuses a change to a reply the caller may not touch, and
moves the reply between personal and shared when the fields ask.
*/
func moveOwner(params *models.UpdateSavedReplyParams, m authz.Membership, current models.SavedReply, f Fields) error {
	wasShared := current.OwnerMemberID == nil
	if !canChange(m, wasShared) {
		return exceptions.ErrForbidden
	}

	if f.Shared == nil || *f.Shared == wasShared {
		return nil
	}

	if !canChange(m, true) {
		return exceptions.ErrForbidden
	}

	params.SetOwner = true
	if !*f.Shared {
		params.NewOwnerMemberID = &m.Member.ID
	}

	return nil
}

/*
changeText checks and applies a new title, text or shortcut.
*/
func changeText(params *models.UpdateSavedReplyParams, f Fields) error {
	var err error
	if f.Title != nil {
		if params.Title, err = cleanTitle(*f.Title); err != nil {
			return err
		}
	}

	if f.Body != nil {
		if params.Body, err = cleanBody(*f.Body); err != nil {
			return err
		}
	}

	if f.Shortcut != nil {
		params.Shortcut = cleanShortcut(f.Shortcut)
	}

	if f.ClearShortcut {
		params.Shortcut = nil
	}

	return nil
}

/*
deref returns the string a pointer holds, or the empty string.
*/
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
