/*
|--------------------------------------------------------------------------
| Widget Realtime Token Controller
|--------------------------------------------------------------------------
|
| A visitor's messenger asks for a fresh connect token each time it opens or
| reopens its stream, so a stream URL is never reused once its minute is up.
|
*/

package widget

import (
	"context"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/http/requests"
)

/*
RealtimeTokenResource is a stream's connect token and where to use it.
*/
type RealtimeTokenResource struct {
	Object    string    `json:"object"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	URL       string    `json:"url"`
	Channel   string    `json:"channel"`
}

/*
RealtimeTokenOutput is a new connect token.
*/
type RealtimeTokenOutput struct {
	Status int
	Body   fhttp.Resource[RealtimeTokenResource]
}

/*
RealtimeTokenController issues connect tokens. BaseURL returns the base
address a request reached this install on.
*/
type RealtimeTokenController struct {
	Realtime *actions.Realtime
	BaseURL  func(ctx context.Context) string
}

/*
Store issues a connect token for the session's stream.
*/
func (c *RealtimeTokenController) Store(ctx context.Context, _ *requests.NoInput) (*RealtimeTokenOutput, error) {
	session, _ := actions.CurrentSession(ctx)

	token, err := c.Realtime.Issue(session)
	if err != nil {
		return nil, err
	}

	return &RealtimeTokenOutput{Status: fhttp.StatusCreated, Body: fhttp.NewResource(RealtimeTokenResource{
		Object: "realtime_token", Token: token.Token, ExpiresAt: token.ExpiresAt.UTC(),
		URL: c.BaseURL(ctx) + "/connection/uni_sse", Channel: token.Channel,
	})}, nil
}
