/*
|--------------------------------------------------------------------------
| Apply Conversation Changes
|--------------------------------------------------------------------------
|
| One request may change several things about a conversation at once:
| spam, who holds it, its status, its details and the caller's read
| state. They are applied in that fixed order, whatever order the
| request named them in, under one lock and in one transaction: spam
| first so status events carry the final flag, assignment before status
| so assign-then-close reads as one handover, details last since no rule
| depends on them. Any field failing undoes every field. A field already
| as asked changes nothing and says nothing.
|
*/

package conversations

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Changes are what a request changes, nil or false to leave a field as it
is. Assign carries the assignment when AssignMember or AssignTeam says
that part was sent; the other part stays. Status comes with
SnoozedUntil, SnoozePreset or SnoozeOption when snoozing, and Note
when closing.
*/
type Changes struct {
	Spam         *bool
	AssignMember bool
	AssignTeam   bool
	Assign       Assignment
	Status       *string
	SnoozedUntil *time.Time
	SnoozePreset string
	SnoozeOption string
	Note         *string
	Attributes   Attributes
	Unread       *bool
}

/*
ApplyChangesInput is who changes which conversation, and how.
MutationID is the id the client gave the change, empty when it gave
none.
*/
type ApplyChangesInput struct {
	Membership authz.Membership
	Ref        Ref
	Changes    Changes
	MutationID string
}

/*
Applied is the conversation as it now is and the fields that changed,
in the order they were applied.
*/
type Applied struct {
	Conversation Loaded
	Changed      []string
}

/*
ApplyConversationChanges changes a conversation.

Rules:
  - The caller needs conversations:write.
  - Fields apply in the order spam, assignment, status, details, unread,
    in one transaction; one failing field undoes them all.
  - A field already as asked is skipped.

Events: [appevents.ConversationMarkedSpam] or
[appevents.ConversationUnmarkedSpam],
[appevents.ConversationAssigned], the status events,
[appevents.ConversationUpdated], [appevents.ConversationPriorityChanged]
and [appevents.ConversationRead], for what changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrMemberInactive], [exceptions.ErrTeamNotFound],
[exceptions.ErrInvalidTransition], [exceptions.ErrSnoozeInPast],
[exceptions.ErrSnoozeTooFar], [exceptions.ErrInboxNotFound], or a
wrapped database error.
*/
type ApplyConversationChanges struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle applies the changes and returns the conversation as it now is.
*/
func (a *ApplyConversationChanges) Handle(ctx context.Context, in ApplyChangesInput) (Applied, error) {
	if err := CanWrite(in.Membership); err != nil {
		return Applied{}, err
	}

	var out Applied

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := NewWriter(ctx, tx, Services{Events: a.Events, Clock: a.Clock}, in.Membership)
		if err != nil {
			return err
		}

		w.MutationID = in.MutationID

		c, err := Lock(ctx, w.Q, in.Membership, in.Ref)
		if err != nil {
			return err
		}

		ap := applier{w: w, m: in.Membership, c: c}
		if err := ap.apply(ctx, in.Changes); err != nil {
			return err
		}

		out.Changed = ap.changed
		out.Conversation, err = LoadOne(ctx, w.Q, in.Membership.Member.ID, ap.c)

		return err
	})

	return out, err
}

/*
applier applies the fields of one request to a conversation, keeping
it as it stands after each step and the names of the fields that
changed.
*/
type applier struct {
	w       *Writer
	m       authz.Membership
	c       models.Conversation
	changed []string
}

/*
apply runs each step in order.
*/
func (ap *applier) apply(ctx context.Context, ch Changes) error {
	for _, step := range []func(context.Context, Changes) error{
		ap.spam, ap.assign, ap.status, ap.attributes, ap.unread,
	} {
		if err := step(ctx, ch); err != nil {
			return err
		}
	}

	return nil
}

