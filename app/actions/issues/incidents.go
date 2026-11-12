/*
|--------------------------------------------------------------------------
| Mirrored Incidents
|--------------------------------------------------------------------------
|
| When the workspace turned it on, a new incident in incident.io, Rootly
| or PagerDuty declares a dimmi incident at once, linked to it, so
| support is ready before the first chat arrives. Only incidents at or
| above the chosen severity are mirrored: the tool's own severity picked
| on its settings when one is, otherwise the workspace's level read
| from the severity's name. An incident already linked is only updated.
|
*/

package issues

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
mirror declares a dimmi incident for a new incident in the tool.
*/
func (s *Issues) mirror(ctx context.Context, row models.IntegrationInstallation, e trackers.Event) error {
	if e.Issue == nil {
		return nil
	}

	ws, is := row.WorkspaceID, *e.Issue

	var (
		settings TicketSettings
		existing []models.TicketIssue
	)

	err := s.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		var err error
		if settings, _, err = readTicketSettings(ctx, q, ws); err != nil {
			return err
		}

		existing, err = q.TicketIssuesOfExternal(ctx, models.TicketIssuesOfExternalParams{
			WorkspaceID: ws, Provider: row.Provider, ExternalID: is.ExternalID,
		})

		return err
	})
	if err != nil {
		return err
	}

	for _, l := range existing {
		if err := s.applyIssue(ctx, ws, l.ID, is); err != nil {
			return err
		}
	}

	if len(existing) > 0 || !settings.IncidentMirror || !s.severeEnough(ctx, row, is, settings.IncidentMirrorSeverity) {
		return nil
	}

	return s.declare(ctx, row, is)
}

/*
declare makes the dimmi incident and links the tool's incident to it.
*/
func (s *Issues) declare(ctx context.Context, row models.IntegrationInstallation, is trackers.Issue) error {
	ws := row.WorkspaceID

	return s.Tickets.AsTool(ctx, ws, Label(row.Provider), func(ctx context.Context, w *tickets.Writer) error {
		t, err := w.Declare(ctx, ws, tickets.DeclareInput{
			Title:       clip(first(is.Title, "Incident in "+Label(row.Provider)), 200),
			Description: fmt.Sprintf("Opened in %s: %s", Label(row.Provider), is.URL),
		})
		if err != nil {
			return err
		}

		installation := row.ID

		link, err := w.Q.CreateTicketIssue(ctx, models.CreateTicketIssueParams{
			WorkspaceID: ws, TicketID: t.ID, InstallationID: &installation, Provider: row.Provider,
			ExternalID: is.ExternalID, Key: first(is.Key, is.ExternalID), Url: first(is.URL, "about:blank"),
			Title: clip(is.Title, 500), Status: clip(is.Status, 100), StatusGroup: first(is.StatusGroup, "unstarted"),
			TeamName: clip(is.TeamName, 200), Now: &w.Now,
		})
		if err != nil {
			return err
		}

		if link, err = w.Q.SyncTicketIssueDetails(ctx, detailsOf(link, is, w.Now)); err != nil {
			return err
		}

		return w.AnnounceIssue(ctx, t, tickets.IssueChange{Issue: link, Linked: true, Data: map[string]any{
			"mirrored": true,
		}})
	})
}

/*
severeEnough reports whether the incident reaches the threshold: the
rank of the tool's severity picked on its settings, or the workspace's
level otherwise.
*/
func (s *Issues) severeEnough(ctx context.Context, row models.IntegrationInstallation, is trackers.Issue,
	level string,
) bool {
	if picked := settingsOf(row.Metadata).MirrorSeverity; picked != "" {
		tl, err := s.toolFor(ctx, row.WorkspaceID, row.ID)
		if err != nil {
			return false
		}

		options, err := tl.tracker.Options(ctx, tl.call, trackers.ListSeverities, "")
		if err != nil {
			return false
		}

		for _, o := range options {
			if o.ID == picked {
				return is.SeverityID == picked || is.Rank >= o.Rank
			}
		}
	}

	return levelOf(is.Severity) >= levelOf(level)
}

/*
levelOf reads a level from a severity's name: critical, major or
minor, with all below them.
*/
func levelOf(name string) int {
	lower := strings.ToLower(name)

	switch {
	case lower == "all" || lower == "":
		return 0
	case strings.Contains(lower, "critical") || strings.Contains(lower, "sev1") || strings.Contains(lower, "sev 1") ||
		strings.Contains(lower, "p1"):
		return 3
	case strings.Contains(lower, "major") || strings.Contains(lower, "high") || strings.Contains(lower, "sev2") ||
		strings.Contains(lower, "sev 2") || strings.Contains(lower, "p2"):
		return 2
	}

	return 1
}
