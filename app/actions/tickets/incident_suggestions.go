/*
|--------------------------------------------------------------------------
| Incident Suggestions
|--------------------------------------------------------------------------
|
| What the team does with a suggested incident. Accepting declares it
| with the group's title and affected area, linking every conversation
| of the group at once; dismissing mutes the group for an hour, so the
| detector does not suggest it again while the burst lasts.
|
*/

package tickets

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of deciding a suggestion.
*/
var (
	ErrSuggestionNotFound = fault.New(fault.NotFound, "incident_suggestion_not_found",
		"No incident suggestion of this workspace has this id.")
	ErrSuggestionDecided = fault.New(fault.Conflict, "incident_suggestion_decided",
		"This suggestion was already accepted or dismissed.").OnField("status")
	ErrSuggestionStatus = fault.New(fault.Invalid, "incident_suggestion_status_invalid",
		"A suggestion is accepted or dismissed.").OnField("status")
)

/*
IncidentSuggestions returns the workspace's open suggestions, newest
first.
*/
func (t *Tickets) IncidentSuggestions(ctx context.Context, m authz.Membership) ([]models.IncidentSuggestion, error) {
	if err := canRead(m); err != nil {
		return nil, err
	}

	var out []models.IncidentSuggestion

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = models.New(tx).ListIncidentSuggestions(ctx, m.Workspace.ID)

		return err
	})

	return out, err
}

/*
DecideSuggestion accepts or dismisses an open suggestion. Accepting
declares the incident and links the group's conversations.
*/
func (t *Tickets) DecideSuggestion(
	ctx context.Context, m authz.Membership, id ids.UUID, status string,
) (models.IncidentSuggestion, error) {
	if err := canWrite(m); err != nil {
		return models.IncidentSuggestion{}, err
	}

	if status != SuggestionAccepted && status != SuggestionDismissed {
		return models.IncidentSuggestion{}, ErrSuggestionStatus
	}

	var out models.IncidentSuggestion

	err := t.write(ctx, m, func(ctx context.Context, w *Writer) error {
		cur, err := w.Q.FindIncidentSuggestion(ctx, models.FindIncidentSuggestionParams{
			WorkspaceID: m.Workspace.ID, ID: id,
		})
		if err != nil {
			return ErrSuggestionNotFound
		}

		if cur.Status != SuggestionOpen {
			return ErrSuggestionDecided
		}

		params := models.DecideIncidentSuggestionParams{
			Status: status, DecidedByMemberID: &m.Member.ID, Now: w.Now, WorkspaceID: m.Workspace.ID, ID: id,
		}

		if status == SuggestionDismissed {
			params.MutedUntil = new(w.Now.Add(SpikeMute))
		} else {
			incident, err := w.declareSuggested(ctx, cur)
			if err != nil {
				return err
			}

			params.TicketID = &incident.ID
		}

		out, err = w.Q.DecideIncidentSuggestion(ctx, params)
		if err != nil {
			return fmt.Errorf("decide suggestion: %w", err)
		}

		return nil
	})

	return out, err
}

/*
declareSuggested declares the incident a suggestion describes, with its
area when the incident type offers it, linking its live conversations.
*/
func (w *Writer) declareSuggested(ctx context.Context, s models.IncidentSuggestion) (models.Ticket, error) {
	ty, ok := w.Setup.FirstType(KindIncident)
	if !ok {
		return models.Ticket{}, ErrTypeNotFound
	}

	var live []ids.UUID

	for _, id := range s.ConversationIds {
		c, err := w.Q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: s.WorkspaceID, ID: id})
		if err == nil && c.DeletedAt == nil {
			live = append(live, id)
		}
	}

	fields := map[string]any{}
	if area := w.areaOption(ty.ID, s.Area); area != "" {
		fields["areas"] = []any{area}
	}

	return w.Declare(ctx, s.WorkspaceID, DeclareInput{
		TypeID: &ty.ID, Title: s.Title, Description: s.Summary, Fields: fields, Conversations: live,
	})
}

/*
areaOption returns the incident type's area option the suggestion
names, ignoring case, or empty when the type has none that fits.
*/
func (w *Writer) areaOption(typeID ids.UUID, area *string) string {
	if area == nil || strings.TrimSpace(*area) == "" {
		return ""
	}

	for _, f := range w.Setup.FieldsOf(typeID) {
		if f.Key != "areas" || f.ArchivedAt != nil {
			continue
		}

		i := slices.IndexFunc(f.Options, func(o string) bool { return strings.EqualFold(o, strings.TrimSpace(*area)) })
		if i >= 0 {
			return f.Options[i]
		}
	}

	return ""
}
