/*
|--------------------------------------------------------------------------
| Operator Actions
|--------------------------------------------------------------------------
|
| The person running an install already holds every key, so they may vouch
| for a domain no public DNS can see, such as a company's internal one,
| and may switch a company's single sign-on off when its identity
| provider is down and no owner can get in. Both land in the audit log as
| the system. A domain proven this way is never checked again.
|
*/

package sso

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrDomainUnclear means more than one organization waits on the domain
and the operator did not say which.
*/
var ErrDomainUnclear = errors.New("sso: several workspaces added this domain; name one with --workspace")

/*
Operator holds the actions an install's operator runs from the command
line.

Errors: [exceptions.ErrNotFound], [exceptions.ErrDomainTaken],
ErrDomainUnclear, or a wrapped database error.
*/
type Operator struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
VerifyDomain marks the domain proven for the organization that added it,
the one behind the workspace with the slug when several did.

Events: [appevents.OrganizationDomainChanged] verified, by the system.
*/
func (a *Operator) VerifyDomain(ctx context.Context, domain, slug string) (models.OrganizationDomain, error) {
	name, err := claimable(domain)
	if err != nil {
		return models.OrganizationDomain{}, err
	}

	var out models.OrganizationDomain

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		row, err := pick(ctx, q, name, slug)
		if err != nil {
			return err
		}

		locked, err := lockDomain(ctx, q, row.OrganizationID, row.ID)
		if err != nil {
			return err
		}

		if out, err = markVerified(ctx, q, locked, proof{ViaCLI, a.Clock.Now()}); err != nil {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.OrganizationDomainChanged{
			WorkspaceID: row.WorkspaceID, DomainID: out.ID, Domain: out.Domain, Change: "verified",
			Changes: map[string]any{"verified_via": ViaCLI}, ChangedBy: ids.Nil(),
		})
	})

	return out, err
}

/*
pick returns the one waiting row of the domain the operator means.
*/
func pick(ctx context.Context, q *models.Queries, domain, slug string) (models.FindPendingDomainByNameRow, error) {
	rows, err := q.FindPendingDomainByName(ctx, domain)
	if err != nil {
		return models.FindPendingDomainByNameRow{}, fmt.Errorf("find domain: %w", err)
	}

	var matched []models.FindPendingDomainByNameRow

	for _, row := range rows {
		if slug == "" || row.WorkspaceSlug == slug {
			matched = append(matched, row)
		}
	}

	switch len(matched) {
	case 0:
		return models.FindPendingDomainByNameRow{}, exceptions.ErrNotFound
	case 1:
		return matched[0], nil
	default:
		return models.FindPendingDomainByNameRow{}, ErrDomainUnclear
	}
}

/*
Disable switches off every active connection of the organization behind
the workspace with the slug, and returns their names.

Events: [appevents.SSOConnectionChanged] updated, by the system, for
each.
*/
func (a *Operator) Disable(ctx context.Context, slug string) ([]string, error) {
	var names []string

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		w, err := q.FindWorkspaceBySlug(ctx, slug)
		if errors.Is(err, database.ErrNotFound) || (err == nil && w.DeletedAt != nil) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find workspace: %w", err)
		}

		disabled, err := q.DisableSSOConnections(ctx, models.DisableSSOConnectionsParams{
			OrganizationID: w.OrganizationID, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("disable connections: %w", err)
		}

		for _, row := range disabled {
			names = append(names, row.Name)

			if err := a.Events.Dispatch(ctx, tx, appevents.SSOConnectionChanged{
				WorkspaceID: w.ID, ConnectionID: row.ID, Name: row.Name, Change: "updated",
				Changes: map[string]any{"status": map[string]any{"to": StatusDisabled}}, ChangedBy: ids.Nil(),
			}); err != nil {
				return err
			}
		}

		return Mirror(ctx, q, w.OrganizationID, a.Clock.Now())
	})

	return names, err
}
