/*
|--------------------------------------------------------------------------
| Feedback Events
|--------------------------------------------------------------------------
|
| Posting a request, moving it, counting a vote, merging a duplicate and
| linking a conversation each dispatch an event in the transaction that
| made the change, so webhooks hear about the request. Webhooks read the
| request back the way the API shows it.
|
*/

package feedback

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Load reads one request as the API shows it, with who posted it. A
missing request answers database.ErrNotFound.
*/
func Load(ctx context.Context, q *models.Queries, ws, id ids.UUID) (Post, error) {
	row, err := q.FindFeedbackPost(ctx, models.FindFeedbackPostParams{WorkspaceID: ws, ID: id})
	if err != nil {
		return Post{}, err
	}

	return one(ctx, q, ws, row)
}

/*
announce dispatches the event in the transaction, when events are
wired.
*/
func (p *Posts) announce(ctx context.Context, tx *database.Tx, ev appevents.Named) error {
	if p.Events == nil {
		return nil
	}

	if err := p.Events.Dispatch(ctx, tx, ev); err != nil {
		return fmt.Errorf("dispatch %s: %w", ev.EventName(), err)
	}

	return nil
}

/*
byMember is the header of an event a teammate caused.
*/
func (p *Posts) byMember(ws, member ids.UUID) appevents.Base {
	return appevents.Base{
		ID: ids.New(), WorkspaceID: ws, OccurredAt: p.Clock.Now(), Actor: appevents.Actor{Type: "member", ID: member},
	}
}

/*
byContact is the header of an event a customer caused on the board.
*/
func (p *Posts) byContact(ws, contact ids.UUID) appevents.Base {
	return appevents.Base{
		ID: ids.New(), WorkspaceID: ws, OccurredAt: p.Clock.Now(), Actor: appevents.Actor{Type: "contact", ID: contact},
	}
}

/*
voted announces a counted vote for the request.
*/
func (p *Posts) voted(ctx context.Context, tx *database.Tx, base appevents.Base, v vote) error {
	return p.announce(ctx, tx, appevents.FeedbackPostVoted{
		Base: base, PostID: v.post.ID, ContactID: v.contact, MemberID: v.member, Priority: v.priority,
	})
}

/*
insideTx runs work in the help center's workspace with its transaction.
*/
func (p *Posts) insideTx(ctx context.Context, h models.HelpCenter,
	fn func(context.Context, *database.Tx, *models.Queries) error,
) error {
	scoped := tenancy.With(ctx, h.WorkspaceID.String())

	return p.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, tx, models.New(tx))
	})
}
