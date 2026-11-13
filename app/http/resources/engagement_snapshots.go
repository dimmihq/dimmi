/*
|--------------------------------------------------------------------------
| Engagement Snapshots
|--------------------------------------------------------------------------
|
| What an outbound webhook carries about a changelog entry, a feature
| request, a broadcast, a series or a journey through one: the same
| resource the API answers with for it.
|
*/

package resources

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/changelog"
	"github.com/dimmihq/dimmi/app/actions/feedback"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/actions/webhooks"
	"github.com/dimmihq/dimmi/app/models"
)

/*
engagementSnapshot reads and shapes one changelog entry, request,
broadcast, series or journey. Any other kind renders as nothing.
*/
func engagementSnapshot(ctx context.Context, q *models.Queries, ws ids.UUID, ref objectRef,
) (webhooks.Snapshot, error) {
	switch ref.kind {
	case "changelog_entry":
		e, err := changelog.Load(ctx, q, ws, ref.id)

		return webhooks.Snapshot{Object: NewChangelogEntry(e)}, err
	case "feedback_post":
		p, err := feedback.Load(ctx, q, ws, ref.id)

		return webhooks.Snapshot{Object: NewFeedbackPost(p)}, err
	case "broadcast":
		b, err := q.FindBroadcast(ctx, models.FindBroadcastParams{WorkspaceID: ws, ID: ref.id})

		return webhooks.Snapshot{Object: NewBroadcast(b)}, err
	case "series":
		return seriesSnapshot(ctx, q, ws, ref.id)
	case "ticket":
		t, err := tickets.Load(ctx, q, ws, ref.id)

		return webhooks.Snapshot{Object: NewTicket(t), ContactID: t.Ticket.ContactID}, err
	case "series_enrollment":
		e, err := q.FindSeriesEnrollment(ctx, models.FindSeriesEnrollmentParams{WorkspaceID: ws, ID: ref.id})

		return webhooks.Snapshot{Object: NewSeriesEnrollment(e), ContactID: e.ContactID}, err
	}

	return webhooks.Snapshot{}, nil
}

/*
seriesSnapshot shapes a series with the hash of its live version, as
the API does.
*/
func seriesSnapshot(ctx context.Context, q *models.Queries, ws, id ids.UUID) (webhooks.Snapshot, error) {
	s, err := q.FindSeries(ctx, models.FindSeriesParams{WorkspaceID: ws, ID: id})
	if err != nil {
		return webhooks.Snapshot{}, err
	}

	hash := ""
	if s.LiveVersionID != nil {
		v, err := q.FindSeriesVersion(ctx, models.FindSeriesVersionParams{WorkspaceID: ws, ID: *s.LiveVersionID})
		if err != nil {
			return webhooks.Snapshot{}, err
		}

		hash = v.SpecHash
	}

	return webhooks.Snapshot{Object: NewSeries(s, hash)}, nil
}
