/*
|--------------------------------------------------------------------------
| Conversation Attributes
|--------------------------------------------------------------------------
|
| The details of a conversation a teammate edits: its subject, whether
| it is a priority, which inbox it sits in, and whether it is spam. A
| new subject and a move to another inbox are one update, saying what
| each was and became; priority and spam each say so on their own.
| Marking spam leaves the status as it was, so taking a conversation out
| of spam puts it back where it stood.
|
*/

package conversations

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Attributes are the details to change, nil to leave as they are.
*/
type Attributes struct {
	Subject  *string
	Priority *enums.ConversationPriority
	InboxID  *ids.UUID
}

/*
SetAttributes changes the conversation's details and returns the names
of the fields that changed, in the order subject, priority, inbox_id. A
new subject passes the workspace's redaction rules first.
*/
func (w *Writer) SetAttributes(
	ctx context.Context, c models.Conversation, a Attributes,
) (models.Conversation, []string, error) {
	params := models.SetConversationAttributesParams{
		Subject: c.Subject, Priority: c.Priority, InboxID: c.InboxID, Now: w.Now, WorkspaceID: c.WorkspaceID, ID: c.ID,
	}
	changes := map[string][2]any{}

	var changed []string

	a.Subject = copyText(a.Subject)
	if _, err := redactText(ctx, w.Q, c.WorkspaceID, a.Subject); err != nil {
		return c, nil, err
	}

	if a.Subject != nil && (c.Subject == nil || *c.Subject != *a.Subject) {
		changes["subject"] = [2]any{c.Subject, *a.Subject}
		params.Subject, changed = a.Subject, append(changed, "subject")
	}

	if a.Priority != nil {
		if level, ok := a.Priority.Level(); ok && level != c.Priority {
			params.Priority, changed = level, append(changed, "priority")
		}
	}

	if a.InboxID != nil && *a.InboxID != c.InboxID {
		if err := w.checkInbox(ctx, c.WorkspaceID, *a.InboxID); err != nil {
			return c, nil, err
		}

		changes["inbox_id"] = [2]any{
			models.PublicID(models.PrefixInbox, c.InboxID), models.PublicID(models.PrefixInbox, *a.InboxID),
		}
		params.InboxID, changed = *a.InboxID, append(changed, "inbox_id")
	}

	if len(changed) == 0 {
		return c, nil, nil
	}

	updated, err := w.Q.SetConversationAttributes(ctx, params)
	if err != nil {
		return c, nil, fmt.Errorf("update conversation: %w", err)
	}

	return updated, changed, w.attributeEvents(ctx, c, updated, changes)
}

/*
attributeEvents records and dispatches the updates: one for subject and
inbox together, one for priority.
*/
func (w *Writer) attributeEvents(
	ctx context.Context, before, after models.Conversation, changes map[string][2]any,
) error {
	if len(changes) > 0 {
		data := map[string]any{"changes": changes}
		updated := appevents.ConversationUpdated{
			Base: w.Base(after.WorkspaceID), ConversationRef: ref(after), Changes: changes,
		}
		if before.InboxID != after.InboxID {
			updated.FromInboxID = &before.InboxID
		}
		if err := w.emit(ctx, after, entry{"conversation.updated", data}, updated); err != nil {
			return err
		}
	}

	if before.Priority == after.Priority {
		return nil
	}

	from := string(enums.ConversationPriorityOf(before.Priority))
	to := string(enums.ConversationPriorityOf(after.Priority))

	return w.emit(ctx, after, entry{"conversation.priority_changed", map[string]any{"from": from, "to": to}},
		appevents.ConversationPriorityChanged{
			Base: w.Base(after.WorkspaceID), ConversationRef: ref(after), From: from, To: to,
		})
}

/*
checkInbox refuses an inbox the workspace does not have.
*/
func (w *Writer) checkInbox(ctx context.Context, workspaceID, inboxID ids.UUID) error {
	inbox, err := w.Q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: workspaceID, ID: inboxID})
	if errors.Is(err, database.ErrNotFound) || (err == nil && inbox.DeletedAt != nil) {
		return exceptions.ErrInboxNotFound
	}

	if err != nil {
		return fmt.Errorf("find inbox: %w", err)
	}

	return nil
}

/*
MarkSpam flags the conversation as spam, and reports false when it
already was.
*/
func (w *Writer) MarkSpam(ctx context.Context, c models.Conversation) (models.Conversation, bool, error) {
	return w.setSpam(ctx, c, spamChange{spam: true, event: "conversation.marked_spam"})
}

/*
UnmarkSpam takes the conversation out of spam, and reports false when
it was not spam.
*/
func (w *Writer) UnmarkSpam(ctx context.Context, c models.Conversation) (models.Conversation, bool, error) {
	return w.setSpam(ctx, c, spamChange{spam: false, event: "conversation.unmarked_spam"})
}

/*
spamChange is a change of the spam flag and the timeline event naming
it.
*/
type spamChange struct {
	spam  bool
	event string
}

/*
setSpam writes the spam flag unless it already stands so.
*/
func (w *Writer) setSpam(ctx context.Context, c models.Conversation, to spamChange) (models.Conversation, bool, error) {
	if c.Spam == to.spam {
		return c, false, nil
	}

	updated, err := w.Q.SetConversationSpam(ctx, models.SetConversationSpamParams{
		Spam: to.spam, Now: w.Now, WorkspaceID: c.WorkspaceID, ID: c.ID,
	})
	if err != nil {
		return c, false, fmt.Errorf("mark spam: %w", err)
	}

	var event any = appevents.ConversationUnmarkedSpam{Base: w.Base(c.WorkspaceID), ConversationRef: ref(c)}
	if to.spam {
		event = appevents.ConversationMarkedSpam{Base: w.Base(c.WorkspaceID), ConversationRef: ref(c)}
	}

	return updated, true, w.emit(ctx, updated, entry{to.event, nil}, event)
}
