/*
|--------------------------------------------------------------------------
| Impact Written Back
|--------------------------------------------------------------------------
|
| Engineering ranks bugs by who they hurt, so each linked issue carries
| how many customers, companies and conversations the ticket reaches,
| the revenue at stake and the largest companies by name. It is written
| when the issue is linked and again whenever links change, but only
| when the numbers moved: a fingerprint of the last impact written is
| kept on the link.
|
| Tools also need to deliver back. A GitHub repository gets a webhook
| the first time one of its issues is linked, and a Jira site gets the
| app's webhook once, refreshed before Jira lets it lapse.
|
*/

package issues

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
jiraRefreshEvery is how often the app's Jira webhooks are refreshed;
Jira drops them after thirty days.
*/
const jiraRefreshEvery = 7 * 24 * time.Hour

/*
ticketImpact reads the ticket the member may change and what it
reaches.
*/
func (s *Issues) ticketImpact(ctx context.Context, m authz.Membership, ref tickets.Ref,
) (models.Ticket, trackers.Impact, error) {
	var (
		t      models.Ticket
		impact trackers.Impact
	)

	err := s.Tickets.AsMember(ctx, m, func(ctx context.Context, w *tickets.Writer) error {
		var err error
		if t, err = w.Find(ctx, m, ref); err != nil {
			return err
		}

		impact, err = s.impactOf(ctx, w.Q, t)

		return err
	})

	return t, impact, err
}

/*
impactOf counts what the ticket reaches.
*/
func (s *Issues) impactOf(ctx context.Context, q *models.Queries, t models.Ticket) (trackers.Impact, error) {
	counts, err := q.CountTrackerImpact(ctx, models.CountTrackerImpactParams{
		WorkspaceID: t.WorkspaceID, TicketID: t.ID,
	})
	if err != nil {
		return trackers.Impact{}, fmt.Errorf("count impact: %w", err)
	}

	companies, err := q.TicketImpactCompanies(ctx, models.TicketImpactCompaniesParams{
		WorkspaceID: t.WorkspaceID, TicketID: t.ID,
	})
	if err != nil {
		return trackers.Impact{}, fmt.Errorf("impact companies: %w", err)
	}

	impact := trackers.Impact{
		Customers: counts.Customers, Companies: counts.Companies, Conversations: counts.Conversations,
		Revenue: counts.Revenue, Link: s.ticketLink(ctx, q, t),
	}

	for i, c := range companies {
		if i < 10 {
			impact.CompanyNames = append(impact.CompanyNames, c.Name)
		}
	}

	return impact, nil
}

/*
fingerprint is a short hash of the impact.
*/
func fingerprint(impact trackers.Impact) string {
	raw, _ := json.Marshal(impact)

	return hashing.SHA256Hex(raw)[:32]
}

/*
writeImpact writes the impact to the issue when it changed since last
written, best effort: a tool that refuses keeps the old numbers until
the next change.
*/
func (s *Issues) writeImpact(ctx context.Context, tl tool, link models.TicketIssue, impact trackers.Impact) {
	hash := fingerprint(impact)
	if hash == link.ImpactHash {
		return
	}

	is := trackers.Issue{ExternalID: link.ExternalID, Key: link.Key, URL: link.Url}
	if err := tl.tracker.Impact(ctx, tl.call, is, impact); err != nil {
		log.From(ctx).WarnContext(ctx, "write issue impact", log.String("provider", link.Provider), log.Err(err))

		return
	}

	_ = s.inWorkspace(ctx, link.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		return q.SetTicketIssueImpact(ctx, models.SetTicketIssueImpactParams{
			ImpactHash: hash, WorkspaceID: link.WorkspaceID, ID: link.ID,
		})
	})
}

