/*
|--------------------------------------------------------------------------
| Seeding The Help Center, Changelog And Feedback
|--------------------------------------------------------------------------
|
| The help center's sections and articles go through the dashboard's own
| actions, each article created and published by a teammate, so its
| versions, rendered page and knowledge chunks are exactly what an
| editor's publish leaves. The changelog and the feedback board follow,
| with customers' votes on the requests.
|
*/

package stowline

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/changelog"
	"github.com/dimmihq/dimmi/app/actions/feedback"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
)

func (r *run) helpCenter(ctx context.Context) error {
	a := r.s.App
	center, err := models.New(a.DB).FindDefaultHelpCenter(ctx, r.ws.ID)
	if err != nil {
		return fmt.Errorf("find help center: %w", err)
	}
	r.center = center.ID
	if _, err := a.DB.Exec(ctx, `UPDATE help_centers SET name = 'Stowline Help Center'
		WHERE workspace_id = $1 AND id = $2`,
		r.ws.ID, center.ID); err != nil {
		return fmt.Errorf("name help center: %w", err)
	}

	sections, err := r.collections(ctx)
	if err != nil {
		return err
	}

	r.articles = map[string]ids.UUID{}
	for i, art := range Articles() {
		author := r.team[i%min(len(r.team), 6)]
		made, err := a.Actions.Articles.Create(ctx, helpcenters.ArticleInput{
			Membership: author, HelpCenterID: center.ID, CollectionID: new(sections[art.Collection]),
			Title: new(art.Title), Slug: new(art.Slug), Description: new(art.Description), Body: new(art.Body),
			Visibility: new(art.Visibility),
		})
		if err != nil {
			return fmt.Errorf("create article %q: %w", art.Title, err)
		}
		if _, err := a.Actions.Articles.Publish(ctx, author, made.ID); err != nil {
			return fmt.Errorf("publish article %q: %w", art.Title, err)
		}
		r.articles[art.Title] = made.ID
	}

	// Articles were written over the last two years, not all this minute.
	_, err = a.DB.Exec(ctx, `WITH aged AS (
			SELECT id, now() - (row_number() OVER (ORDER BY id DESC) * interval '4 days 7 hours') AS at
			FROM articles WHERE workspace_id = $1)
		UPDATE articles a SET created_at = aged.at, first_published_at = aged.at,
			published_at = aged.at + interval '30 days', updated_at = aged.at + interval '30 days'
		FROM aged WHERE a.id = aged.id`, r.ws.ID)
	if err != nil {
		return fmt.Errorf("age articles: %w", err)
	}
	return nil
}

/*
collections creates the help center's sections in order and returns
their ids by slug.
*/
func (r *run) collections(ctx context.Context) (map[string]ids.UUID, error) {
	sections := map[string]ids.UUID{}
	for i, c := range Collections() {
		made, err := r.s.App.Actions.Collections.Create(ctx, helpcenters.CollectionInput{
			Membership: r.owner, HelpCenterID: r.center, Name: new(c.Name), Slug: new(c.Slug),
			Description: new(c.Description), Icon: new(c.Icon), Position: new(int32(i)),
		})
		if err != nil {
			return nil, fmt.Errorf("create collection %s: %w", c.Name, err)
		}
		sections[c.Slug] = made.ID
	}
	return sections, nil
}

/*
community publishes the changelog and fills the feedback board with
requests and customers' votes.
*/
func (r *run) community(ctx context.Context) error {
	if err := r.changelog(ctx); err != nil {
		return err
	}
	var voters []ids.UUID
	for _, c := range r.plan.Contacts {
		if !c.Lead && len(voters) < 300 {
			voters = append(voters, c.ID)
		}
	}
	if len(voters) == 0 {
		return nil
	}
	for i, f := range FeedbackRequests() {
		if err := r.request(ctx, i, f, voters); err != nil {
			return err
		}
	}
	return nil
}

/*
changelog publishes the entries, about three weeks apart.
*/
func (r *run) changelog(ctx context.Context) error {
	a, now := r.s.App, r.s.App.Clock.Now()
	entries := ChangelogEntries()
	for i, e := range entries {
		made, err := a.Actions.ChangelogEntries.Create(ctx, changelog.EntryInput{
			Membership: r.owner, HelpCenterID: r.center, Title: new(e[0]), Summary: new(e[1]), Body: new(e[2]),
		})
		if err != nil {
			return fmt.Errorf("create changelog entry %q: %w", e[0], err)
		}
		at := now.Add(-time.Duration(len(entries)-i) * 23 * 24 * time.Hour)
		if _, err := a.Actions.ChangelogEntries.Publish(ctx, r.owner, made.ID,
			changelog.PublishInput{At: &at}); err != nil {
			return fmt.Errorf("publish changelog entry %q: %w", e[0], err)
		}
	}
	return nil
}

/*
request posts the i-th feedback request for a customer and has others
vote for it, the oldest requests the most.
*/
func (r *run) request(ctx context.Context, i int, f [2]string, voters []ids.UUID) error {
	a := r.s.App
	post, err := a.Actions.Feedback.Create(ctx, feedback.PostInput{
		Membership: r.owner, HelpCenterID: r.center, Title: new(f[0]), Body: new(f[1]),
		Contact: &voters[(i*37)%len(voters)],
	})
	if err != nil {
		return fmt.Errorf("create feedback %q: %w", f[0], err)
	}
	votes := min(len(voters), 3+(len(FeedbackRequests())-i)*4)
	for v := range votes {
		if _, err := a.Actions.Feedback.VoteFor(ctx, feedback.VoteInput{
			Membership: r.owner, Post: post.ID, Contact: voters[(i*11+v*7)%len(voters)],
			Priority: []string{"nice", "important", "must"}[(i+v)%3],
		}); err != nil {
			return fmt.Errorf("vote on %q: %w", f[0], err)
		}
	}
	return nil
}
