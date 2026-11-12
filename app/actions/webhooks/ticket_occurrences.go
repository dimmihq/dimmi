/*
|--------------------------------------------------------------------------
| Ticket Occurrences
|--------------------------------------------------------------------------
|
| Ticket events show the ticket and what changed from and to. Ids in the
| changes leave in their public form, named by what they are: states,
| types, teammates, teams and policies.
|
*/

package webhooks

import (
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Subjects of the ticket events.
*/
const (
	subjectTicket             = "ticket"
	subjectIncidentSuggestion = "incident_suggestion"
)

/*
ticketOccurrence names the ticket of a ticket event, or the suggestion
of a spike, and reports false for any other event.
*/
func ticketOccurrence(e any, args jobs.FanOutWebhookEventArgs) (jobs.FanOutWebhookEventArgs, bool) {
	switch ev := e.(type) {
	case appevents.TicketEvent:
		args.Subject, args.SubjectID = subjectTicket, ev.Ticket().TicketID
	case appevents.IncidentSuggested:
		args.Subject, args.SubjectID = subjectIncidentSuggestion, ev.SuggestionID
	default:
		return args, false
	}

	return args, true
}

/*
ticketExtras shapes what a ticket event changed.
*/
func ticketExtras(ev appevents.TicketChanged) map[string]any {
	out := map[string]any{}
	for key, c := range ev.Changes {
		out[key] = map[string]any{"from": publicValue(key, c.From), "to": publicValue(key, c.To)}
	}

	return map[string]any{"kind": ev.Kind, "changes": out}
}

/*
publicValue writes ids in a change as public ids of their kind.
*/
func publicValue(key string, v any) any {
	prefix, ok := changePrefix(key)
	if !ok {
		return v
	}

	switch id := v.(type) {
	case ids.UUID:
		return models.PublicID(prefix, id)
	case *ids.UUID:
		if id == nil {
			return nil
		}

		return models.PublicID(prefix, *id)
	}

	return v
}

/*
changePrefix returns the public prefix of the ids a change key holds.
*/
func changePrefix(key string) (models.Prefix, bool) {
	switch {
	case key == "state":
		return models.PrefixTicketState, true
	case key == "type":
		return models.PrefixTicketType, true
	case key == "assignee_member_id":
		return models.PrefixMember, true
	case key == "assignee_team_id":
		return models.PrefixTeam, true
	case key == "sla_policy_id":
		return models.PrefixSLAPolicy, true
	case key == "message_id":
		return models.PrefixMessage, true
	case strings.HasPrefix(key, "fields."):
		return "", false
	}

	return "", false
}
