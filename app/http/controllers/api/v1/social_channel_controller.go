/*
|--------------------------------------------------------------------------
| Messenger And Instagram Channels
|--------------------------------------------------------------------------
|
| The dashboard's endpoints for one platform's channel: what Meta's
| sign-in popup opens with, the accounts a sign-in shared, connecting
| one, listing, moving and disconnecting connections, and where a
| conversation stands on the platform.
|
*/

package v1

import (
	"context"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SocialCheckOutput is what a connection's check found.
*/
type SocialCheckOutput struct {
	Body fhttp.Collection[resources.SocialFinding]
}

/*
SocialChannelController serves one platform's channel.
*/
type SocialChannelController struct {
	Channel *channels.Social
	IDs     *ids.Registry
}

/*
Index lists the workspace's connections.
*/
func (c *SocialChannelController) Index(ctx context.Context, _ *struct{}) (*SocialChannelsOutput, error) {
	rows, err := c.Channel.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.SocialChannel, 0, len(rows))
	for _, row := range rows {
		items = append(items, socialChannel(row))
	}

	page := pagination.Page[resources.SocialChannel]{Items: items, Limit: 100}

	return &SocialChannelsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
SignupConfig says what Meta's popup needs to open.
*/
func (c *SocialChannelController) SignupConfig(ctx context.Context, _ *struct{}) (*SocialSignupConfigOutput, error) {
	if !membership(ctx).Can("channels:manage") {
		return nil, exceptions.ErrForbidden
	}

	cfg := c.Channel.SignupConfig()
	out := resources.SocialSignupConfig{Available: cfg.Available, GraphVersion: cfg.GraphVersion}

	if cfg.Available {
		out.AppID, out.ConfigID = cfg.AppID, cfg.ConfigID
	}

	return &SocialSignupConfigOutput{Body: fhttp.NewResource(out)}, nil
}

/*
Signup trades the popup's code for the accounts Meta shared.
*/
func (c *SocialChannelController) Signup(ctx context.Context, in *requests.StoreSocialSignupRequest) (
	*SocialGrantOutput, error,
) {
	grant, err := c.Channel.Signup(ctx, membership(ctx), channels.SocialSignupInput{
		Code: in.Body.Code, RedirectURI: in.Body.RedirectURI,
	})
	if err != nil {
		return nil, err
	}

	accounts := make([]resources.SocialAccount, 0, len(grant.Accounts))
	for _, a := range grant.Accounts {
		accounts = append(accounts, resources.SocialAccount{
			ID: a.ID, Name: a.Name, Username: a.Username, PictureURL: a.PictureURL, Connected: a.Connected,
		})
	}

	return &SocialGrantOutput{Body: fhttp.NewResource(resources.SocialGrant{
		Grant: grant.Grant, Accounts: accounts,
	})}, nil
}

/*
Store connects the account picked from a sign-in.
*/
func (c *SocialChannelController) Store(ctx context.Context, in *requests.StoreSocialChannelRequest) (
	*SocialChannelOutput, error,
) {
	inbox, err := c.inbox(in.Body.InboxID)
	if err != nil {
		return nil, err
	}

	conn, err := c.Channel.Connect(ctx, channels.ConnectSocialInput{
		Membership: membership(ctx), Grant: in.Body.Grant, AccountID: in.Body.AccountID, InboxID: inbox,
	})
	if err != nil {
		return nil, err
	}

	return &SocialChannelOutput{Body: fhttp.NewResource(socialChannel(conn))}, nil
}

/*
Update moves a connection's new conversations to another inbox.
*/
func (c *SocialChannelController) Update(ctx context.Context, in *requests.UpdateSocialChannelRequest) (
	*SocialChannelOutput, error,
) {
	id, err := c.channel(in.Channel)
	if err != nil {
		return nil, err
	}

	inbox, err := c.inbox(in.Body.InboxID)
	if err != nil {
		return nil, err
	}

	conn, err := c.Channel.Update(ctx, membership(ctx), id, inbox)
	if err != nil {
		return nil, err
	}

	return &SocialChannelOutput{Body: fhttp.NewResource(socialChannel(conn))}, nil
}

/*
Destroy disconnects a connection.
*/
func (c *SocialChannelController) Destroy(ctx context.Context, in *requests.SocialChannelRequest) (
	*EmptyOutput, error,
) {
	id, err := c.channel(in.Channel)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Channel.Disconnect(ctx, membership(ctx), id)
}

/*
Check asks Meta what stands between a connection and its messages.
*/
func (c *SocialChannelController) Check(ctx context.Context, in *requests.SocialChannelRequest) (
	*SocialCheckOutput, error,
) {
	id, err := c.channel(in.Channel)
	if err != nil {
		return nil, err
	}

	found, err := c.Channel.Check(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	items := make([]resources.SocialFinding, 0, len(found))
	for _, f := range found {
		items = append(items, resources.SocialFinding{Check: f.Check, OK: f.OK, Detail: f.Detail})
	}

	return &SocialCheckOutput{Body: fhttp.NewCollection(pagination.Page[resources.SocialFinding]{
		Items: items, Limit: len(items),
	})}, nil
}

/*
Conversation says where a conversation stands on the platform.
*/
func (c *SocialChannelController) Conversation(ctx context.Context, in *requests.SocialConversationRequest) (
	*SocialConversationOutput, error,
) {
	id, err := c.IDs.Decode(string(models.PrefixConversation), in.Conversation)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	state, err := c.Channel.Conversation(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	p := state.Profile

	return &SocialConversationOutput{Body: fhttp.NewResource(resources.SocialConversation{
		ChannelID: models.PublicID(models.PrefixChannelConnection, state.Connection.ID),
		Platform:  state.Connection.Type, Phase: string(state.Phase),
		LastWrote: moment(state.LastWrote), OpenUntil: moment(state.OpenUntil), HumanUntil: moment(state.HumanUntil),
		Profile: resources.SocialProfile{
			Name: p.Name, Username: p.Username, PictureURL: p.PictureURL, Followers: p.Followers,
			Verified: p.Verified, Follows: p.Follows,
		},
	})}, nil
}

/*
moment is the time, nil when it is not set.
*/
func moment(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}

	return &t
}

/*
socialChannel shows a connection.
*/
func socialChannel(conn models.ChannelConnection) resources.SocialChannel {
	f := channels.SocialFactsOf(conn)

	return resources.NewSocialChannel(conn, resources.SocialFacts{
		AccountID: f.AccountID, Username: f.Username, PictureURL: f.PictureURL,
	})
}

/*
channel decodes a connection's public id.
*/
func (c *SocialChannelController) channel(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixChannelConnection), public)
	if err != nil {
		return ids.Nil(), exceptions.ErrNotFound
	}

	return id, nil
}

/*
inbox decodes the inbox a body names, nil when it names none.
*/
func (c *SocialChannelController) inbox(public *string) (*ids.UUID, error) {
	if public == nil || *public == "" {
		return nil, nil //nolint:nilnil // no inbox named is the default inbox, not an error
	}

	id, err := c.IDs.Decode(string(models.PrefixInbox), *public)
	if err != nil {
		return nil, exceptions.ErrNotFound.OnField("inbox_id")
	}

	return &id, nil
}
