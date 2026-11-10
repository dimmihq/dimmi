/*
|--------------------------------------------------------------------------
| Opens, Clicks And Unsubscribes
|--------------------------------------------------------------------------
|
| What recipients do with a broadcast email comes back through its
| signed links. An open is an estimate, since mail apps load images on
| their own; a click proves the open; an unsubscribe is honored at once
| and for good. A one-click unsubscribe leaves the broadcast's topic, or
| every broadcast when it has none, and never stops service notices or
| the replies of a conversation.
|
*/

package broadcasts

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Opened records that a recipient's email was opened.
*/
func (b *Broadcasts) Opened(ctx context.Context, token string) error {
	ref, err := b.Links.Read(token)
	if err != nil {
		return exceptions.ErrNotFound
	}

	err = b.inWorkspace(ctx, ref.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		r, err := q.MarkBroadcastRecipientOpened(ctx, models.MarkBroadcastRecipientOpenedParams{
			WorkspaceID: ref.WorkspaceID, ID: ref.RecipientID, Now: b.Clock.Now(),
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return wrap("mark opened", err)
		}

		return b.recount(ctx, r.WorkspaceID, r.BroadcastID)
	})

	return err
}

/*
Clicked records a click and returns where the link goes.
*/
func (b *Broadcasts) Clicked(ctx context.Context, token string) (string, error) {
	ref, err := b.Links.Read(token)
	if err != nil || ref.N < 1 {
		return "", exceptions.ErrNotFound
	}

	var url string

	err = b.inWorkspace(ctx, ref.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		r, err := q.FindBroadcastRecipient(ctx, models.FindBroadcastRecipientParams{
			WorkspaceID: ref.WorkspaceID, ID: ref.RecipientID,
		})
		if err != nil {
			return notFound(err, "find recipient")
		}

		link, err := q.FindBroadcastLink(ctx, models.FindBroadcastLinkParams{
			WorkspaceID: ref.WorkspaceID, BroadcastID: r.BroadcastID, N: int32Of(ref.N),
		})
		if err != nil {
			return notFound(err, "find link")
		}

		url = link.Url

		return b.countClick(ctx, q, r, link)
	})

	return url, err
}

/*
countClick records the first click of a recipient and every click of a
link.
*/
func (b *Broadcasts) countClick(ctx context.Context, q *models.Queries, r models.BroadcastRecipient,
	link models.BroadcastLink,
) error {
	if err := q.CountBroadcastLinkClick(ctx, models.CountBroadcastLinkClickParams{
		WorkspaceID: r.WorkspaceID, BroadcastID: r.BroadcastID, N: link.N,
	}); err != nil {
		return wrap("count click", err)
	}

	_, err := q.MarkBroadcastRecipientClicked(ctx, models.MarkBroadcastRecipientClickedParams{
		WorkspaceID: r.WorkspaceID, ID: r.ID, Now: b.Clock.Now(),
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return wrap("mark clicked", err)
	}

	return b.recount(ctx, r.WorkspaceID, r.BroadcastID)
}

/*
OneClick unsubscribes the recipient from what the broadcast was about,
as a mail app's unsubscribe button asks.
*/
func (b *Broadcasts) OneClick(ctx context.Context, token string) error {
	ref, err := b.Links.Read(token)
	if err != nil {
		return exceptions.ErrNotFound
	}

	return b.inWorkspace(ctx, ref.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		r, row, err := recipientOf(ctx, q, ref)
		if err != nil || r.ContactID == nil {
			return err
		}

		if row.TopicID != nil {
			err = q.SetContactTopicSubscription(ctx, models.SetContactTopicSubscriptionParams{
				WorkspaceID: r.WorkspaceID, ContactID: *r.ContactID, TopicID: *row.TopicID, Subscribed: false,
				Source: "one_click", Now: b.Clock.Now(),
			})
		} else {
			err = q.OptOutOfBroadcasts(ctx, models.OptOutOfBroadcastsParams{
				WorkspaceID: r.WorkspaceID, ContactID: *r.ContactID, Source: "one_click", Now: b.Clock.Now(),
			})
		}

		if err != nil {
			return wrap("unsubscribe", err)
		}

		return b.markUnsubscribed(ctx, q, r)
	})
}

/*
markUnsubscribed records on the recipient that they left.
*/
func (b *Broadcasts) markUnsubscribed(ctx context.Context, q *models.Queries, r models.BroadcastRecipient) error {
	err := q.MarkBroadcastRecipientUnsubscribed(ctx, models.MarkBroadcastRecipientUnsubscribedParams{
		WorkspaceID: r.WorkspaceID, ID: r.ID, Now: b.Clock.Now(),
	})
	if err != nil {
		return wrap("mark unsubscribed", err)
	}

	return b.recount(ctx, r.WorkspaceID, r.BroadcastID)
}

/*
recipientOf loads the recipient a link names and its broadcast.
*/
func recipientOf(ctx context.Context, q *models.Queries, ref Ref) (models.BroadcastRecipient, models.Broadcast, error) {
	r, err := q.FindBroadcastRecipient(ctx, models.FindBroadcastRecipientParams{
		WorkspaceID: ref.WorkspaceID, ID: ref.RecipientID,
	})
	if err != nil {
		return r, models.Broadcast{}, notFound(err, "find recipient")
	}

	row, err := q.FindBroadcast(ctx, models.FindBroadcastParams{WorkspaceID: r.WorkspaceID, ID: r.BroadcastID})
	if err != nil {
		return r, row, notFound(err, "find broadcast")
	}

	return r, row, nil
}
