/*
|--------------------------------------------------------------------------
| WhatsApp Channel Controller
|--------------------------------------------------------------------------
|
| The settings endpoints of WhatsApp: the numbers a workspace connected,
| what Meta's popup needs to open, an account read with a token before a
| number is picked, connecting by token or by the popup, moving a number
| to another inbox and disconnecting it, and its templates. In the inbox,
| where a conversation stands on WhatsApp and a template sent as a reply.
| Connecting needs channels:manage; an id of another workspace's
| connection is simply not found.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/messages"
	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
WhatsAppConversationOutput is where a conversation stands on WhatsApp.
*/
type WhatsAppConversationOutput struct {
	Body fhttp.Resource[resources.WhatsAppConversation]
}

/*
WhatsAppChannelController serves WhatsApp's settings and its part of
the inbox.
*/
type WhatsAppChannelController struct {
	WhatsApp *channels.WhatsApp
	Reply    *messages.ReplyToConversation
	Service  appconfig.WhatsAppServiceConfig
	Version  string
	IDs      *ids.Registry
}

/*
Index lists the workspace's WhatsApp connections.
*/
func (c *WhatsAppChannelController) Index(ctx context.Context, _ *struct{}) (*WhatsAppChannelsOutput, error) {
	rows, err := c.WhatsApp.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.WhatsAppChannel, 0, len(rows))
	for _, row := range rows {
		items = append(items, c.resource(ctx, row))
	}

	page := pagination.Page[resources.WhatsAppChannel]{Items: items, Limit: 100}

	return &WhatsAppChannelsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
SignupConfig says what Meta's popup needs to open.
*/
func (c *WhatsAppChannelController) SignupConfig(ctx context.Context, _ *struct{}) (
	*WhatsAppSignupConfigOutput, error,
) {
	m := membership(ctx)
	if !m.Can("channels:manage") {
		return nil, exceptions.ErrForbidden
	}

	out := &WhatsAppSignupConfigOutput{}
	out.Body.Data.Available = c.Service.SignupReady()
	out.Body.Data.GraphVersion = c.Version

	if out.Body.Data.Available {
		out.Body.Data.AppID, out.Body.Data.ConfigID = c.Service.AppID, c.Service.ConfigID
	}

	return out, nil
}

/*
Account reads a WhatsApp account and its numbers with a token.
*/
func (c *WhatsAppChannelController) Account(ctx context.Context, in *requests.WhatsAppAccountRequest) (
	*WhatsAppAccountOutput, error,
) {
	account, err := c.WhatsApp.Account(ctx, channels.WhatsAppTokenInput{
		Membership: membership(ctx), Token: in.Body.AccessToken, AccountID: in.Body.WabaID,
		AppSecret: in.Body.AppSecret,
	})
	if err != nil {
		return nil, err
	}

	numbers := make([]resources.WhatsAppNumber, 0, len(account.Numbers))
	for _, n := range account.Numbers {
		numbers = append(numbers, resources.WhatsAppNumber{
			ID: n.ID, DisplayPhoneNumber: n.DisplayPhoneNumber, VerifiedName: n.VerifiedName,
			QualityRating: n.QualityRating, Connected: n.Connected,
		})
	}

	return &WhatsAppAccountOutput{Body: fhttp.NewResource(resources.WhatsAppAccount{
		ID: account.ID, Name: account.Name, Business: account.Business, Numbers: numbers,
	})}, nil
}

/*
Store connects a number with a token.
*/
func (c *WhatsAppChannelController) Store(ctx context.Context, in *requests.StoreWhatsAppChannelRequest) (
	*WhatsAppChannelOutput, error,
) {
	inbox, err := c.inbox(in.Body.InboxID)
	if err != nil {
		return nil, err
	}

	conn, err := c.WhatsApp.Connect(ctx, channels.ConnectWhatsAppInput{
		WhatsAppTokenInput: channels.WhatsAppTokenInput{
			Membership: membership(ctx), Token: in.Body.AccessToken, AccountID: in.Body.WabaID,
			AppSecret: in.Body.AppSecret,
		},
		PhoneNumberID: in.Body.PhoneNumberID, InboxID: inbox, Name: in.Body.Name,
	})
	if err != nil {
		return nil, err
	}

	return &WhatsAppChannelOutput{Body: fhttp.NewResource(c.resource(ctx, conn))}, nil
}

