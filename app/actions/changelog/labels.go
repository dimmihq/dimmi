/*
|--------------------------------------------------------------------------
| Changelog Labels
|--------------------------------------------------------------------------
|
| Labels file entries under kinds of change and give readers a filter.
| A changelog starts with New, Improved and Fixed the first time anyone
| looks, holds at most twenty, and keeps a label's entries when the
| label goes.
|
*/

package changelog

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
colorPattern is a label's color, as hex.
*/
var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

/*
LabelInput is a label to create or a change to one; nil leaves a field
as it is.
*/
type LabelInput struct {
	Membership   authz.Membership
	HelpCenterID ids.UUID
	ID           ids.UUID
	Name         *string
	Color        *string
	Position     *int32
}

/*
Labels returns a changelog's labels, giving a changelog that has none
and no entries yet the default ones.
*/
func (e *Entries) Labels(ctx context.Context, m authz.Membership, hc ids.UUID) ([]models.ChangelogLabel, error) {
	if !m.Can("changelog:read") {
		return nil, exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	var out []models.ChangelogLabel
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := q.FindHelpCenter(ctx, models.FindHelpCenterParams{WorkspaceID: ws, ID: hc}); err != nil {
			return notFound(err, helpcenters.ErrHelpCenterNotFound)
		}
		var err error
		if out, err = q.ListChangelogLabels(ctx, models.ListChangelogLabelsParams{
			WorkspaceID: ws, HelpCenterID: hc,
		}); err != nil {
			return fmt.Errorf("list changelog labels: %w", err)
		}
		if len(out) > 0 || !m.Can("changelog:write") {
			return nil
		}
		out, err = e.seedLabels(ctx, q, ws, hc)
		return err
	})
	return out, err
}

/*
seedLabels gives a changelog its default labels when it has no entries
yet, so a team that cleared its labels on purpose keeps none.
*/
func (e *Entries) seedLabels(ctx context.Context, q *models.Queries, ws, hc ids.UUID,
) ([]models.ChangelogLabel, error) {
	counts, err := countByStatus(ctx, q, ws, hc)
	if err != nil {
		return nil, err
	}
	if counts["draft"]+counts["scheduled"]+counts["published"] > 0 {
		return nil, nil
	}
	out := make([]models.ChangelogLabel, 0, 3)
	for _, d := range defaultLabels() {
		label, err := q.CreateChangelogLabel(ctx, models.CreateChangelogLabelParams{
			WorkspaceID: ws, HelpCenterID: hc, Name: d.name, Slug: helpcenters.Slugify(d.name, 60), Color: d.color,
			Now: e.Clock.Now(),
		})
		if err != nil {
			return nil, fmt.Errorf("create default changelog label: %w", err)
		}
		out = append(out, label)
	}
	return out, nil
}

/*
CreateLabel adds a label at the end of the changelog's list.
*/
func (e *Entries) CreateLabel(ctx context.Context, in LabelInput) (models.ChangelogLabel, error) {
	if !in.Membership.Can("changelog:write") {
		return models.ChangelogLabel{}, exceptions.ErrForbidden
	}
	ws := in.Membership.Workspace.ID
	name, color := strings.TrimSpace(deref(in.Name)), strings.TrimSpace(deref(in.Color))
	if color == "" {
		color = "#64748b"
	}
	var out models.ChangelogLabel
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := q.FindHelpCenter(ctx, models.FindHelpCenterParams{
			WorkspaceID: ws, ID: in.HelpCenterID,
		}); err != nil {
			return notFound(err, helpcenters.ErrHelpCenterNotFound)
		}
		count, err := q.CountChangelogLabels(ctx, models.CountChangelogLabelsParams{
			WorkspaceID: ws, HelpCenterID: in.HelpCenterID,
		})
		if err != nil {
			return fmt.Errorf("count changelog labels: %w", err)
		}
		if count >= maxLabels {
			return ErrTooManyLabels
		}
		slug, err := labelSlug(ctx, q, labelAt{ws: ws, hc: in.HelpCenterID, name: name, color: color})
		if err != nil {
			return err
		}
		out, err = q.CreateChangelogLabel(ctx, models.CreateChangelogLabelParams{
			WorkspaceID: ws, HelpCenterID: in.HelpCenterID, Name: name, Slug: slug, Color: color, Now: e.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("create changelog label: %w", err)
		}
		return nil
	})
	return out, err
}

