/*
|--------------------------------------------------------------------------
| Incident Controller
|--------------------------------------------------------------------------
|
| Suggested incidents, which the team accepts, declaring the incident
| with the group's conversations linked, or dismisses for an hour; the
| trackers the AI suggests for a conversation; and what a tracker's
| customers report.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
IncidentController serves suggested incidents, tracker suggestions and
summaries.
*/
type IncidentController struct {
	TicketController
}

/*
IncidentSuggestionsOutput is the open suggestions.
*/
type IncidentSuggestionsOutput struct {
	Body fhttp.Resource[[]resources.IncidentSuggestion]
}

/*
IncidentSuggestionOutput is one suggestion.
*/
type IncidentSuggestionOutput struct {
	Body fhttp.Resource[resources.IncidentSuggestion]
}

/*
Suggestions lists the workspace's open suggestions, newest first.
*/
func (c *IncidentController) Suggestions(ctx context.Context, _ *requests.IncidentSuggestionsRequest,
) (*IncidentSuggestionsOutput, error) {
	list, err := c.Tickets.IncidentSuggestions(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	return &IncidentSuggestionsOutput{Body: fhttp.NewResource(resources.NewIncidentSuggestions(list))}, nil
}

/*
Decide accepts or dismisses a suggestion.
*/
func (c *IncidentController) Decide(ctx context.Context, in *requests.UpdateIncidentSuggestionRequest,
) (*IncidentSuggestionOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixIncidentSuggestion), in.Suggestion)
	if err != nil {
		return nil, tickets.ErrSuggestionNotFound
	}

	s, err := c.Tickets.DecideSuggestion(ctx, membership(ctx), id, in.Body.Status)
	if err != nil {
		return nil, err
	}

	return &IncidentSuggestionOutput{Body: fhttp.NewResource(resources.NewIncidentSuggestion(s))}, nil
}

/*
TrackerSuggestionsOutput is the trackers suggested for a conversation.
*/
type TrackerSuggestionsOutput struct {
	Body fhttp.Resource[[]resources.TrackerSuggestion]
}

/*
TrackerSuggestions lists the trackers suggested for a conversation.
*/
func (c *IncidentController) TrackerSuggestions(ctx context.Context, in *requests.TrackerSuggestionsRequest,
) (*TrackerSuggestionsOutput, error) {
	conv, err := c.IDs.Decode(string(models.PrefixConversation), in.Conversation)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	rows, err := c.Tickets.TrackerSuggestions(ctx, membership(ctx), conversations.Ref{ID: conv})
	if err != nil {
		return nil, err
	}

	return &TrackerSuggestionsOutput{Body: fhttp.NewResource(resources.NewTrackerSuggestions(rows))}, nil
}

/*
TicketSummaryOutput is a tracker's summary.
*/
type TicketSummaryOutput struct {
	Body fhttp.Resource[resources.TicketSummary]
}

/*
Summary answers what a tracker or incident's customers report.
*/
func (c *IncidentController) Summary(ctx context.Context, in *requests.TicketSummaryRequest,
) (*TicketSummaryOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	s, err := c.Tickets.Summary(ctx, membership(ctx), ref)
	if err != nil {
		return nil, err
	}

	return &TicketSummaryOutput{Body: fhttp.NewResource(resources.NewTicketSummary(s))}, nil
}
