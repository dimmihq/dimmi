/*
|--------------------------------------------------------------------------
| Update Workspace
|--------------------------------------------------------------------------
|
| Admins rename a workspace, move it to a new address, and set the
| language and time zone members and customers fall back to. A renamed
| workspace renames its organization too while that organization holds
| no other workspace, so the two never drift apart.
|
| A new address keeps the old one held for thirty days: links to it keep
| reaching this workspace and nobody else may take it, while this
| workspace may take it back. Whether a workspace is a test workspace is
| decided when it is created and never changes.
|
*/

package workspaces

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UpdateWorkspaceInput is who changes the workspace, through their
membership of it, and the fields to change; nil leaves a field as it
is.
*/
type UpdateWorkspaceInput struct {
	Membership authz.Membership
	Name       *string
	Slug       *string
	Timezone   *string
	Locale     *string
	TestMode   *bool
}

/*
UpdateWorkspace changes the workspace's settings.

Rules:
  - The caller needs settings:write.
  - Test mode cannot change.
  - A new slug must not be reserved, used by another workspace, or held
    by one that gave it up within thirty days; the old slug is held for
    this workspace.
  - A rename renames the organization while it has no other workspace.
  - Values equal to the current ones change nothing.

Events: [appevents.WorkspaceUpdated] when anything changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrTestModeImmutable],
[exceptions.ErrSlugReserved], [exceptions.ErrSlugTaken], or a wrapped
database error.
*/
type UpdateWorkspace struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle applies the changes and returns the workspace as it now is.
*/
func (a *UpdateWorkspace) Handle(ctx context.Context, in UpdateWorkspaceInput) (models.Workspace, error) {
	if !in.Membership.Can("settings:write") {
		return models.Workspace{}, exceptions.ErrForbidden
	}

	var updated models.Workspace

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		current, err := q.LockWorkspace(ctx, in.Membership.Workspace.ID)
		if err != nil {
			return fmt.Errorf("lock workspace: %w", err)
		}

		if in.TestMode != nil && *in.TestMode != current.TestMode {
			return exceptions.ErrTestModeImmutable
		}

		params, changes, err := a.changes(ctx, q, current, in)
		if err != nil || len(changes) == 0 {
			updated = current

			return err
		}

		updated, err = q.UpdateWorkspace(ctx, params)
		if err != nil {
			return fmt.Errorf("update workspace: %w", err)
		}

		return a.finish(ctx, tx, updated, appevents.WorkspaceUpdated{
			WorkspaceID: updated.ID, UpdatedBy: in.Membership.Member.ID, Changes: changes,
		})
	})

	return updated, err
}

/*
changes returns the stored values after the update and the names of the
fields that change, sorted.
*/
func (a *UpdateWorkspace) changes(
	ctx context.Context, q *models.Queries, w models.Workspace, in UpdateWorkspaceInput,
) (models.UpdateWorkspaceParams, []string, error) {
	now := a.Clock.Now()
	params := models.UpdateWorkspaceParams{
		ID: w.ID, Name: w.Name, Slug: w.Slug, Timezone: w.Timezone, Locale: w.Locale, Settings: w.Settings, Now: now,
	}

	var changes []string

	change := func(field string, value, target *string) {
		if value != nil && *value != *target {
			*target = *value
			changes = append(changes, field)
		}
	}

	change("name", cleaned(in.Name), &params.Name)
	change("timezone", in.Timezone, &params.Timezone)
	change("locale", in.Locale, &params.Locale)

	if in.Slug != nil && *in.Slug != w.Slug {
		settings, err := moveSlug(ctx, q, w, models.CountWorkspacesWithSlugParams{
			WorkspaceID: w.ID, Slug: *in.Slug, Now: now,
		})
		if err != nil {
			return params, nil, err
		}

		params.Slug, params.Settings = *in.Slug, settings
		changes = append(changes, "slug")
	}

	slices.Sort(changes)

	return params, changes, nil
}

/*
finish renames the organization when the name changed and announces
the update.
*/
func (a *UpdateWorkspace) finish(
	ctx context.Context, tx *database.Tx, w models.Workspace, updated appevents.WorkspaceUpdated,
) error {
	if slices.Contains(updated.Changes, "name") {
		err := models.New(tx).RenameOrganization(ctx, models.RenameOrganizationParams{
			ID: w.OrganizationID, Name: w.Name, Now: w.UpdatedAt,
		})
		if err != nil {
			return fmt.Errorf("rename organization: %w", err)
		}
	}

	return a.Events.Dispatch(ctx, tx, updated)
}

/*
cleaned returns the name without invisible characters and outer
spaces, nil for none.
*/
func cleaned(name *string) *string {
	if name == nil {
		return nil
	}

	clean := str.Clean(*name)

	return &clean
}

/*
moveSlug checks that the workspace may take the slug the check names
and returns its settings with the old slug held for it.
*/
func moveSlug(
	ctx context.Context, q *models.Queries, w models.Workspace, check models.CountWorkspacesWithSlugParams,
) (json.RawMessage, error) {
	if reservedSlug(check.Slug) {
		return nil, exceptions.ErrSlugReserved
	}

	taken, err := slugTaken(ctx, q, check)
	if err != nil {
		return nil, err
	}

	if taken {
		return nil, takenBy(ctx, q, check.Slug)
	}

	return holdSlug(w.Settings, w.Slug, check.Slug, check.Now)
}

/*
takenBy returns why a taken slug cannot be had: another workspace uses
it, or one gave it up recently and still holds it.
*/
func takenBy(ctx context.Context, q *models.Queries, slug string) error {
	_, err := q.FindWorkspaceBySlug(ctx, slug)
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrSlugReserved
	}

	if err != nil {
		return fmt.Errorf("find workspace by slug: %w", err)
	}

	return exceptions.ErrSlugTaken
}