/*
UpdateLabel renames, recolors or moves a label.
*/
func (e *Entries) UpdateLabel(ctx context.Context, in LabelInput) (models.ChangelogLabel, error) {
	if !in.Membership.Can("changelog:write") {
		return models.ChangelogLabel{}, exceptions.ErrForbidden
	}
	ws, now := in.Membership.Workspace.ID, e.Clock.Now()
	var out models.ChangelogLabel
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := q.FindChangelogLabel(ctx, models.FindChangelogLabelParams{WorkspaceID: ws, ID: in.ID})
		if err != nil {
			return notFound(err, ErrLabelNotFound)
		}
		name, color, position := current.Name, current.Color, current.Position
		if in.Name != nil {
			name = strings.TrimSpace(*in.Name)
		}
		if in.Color != nil {
			color = strings.TrimSpace(*in.Color)
		}
		if in.Position != nil {
			position = *in.Position
		}
		at := labelAt{ws: ws, hc: current.HelpCenterID, self: current.ID, name: name, color: color}
		slug, err := labelSlug(ctx, q, at)
		if err != nil {
			return err
		}
		out, err = q.UpdateChangelogLabel(ctx, models.UpdateChangelogLabelParams{
			Name: name, Slug: slug, Color: color, Position: position, Now: now, WorkspaceID: ws, ID: current.ID,
		})
		if err != nil {
			return fmt.Errorf("update changelog label: %w", err)
		}
		return q.TouchHelpCenterCache(ctx, models.TouchHelpCenterCacheParams{
			WorkspaceID: ws, ID: current.HelpCenterID, Now: now,
		})
	})
	return out, err
}

/*
DeleteLabel deletes a label; its entries stay.
*/
func (e *Entries) DeleteLabel(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("changelog:write") {
		return exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	return e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := q.FindChangelogLabel(ctx, models.FindChangelogLabelParams{WorkspaceID: ws, ID: id})
		if err != nil {
			return notFound(err, ErrLabelNotFound)
		}
		if err := q.DeleteChangelogLabel(ctx, models.DeleteChangelogLabelParams{WorkspaceID: ws, ID: id}); err != nil {
			return fmt.Errorf("delete changelog label: %w", err)
		}
		return q.TouchHelpCenterCache(ctx, models.TouchHelpCenterCacheParams{
			WorkspaceID: ws, ID: current.HelpCenterID, Now: e.Clock.Now(),
		})
	})
}

/*
labelAt is a label's name and color and where the name must be free.
*/
type labelAt struct {
	ws, hc, self ids.UUID
	name, color  string
}

/*
labelSlug checks a label's name and color and returns its free slug.
*/
func labelSlug(ctx context.Context, q *models.Queries, at labelAt) (string, error) {
	slug := helpcenters.Slugify(at.name, 60)
	if at.name == "" || len([]rune(at.name)) > 40 || slug == "" || !colorPattern.MatchString(at.color) {
		return "", ErrLabelInvalid
	}
	taken, err := q.ChangelogLabelSlugTaken(ctx, models.ChangelogLabelSlugTakenParams{
		WorkspaceID: at.ws, HelpCenterID: at.hc, Slug: slug, ID: at.self,
	})
	if err != nil {
		return "", fmt.Errorf("check changelog label: %w", err)
	}
	if taken {
		return "", ErrLabelTaken
	}
	return slug, nil
}
