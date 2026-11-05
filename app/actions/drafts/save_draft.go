/*
|--------------------------------------------------------------------------
| Save Draft
|--------------------------------------------------------------------------
|
| Every save sends the draft whole with the moment the client last
| changed it, and the newest change wins: a save older than the stored
| draft writes nothing and answers with what is stored, so a stale tab
| catches up instead of overwriting the phone. Each save tells the
| member's other screens.
|
*/

package drafts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The largest draft body in bytes, and the most files a draft holds.
*/
const (
	maxBody        = 64 * 1024
	maxAttachments = 10
)

/*
Content is what a draft holds: its Markdown body, the files staged with
it, the recipients staged for it, and when the client last changed it.
*/
type Content struct {
	Body            string
	AttachmentIDs   []ids.UUID
	Recipients      json.RawMessage
	ClientUpdatedAt time.Time
}

/*
Saved is a draft after a save. Created is true when the save started
it; Stale is true when the stored draft was newer and nothing was
written.
*/
type Saved struct {
	Draft   models.ConversationDraft
	Created bool
	Stale   bool
}

/*
Saver writes drafts and tells the member's screens.
*/
type Saver struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
SaveInput is who saves which draft. Conversation is nil for a new
conversation's draft. Upsert updates the draft already there instead of
refusing it.
*/
type SaveInput struct {
	Membership   authz.Membership
	Conversation *ids.UUID
	Mode         string
	Content      Content
	Upsert       bool
	MutationID   string
}

/*
SaveDraft starts the caller's draft of a conversation and mode, or
updates it when asked to.

Rules:
  - The caller needs conversations:write.
  - A reply or note draft names a conversation the caller can see; a
    new conversation draft names none.
  - The body is 64 KB at most; up to ten files of the workspace.
  - Without Upsert, a draft already there is refused.
  - A save older than the stored draft writes nothing.

Events: [appevents.DraftUpdated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrDraftTarget], [exceptions.ErrDraftTooLarge],
[exceptions.ErrDraftAttachments], [exceptions.ErrDraftExists], or a
wrapped database error.
*/
type SaveDraft struct {
	Saver
}

/*
Handle saves the draft.
*/
func (a *SaveDraft) Handle(ctx context.Context, in SaveInput) (Saved, error) {
	if err := conversations.CanWrite(in.Membership); err != nil {
		return Saved{}, err
	}

	if (in.Mode == conversations.DraftNew) != (in.Conversation == nil) {
		return Saved{}, exceptions.ErrDraftTarget
	}

	var out Saved

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		m := in.Membership

		if err := reachable(ctx, q, m, in.Conversation); err != nil {
			return err
		}

		if err := check(ctx, q, m.Workspace.ID, in.Content); err != nil {
			return err
		}

		existing, err := q.FindMemberDraftFor(ctx, models.FindMemberDraftForParams{
			WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, ConversationID: in.Conversation, Mode: in.Mode,
		})
		if err == nil && !in.Upsert {
			return exceptions.ErrDraftExists
		}

		if err == nil {
			out, err = a.write(ctx, tx, edit{m: m, d: existing, c: in.Content, mutation: in.MutationID})

			return err
		}

		if !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find draft: %w", err)
		}

		out, err = a.create(ctx, tx, in)

		return err
	})

	return out, err
}

/*
create starts the draft.
*/
func (a *SaveDraft) create(ctx context.Context, tx *database.Tx, in SaveInput) (Saved, error) {
	m := in.Membership

	d, err := models.New(tx).CreateDraft(ctx, models.CreateDraftParams{
		WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, ConversationID: in.Conversation, Mode: in.Mode,
		Body: in.Content.Body, AttachmentIds: attachments(in.Content), Recipients: recipients(in.Content),
		ClientUpdatedAt: in.Content.ClientUpdatedAt, Now: a.Clock.Now(),
	})
	if err != nil {
		return Saved{}, fmt.Errorf("create draft: %w", err)
	}

	return Saved{Draft: d, Created: true}, a.tell(ctx, tx, m, changed(d, in.MutationID))
}

/*
edit is a save of a stored draft: who saves it, the draft, the content
it now holds and the client's mutation id.
*/
type edit struct {
	m        authz.Membership
	d        models.ConversationDraft
	c        Content
	mutation string
}

/*
write replaces the stored draft's content unless the stored one is
newer.
*/
func (s Saver) write(ctx context.Context, tx *database.Tx, e edit) (Saved, error) {
	d, c := e.d, e.c
	if d.ClientUpdatedAt.After(c.ClientUpdatedAt) {
		return Saved{Draft: d, Stale: true}, nil
	}

	updated, err := models.New(tx).UpdateDraft(ctx, models.UpdateDraftParams{
		WorkspaceID: d.WorkspaceID, ID: d.ID, Body: c.Body, AttachmentIds: attachments(c),
		Recipients: recipients(c), ClientUpdatedAt: c.ClientUpdatedAt, Now: s.Clock.Now(),
	})
	if err != nil {
		return Saved{}, fmt.Errorf("update draft: %w", err)
	}

	return Saved{Draft: updated}, s.tell(ctx, tx, e.m, changed(updated, e.mutation))
}

/*
tell lets the member's screens know their draft changed, the event's
DraftID nil once it went away.
*/
func (s Saver) tell(ctx context.Context, tx *database.Tx, m authz.Membership, e appevents.DraftUpdated) error {
	e.Base = appevents.Base{
		ID: ids.New(), WorkspaceID: m.Workspace.ID, OccurredAt: s.Clock.Now(),
		Actor:            appevents.Actor{Type: "member", ID: m.Member.ID},
		ClientMutationID: e.ClientMutationID,
	}
	e.MemberID = m.Member.ID

	return s.Events.Dispatch(ctx, tx, e)
}

/*
changed is the event of a saved draft.
*/
func changed(d models.ConversationDraft, mutation string) appevents.DraftUpdated {
	return appevents.DraftUpdated{
		Base:           appevents.Base{ClientMutationID: mutation},
		ConversationID: d.ConversationID, Mode: d.Mode, DraftID: &d.ID,
	}
}

/*
check refuses a body over 64 KB and files past ten or of another
workspace.
*/
func check(ctx context.Context, q *models.Queries, workspace ids.UUID, c Content) error {
	if len(c.Body) > maxBody {
		return exceptions.ErrDraftTooLarge
	}

	if len(c.AttachmentIDs) == 0 {
		return nil
	}

	if len(c.AttachmentIDs) > maxAttachments {
		return exceptions.ErrDraftAttachments
	}

	found, err := q.ListAttachmentsByID(ctx, models.ListAttachmentsByIDParams{
		WorkspaceID: workspace, Ids: c.AttachmentIDs,
	})
	if err != nil {
		return fmt.Errorf("find draft attachments: %w", err)
	}

	if len(found) != len(c.AttachmentIDs) {
		return exceptions.ErrDraftAttachments
	}

	return nil
}

/*
attachments returns the draft's files, an empty list for none.
*/
func attachments(c Content) []ids.UUID {
	if c.AttachmentIDs == nil {
		return []ids.UUID{}
	}

	return c.AttachmentIDs
}

/*
recipients returns the staged recipients, an empty object for none.
*/
func recipients(c Content) json.RawMessage {
	if len(c.Recipients) == 0 {
		return json.RawMessage("{}")
	}

	return c.Recipients
}
