/*
|--------------------------------------------------------------------------
| Slack Channel Controller
|--------------------------------------------------------------------------
|
| POST /channels/slack connects a workspace's own Slack app: at once when
| its secrets come along, or waiting for them, answering with the
| manifest that creates the app. POST /channels/{channel}/credentials
| sets or rotates a connection's secrets. Both need channels:manage; an
| id of another workspace's connection is simply not found.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SlackChannelOutput is one Slack connection.
*/
type SlackChannelOutput struct {
	Body fhttp.Resource[resources.SlackChannel]
}

/*
SlackChannelController connects workspaces' own Slack apps.
*/
type SlackChannelController struct {
	Slack *channels.Slack
	IDs   *ids.Registry
}

/*
Store connects a Slack app.
*/
func (c *SlackChannelController) Store(
	ctx context.Context, in *requests.StoreSlackChannelRequest,
) (*SlackChannelOutput, error) {
	input := channels.ConnectSlackInput{
		Membership: membership(ctx), Name: in.Body.Name, Transport: in.Body.InboundTransport,
		Secrets: secretsOf(in.Body.SlackSecretsBody),
	}

	if in.Body.InboxID != nil {
		inbox, err := c.IDs.Decode(string(models.PrefixInbox), *in.Body.InboxID)
		if err != nil {
			return nil, exceptions.ErrNotFound.OnField("inbox_id")
		}

		input.InboxID = &inbox
	}

	connected, err := c.Slack.Connect(ctx, input)
	if err != nil {
		return nil, err
	}

	return c.output(connected.Connection), nil
}

/*
StoreCredentials sets the secrets of a Slack connection.
*/
func (c *SlackChannelController) StoreCredentials(
	ctx context.Context, in *requests.StoreChannelCredentialsRequest,
) (*SlackChannelOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixChannelConnection), in.Channel)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	conn, err := c.Slack.SetSecrets(ctx, membership(ctx), id, secretsOf(in.Body.SlackSecretsBody))
	if err != nil {
		return nil, err
	}

	return c.output(conn), nil
}

/*
output answers with the connection and how to set its app up.
*/
func (c *SlackChannelController) output(conn models.ChannelConnection) *SlackChannelOutput {
	facts, setup := channels.FactsOf(conn), c.Slack.SetupOf(conn)

	return &SlackChannelOutput{Body: fhttp.NewResource(resources.NewSlackChannel(conn, resources.SlackFacts{
		Mode: facts.Mode, InboundTransport: facts.InboundTransport, TeamName: facts.TeamName,
	}, resources.SlackSetup{
		Manifest: setup.Manifest, CreateAppURL: setup.CreateAppURL, RequestURL: setup.EventsURL,
		InteractionsURL: setup.InteractionsURL,
	}))}
}

/*
secretsOf reads the pasted secrets.
*/
func secretsOf(body requests.SlackSecretsBody) channels.Secrets {
	return channels.Secrets{BotToken: body.BotToken, SigningSecret: body.SigningSecret, AppToken: body.AppToken}
}
