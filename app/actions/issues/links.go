/*
|--------------------------------------------------------------------------
| Linking Issues
|--------------------------------------------------------------------------
|
| From any ticket a teammate creates an issue in a tool, in the team and
| project picked from the tool's own lists, or links one by pasting its
| URL or key or picking it from a search. A new issue carries the
| customer impact and a link back; incident tools get an incident of the
| picked severity, PagerDuty on the picked service. Unlinking leaves the
| issue in the tool and, when the teammate chooses, takes back what dimmi
| wrote on it for the ticket. An issue linked to more tickets gets their
| impact written again; an incident keeps its impact for them.
|
*/

package issues

import (
	"context"
	"errors"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
LinkInput creates an issue or links one. With Ref (a URL or key) it
links; otherwise it creates in the team and project with the title.
*/
type LinkInput struct {
	InstallationID *ids.UUID
	Team           string
	Project        string
	Service        string
	Severity       string
	Title          string
	Ref            string
}

/*
List returns the issues linked to a ticket the member may see.
*/
func (s *Issues) List(ctx context.Context, m authz.Membership, ref tickets.Ref) ([]models.TicketIssue, error) {
	if err := tickets.MayRead(m); err != nil {
		return nil, err
	}

	var out []models.TicketIssue

	err := s.Tickets.AsMember(ctx, m, func(ctx context.Context, w *tickets.Writer) error {
		t, err := w.Find(ctx, m, ref)
		if err != nil {
			return err
		}

		out, err = w.Q.ListTicketIssues(ctx, models.ListTicketIssuesParams{WorkspaceID: t.WorkspaceID, TicketID: t.ID})

		return err
	})

	return out, err
}

/*
Store creates or links an issue on the ticket.
*/
func (s *Issues) Store(ctx context.Context, m authz.Membership, ref tickets.Ref, in LinkInput,
) (models.TicketIssue, error) {
	if err := tickets.MayWrite(m); err != nil {
		return models.TicketIssue{}, err
	}

	t, impact, err := s.ticketImpact(ctx, m, ref)
	if err != nil {
		return models.TicketIssue{}, err
	}

	tl, err := s.pick(ctx, m, in)
	if err != nil {
		return models.TicketIssue{}, err
	}

	var is trackers.Issue

	if strings.TrimSpace(in.Ref) != "" {
		is, err = tl.tracker.Read(ctx, tl.call, in.Ref)
	} else {
		is, err = s.create(ctx, tl, creating{ticket: t, in: in, impact: impact})
	}

	if err != nil {
		return models.TicketIssue{}, toolError(err)
	}

	link, err := s.save(ctx, m, found{ticket: t, tool: tl, issue: is})
	if err != nil {
		return link, err
	}

	s.watch(ctx, tl, is)
	s.writeImpact(ctx, tl, link, impact)

	return link, nil
}

/*
creating is an issue to create for the ticket with its impact.
*/
type creating struct {
	ticket models.Ticket
	in     LinkInput
	impact trackers.Impact
}

/*
create files the issue or incident, with the defaults the tool's
settings picked for what the teammate left out.
*/
func (*Issues) create(ctx context.Context, tl tool, c creating) (trackers.Issue, error) {
	t, in, impact := c.ticket, c.in, c.impact
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		in.Title = t.Title
	}

	set := tl.settings
	team, project, service, severity := first(in.Team, set.Team), first(in.Project, set.Project),
		first(in.Service, set.Service), first(in.Severity, set.Severity)

	if !tl.tracker.Incidents() && team == "" || tl.row.Provider == "pagerduty" && service == "" {
		return trackers.Issue{}, ErrCreateInvalid
	}

	return tl.tracker.Create(ctx, tl.call, trackers.CreateInput{
		Team: team, Project: project, Service: service, Severity: severity, Title: in.Title,
		Description: t.Description, Impact: impact,
	})
}

/*
first returns the first value that is not empty.
*/
func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}

	return ""
}

/*
pick finds the tool the input names: the installation given, or the
workspace's installation of the tool a pasted URL belongs to.
*/
func (s *Issues) pick(ctx context.Context, m authz.Membership, in LinkInput) (tool, error) {
	if in.InstallationID != nil {
		return s.toolFor(ctx, m.Workspace.ID, *in.InstallationID)
	}

	provider := providerOf(in.Ref)

	var found *ids.UUID

	err := s.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		rows, err := q.ListIntegrationInstallations(ctx, models.ListIntegrationInstallationsParams{
			WorkspaceID: m.Workspace.ID,
		})

		for _, r := range rows {
			_, isTool := s.Trackers.Get(r.Provider)
			if isTool && r.Status == "active" && (provider == "" || r.Provider == provider) && found == nil {
				id := r.ID
				found = &id
			}
		}

		return err
	})
	if err != nil || found == nil {
		return tool{}, ErrInstallation
	}

	return s.toolFor(ctx, m.Workspace.ID, *found)
}

/*
found is an issue the tool returned for the ticket.
*/
type found struct {
	ticket models.Ticket
	tool   tool
	issue  trackers.Issue
}

