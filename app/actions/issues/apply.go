/*
|--------------------------------------------------------------------------
| Applying What A Tool Says
|--------------------------------------------------------------------------
|
| Whatever brought the news, a webhook or the re-check, a linked issue
| is updated the same way, as the tool: the link keeps the new status,
| the ticket's history notes it, and the ticket moves to the dimmi state
| the status maps to when its type allows it. A move the ticket's rules
| refuse, such as fields required to resolve, leaves a warning on the
| link instead of failing silently. A done or canceled issue, or a
| resolved incident, drafts the message closing the loop.
|
*/

package issues

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
applyIssue stores what the tool says of the issue now on one link and
moves its ticket.
*/
func (s *Issues) applyIssue(ctx context.Context, ws, linkID ids.UUID, is trackers.Issue) error {
	return s.asTool(ctx, ws, linkID, func(ctx context.Context, w *tickets.Writer, link models.TicketIssue) error {
		updated, err := w.Q.SyncTicketIssueDetails(ctx, detailsOf(link, is, w.Now))
		if err != nil {
			return fmt.Errorf("sync issue: %w", err)
		}

		if updated.Status == link.Status && updated.StatusGroup == link.StatusGroup {
			return nil
		}

		t, err := w.Lock(ctx, ws, link.TicketID)
		if err != nil || t.DeletedAt != nil {
			return nil //nolint:nilerr // a ticket gone has nothing to follow
		}

		err = w.AnnounceIssue(ctx, t, tickets.IssueChange{Issue: updated, Data: map[string]any{
			"from_status": link.Status, "from_group": link.StatusGroup,
		}})
		if err != nil {
			return err
		}

		t, err = s.follow(ctx, w, t, statusUpdate{link: updated, statusID: is.StatusID})
		if err != nil {
			return err
		}

		if updated.StatusGroup != link.StatusGroup {
			return s.closeLoop(ctx, w, t, updated)
		}

		return nil
	})
}

/*
asTool runs the work on one link in a transaction acting as its tool.
*/
func (s *Issues) asTool(ctx context.Context, ws, linkID ids.UUID,
	fn func(ctx context.Context, w *tickets.Writer, link models.TicketIssue) error,
) error {
	var provider string

	_ = s.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		link, err := q.FindTicketIssue(ctx, models.FindTicketIssueParams{WorkspaceID: ws, ID: linkID})
		provider = link.Provider

		return err
	})

	return s.Tickets.AsTool(ctx, ws, Label(provider), func(ctx context.Context, w *tickets.Writer) error {
		link, err := w.Q.FindTicketIssue(ctx, models.FindTicketIssueParams{WorkspaceID: ws, ID: linkID})
		if err != nil {
			return nil //nolint:nilerr // a link removed meanwhile has nothing to update
		}

		return fn(ctx, w, link)
	})
}

/*
follow moves the ticket to the state the status maps to. A move the
ticket's rules refuse becomes a warning on the link.
*/
func (s *Issues) follow(ctx context.Context, w *tickets.Writer, t models.Ticket, u statusUpdate,
) (models.Ticket, error) {
	link := u.link
	u.settings = s.settingsOfLink(ctx, w.Q, link)

	state, ok := stateFor(w.Setup, t, u)
	if !ok || state.ID == t.StateID || !w.Setup.Allows(t.TypeID, state.ID) {
		return t, nil
	}

	moved, err := w.Apply(ctx, t, tickets.Change{StateID: &state.ID})

	var f *fault.Error
	if errors.As(err, &f) && f.Kind() == fault.Invalid {
		warning := clip(fmt.Sprintf("%s says %s, but the ticket could not move to %s: %s", Label(link.Provider),
			link.Status, state.Name, f.Message()), 500)

		return t, w.Q.WarnTicketIssue(ctx, models.WarnTicketIssueParams{
			Warning: &warning, Now: w.Now, WorkspaceID: link.WorkspaceID, ID: link.ID,
		})
	}

	return moved, err
}

/*
statusUpdate is a link's new status: the link, the tool's id of the
status and the settings mapping it.
*/
type statusUpdate struct {
	link     models.TicketIssue
	statusID string
	settings SyncSettings
}

/*
settingsOfLink reads the settings of the link's installation.
*/
func (s *Issues) settingsOfLink(ctx context.Context, q *models.Queries, link models.TicketIssue) SyncSettings {
	if link.InstallationID == nil {
		return SyncSettings{}
	}

	row, err := q.FindIntegrationInstallation(ctx, models.FindIntegrationInstallationParams{
		WorkspaceID: link.WorkspaceID, ID: *link.InstallationID,
	})
	if err != nil {
		return SyncSettings{}
	}

	return settingsOf(row.Metadata)
}

/*
stateFor returns the dimmi state a tool status maps to: the one the
workspace picked for it, or the default for its group.
*/
func stateFor(setup tickets.Setup, t models.Ticket, u statusUpdate) (models.TicketState, bool) {
	link := u.link

	for _, key := range []string{u.statusID, link.Status} {
		if picked, ok := u.settings.States[key]; ok && key != "" {
			if id, err := ids.Parse(picked); err == nil {
				if state, ok := setup.State(id); ok && state.ArchivedAt == nil {
					return state, true
				}
			}
		}
	}

	if t.Kind == tickets.KindIncident {
		return incidentState(setup, t, link)
	}

	switch link.StatusGroup {
	case trackers.GroupStarted:
		return setup.StateByPurpose(tickets.PurposeBeingWorked)
	case trackers.GroupDone:
		return setup.StateByPurpose(tickets.PurposeCloseLoop)
	case trackers.GroupCanceled:
		return setup.StateByPurpose(tickets.PurposeWontFix)
	}

	return models.TicketState{}, false
}

/*
incidentState maps an incident tool's status to the incident states:
resolved when it is over, monitoring once mitigated, identified once
someone acknowledged or is fixing it, investigating before.
*/
func incidentState(setup tickets.Setup, t models.Ticket, link models.TicketIssue) (models.TicketState, bool) {
	name := strings.ToLower(link.Status)

	switch {
	case link.StatusGroup == trackers.GroupDone || link.StatusGroup == trackers.GroupCanceled:
		return setup.DefaultResolved(t.TypeID)
	case strings.Contains(name, "monitor") || strings.Contains(name, "mitigat"):
		return setup.StateByPurpose(tickets.PurposeMonitoring)
	case strings.Contains(name, "identif") || strings.Contains(name, "fix") || strings.Contains(name, "acknowledg"):
		return setup.StateByPurpose(tickets.PurposeIdentified)
	}

	return setup.StateByPurpose(tickets.PurposeInvestigating)
}
