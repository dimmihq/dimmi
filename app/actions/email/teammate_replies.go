/*
|--------------------------------------------------------------------------
| Teammate Replies
|--------------------------------------------------------------------------
|
| A teammate who answers a conversation from their own mailbox is not
| its customer. Their mail joins the thread as their internal note,
| under the conversation's customer, and never wakes the conversation
| or reaches anyone: only a reply written in dimmi is sent. Only an
| address proven to be the teammate's counts, so a stranger using it
| stays a stranger.
|
*/

package email

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/app/models"
)

/*
teammateOnThread makes the message a teammate's note when a proven
member answered a conversation the mail threads into, with the
conversation's customer as the person it is about.
*/
func teammateOnThread(ctx context.Context, q *models.Queries, d *decision) error {
	conversation := d.hit.conversation
	if d.insp.member == nil || d.insp.forward != nil || conversation == nil || d.who.fromWorkspace {
		return nil
	}

	contact, err := q.FindContact(ctx, models.FindContactParams{
		WorkspaceID: conversation.WorkspaceID, ID: conversation.ContactID,
	})
	if err != nil {
		return fmt.Errorf("find conversation contact: %w", err)
	}

	if contact.Email == nil {
		return nil
	}

	d.who = sender{name: deref(contact.Name), email: lower(*contact.Email), member: d.insp.member}
	d.contact = &contact

	return nil
}
