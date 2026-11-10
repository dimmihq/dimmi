/*
|--------------------------------------------------------------------------
| Briefs In The Conversation
|--------------------------------------------------------------------------
|
| A brief a teammate asks for is useful to the whole team, not only to
| the teammate who asked, so it is also kept on the conversation as an
| internal note: anyone who opens the conversation later reads it
| without asking Operator again. A teammate who may not write notes
| still gets the brief in their own thread.
|
*/

package operator

import (
	"context"
	"errors"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
briefHeading opens the note, so readers know Operator wrote the brief.
*/
const briefHeading = "**Brief from Operator**\n\n"

/*
NoteBrief keeps a finished brief on the conversation as an internal
note, written by the teammate who asked for it. A brief that failed,
stopped or said nothing is not kept.
*/
func (e *Engine) NoteBrief(ctx context.Context, m authz.Membership, conversation ids.UUID,
	brief models.OperatorMessage,
) error {
	text := strings.TrimSpace(brief.Body)
	if brief.Status != "done" || text == "" || e.Services.Note == nil {
		return nil
	}
	_, err := e.Services.Note.Handle(ctx, messages.NoteInput{
		Membership: m, Ref: conversations.Ref{ID: conversation}, Body: briefHeading + text,
	})
	if errors.Is(err, exceptions.ErrForbidden) {
		return nil
	}
	return err
}
