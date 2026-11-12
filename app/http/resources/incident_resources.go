/*
|--------------------------------------------------------------------------
| Tracker Suggestion And Summary Resources
|--------------------------------------------------------------------------
|
| A tracker the AI suggests for a conversation, with how many
| conversations it already reaches, and what a tracker's customers
| report: the summary, the number of reports and the plans they are on.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TrackerSuggestion is one tracker suggested for a conversation.
*/
type TrackerSuggestion struct {
	Object      string    `json:"object"`
	ID          string    `json:"id"`
	TicketID    string    `json:"ticket_id"`
	Reference   string    `json:"reference"`
	Title       string    `json:"title"`
	StateGroup  string    `json:"state_group"`
	LinkedCount int64     `json:"linked_count"`
	CreatedAt   time.Time `json:"created_at"`
}

/*
NewTrackerSuggestions shapes the suggestions.
*/
func NewTrackerSuggestions(rows []models.ListTrackerSuggestionsRow) []TrackerSuggestion {
	out := make([]TrackerSuggestion, len(rows))
	for i, r := range rows {
		out[i] = TrackerSuggestion{
			Object: "tracker_suggestion", ID: models.PublicID(models.PrefixTrackerSuggestion, r.SuggestionID),
			TicketID: models.PublicID(models.PrefixTicket, r.ID), Reference: tickets.Reference(r.Number),
			Title: r.Title, StateGroup: r.StateGroup, LinkedCount: r.LinkedCount, CreatedAt: r.SuggestedAt,
		}
	}

	return out
}

/*
TicketPlanCount is how many reports came from one plan; empty for
customers without one.
*/
type TicketPlanCount struct {
	Plan  string `json:"plan"`
	Count int    `json:"count"`
}

/*
TicketSummary is what a tracker's customers report.
*/
type TicketSummary struct {
	Object      string            `json:"object"`
	TicketID    string            `json:"ticket_id"`
	Summary     string            `json:"summary"`
	Reports     int               `json:"reports"`
	Plans       []TicketPlanCount `json:"plans"`
	GeneratedAt *time.Time        `json:"generated_at"`
}

/*
NewTicketSummary shapes a tracker's summary.
*/
func NewTicketSummary(s tickets.TrackerSummary) TicketSummary {
	plans := make([]TicketPlanCount, len(s.Plans))
	for i, p := range s.Plans {
		plans[i] = TicketPlanCount{Plan: p.Plan, Count: p.Count}
	}

	return TicketSummary{
		Object: "ticket_summary", TicketID: models.PublicID(models.PrefixTicket, s.Ticket.ID), Summary: s.Summary,
		Reports: s.Reports, Plans: plans, GeneratedAt: s.GeneratedAt,
	}
}
