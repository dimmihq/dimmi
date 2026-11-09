/*
|--------------------------------------------------------------------------
| Collections
|--------------------------------------------------------------------------
|
| Collections group articles, three levels deep at most. A slug is unique
| in its help center, made from the name when none is given. Renaming a
| slug leaves a redirect from the old address. Deleting a collection
| keeps its articles: they simply become uncollected. One with
| sub-collections must be emptied first.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of managing collections.
*/
var (
	ErrCollectionNotFound = fault.New(fault.NotFound, "collection_not_found",
		"No collection of this help center has this id.")
	ErrCollectionDepth = fault.New(fault.Invalid, "collection_too_deep",
		"Collections go three levels deep: this move would nest one deeper.").OnField("parent_id")
	ErrCollectionHasChildren = fault.New(fault.Conflict, "collection_has_children",
		"Move or delete its sub-collections first.")
	ErrSlugTaken = fault.New(fault.Conflict, "slug_taken",
		"Another item of this help center already uses this slug.").OnField("slug")
	ErrSlugInvalid = fault.New(fault.Invalid, "slug_invalid",
		"Use lowercase letters, digits and single dashes.").OnField("slug")
)

/*
Collections manages a help center's collections.
*/
type Collections struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
CollectionInput is a collection to create or a change to one: nil
leaves a field as it is on change. A nil parent on create means top
level; on change ClearParent moves it to the top.
*/
type CollectionInput struct {
	Membership   authz.Membership
	HelpCenterID ids.UUID
	ID           ids.UUID
	ParentID     *ids.UUID
	ClearParent  bool
	Name         *string
	Slug         *string
	Description  *string
	Icon         *string
	Position     *int32
}

/*
List returns a help center's collections in order.
*/
func (c *Collections) List(ctx context.Context, m authz.Membership, helpCenter ids.UUID,
) ([]models.Collection, error) {
	if !m.Can("articles:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []models.Collection
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findHelpCenter(ctx, q, m.Workspace.ID, helpCenter); err != nil {
			return err
		}
		var err error
		out, err = q.ListCollections(ctx, models.ListCollectionsParams{
			WorkspaceID: m.Workspace.ID, HelpCenterID: helpCenter,
		})
		return err
	})
	return out, err
}

/*
Create makes a collection at the end of the help center.
*/
func (c *Collections) Create(ctx context.Context, in CollectionInput) (models.Collection, error) {
	if !in.Membership.Can("articles:write") {
		return models.Collection{}, exceptions.ErrForbidden
	}
	ws, now := in.Membership.Workspace.ID, c.Clock.Now()
	var out models.Collection
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findHelpCenter(ctx, q, ws, in.HelpCenterID); err != nil {
			return err
		}
		name := truncate(deref(in.Name), 80)
		if name == "" {
			return fault.New(fault.Invalid, "name_required", "Name the collection.").OnField("name")
		}
		slug, err := collectionSlug(ctx, q, in, slugFor{name: name})
		if err != nil {
			return err
		}
		if err := checkParent(ctx, q, models.Collection{WorkspaceID: ws}, in.ParentID); err != nil {
			return err
		}
		position, err := q.NextCollectionPosition(ctx, models.NextCollectionPositionParams{
			WorkspaceID: ws, HelpCenterID: in.HelpCenterID,
		})
		if err != nil {
			return fmt.Errorf("place collection: %w", err)
		}
		out, err = q.CreateCollection(ctx, models.CreateCollectionParams{
			ID: ids.NewV7At(now), WorkspaceID: ws, HelpCenterID: in.HelpCenterID, ParentID: in.ParentID,
			Name: name, Slug: slug, Description: optional(in.Description, 300), Icon: optional(in.Icon, 40),
			Position: position, Now: now,
		})
		if err != nil {
			return fmt.Errorf("create collection: %w", err)
		}
		return touch(ctx, q, place{workspace: ws, helpCenter: in.HelpCenterID}, now)
	})
	return out, err
}

/*
Update changes a collection, leaving a redirect when its slug changes.
*/
func (c *Collections) Update(ctx context.Context, in CollectionInput) (models.Collection, error) {
	if !in.Membership.Can("articles:write") {
		return models.Collection{}, exceptions.ErrForbidden
	}
	ws, now := in.Membership.Workspace.ID, c.Clock.Now()
	var out models.Collection
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := findCollection(ctx, q, ws, in.ID)
		if err != nil {
			return err
		}
		params, err := changedCollection(ctx, q, current, in)
		if err != nil {
			return err
		}
		params.Now = now
		if out, err = q.UpdateCollection(ctx, params); err != nil {
			return fmt.Errorf("update collection: %w", err)
		}
		if out.Slug != current.Slug {
			from, at := "/collections/"+current.Slug, place{workspace: ws, helpCenter: current.HelpCenterID}
			if err := redirect(ctx, q, at, move{from: from, to: target{collection: &out.ID}, now: now}); err != nil {
				return err
			}
		}
		return touch(ctx, q, place{workspace: ws, helpCenter: current.HelpCenterID}, now)
	})
	return out, err
}