/*
RefreshImpact writes the current impact to every issue linked to the
ticket, run when the ticket's links change.
*/
func (s *Issues) RefreshImpact(ctx context.Context, ws, ticketID ids.UUID) error {
	var (
		links  []models.TicketIssue
		impact trackers.Impact
	)

	err := s.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		t, err := q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: ws, ID: ticketID})
		if err != nil {
			return err
		}

		links, err = q.ListTicketIssues(ctx, models.ListTicketIssuesParams{WorkspaceID: ws, TicketID: ticketID})
		if err != nil {
			return err
		}

		impact, err = s.impactOf(ctx, q, t)

		return err
	})
	if err != nil {
		return fmt.Errorf("refresh impact: %w", err)
	}

	for _, link := range links {
		if link.InstallationID == nil {
			continue
		}

		if tl, err := s.toolFor(ctx, ws, *link.InstallationID); err == nil {
			s.writeImpact(ctx, tl, link, impact)
		}
	}

	return nil
}

/*
watch makes sure the tool delivers changes of the issue: a webhook on
a GitHub repository, the app's webhook on a Jira site. Linear's app
webhook and the key-connected tools' endpoints need nothing here.
*/
func (s *Issues) watch(ctx context.Context, tl tool, is trackers.Issue) {
	switch tracker := tl.tracker.(type) {
	case *trackers.GitHub:
		secret := s.Services.Trackers.GitHubWebhookSecret
		if secret == "" || is.TeamID == "" || tl.hasHook(is.TeamID) {
			return
		}

		hook := trackers.Hook{Repository: is.TeamID, URL: s.WebhookURL(tl.row), Secret: secret}
		if err := tracker.Subscribe(ctx, tl.call, hook); err == nil {
			s.remember(ctx, tl.row, map[string]any{"github_hook_" + is.TeamID: true})
		}
	case *trackers.Jira:
		s.watchJira(ctx, tl, tracker)
	}
}

/*
hasHook reports a GitHub repository already given the webhook.
*/
func (t tool) hasHook(repo string) bool {
	done, _ := t.call.Meta["github_hook_"+repo].(bool)

	return done
}

/*
watchJira registers the app's webhook on the site once and refreshes
it before Jira lets it lapse.
*/
func (s *Issues) watchJira(ctx context.Context, tl tool, jira *trackers.Jira) {
	id, _ := tl.call.Meta["jira_webhook_id"].(string)
	at, _ := tl.call.Meta["jira_webhook_at"].(string)
	stamp, _ := time.Parse(time.RFC3339, at)
	now := s.Clock.Now()

	switch {
	case id == "":
		made, err := jira.Subscribe(ctx, tl.call, s.WebhookURL(tl.row))
		if err == nil && made != "" {
			s.remember(ctx, tl.row, map[string]any{
				"jira_webhook_id": made, "jira_webhook_at": now.Format(time.RFC3339),
			})
		}
	case now.Sub(stamp) > jiraRefreshEvery:
		n, _ := strconv.ParseInt(id, 10, 64)
		if jira.Refresh(ctx, tl.call, []int64{n}) == nil {
			s.remember(ctx, tl.row, map[string]any{"jira_webhook_at": now.Format(time.RFC3339)})
		}
	}
}

/*
remember adds facts to the installation's metadata.
*/
func (s *Issues) remember(ctx context.Context, row models.IntegrationInstallation, facts map[string]any) {
	raw, err := json.Marshal(facts)
	if err != nil {
		return
	}

	_ = s.inWorkspace(tenancy.With(ctx, row.WorkspaceID.String()), row.WorkspaceID,
		func(ctx context.Context, q *models.Queries) error {
			_, err := q.MergeIntegrationMetadata(ctx, models.MergeIntegrationMetadataParams{
				Metadata: raw, Now: s.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
			})

			return err
		})
}

/*
detailsOf is the update storing what the tool says of the issue now.
*/
func detailsOf(link models.TicketIssue, is trackers.Issue, now time.Time) models.SyncTicketIssueDetailsParams {
	return models.SyncTicketIssueDetailsParams{
		Title: clip(first(is.Title, link.Title), 500), Status: clip(first(is.Status, link.Status), 100),
		StatusGroup: first(is.StatusGroup, link.StatusGroup), TeamName: clip(first(is.TeamName, link.TeamName), 200),
		TeamID: clip(first(is.TeamID, link.TeamID), 200), Severity: clip(first(is.Severity, link.Severity), 100),
		Now: now, WorkspaceID: link.WorkspaceID, ID: link.ID,
	}
}