/*
Signup connects the number Meta's popup handed back.
*/
func (c *WhatsAppChannelController) Signup(ctx context.Context, in *requests.StoreWhatsAppSignupRequest) (
	*WhatsAppChannelOutput, error,
) {
	inbox, err := c.inbox(in.Body.InboxID)
	if err != nil {
		return nil, err
	}

	conn, err := c.WhatsApp.Signup(ctx, channels.WhatsAppSignupInput{
		Membership: membership(ctx), Code: in.Body.Code, RedirectURI: in.Body.RedirectURI, AccountID: in.Body.WabaID,
		PhoneNumberID: in.Body.PhoneNumberID, BusinessID: in.Body.BusinessID, Coexistence: in.Body.Coexistence,
		InboxID: inbox,
	})
	if err != nil {
		return nil, err
	}

	return &WhatsAppChannelOutput{Body: fhttp.NewResource(c.resource(ctx, conn))}, nil
}

/*
Update moves a number's new conversations to another inbox.
*/
func (c *WhatsAppChannelController) Update(ctx context.Context, in *requests.UpdateWhatsAppChannelRequest) (
	*WhatsAppChannelOutput, error,
) {
	id, err := c.channel(in.Channel)
	if err != nil {
		return nil, err
	}

	inbox, err := c.inbox(in.Body.InboxID)
	if err != nil {
		return nil, err
	}

	conn, err := c.WhatsApp.Update(ctx, membership(ctx), id, inbox)
	if err != nil {
		return nil, err
	}

	return &WhatsAppChannelOutput{Body: fhttp.NewResource(c.resource(ctx, conn))}, nil
}

/*
Destroy disconnects a number.
*/
func (c *WhatsAppChannelController) Destroy(ctx context.Context, in *requests.WhatsAppChannelRequest) (
	*EmptyOutput, error,
) {
	id, err := c.channel(in.Channel)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.WhatsApp.Disconnect(ctx, membership(ctx), id)
}

