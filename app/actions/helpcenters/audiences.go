/*
|--------------------------------------------------------------------------
| Article Audiences
|--------------------------------------------------------------------------
|
| An article or a whole collection may be kept to one saved audience.
| Reading reveals the team's content, so the audience may only read what
| the team's own systems said: no page conditions, no value a browser
| could claim, and custom attributes only as the team's systems set them.
| A change moves the help center's cache on, so readers see it at once.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrAudienceUnfit refuses an audience that cannot decide who may read.
*/
var ErrAudienceUnfit = fault.New(fault.Invalid, "audience_unfit",
	"This audience cannot decide who may read.").OnField("audience_id")

/*
SetArticleAudience keeps an article to a saved audience, or opens it to
everyone with nil.
*/
func (a *Articles) SetArticleAudience(ctx context.Context, m authz.Membership, id ids.UUID, audience *ids.UUID,
) (models.Article, error) {
	if !m.Can("articles:write") {
		return models.Article{}, exceptions.ErrForbidden
	}

	var out models.Article
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws, now := models.New(tx), m.Workspace.ID, a.Clock.Now()
		if err := fitToRead(ctx, q, ws, audience); err != nil {
			return err
		}

		var err error
		out, err = q.SetArticleAudience(ctx, models.SetArticleAudienceParams{
			AudienceID: audience, Now: now, WorkspaceID: ws, ID: id,
		})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("set article audience: %w", err)
		}

		return wrapTouch(q.TouchHelpCenterCache(ctx, models.TouchHelpCenterCacheParams{
			Now: now, WorkspaceID: ws, ID: out.HelpCenterID,
		}))
	})

	return out, err
}

/*
SetCollectionAudience keeps a collection and everything in it to a saved
audience, or opens it to everyone with nil.
*/
func (c *Collections) SetCollectionAudience(ctx context.Context, m authz.Membership, id ids.UUID, audience *ids.UUID,
) (models.Collection, error) {
	if !m.Can("articles:write") {
		return models.Collection{}, exceptions.ErrForbidden
	}

	var out models.Collection
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws, now := models.New(tx), m.Workspace.ID, c.Clock.Now()
		if err := fitToRead(ctx, q, ws, audience); err != nil {
			return err
		}

		var err error
		out, err = q.SetCollectionAudience(ctx, models.SetCollectionAudienceParams{
			AudienceID: audience, Now: now, WorkspaceID: ws, ID: id,
		})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("set collection audience: %w", err)
		}

		return wrapTouch(q.TouchHelpCenterCache(ctx, models.TouchHelpCenterCacheParams{
			Now: now, WorkspaceID: ws, ID: out.HelpCenterID,
		}))
	})

	return out, err
}

/*
fitToRead refuses an audience that does not exist or reads what a
browser could claim.
*/
func fitToRead(ctx context.Context, q *models.Queries, ws ids.UUID, audience *ids.UUID) error {
	if audience == nil {
		return nil
	}

	resolver, err := audiences.LoadResolver(ctx, q, ws)
	if err != nil {
		return err
	}

	if problems := resolver.CheckTrusted(models.PublicID(models.PrefixAudience, *audience)); len(problems) > 0 {
		return ErrAudienceUnfit.With("errors", problems)
	}

	return nil
}

func wrapTouch(err error) error {
	if err != nil {
		return fmt.Errorf("touch help center cache: %w", err)
	}

	return nil
}

/*
readable keeps the articles and collections the reader may see.
*/
func readable(ctx context.Context, q *models.Queries, s Snapshot, reader audiences.Reader) (Snapshot, error) {
	r, err := audiences.LoadRestrictions(ctx, q, s.HelpCenter.WorkspaceID)
	if err != nil {
		return s, err
	}

	needed := r.Needed()
	if len(needed) == 0 {
		return s, nil
	}

	reader.Needed = needed
	allowed, err := audiences.Allowed(ctx, q, reader)
	if err != nil {
		return s, err
	}

	out := Snapshot{HelpCenter: s.HelpCenter}
	for _, c := range s.Collections {
		if r.Collection(c.ID, allowed) {
			out.Collections = append(out.Collections, c)
		}
	}

	for _, a := range s.Articles {
		if r.Article(a.ID, allowed) {
			out.Articles = append(out.Articles, a)
		}
	}

	return out, nil
}
