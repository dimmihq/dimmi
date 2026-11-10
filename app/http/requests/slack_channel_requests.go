/*
|--------------------------------------------------------------------------
| Slack Channel Requests
|--------------------------------------------------------------------------
|
| Connecting a workspace's own Slack app: its name, its inbox, how it
| receives, and its secrets when they are already at hand, or later on
| their own. Tokens are bounded here and checked for their shape and
| with Slack by the action; they never come back in any answer.
|
*/

package requests

/*
SlackSecretsBody are a Slack app's pasted secrets.
*/
type SlackSecretsBody struct {
	BotToken      string `json:"bot_token,omitempty"      validate:"max=255"`
	SigningSecret string `json:"signing_secret,omitempty" validate:"max=255"`
	AppToken      string `json:"app_token,omitempty"      validate:"max=255"`
}

/*
StoreSlackChannelRequest is the input of POST /channels/slack.
*/
type StoreSlackChannelRequest struct {
	Body struct {
		SlackSecretsBody

		Name             string  `json:"name,omitempty"              validate:"omitempty,max=80"`
		InboxID          *string `json:"inbox_id,omitempty"`
		InboundTransport string  `json:"inbound_transport,omitempty" validate:"omitempty,oneof=webhook socket"`
	}
}

/*
StoreChannelCredentialsRequest is the input of POST
/channels/{channel}/credentials.
*/
type StoreChannelCredentialsRequest struct {
	Channel string `path:"channel"`
	Body    struct {
		SlackSecretsBody
	}
}
