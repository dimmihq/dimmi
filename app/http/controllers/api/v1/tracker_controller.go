/*
|--------------------------------------------------------------------------
| Tracker Controller
|--------------------------------------------------------------------------
|
| The trackers the AI suggests for a conversation, and what a tracker's
| customers report.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TrackerController serves tracker suggestions and summaries.
*/
type TrackerController struct {
	TicketController
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
func (c *TrackerController) TrackerSuggestions(ctx context.Context, in *requests.TrackerSuggestionsRequest,
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
Summary answers what a tracker's customers report.
*/
func (c *TrackerController) Summary(ctx context.Context, in *requests.TicketSummaryRequest,
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