/*
spam applies the spam flag.
*/
func (ap *applier) spam(ctx context.Context, ch Changes) error {
	if ch.Spam == nil {
		return nil
	}

	set := ap.w.UnmarkSpam
	if *ch.Spam {
		set = ap.w.MarkSpam
	}

	c, changed, err := set(ctx, ap.c)
	ap.c = c

	if changed {
		ap.changed = append(ap.changed, "spam")
	}

	return err
}

/*
assign applies the assignment, keeping the part the request left out.
*/
func (ap *applier) assign(ctx context.Context, ch Changes) error {
	if !ch.AssignMember && !ch.AssignTeam {
		return nil
	}

	to := Assignment{Member: ap.c.AssigneeMemberID, Team: ap.c.AssigneeTeamID}
	if ch.AssignMember {
		to.Member = ch.Assign.Member
	}

	if ch.AssignTeam {
		to.Team = ch.Assign.Team
	}

	c, changed, err := ap.w.Assign(ctx, ap.c, to)
	ap.c = c

	var fields []string
	if ch.AssignMember {
		fields = append(fields, "assignee_id")
	}

	if ch.AssignTeam {
		fields = append(fields, "team_id")
	}

	if changed {
		ap.changed = append(ap.changed, fields...)
	}

	if err != nil || !changed || ch.Status != nil || sameID(c.AssigneeMemberID, &ap.m.Member.ID) {
		return err
	}

	ap.c, err = ap.w.WakeIf(ctx, ap.m.Workspace, ap.c, ReasonAssigned)

	return err
}

/*
status applies the status, working out when a snooze ends, gives a
conversation woken by hand to a member who always takes what they work
on, and adds the closing note.
*/
func (ap *applier) status(ctx context.Context, ch Changes) error {
	if ch.Status == nil {
		return nil
	}

	to := StatusChange{Status: *ch.Status}
	if *ch.Status == "snoozed" {
		until, err := ap.snoozeUntil(ctx, ch)
		if err != nil {
			return err
		}

		to.Until = until
	}

	from := ap.c.Status

	c, changed, err := ap.w.ChangeStatus(ctx, ap.c, to)
	ap.c = c

	if changed {
		ap.changed = append(ap.changed, "status")
	}

	if err != nil {
		return err
	}

	if ap.c, err = ap.w.takeOnWake(ctx, from, ap.c, ap.m.Member); err != nil || ch.Note == nil {
		return err
	}

	_, err = ap.w.AddNote(ctx, ap.c, *ch.Note)

	return err
}

/*
attributes applies the subject, priority and inbox.
*/
func (ap *applier) attributes(ctx context.Context, ch Changes) error {
	c, fields, err := ap.w.SetAttributes(ctx, ap.c, ch.Attributes)
	ap.c = c
	ap.changed = append(ap.changed, fields...)

	return err
}

/*
unread applies the caller's read state.
*/
func (ap *applier) unread(ctx context.Context, ch Changes) error {
	if ch.Unread == nil {
		return nil
	}

	var (
		changed bool
		err     error
	)

	if *ch.Unread {
		changed, err = ap.w.MarkUnread(ctx, ap.c, ap.m.Member.ID)
	} else {
		changed, err = ap.w.MarkRead(ctx, ap.c, ap.m.Member.ID, nil)
	}

	if changed {
		ap.changed = append(ap.changed, "unread")
	}

	if err != nil {
		return fmt.Errorf("change read state: %w", err)
	}

	return nil
}

/*
snoozeUntil works out when the snooze ends: at the time given, the
preset's time, or the option's time in the caller's location.
*/
func (ap *applier) snoozeUntil(ctx context.Context, ch Changes) (time.Time, error) {
	return SnoozeEnd(ctx, ap.w, ap.c, SnoozeWhen{
		Until: ch.SnoozedUntil, Preset: ch.SnoozePreset, Option: ch.SnoozeOption,
		Location: Location(ap.w.Timezone, ap.m.Workspace),
	})
}
