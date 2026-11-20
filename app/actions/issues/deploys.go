/*
|--------------------------------------------------------------------------
| Deployments
|--------------------------------------------------------------------------
|
| A successful deployment of the watched repository and environment
| marks every issue done before it as shipped, and tells each ticket, so
| a workflow closing the loop can wait for the fix to reach production.
|
*/

package issues

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
deployed records a successful deployment of the watched repository and
environment: every issue done before it now counts as shipped, and its
ticket hears ticket.issue_updated with deployed, so a workflow closing
the loop can wait for the fix to reach production.
*/
func (s *Issues) deployed(ctx context.Context, row models.IntegrationInstallation, e trackers.Event) error {
	set := settingsOf(row.Metadata)
	if !e.Succeeded || set.DeployRepository == "" || !strings.EqualFold(e.Repository, set.DeployRepository) ||
		(set.DeployEnvironment != "" && !strings.EqualFold(e.Environment, set.DeployEnvironment)) {
		return nil
	}

	ws := row.WorkspaceID

	return s.Tickets.AsTool(ctx, ws, "GitHub", func(ctx context.Context, w *tickets.Writer) error {
		shipped, err := w.Q.TicketIssuesAwaitingDeploy(ctx, models.TicketIssuesAwaitingDeployParams{
			WorkspaceID: ws, Before: w.Now,
		})
		if err != nil {
			return err
		}

		for _, is := range shipped {
			if err := ship(ctx, w, is, e.Environment); err != nil {
				return err
			}
		}

		return nil
	})
}

/*
ship marks one issue's fix deployed and tells its ticket.
*/
func ship(ctx context.Context, w *tickets.Writer, is models.TicketIssue, environment string) error {
	err := w.Q.MarkTicketIssueDeployed(ctx, models.MarkTicketIssueDeployedParams{
		Now: w.Now, WorkspaceID: is.WorkspaceID, ID: is.ID,
	})
	if err != nil {
		return err
	}

	t, err := w.Lock(ctx, is.WorkspaceID, is.TicketID)
	if err != nil || t.DeletedAt != nil {
		return nil //nolint:nilerr // a ticket gone hears nothing
	}

	is.DeployedAt = &w.Now
	data := map[string]any{"deployed": true, "environment": environment, "from_group": is.StatusGroup}

	return w.AnnounceIssue(ctx, t, tickets.IssueChange{Issue: is, Data: data})
}
