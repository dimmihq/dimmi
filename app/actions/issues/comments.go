/*
|--------------------------------------------------------------------------
| Comments From Tools
|--------------------------------------------------------------------------
|
| A comment on a linked issue, or an update posted on a linked
| incident, reaches its ticket as an internal note that names the tool,
| the issue and who wrote it. Nothing in a comment writes to customers:
| what they are told comes from a person, or from the workflows the
| ticket.close_loop trigger runs.
|
*/

package issues

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
commentOn turns a tool's comment into an internal note on the link's
ticket.
*/
func (s *Issues) commentOn(ctx context.Context, ws, linkID ids.UUID, e trackers.Event) error {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return nil
	}

	return s.asTool(ctx, ws, linkID, func(ctx context.Context, w *tickets.Writer, link models.TicketIssue) error {
		t, err := w.Lock(ctx, ws, link.TicketID)
		if err != nil || t.DeletedAt != nil {
			return nil //nolint:nilerr // a ticket gone takes no notes
		}

		note := clip(noteText(e, Label(link.Provider)+" "+link.Key, body), 20000)

		_, err = w.Note(ctx, t, note)

		return err
	})
}

/*
noteText says who wrote on which issue, and what. A tool that does not
say who wrote, such as Rootly's timeline or an incident.io update, is
named instead.
*/
func noteText(e trackers.Event, issue, body string) string {
	what := "Comment"
	if e.Kind == trackers.EventUpdate {
		what = "Update"
	}

	if e.Author == "" {
		return fmt.Sprintf("%s on %s: %s", what, issue, body)
	}

	did := "commented on"
	if e.Kind == trackers.EventUpdate {
		did = "posted an update on"
	}

	return fmt.Sprintf("%s %s %s: %s", e.Author, did, issue, body)
}
