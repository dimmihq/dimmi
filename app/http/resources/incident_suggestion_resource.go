/*
|--------------------------------------------------------------------------
| Incident Suggestion Resource
|--------------------------------------------------------------------------
|
| A spike of new conversations about one thing, suggested as an
| incident: what it seems to be about, the area it touches, the
| conversations in it, and whether the team accepted or dismissed it.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
IncidentSuggestion is one suggested incident.
*/
type IncidentSuggestion struct {
	Object            string     `json:"object"`
	ID                string     `json:"id"`
	Title             string     `json:"title"`
	Summary           string     `json:"summary"`
	Area              *string    `json:"area"`
	ConversationIDs   []string   `json:"conversation_ids"`
	ConversationCount int32      `json:"conversation_count"`
	WindowMinutes     int32      `json:"window_minutes"`
	Status            string     `json:"status"`
	MutedUntil        *time.Time `json:"muted_until"`
	TicketID          *string    `json:"ticket_id"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

/*
NewIncidentSuggestion shapes a suggestion.
*/
func NewIncidentSuggestion(s models.IncidentSuggestion) IncidentSuggestion {
	convs := make([]string, len(s.ConversationIds))
	for i, id := range s.ConversationIds {
		convs[i] = models.PublicID(models.PrefixConversation, id)
	}

	return IncidentSuggestion{
		Object: "incident_suggestion", ID: models.PublicID(models.PrefixIncidentSuggestion, s.ID), Title: s.Title,
		Summary: s.Summary, Area: s.Area, ConversationIDs: convs, ConversationCount: s.ConversationCount,
		WindowMinutes: s.WindowMinutes, Status: s.Status, MutedUntil: s.MutedUntil,
		TicketID: optionalID(models.PrefixTicket, s.TicketID), CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

/*
NewIncidentSuggestions shapes suggestions.
*/
func NewIncidentSuggestions(list []models.IncidentSuggestion) []IncidentSuggestion {
	out := make([]IncidentSuggestion, len(list))
	for i, s := range list {
		out[i] = NewIncidentSuggestion(s)
	}

	return out
}