/*
save stores the link, announces it on the ticket and moves the ticket
to the state the issue's status maps to, so linking an issue already
done closes the loop as its finishing later would.
*/
func (s *Issues) save(ctx context.Context, m authz.Membership, f found) (models.TicketIssue, error) {
	t, tl, is := f.ticket, f.tool, f.issue

	var out models.TicketIssue

	err := s.Tickets.AsMember(ctx, m, func(ctx context.Context, w *tickets.Writer) error {
		locked, err := w.Lock(ctx, t.WorkspaceID, t.ID)
		if err != nil {
			return err
		}

		member := m.Member.ID
		installation := tl.row.ID

		out, err = w.Q.CreateTicketIssue(ctx, models.CreateTicketIssueParams{
			WorkspaceID: t.WorkspaceID, TicketID: t.ID, InstallationID: &installation, Provider: tl.row.Provider,
			ExternalID: is.ExternalID, Key: first(is.Key, is.ExternalID), Url: first(is.URL, "about:blank"),
			Title: clip(is.Title, 500), Status: clip(is.Status, 100), StatusGroup: first(is.StatusGroup, "unstarted"),
			TeamName: clip(is.TeamName, 200), Now: &w.Now, CreatedByMemberID: &member,
		})
		if err != nil {
			return err
		}

		out, err = w.Q.SyncTicketIssueDetails(ctx, detailsOf(out, is, w.Now))
		if err != nil {
			return err
		}

		if err := w.AnnounceIssue(ctx, locked, tickets.IssueChange{Issue: out, Linked: true}); err != nil {
			return err
		}

		return s.follow(ctx, w, locked, statusUpdate{link: out, statusID: is.StatusID})
	})

	return out, err
}

/*
RemoveOptions say what unlinking does in the tool. FromTool takes back
what dimmi wrote on the issue for the ticket; otherwise the tool keeps
it, and only an issue other tickets still link gets their impact
written again.
*/
type RemoveOptions struct {
	FromTool bool
}

/*
Remove unlinks an issue. The issue itself stays in the tool; what dimmi
wrote on it for this ticket, such as Linear's impact attachment, is
taken back, best effort, only when the options ask.
*/
func (s *Issues) Remove(ctx context.Context, m authz.Membership, id ids.UUID, opts RemoveOptions) error {
	if err := tickets.MayWrite(m); err != nil {
		return err
	}

	var (
		gone models.TicketIssue
		back string
	)

	err := s.Tickets.AsMember(ctx, m, func(ctx context.Context, w *tickets.Writer) error {
		link, err := w.Q.FindTicketIssue(ctx, models.FindTicketIssueParams{WorkspaceID: m.Workspace.ID, ID: id})
		if errors.Is(err, database.ErrNotFound) {
			return ErrIssueNotFound
		}

		if err != nil {
			return err
		}

		t, err := w.Find(ctx, m, tickets.Ref{ID: link.TicketID})
		if err != nil {
			return ErrIssueNotFound
		}

		err = w.Q.DeleteTicketIssue(ctx, models.DeleteTicketIssueParams{WorkspaceID: m.Workspace.ID, ID: id})
		if err != nil {
			return err
		}

		gone, back = link, s.ticketLink(ctx, w.Q, t)

		return w.AnnounceIssue(ctx, t, tickets.IssueChange{Issue: link, Data: map[string]any{"unlinked": true}})
	})
	if err != nil {
		return err
	}

	if opts.FromTool {
		s.forget(ctx, gone, back)
	}

	return nil
}

/*
forget takes back what dimmi wrote on an unlinked issue for its ticket,
when the tool can. A tool that refuses keeps it; the link is gone
either way.
*/
func (s *Issues) forget(ctx context.Context, link models.TicketIssue, back string) {
	if link.InstallationID == nil {
		return
	}

	tl, err := s.toolFor(ctx, link.WorkspaceID, *link.InstallationID)
	if err != nil {
		return
	}

	other, linked := s.otherLink(ctx, link)
	shared, _ := tl.tracker.(trackers.Shared)

	keep := linked && shared != nil && shared.SharedImpact()

	if f, ok := tl.tracker.(trackers.Forgetter); ok && !keep {
		is := trackers.Issue{ExternalID: link.ExternalID, Key: link.Key, URL: link.Url}
		if err := f.Forget(ctx, tl.call, is, back); err != nil {
			log.From(ctx).WarnContext(ctx, "take back issue impact",
				log.String("provider", link.Provider), log.Err(err))
		}
	}

	if linked {
		s.rewriteImpact(ctx, other)
	}
}

/*
otherLink finds another ticket of the workspace still linked to the
same issue, whose impact is then written again: an incident keeps one
impact for all its tickets, and Jira's impact property is the issue's.
*/
func (s *Issues) otherLink(ctx context.Context, link models.TicketIssue) (models.TicketIssuesByExternalIDRow, bool) {
	links, err := s.linksOf(ctx, link.Provider, nil, link.ExternalID)
	if err != nil {
		return models.TicketIssuesByExternalIDRow{}, false
	}

	for _, l := range links {
		if l.WorkspaceID == link.WorkspaceID && l.ID != link.ID {
			return l, true
		}
	}

	return models.TicketIssuesByExternalIDRow{}, false
}

/*
rewriteImpact writes the impact of the link's ticket to its issue again,
even when it did not change since dimmi last wrote it.
*/
func (s *Issues) rewriteImpact(ctx context.Context, l models.TicketIssuesByExternalIDRow) {
	var ticket ids.UUID

	err := s.inWorkspace(ctx, l.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		link, err := q.FindTicketIssue(ctx, models.FindTicketIssueParams(l))
		if err != nil {
			return err
		}

		ticket = link.TicketID

		return q.SetTicketIssueImpact(ctx, models.SetTicketIssueImpactParams{
			ImpactHash: "", WorkspaceID: l.WorkspaceID, ID: l.ID,
		})
	})
	if err == nil {
		err = s.RefreshImpact(ctx, l.WorkspaceID, ticket)
	}

	if err != nil {
		log.From(ctx).WarnContext(ctx, "rewrite issue impact", log.String("issue_id", l.ID.String()), log.Err(err))
	}
}

/*
clip cuts text to the length a column takes.
*/
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}

	return string(r[:n])
}
