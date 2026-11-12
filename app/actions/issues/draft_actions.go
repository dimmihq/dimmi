/*
|--------------------------------------------------------------------------
| Drafts On The Tracker
|--------------------------------------------------------------------------
|
| The tracker page shows each draft with who it would reach and who
| would be held and why, counted the way the send will count them. A
| teammate sends it, edited or not, or discards it. A deployment that
| ships the fix moves the drafts waiting for it to ready, and sends them
| when the workspace chose automatic.
|
*/

package issues

import (
	"context"
	"errors"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of deciding drafts.
*/
var (
	ErrDraftNotFound = fault.New(fault.NotFound, "ticket_update_draft_not_found",
		"No draft of this workspace has this id.")
	ErrDraftDecided = fault.New(fault.Conflict, "ticket_update_draft_decided",
		"This draft was already sent or discarded.")
	ErrDraftAction = fault.New(fault.Invalid, "ticket_update_draft_action_invalid",
		"The status is sent or discarded.").OnField("status")
)

/*
DraftView is a draft with who it reaches now and who would be held,
by reason.
*/
type DraftView struct {
	Draft      models.TicketUpdateDraft
	Recipients int
	Held       map[string]int
}

/*
Drafts returns a ticket's drafts, the open ones counted.
*/
func (s *Issues) Drafts(ctx context.Context, m authz.Membership, ref tickets.Ref) ([]DraftView, error) {
	if err := tickets.MayRead(m); err != nil {
		return nil, err
	}

	var out []DraftView

	err := s.Tickets.AsMember(ctx, m, func(ctx context.Context, w *tickets.Writer) error {
		t, err := w.Find(ctx, m, ref)
		if err != nil {
			return err
		}

		list, err := w.Q.TicketUpdateDrafts(ctx, models.TicketUpdateDraftsParams{
			WorkspaceID: t.WorkspaceID, TicketID: t.ID,
		})
		for _, d := range list {
			out = append(out, counted(ctx, w, t, d))
		}

		return err
	})

	return out, err
}

/*
counted is the draft with who it reaches now, counted by a dry run
while it is open.
*/
func counted(ctx context.Context, w *tickets.Writer, t models.Ticket, d models.TicketUpdateDraft) DraftView {
	view := DraftView{Draft: d, Held: map[string]int{}}
	if d.Status != DraftReady {
		return view
	}

	plan, err := w.PlanMessage(ctx, t, tickets.UpdateInput{Body: d.Body, DryRun: true})
	if err != nil {
		return view
	}

	view.Recipients = len(plan.Recipients)

	for _, r := range plan.Recipients {
		if r.Held != "" {
			view.Held[r.Held]++
		}
	}

	return view
}

/*
DecideInput sends or discards a draft: Status is sent or discarded;
Body replaces the text and Resolve closes the customers' conversations
after it is sent.
*/
type DecideInput struct {
	Status  string
	Body    *string
	Resolve *bool
}

/*
Decide sends or discards a draft.
*/
func (s *Issues) Decide(ctx context.Context, m authz.Membership, id ids.UUID, in DecideInput,
) (models.TicketUpdateDraft, error) {
	if err := tickets.MayWrite(m); err != nil {
		return models.TicketUpdateDraft{}, err
	}

	if in.Status != DraftSent && in.Status != DraftDiscarded {
		return models.TicketUpdateDraft{}, ErrDraftAction
	}

	var out models.TicketUpdateDraft

	err := s.Tickets.AsMember(ctx, m, func(ctx context.Context, w *tickets.Writer) error {
		d, err := w.Q.TicketUpdateDraft(ctx, models.TicketUpdateDraftParams{WorkspaceID: m.Workspace.ID, ID: id})
		if errors.Is(err, database.ErrNotFound) {
			return ErrDraftNotFound
		}

		if err != nil {
			return err
		}

		t, err := w.Find(ctx, m, tickets.Ref{ID: d.TicketID})
		if err != nil {
			return ErrDraftNotFound
		}

		if d.Status == DraftSent || d.Status == DraftDiscarded {
			return ErrDraftDecided
		}

		out, err = s.applyDecision(ctx, w, d, decision{in: in, member: m.Member.ID, ticket: t})

		return err
	})

	return out, err
}

/*
decision is what a member decided of a draft.
*/
type decision struct {
	in     DecideInput
	member ids.UUID
	ticket models.Ticket
}

/*
applyDecision applies the decision.
*/
func (s *Issues) applyDecision(ctx context.Context, w *tickets.Writer, d models.TicketUpdateDraft, dec decision,
) (models.TicketUpdateDraft, error) {
	if dec.in.Body != nil {
		body := strings.TrimSpace(*dec.in.Body)
		if n := len([]rune(body)); n < 1 || n > 20000 {
			return d, tickets.ErrNoteInvalid
		}

		d.Body = body
	}

	if dec.in.Resolve != nil {
		d.Resolve = *dec.in.Resolve
	}

	if dec.in.Status == DraftSent {
		return s.sendDraft(ctx, w, sending{ticket: dec.ticket, draft: d, member: &dec.member})
	}

	now := w.Now

	return w.Q.DecideTicketUpdateDraft(ctx, models.DecideTicketUpdateDraftParams{
		Status: DraftDiscarded, Body: d.Body, DecidedByMemberID: &dec.member, DecidedAt: &now, Now: now,
		WorkspaceID: d.WorkspaceID, ID: d.ID,
	})
}

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
