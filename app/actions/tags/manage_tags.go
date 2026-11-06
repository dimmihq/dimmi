/*
|--------------------------------------------------------------------------
| Manage Tags
|--------------------------------------------------------------------------
|
| Anyone who works conversations may add to the vocabulary, since tagging
| should never wait on an admin. Renaming, recoloring and deleting change
| the meaning of every record carrying the tag, so they take tags:manage.
| A tag's color is the workspace's own data, a hex value drawn as a dot
| beside text that stays on the theme.
|
*/

package tags

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
nameConstraint keeps a workspace's tag names apart, without regard to
case.
*/
const nameConstraint = "tags_workspace_id_name_key"

/*
Payload shapes a tag for an event's broadcast, which the resources
package fills in so events stay free of API shapes.
*/
type Payload func(models.Tag) any

/*
CreateTagInput is who adds which tag.
*/
type CreateTagInput struct {
	Membership authz.Membership
	Name       string
	Color      *string
	Icon       *string
}

/*
CreateTag adds a tag to the vocabulary.

Rules:
  - The caller needs conversations:write.
  - A name is unique without regard to case.
  - A workspace holds at most 10,000 tags.

Events: [appevents.TagCreated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrTagExists],
[exceptions.ErrTagNameRequired], [exceptions.ErrTagIconUnknown],
[exceptions.ErrTagLimitReached], or a wrapped database error.
*/
type CreateTag struct {
	DB      *database.DB
	Events  *events.Dispatcher
	Clock   clock.Clock
	Payload Payload
}

/*
Handle adds the tag and returns it.
*/
func (a *CreateTag) Handle(ctx context.Context, in CreateTagInput) (models.Tag, error) {
	if !in.Membership.Can("conversations:write") {
		return models.Tag{}, exceptions.ErrForbidden
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		return models.Tag{}, exceptions.ErrTagNameRequired
	}

	if !knownIcon(in.Icon) {
		return models.Tag{}, exceptions.ErrTagIconUnknown
	}

	var out models.Tag

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws, now := models.New(tx), in.Membership.Workspace.ID, a.Clock.Now()

		count, err := q.CountWorkspaceTags(ctx, ws)
		if err != nil {
			return fmt.Errorf("count tags: %w", err)
		}

		if count >= MaxInWorkspace {
			return exceptions.ErrTagLimitReached
		}

		out, err = q.CreateTag(ctx, models.CreateTagParams{
			WorkspaceID: ws, Name: name, Color: in.Color, Icon: in.Icon, Now: now,
		})
		if database.IsUniqueViolation(err, nameConstraint) {
			return exceptions.ErrTagExists
		}

		if err != nil {
			return fmt.Errorf("create tag: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.TagCreated{
			Base: base(ws, in.Membership, now), TagID: out.ID, Data: a.Payload(out),
		})
	})

	return out, err
}

/*
UpdateTagInput is who changes which tag and how: Name nil keeps the
name, Color nil keeps the color unless ClearColor, and Icon nil keeps
the icon unless ClearIcon.
*/
type UpdateTagInput struct {
	Membership authz.Membership
	TagID      ids.UUID
	Name       *string
	Color      *string
	ClearColor bool
	Icon       *string
	ClearIcon  bool
}

/*
UpdateTag renames a tag or changes its color or icon.

Rules:
  - The caller needs tags:manage.
  - Values equal to the current ones change nothing.

Events: [appevents.TagUpdated] when anything changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrTagExists], [exceptions.ErrTagNameRequired],
[exceptions.ErrTagIconUnknown], or a wrapped database error.
*/
type UpdateTag struct {
	DB      *database.DB
	Events  *events.Dispatcher
	Clock   clock.Clock
	Payload Payload
}

/*
Handle changes the tag and returns it as it now is.
*/
func (a *UpdateTag) Handle(ctx context.Context, in UpdateTagInput) (models.Tag, error) {
	if !in.Membership.Can("tags:manage") {
		return models.Tag{}, exceptions.ErrForbidden
	}

	var out models.Tag

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws, now := models.New(tx), in.Membership.Workspace.ID, a.Clock.Now()

		current, err := find(ctx, q, ws, in.TagID)
		if err != nil {
			return err
		}

		params, changes, err := nextValues(current, in)
		if err != nil {
			return err
		}

		out = current
		if len(changes) == 0 {
			return nil
		}

		params.Now = now

		out, err = q.UpdateTag(ctx, params)
		if database.IsUniqueViolation(err, nameConstraint) {
			return exceptions.ErrTagExists
		}

		if err != nil {
			return fmt.Errorf("update tag: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.TagUpdated{
			Base: base(ws, in.Membership, now), TagID: out.ID, Changes: changes, Data: a.Payload(out),
		})
	})

	return out, err
}

/*
nextValues works out what the tag becomes and the fields that change.
*/
func nextValues(current models.Tag, in UpdateTagInput) (models.UpdateTagParams, []string, error) {
	params := models.UpdateTagParams{
		Name: current.Name, Color: current.Color, Icon: current.Icon, WorkspaceID: current.WorkspaceID, ID: current.ID,
	}

	var changes []string

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return params, nil, exceptions.ErrTagNameRequired
		}

		if name != current.Name {
			params.Name, changes = name, append(changes, "name")
		}
	}

	next := current.Color
	if in.Color != nil {
		next = in.Color
	}

	if in.ClearColor {
		next = nil
	}

	if !sameColor(current.Color, next) {
		params.Color, changes = next, append(changes, "color")
	}

	icon, err := nextIcon(current.Icon, in)
	if err != nil {
		return params, nil, err
	}

	if !sameColor(current.Icon, icon) {
		params.Icon, changes = icon, append(changes, "icon")
	}

	return params, changes, nil
}

/*
nextIcon is the icon the tag carries after the update.
*/
func nextIcon(current *string, in UpdateTagInput) (*string, error) {
	if in.ClearIcon {
		return nil, nil
	}

	if in.Icon == nil {
		return current, nil
	}

	if !knownIcon(in.Icon) {
		return nil, exceptions.ErrTagIconUnknown
	}

	return in.Icon, nil
}

/*
sameColor reports whether two optional colors are equal.
*/
func sameColor(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return strings.EqualFold(*a, *b)
}

/*
find returns the tag, or exceptions.ErrNotFound.
*/
func find(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Tag, error) {
	tag, err := q.FindTag(ctx, models.FindTagParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return tag, exceptions.ErrNotFound
	}

	if err != nil {
		return tag, fmt.Errorf("find tag: %w", err)
	}

	return tag, nil
}
