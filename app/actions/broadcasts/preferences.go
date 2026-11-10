/*
|--------------------------------------------------------------------------
| The Preference Page
|--------------------------------------------------------------------------
|
| Every broadcast email links to a page where the person chooses which
| topics they still want, or leaves every broadcast at once. It needs no
| account: the signed link is the proof. Leaving everything never stops
| service notices, which the page says plainly.
|
*/

package broadcasts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Preferences is what the page shows: the workspace, the address, each
topic and whether the person gets it, and whether they left
everything.
*/
type Preferences struct {
	Workspace string
	Email     string
	Topics    []TopicChoice
	OptedOut  bool
}

/*
TopicChoice is one topic on the page and whether the person gets it.
*/
type TopicChoice struct {
	ID          ids.UUID
	Name        string
	Description string
	Subscribed  bool
}

/*
Preferences returns what the page a link names shows.
*/
func (b *Broadcasts) Preferences(ctx context.Context, token string) (Preferences, error) {
	ref, err := b.Links.Read(token)
	if err != nil {
		return Preferences{}, exceptions.ErrNotFound
	}

	var out Preferences

	err = b.inWorkspace(ctx, ref.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		var err error

		out, err = b.readPreferences(ctx, q, ref)

		return err
	})

	return out, err
}

/*
SavePreferences stores the person's choices: the topics they keep, or
leaving every broadcast.
*/
func (b *Broadcasts) SavePreferences(
	ctx context.Context, token string, keep []ids.UUID, all bool,
) (Preferences, error) {
	ref, err := b.Links.Read(token)
	if err != nil {
		return Preferences{}, exceptions.ErrNotFound
	}

	var out Preferences

	err = b.inWorkspace(ctx, ref.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		r, _, err := recipientOf(ctx, q, ref)
		if err != nil {
			return err
		}

		if r.ContactID == nil {
			return exceptions.ErrNotFound
		}

		if err := b.choose(ctx, q, r, choice{keep: keep, all: all}); err != nil {
			return err
		}

		out, err = b.readPreferences(ctx, q, ref)

		return err
	})

	return out, err
}

/*
choice is what the person picked on the page: the topics they keep, or
leaving every broadcast.
*/
type choice struct {
	keep []ids.UUID
	all  bool
}

/*
choose records each topic choice and the choice about every broadcast.
*/
func (b *Broadcasts) choose(ctx context.Context, q *models.Queries, r models.BroadcastRecipient, c choice) error {
	topics, err := q.ListBroadcastTopics(ctx, r.WorkspaceID)
	if err != nil {
		return wrap("list topics", err)
	}

	kept := map[ids.UUID]bool{}
	for _, id := range c.keep {
		kept[id] = true
	}

	for _, t := range topics {
		if err := q.SetContactTopicSubscription(ctx, models.SetContactTopicSubscriptionParams{
			WorkspaceID: r.WorkspaceID, ContactID: *r.ContactID, TopicID: t.ID, Subscribed: kept[t.ID] && !c.all,
			Source: "preference_page", Now: b.Clock.Now(),
		}); err != nil {
			return wrap("set topic choice", err)
		}
	}

	if !c.all {
		return wrap("opt back in", q.OptBackIntoBroadcasts(ctx, models.OptBackIntoBroadcastsParams{
			WorkspaceID: r.WorkspaceID, ContactID: *r.ContactID,
		}))
	}

	if err := q.OptOutOfBroadcasts(ctx, models.OptOutOfBroadcastsParams{
		WorkspaceID: r.WorkspaceID, ContactID: *r.ContactID, Source: "preference_page", Now: b.Clock.Now(),
	}); err != nil {
		return wrap("opt out", err)
	}

	return b.markUnsubscribed(ctx, q, r)
}

/*
readPreferences reads what the page shows for a link.
*/
func (b *Broadcasts) readPreferences(ctx context.Context, q *models.Queries, ref Ref) (Preferences, error) {
	r, _, err := recipientOf(ctx, q, ref)
	if err != nil {
		return Preferences{}, err
	}

	workspace, err := q.FindWorkspace(ctx, r.WorkspaceID)
	if err != nil {
		return Preferences{}, wrap("find workspace", err)
	}

	out := Preferences{Workspace: workspace.Name, Email: r.Address}
	if r.ContactID == nil {
		return out, nil
	}

	if out.OptedOut, err = q.ContactOptedOutOfBroadcasts(ctx, models.ContactOptedOutOfBroadcastsParams{
		WorkspaceID: r.WorkspaceID, ContactID: *r.ContactID,
	}); err != nil {
		return out, wrap("read opt-out", err)
	}

	if err := b.topicChoices(ctx, q, r, &out); err != nil {
		return out, err
	}

	return out, nil
}

/*
topicChoices fills in each topic and whether the person gets it.
*/
func (b *Broadcasts) topicChoices(ctx context.Context, q *models.Queries, r models.BroadcastRecipient,
	out *Preferences,
) error {
	topics, err := q.ListBroadcastTopics(ctx, r.WorkspaceID)
	if err != nil {
		return wrap("list topics", err)
	}

	choices, err := q.ListContactTopicSubscriptions(ctx, models.ListContactTopicSubscriptionsParams{
		WorkspaceID: r.WorkspaceID, ContactID: *r.ContactID,
	})
	if err != nil {
		return wrap("list topic choices", err)
	}

	chosen := map[ids.UUID]bool{}
	for _, c := range choices {
		chosen[c.TopicID] = c.Subscribed
	}

	for _, t := range topics {
		subscribed, ok := chosen[t.ID]
		if !ok {
			subscribed = t.Consent == "opt_out"
		}

		out.Topics = append(out.Topics, TopicChoice{
			ID: t.ID, Name: t.Name, Description: t.Description, Subscribed: subscribed && !out.OptedOut,
		})
	}

	return nil
}
