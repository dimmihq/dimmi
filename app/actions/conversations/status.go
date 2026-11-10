/*
|--------------------------------------------------------------------------
| Conversation Status
|--------------------------------------------------------------------------
|
| A conversation is open, snoozed or closed, and moves between them
| along a fixed table: open closes or sleeps; closed reopens, but never
| goes straight to sleep, since a closed conversation has nothing to
| wake up to; snoozed wakes, closes, or sleeps again until another
| time. Asking for the status a conversation already has changes
| nothing.
|
| A snooze ends at a chosen time, or at a preset read in the person's
| own time zone, falling back to the workspace's: later today is three
| hours on, rounded up to the quarter hour; tomorrow and next week start
| at nine; a week is seven days on, at nine. A snooze ends at least a
| minute and at most a year from now.
|
*/

package conversations

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The snooze presets.
*/
const (
	PresetLaterToday = "later_today"
	PresetTomorrow   = "tomorrow"
	PresetNextWeek   = "next_week"
	PresetOneWeek    = "one_week"
)

/*
Limits on a snooze, and the hour presets wake at.
*/
const (
	minSnooze  = time.Minute
	maxSnooze  = 365 * 24 * time.Hour
	laterToday = 3 * time.Hour
	quarter    = 15 * time.Minute
	wakeHour   = 9
	week       = 7
)

/*
Reasons a conversation opens again.
*/
const (
	ReasonManual          = "manual"
	ReasonReply           = "reply"
	ReasonCustomerMessage = "customer_message"
)

/*
Reasons a conversation closes besides a teammate closing it: the
customer pressed solved on a chat app, or deleted every message they
wrote there.
*/
const (
	ReasonContactSolved = "contact_solved"
	ReasonSourceDeleted = "source_deleted"
)

/*
StatusChange is a requested status: the status and, for a snooze, when
it ends, or the reason for opening.
*/
type StatusChange struct {
	Status string
	Until  time.Time
	Reason string
}

/*
ChangeStatus moves the conversation to the status, and reports false
when it already had it. A move the table does not allow fails with
exceptions.ErrInvalidTransition naming both statuses.
*/
func (w *Writer) ChangeStatus(
	ctx context.Context, c models.Conversation, to StatusChange,
) (models.Conversation, bool, error) {
	from := c.Status

	snoozing := to.Status == string(enums.ConversationStatusSnoozed)
	if from == to.Status && (!snoozing || sameTime(c.SnoozedUntil, to.Until)) {
		return c, false, nil
	}

	if from == string(enums.ConversationStatusClosed) && to.Status == string(enums.ConversationStatusSnoozed) {
		return c, false, exceptions.ErrInvalidTransition.OnField("status").With("from", from).With("to", to.Status)
	}

	params := models.SetConversationStatusParams{
		Status: to.Status, Now: w.Now, WorkspaceID: c.WorkspaceID, ID: c.ID,
	}
	if to.Status == string(enums.ConversationStatusSnoozed) {
		params.SnoozedUntil = &to.Until
	}

	updated, err := w.Q.SetConversationStatus(ctx, params)
	if err != nil {
		return c, false, fmt.Errorf("change status: %w", err)
	}

	return updated, true, w.statusEvent(ctx, updated, from, to)
}

/*
statusEvent records and dispatches what the move was: a close, a snooze,
a wake-up or a reopening.
*/
func (w *Writer) statusEvent(ctx context.Context, c models.Conversation, from string, to StatusChange) error {
	switch {
	case to.Status == string(enums.ConversationStatusClosed):
		return w.emit(ctx, c, entry{"conversation.closed", closeData(to.Reason)}, appevents.ConversationClosed{
			Base: w.Base(c.WorkspaceID), ConversationRef: ref(c), Reason: to.Reason,
		})
	case to.Status == string(enums.ConversationStatusSnoozed):
		return w.emit(ctx, c, entry{"conversation.snoozed", map[string]any{"until": to.Until}},
			appevents.ConversationSnoozed{
				Base: w.Base(c.WorkspaceID), ConversationRef: ref(c), Until: to.Until,
			})
	case from == string(enums.ConversationStatusSnoozed):
		return w.emit(ctx, c, entry{"conversation.unsnoozed", map[string]any{"reason": to.reason()}},
			appevents.ConversationUnsnoozed{
				Base: w.Base(c.WorkspaceID), ConversationRef: ref(c), Reason: to.reason(),
			})
	}

	return w.emit(ctx, c, entry{"conversation.reopened", map[string]any{"reason": to.reason()}},
		appevents.ConversationReopened{
			Base: w.Base(c.WorkspaceID), ConversationRef: ref(c), Reason: to.reason(),
		})
}

/*
closeData is what the timeline keeps of a close: its reason, when it
was not a teammate closing it by hand.
*/
func closeData(reason string) map[string]any {
	if reason == "" {
		return nil
	}

	return map[string]any{"reason": reason}
}

/*
entry is how the timeline names a change, and what it carries.
*/
type entry struct {
	name string
	data map[string]any
}

/*
emit records the change on the timeline and dispatches its event.
*/
func (w *Writer) emit(ctx context.Context, c models.Conversation, e entry, event any) error {
	if err := w.Record(ctx, c, e.name, e.data); err != nil {
		return err
	}

	return w.Dispatch(ctx, event)
}

/*
reason returns why the conversation opens, manual unless said.
*/
func (s StatusChange) reason() string {
	if s.Reason == "" {
		return ReasonManual
	}

	return s.Reason
}

/*
sameTime reports whether the wake-up time is the one given.
*/
func sameTime(current *time.Time, t time.Time) bool {
	return current != nil && current.Equal(t)
}

/*
SnoozeUntil works out when a snooze ends, from a time or a preset in the
location, and checks it lies within the limits.
*/
func SnoozeUntil(now time.Time, until *time.Time, preset string, loc *time.Location) (time.Time, error) {
	t := resolvePreset(now.In(loc), preset)
	if until != nil {
		t = *until
	}

	if t.Before(now.Add(minSnooze)) {
		return t, exceptions.ErrSnoozeInPast
	}

	if t.After(now.Add(maxSnooze)) {
		return t, exceptions.ErrSnoozeTooFar
	}

	return t.UTC(), nil
}

/*
resolvePreset returns when the preset ends, from now in the person's
location.
*/
func resolvePreset(now time.Time, preset string) time.Time {
	day := func(offset int) time.Time {
		return time.Date(now.Year(), now.Month(), now.Day()+offset, wakeHour, 0, 0, 0, now.Location())
	}

	switch preset {
	case PresetTomorrow:
		return day(1)
	case PresetNextWeek:
		return day((int(time.Monday)-int(now.Weekday())+week-1)%week + 1)
	case PresetOneWeek:
		return day(week)
	}

	return now.Add(laterToday).Add(quarter - 1).Truncate(quarter)
}

/*
Location returns the time zone presets are read in: the person's own,
then the workspace's, then UTC.
*/
func Location(timezone *string, workspace models.Workspace) *time.Location {
	for _, name := range []*string{timezone, &workspace.Timezone} {
		if name == nil || *name == "" {
			continue
		}

		if loc, err := time.LoadLocation(*name); err == nil {
			return loc
		}
	}

	return time.UTC
}
