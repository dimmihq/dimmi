/*
|--------------------------------------------------------------------------
| Slack Socket Mode
|--------------------------------------------------------------------------
|
| A self-hosted install without a public URL receives Slack over a
| socket instead of webhooks: the app's own token opens it, and Slack
| delivers the same events and interactions down it. Each delivery is
| acknowledged only after it was kept, so one the product failed to keep
| is delivered again. The Slack SDK runs the socket, its reconnects and
| its pings; this file only hands deliveries over.
|
*/

package slack

import (
	"context"
	"errors"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
The kinds of delivery a socket hands over.
*/
const (
	SocketEvents      = "events"
	SocketInteraction = "interaction"
)

/*
ErrSocketClosed means Slack closed the socket for good, such as for an
app token it no longer accepts.
*/
var ErrSocketClosed = errors.New("slack: socket closed")

/*
SocketHandler keeps one delivery: an Events API envelope or an
interaction payload, by kind. An error leaves it unacknowledged.
*/
type SocketHandler func(ctx context.Context, kind string, payload []byte) error

/*
SocketOptions open a socket: the app's own token and the bot's, over
the HTTP client, against the Web API at APIURL.
*/
type SocketOptions struct {
	AppToken string
	BotToken string
	HTTP     *httpclient.Client
	APIURL   string
}

/*
quiet drops the SDK's log lines; the caller logs what matters.
*/
type quiet struct{}

/*
Output drops the line.
*/
func (quiet) Output(int, string) error {
	return nil
}

/*
RunSocket holds the socket open and hands every delivery to the
handler until the context ends or Slack closes it.
*/
func RunSocket(ctx context.Context, opts SocketOptions, handle SocketHandler) error {
	api := slack.New(opts.BotToken, slack.OptionAppLevelToken(opts.AppToken),
		slack.OptionHTTPClient(opts.HTTP.StdClient()), slack.OptionAPIURL(opts.APIURL))
	client := socketmode.New(api, socketmode.OptionLog(quiet{}))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ended := make(chan error, 1)

	go func() { ended <- client.RunContext(ctx) }()

	for {
		select {
		case err := <-ended:
			return err
		case evt := <-client.Events:
			if evt.Type == socketmode.EventTypeInvalidAuth {
				return ErrSocketClosed
			}

			deliver(ctx, client, evt, handle)
		}
	}
}

/*
deliver hands one socket event over and acknowledges it once kept.
*/
func deliver(ctx context.Context, client *socketmode.Client, evt socketmode.Event, handle SocketHandler) {
	if evt.Request == nil {
		return
	}

	kind := SocketEvents
	if evt.Type == socketmode.EventTypeInteractive {
		kind = SocketInteraction
	}

	if evt.Type != socketmode.EventTypeEventsAPI && evt.Type != socketmode.EventTypeInteractive {
		return
	}

	if handle(ctx, kind, evt.Request.Payload) != nil {
		return
	}

	_ = client.Ack(*evt.Request)
}

/*
CheckAppToken opens a socket connection once to prove the app token
works and Socket Mode is on for the app.
*/
func CheckAppToken(ctx context.Context, opts SocketOptions) error {
	api := slack.New(opts.BotToken, slack.OptionAppLevelToken(opts.AppToken),
		slack.OptionHTTPClient(opts.HTTP.StdClient()), slack.OptionAPIURL(opts.APIURL))

	_, _, err := api.StartSocketModeContext(ctx)

	return MapError("apps.connections.open", err)
}
