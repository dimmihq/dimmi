/*
|--------------------------------------------------------------------------
| Linking Trackers
|--------------------------------------------------------------------------
|
| Once every ticket is in, each one that reported a tracker is linked to
| it: a Zendesk incident to its problem, an Intercom ticket to the
| tracker it was linked to. A ticket with a conversation joins the
| tracker's conversations, so the tracker's messages reach its
| customer; one without is linked as a related ticket. A link made by
| an earlier run is left as it is.
|
*/

package ticketimports

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
linkTrackers links every ticket brought in to the tracker it reports.
*/
func (r *run) linkTrackers(ctx context.Context) error {
	for external, made := range r.made {
		if made.tracker == "" {
			continue
		}

		added, err := r.linkTracker(ctx, external, made)
		if err != nil {
			return err
		}

		if added {
			if err := r.progress(ctx, counts{links: 1}); err != nil {
				return err
			}
		}
	}

	return nil
}

/*
linkTracker links one ticket to its tracker, reporting whether the
link is new.
*/
func (r *run) linkTracker(ctx context.Context, external string, made madeTicket) (bool, error) {
	added := false

	err := r.importer.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := r.writer(ctx, tx, made.ticket.CreatedAt)
		if err != nil {
			return err
		}

		tracker, err := w.Q.FindTicketByExternalID(ctx, models.FindTicketByExternalIDParams{
			WorkspaceID: r.row.WorkspaceID, ExternalID: &made.tracker,
		})
		if err != nil || tracker.DeletedAt != nil || tracker.Kind != tickets.KindTracker {
			return nil //nolint:nilerr // a tracker that did not come in leaves the ticket unlinked
		}

		in := tickets.LinkInput{Kind: tickets.LinkRelated, TicketID: &made.ticket.ID}
		if made.ticket.ConversationID != nil {
			in = tickets.LinkInput{Kind: tickets.LinkTracker, ConversationID: made.ticket.ConversationID}
		}

		link, err := w.Link(ctx, tracker, in)
		if err != nil {
			return fmt.Errorf("link imported tracker: %w", err)
		}

		if link.ID == ids.Nil() {
			return nil
		}

		added = true

		return r.record(ctx, w.Q, touched{
			kind: "link", external: external + ">" + made.tracker, id: link.ID,
			created: true,
		})
	})

	return added, err
}
