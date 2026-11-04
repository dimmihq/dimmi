/*
|--------------------------------------------------------------------------
| Show Widget Availability Controller
|--------------------------------------------------------------------------
|
| GET /widget/v1/apps/{app}/availability tells a visitor whether someone is
| there and how soon they usually answer.
|
*/

package widget

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"

	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/support/widgetstrings"
)

/*
AvailabilityResource is what a visitor is told about the team.
*/
type AvailabilityResource struct {
	Object        string             `json:"object"`
	Online        bool               `json:"online"`
	WithinHours   bool               `json:"within_hours"`
	ReplyTime     string             `json:"reply_time"`
	ReplyTimeText string             `json:"reply_time_text"`
	Team          []actions.TeamFace `json:"team"`
	ServerTime    time.Time          `json:"server_time"`
}

/*
AvailabilityOutput is the availability with how long it may be kept.
*/
type AvailabilityOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         fhttp.Resource[AvailabilityResource]
}

/*
ShowWidgetAvailabilityController answers with a widget's availability.
*/
type ShowWidgetAvailabilityController struct {
	Compute *actions.ComputeTeamAvailability
	Strings *widgetstrings.Strings
	Clock   clock.Clock
}

/*
Invoke answers with the availability, its words in the language asked for.
*/
func (c *ShowWidgetAvailabilityController) Invoke(
	ctx context.Context, in *requests.ShowWidgetAvailabilityRequest,
) (*AvailabilityOutput, error) {
	resolved, _ := actions.CurrentResolved(ctx)

	found, err := c.Compute.Handle(ctx, resolved)
	if err != nil {
		return nil, err
	}

	return &AvailabilityOutput{
		CacheControl: "public, max-age=30",
		Body: fhttp.NewResource(AvailabilityResource{
			Object: "widget_availability", Online: found.Online, WithinHours: found.WithinHours,
			ReplyTime: found.ReplyTime, ReplyTimeText: c.Strings.Word(in.Locale, "reply_"+found.ReplyTime),
			Team: found.Team, ServerTime: c.Clock.Now(),
		}),
	}, nil
}