/*
changedCollection checks the change and writes it over the collection.
*/
func changedCollection(ctx context.Context, q *models.Queries, c models.Collection, in CollectionInput,
) (models.UpdateCollectionParams, error) {
	p := models.UpdateCollectionParams{
		ParentID: c.ParentID, Name: c.Name, Slug: c.Slug, Description: c.Description, Icon: c.Icon,
		Position: c.Position, WorkspaceID: c.WorkspaceID, ID: c.ID,
	}
	if in.Name != nil {
		if p.Name = truncate(*in.Name, 80); p.Name == "" {
			return p, fault.New(fault.Invalid, "name_required", "Name the collection.").OnField("name")
		}
	}
	if in.Slug != nil {
		in.HelpCenterID = c.HelpCenterID
		slug, err := collectionSlug(ctx, q, in, slugFor{name: p.Name, self: c.ID})
		if err != nil {
			return p, err
		}
		p.Slug = slug
	}
	if in.ClearParent {
		p.ParentID = nil
	}
	if in.ParentID != nil {
		if err := checkParent(ctx, q, c, in.ParentID); err != nil {
			return p, err
		}
		p.ParentID = in.ParentID
	}
	if in.Description != nil {
		p.Description = optional(in.Description, 300)
	}
	if in.Icon != nil {
		p.Icon = optional(in.Icon, 40)
	}
	if in.Position != nil {
		p.Position = *in.Position
	}
	return p, nil
}

/*
Delete moves a collection to the trash, uncollecting its articles.
*/
func (c *Collections) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("articles:write") {
		return exceptions.ErrForbidden
	}
	ws, now := m.Workspace.ID, c.Clock.Now()
	return c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := findCollection(ctx, q, ws, id)
		if err != nil {
			return err
		}
		children, err := q.CountChildCollections(ctx, models.CountChildCollectionsParams{
			WorkspaceID: ws, ParentID: &id,
		})
		if err != nil {
			return fmt.Errorf("count sub-collections: %w", err)
		}
		if children > 0 {
			return ErrCollectionHasChildren
		}
		if err := q.UncollectArticles(ctx, models.UncollectArticlesParams{
			WorkspaceID: ws, CollectionID: &id, Now: now,
		}); err != nil {
			return fmt.Errorf("uncollect articles: %w", err)
		}
		deleted := models.DeleteCollectionParams{WorkspaceID: ws, ID: id, Now: now}
		if err := q.DeleteCollection(ctx, deleted); err != nil {
			return fmt.Errorf("delete collection: %w", err)
		}
		return touch(ctx, q, place{workspace: ws, helpCenter: current.HelpCenterID}, now)
	})
}

/*
slugFor is what a collection slug is checked against: the name a slug is
made from when none is asked for, and the collection that may already
hold it, empty for a new one.
*/
type slugFor struct {
	name string
	self ids.UUID
}

/*
collectionSlug returns the slug asked for, or one made from the name,
refusing a malformed or taken one.
*/
func collectionSlug(ctx context.Context, q *models.Queries, in CollectionInput, f slugFor) (string, error) {
	name, self := f.name, f.self
	slug := Slugify(name, 80)
	if in.Slug != nil && strings.TrimSpace(*in.Slug) != "" {
		slug = strings.ToLower(strings.TrimSpace(*in.Slug))
	}
	if slug == "" || len(slug) > 80 || !slugPattern().MatchString(slug) {
		return "", ErrSlugInvalid
	}
	found, err := q.FindCollectionBySlug(ctx, models.FindCollectionBySlugParams{
		WorkspaceID: in.Membership.Workspace.ID, HelpCenterID: in.HelpCenterID, Slug: slug,
	})
	switch {
	case errors.Is(err, database.ErrNotFound):
		return slug, nil
	case err != nil:
		return "", fmt.Errorf("check collection slug: %w", err)
	case found.ID != self:
		return "", ErrSlugTaken
	}
	return slug, nil
}

/*
findCollection returns a live collection of the workspace, or 404.
*/
func findCollection(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Collection, error) {
	out, err := q.FindCollection(ctx, models.FindCollectionParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return out, ErrCollectionNotFound
	}
	if err != nil {
		return out, fmt.Errorf("find collection: %w", err)
	}
	return out, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

/*
optional trims an optional text, nil when empty, cut at limit characters.
*/
func optional(s *string, limit int) *string {
	if s == nil {
		return nil
	}
	clean := truncate(*s, limit)
	if clean == "" {
		return nil
	}
	return &clean
}
