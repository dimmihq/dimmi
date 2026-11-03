/*
|--------------------------------------------------------------------------
| Repair Workspaces
|--------------------------------------------------------------------------
|
| An operator's way back to a consistent workspace after something went
| wrong underneath it, such as a default deleted by hand or a release
| that added a new default. Every initializer runs again and adds only
| what is missing, and the workspace's organization is brought back and
| renamed after it when needed. Running it twice changes nothing the
| second time.
|
*/

package workspaces

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Repaired is one workspace a repair went over, and whether its
organization needed bringing back or renaming.
*/
type Repaired struct {
	Workspace    models.Workspace
	Organization bool
}

/*
RepairWorkspaces repairs one workspace, by slug, or every workspace not
in the trash.

Rules:
  - Every initializer runs again, in its own transaction per workspace.
  - The organization of a live workspace is never left deleted, and
    carries the workspace's name while it has no other.

Errors: [exceptions.ErrWorkspaceNotFound] for an unknown slug, or a
wrapped error naming the workspace that failed.
*/
type RepairWorkspaces struct {
	DB           *database.DB
	Clock        clock.Clock
	Initializers *Initializers
}

/*
Handle repairs the workspace with the slug, or every live one when the
slug is empty.
*/
func (a *RepairWorkspaces) Handle(ctx context.Context, slug string) ([]Repaired, error) {
	targets, err := a.targets(ctx, slug)
	if err != nil {
		return nil, err
	}

	repaired := make([]Repaired, 0, len(targets))

	for _, w := range targets {
		organization, err := a.repair(ctx, w)
		if err != nil {
			return repaired, fmt.Errorf("repair workspace %s: %w", w.Slug, err)
		}

		repaired = append(repaired, Repaired{Workspace: w, Organization: organization})
	}

	return repaired, nil
}

/*
targets returns the workspaces to repair.
*/
func (a *RepairWorkspaces) targets(ctx context.Context, slug string) ([]models.Workspace, error) {
	q := models.New(a.DB)

	if slug == "" {
		all, err := q.ListWorkspaces(ctx)
		if err != nil {
			return nil, fmt.Errorf("list workspaces: %w", err)
		}

		return all, nil
	}

	w, err := q.FindWorkspaceBySlug(ctx, slug)
	if errors.Is(err, database.ErrNotFound) {
		return nil, exceptions.ErrWorkspaceNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find workspace %s: %w", slug, err)
	}

	return []models.Workspace{w}, nil
}

/*
repair brings the workspace's organization back in order and runs the
initializers, and reports whether the organization changed.
*/
func (a *RepairWorkspaces) repair(ctx context.Context, w models.Workspace) (bool, error) {
	var changed int64

	err := a.DB.Transaction(tenancy.With(ctx, w.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		var err error

		changed, err = models.New(tx).RepairOrganization(ctx, models.RepairOrganizationParams{
			WorkspaceID: w.ID, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("repair organization: %w", err)
		}

		return a.Initializers.Run(ctx, tx, w)
	})

	return changed > 0, err
}