/*
Templates lists a number's templates, taking a new copy from Meta when
asked.
*/
func (c *WhatsAppChannelController) Templates(ctx context.Context, in *requests.WhatsAppTemplatesRequest) (
	*WhatsAppTemplatesOutput, error,
) {
	id, err := c.channel(in.Channel)
	if err != nil {
		return nil, err
	}

	load := c.WhatsApp.Templates
	if in.Refresh {
		load = c.WhatsApp.RefreshTemplates
	}

	rows, err := load(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.WhatsAppTemplate]{Items: templateResources(rows), Limit: 1000}

	return &WhatsAppTemplatesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Conversation says where a conversation stands on WhatsApp.
*/
func (c *WhatsAppChannelController) Conversation(ctx context.Context, in *requests.WhatsAppConversationRequest) (
	*WhatsAppConversationOutput, error,
) {
	id, err := c.IDs.Decode(string(models.PrefixConversation), in.Conversation)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	state, err := c.WhatsApp.Conversation(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	out := resources.WhatsAppConversation{
		ChannelID: models.PublicID(models.PrefixChannelConnection, state.Connection.ID),
		Window:    resources.WhatsAppWindow{Open: state.Open},
		Contact: resources.WhatsAppContact{
			Name: textOf(state.Contact.Name), Email: textOf(state.Contact.Email), Phone: textOf(state.Contact.Phone),
		},
		Templates: templateResources(state.Templates),
	}

	if !state.ExpiresAt.IsZero() {
		expires := state.ExpiresAt
		out.Window.ExpiresAt = &expires
	}

	return &WhatsAppConversationOutput{Body: fhttp.NewResource(out)}, nil
}

/*
SendTemplate sends a template as a reply to the conversation.
*/
func (c *WhatsAppChannelController) SendTemplate(ctx context.Context, in *requests.WhatsAppTemplateReplyRequest) (
	*WhatsAppTemplateReplyOutput, error,
) {
	id, err := c.IDs.Decode(string(models.PrefixConversation), in.Conversation)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	m := membership(ctx)

	reply, err := c.WhatsApp.PrepareTemplate(ctx, m, id, wadriver.Choice{
		Name: in.Body.Name, Language: in.Body.Language, Values: in.Body.Values,
	})
	if err != nil {
		return nil, err
	}

	written, err := c.Reply.Handle(ctx, messages.ReplyInput{
		Membership: m, Ref: conversations.Ref{ID: id}, Body: reply.Body, ClientMessageID: in.Body.ClientMessageID,
		Then: messages.ThenKeep, MutationID: mutationID(ctx), ChannelOptions: reply.Options,
	})
	if err != nil {
		return nil, err
	}

	sent := writtenOutput(written)
	out := &WhatsAppTemplateReplyOutput{Status: sent.Status, Replayed: sent.Replayed}
	out.Body.Data = sent.Body.Data
	out.Body.Meta.Conversation = sent.Body.Meta.Conversation

	return out, nil
}

/*
resource shows the connection, with its webhook setup for a
workspace's own Meta app.
*/
func (c *WhatsAppChannelController) resource(
	ctx context.Context, conn models.ChannelConnection,
) resources.WhatsAppChannel {
	f := channels.WhatsAppFactsOf(conn)

	var hook *resources.WhatsAppHookup
	if address, token := c.WhatsApp.WebhookAddress(ctx, conn); address != "" {
		hook = &resources.WhatsAppHookup{CallbackURL: address, VerifyToken: token}
	}

	return resources.NewWhatsAppChannel(conn, resources.WhatsAppFacts{
		Mode: f.Mode, DisplayPhoneNumber: f.DisplayPhoneNumber, VerifiedName: f.VerifiedName,
		NameStatus: f.NameStatus, QualityRating: f.QualityRating, MessagingLimit: f.MessagingLimit,
		WabaID: f.AccountID, PhoneNumberID: f.PhoneNumberID, Coexistence: f.Coexistence,
	}, hook)
}

/*
channel decodes a WhatsApp connection's public id.
*/
func (c *WhatsAppChannelController) channel(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixChannelConnection), public)
	if err != nil {
		return ids.Nil(), exceptions.ErrNotFound
	}

	return id, nil
}

/*
inbox decodes the inbox a body names, nil when it names none.
*/
func (c *WhatsAppChannelController) inbox(public *string) (*ids.UUID, error) {
	if public == nil || *public == "" {
		return nil, nil
	}

	id, err := c.IDs.Decode(string(models.PrefixInbox), *public)
	if err != nil {
		return nil, exceptions.ErrNotFound.OnField("inbox_id")
	}

	return &id, nil
}

/*
templateResources shows templates as the composer reads them.
*/
func templateResources(rows []channels.WhatsAppTemplate) []resources.WhatsAppTemplate {
	out := make([]resources.WhatsAppTemplate, 0, len(rows))

	for _, t := range rows {
		vars := make([]resources.WhatsAppVariable, 0, len(t.Variables))
		for _, v := range t.Variables {
			vars = append(vars, resources.WhatsAppVariable{Part: v.Part, Key: v.Key})
		}

		out = append(out, resources.WhatsAppTemplate{
			ID: models.PublicID(models.PrefixWhatsAppTemplate, t.Row.ID), Name: t.Row.Name,
			Language: t.Row.Language, Category: t.Row.Category, Status: t.Row.Status, Sendable: t.Row.Sendable,
			RejectedReason: t.Row.RejectedReason, Variables: vars, Preview: t.Preview, Content: contentOf(t.Template),
			Bindings: t.Row.Bindings, SyncedAt: t.Row.SyncedAt,
		})
	}

	return out
}

/*
textOf returns the text, empty when there is none.
*/
func textOf(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
