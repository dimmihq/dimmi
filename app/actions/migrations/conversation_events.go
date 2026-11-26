/*
|--------------------------------------------------------------------------
| Conversation Events
|--------------------------------------------------------------------------
|
| Assignments, teams, statuses, priorities and tags are replayed at
| their own time; anything else the other tool kept joins the timeline
| with its own type and text.
|
*/

package migrations

import (
	"context"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
event replays one event entry.
*/
func (r *replay) event(ctx context.Context, e Entry) error {
	ev := e.Event
	w := r.writer(e.Author, r.at(e))

	switch ev.Type {
	case EventAssignment:
		if r.conflicts[fieldAssignee] {
			return nil
		}

		return r.assign(ctx, w, target{kind: KindTeammate, ref: ev.AssigneeRef})
	case EventTeamAssignment:
		if r.conflicts[fieldTeam] {
			return nil
		}

		return r.assign(ctx, w, target{kind: KindTeam, ref: ev.TeamRef})
	case EventStatus:
		if r.conflicts[fieldStatus] {
			return nil
		}

		return r.status(ctx, w, ev.Status, ev.Until)
	case EventPriority:
		if r.conflicts[fieldPriority] {
			return nil
		}

		return r.priority(ctx, w, ev.Priority)
	case EventTag:
		if r.conflicts[fieldTags] {
			return nil
		}

		return r.tags(ctx, w, ev.AddedTagRefs, ev.RemovedTagRef)
	case EventAttribute:
		r.out.unsupported("entries.event.attribute")

		return nil
	}

	return r.custom(ctx, w, ev.RawType, ev.Text)
}

/*
custom adds an event the other tool kept to the timeline, with its own
type and text.
*/
func (r *replay) custom(ctx context.Context, w *conversations.Writer, rawType, text string) error {
	return w.Record(ctx, r.conv, "conversation.imported_event", map[string]any{
		"raw_type": cut(rawType, 100), "text": cut(text, 2000),
	})
}

/*
target is who an assignment names: a teammate or a team by reference,
nobody for an empty one.
*/
type target struct {
	kind string
	ref  *string
}

/*
assign hands the conversation to the teammate or team named, nobody for
an empty one; one who is not active here is kept on the timeline as
text instead.
*/
func (r *replay) assign(ctx context.Context, w *conversations.Writer, t target) error {
	to := conversations.Assignment{Member: r.conv.AssigneeMemberID, Team: r.conv.AssigneeTeamID}
	ref := deref(t.ref)

	var id *ids.UUID

	if ref != "" {
		found, ok := r.s.refs.id(t.kind, ref)
		if !ok {
			return r.custom(ctx, w, "assignment", "Assigned to "+r.nameOf(t.kind, ref))
		}

		id = &found
	}

	if t.kind == KindTeammate {
		to.Member = id
	} else {
		to.Team = id
	}

	conv, _, err := w.Assign(ctx, r.conv, to)
	if errors.Is(err, exceptions.ErrMemberInactive) {
		return r.custom(ctx, w, "assignment", "Assigned to "+r.nameOf(t.kind, ref))
	}

	if err != nil {
		return err
	}

	r.conv = conv

	return nil
}

/*
nameOf returns what the record of the kind was called.
*/
func (r *replay) nameOf(kind, ref string) string {
	if name := r.s.refs.label(kind, ref); name != "" {
		return name
	}

	return "someone no longer here"
}

/*
status moves the conversation to the status, snoozed until the time
given; a move the conversation cannot make from its status is skipped.
*/
func (r *replay) status(ctx context.Context, w *conversations.Writer, status string, until *time.Time) error {
	if status == "" || status == r.conv.Status {
		return nil
	}

	change := conversations.StatusChange{Status: status}

	if status == string(enums.ConversationStatusSnoozed) {
		if until == nil {
			r.out.warn("status", "snooze_without_until", "A snooze without an end was left out.")

			return nil
		}

		change.Until = until.UTC()
	}

	conv, _, err := w.ChangeStatus(ctx, r.conv, change)
	if err := r.allowed(err); err != nil {
		return err
	}

	if err == nil {
		r.conv = conv
	}

	return nil
}

/*
allowed drops a refusal of a status move, keeping it as a warning.
*/
func (r *replay) allowed(err error) error {
	if errors.Is(err, exceptions.ErrInvalidTransition) || errors.Is(err, exceptions.ErrSnoozeInPast) ||
		errors.Is(err, exceptions.ErrSnoozeTooFar) {
		r.out.warn("status", "status_skipped", "A status change the conversation could not make was left out.")

		return nil
	}

	return err
}

/*
priority sets the conversation's priority.
*/
func (r *replay) priority(ctx context.Context, w *conversations.Writer, p string) error {
	if p == "" {
		return nil
	}

	level := enums.ConversationPriority(p)

	conv, _, err := w.SetAttributes(ctx, r.conv, conversations.Attributes{Priority: &level})
	if err != nil {
		return err
	}

	r.conv = conv

	return nil
}

/*
tags puts the tags on the conversation and takes the others off.
*/
func (r *replay) tags(ctx context.Context, w *conversations.Writer, added, removed []string) error {
	if on := r.s.refs.ids(KindTag, added); len(on) > 0 {
		if _, err := w.AddTags(ctx, r.conv, tags.Refs{IDs: on}, nil); err != nil {
			return err
		}
	}

	if off := r.s.refs.ids(KindTag, removed); len(off) > 0 {
		if _, err := w.RemoveTags(ctx, r.conv, off); err != nil {
			return err
		}
	}

	return nil
}
