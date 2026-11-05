/*
|--------------------------------------------------------------------------
| Email Address Controller
|--------------------------------------------------------------------------
|
| The email addresses of the workspace in play: listing them with their
| forwarding addresses and setup steps, connecting a new one, changing
| how one replies, disconnecting it, sending a forwarding test, and the
| log of what arrived at it. Ids of other workspaces' addresses are
| simply not found.
|
*/

package v1

import (
	"context"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EmailAddressesOutput is the workspace's addresses.
*/
type EmailAddressesOutput struct {
	Body fhttp.Collection[resources.EmailAddress]
}

/*
EmailAddressOutput is one address.
*/
type EmailAddressOutput struct {
	Body fhttp.Resource[resources.EmailAddress]
}

/*
EmailAddressTestOutput is the outcome of a forwarding test.
*/
type EmailAddressTestOutput struct {
	Body fhttp.Resource[resources.EmailAddressTest]
}

/*
InboundEmailsOutput is a page of what arrived at an address.
*/
type InboundEmailsOutput struct {
	Body fhttp.Collection[resources.InboundEmail]
}

/*
EmailAddressController serves the workspace's email addresses. Provider
is the install's customer-stream transport, shown on each address,
Senders works out what each address sends as, and Guides picks the setup
guide each address opens on.
*/
type EmailAddressController struct {
	Create   *email.CreateEmailAddress
	List     *email.ListEmailAddresses
	Show     *email.ShowEmailAddress
	Change   *email.UpdateEmailAddress
	Remove   *email.DeleteEmailAddress
	Test     *email.SendEmailAddressTest
	Activity *email.ListInboundEmails
	IDs      *ids.Registry
	Cursors  *pagination.Cursors
	Provider string
	Senders  *email.Domains
	Guides   *email.RecommendGuide
}

/*
inboundPage is where a page of received emails starts, carried in its
cursor.
*/
type inboundPage struct {
	ReceivedAt time.Time `json:"r"`
	ID         ids.UUID  `json:"i"`
}

/*
Index lists the addresses, the default first.
*/
func (c *EmailAddressController) Index(ctx context.Context, _ *requests.NoInput) (*EmailAddressesOutput, error) {
	rows, err := c.List.Handle(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items, err := c.sending(ctx, c.guides(ctx, resources.NewEmailAddresses(rows, c.Provider)))
	if err != nil {
		return nil, err
	}

	return &EmailAddressesOutput{Body: fhttp.NewCollection(pagination.Page[resources.EmailAddress]{
		Items: items, Limit: len(items),
	})}, nil
}

/*
Store connects a public address and answers with its forwarding
address.
*/
func (c *EmailAddressController) Store(
	ctx context.Context, in *requests.StoreEmailAddressRequest,
) (*EmailAddressOutput, error) {
	input := email.CreateEmailAddressInput{Membership: membership(ctx), Address: in.Body.Address}
	if in.Body.DisplayName != nil {
		input.DisplayName = *in.Body.DisplayName
	}

	if in.Body.InboxID != nil {
		inbox, err := c.IDs.Decode(string(models.PrefixInbox), *in.Body.InboxID)
		if err != nil {
			return nil, exceptions.ErrNotFound
		}

		input.InboxID = &inbox
	}

	created, err := c.Create.Handle(ctx, input)
	if err != nil {
		return nil, err
	}

	return c.answer(ctx, created.ID)
}

/*
ShowAddress answers with one address.
*/
func (c *EmailAddressController) ShowAddress(
	ctx context.Context, in *requests.EmailAddressRequest,
) (*EmailAddressOutput, error) {
	id, err := c.addressID(in.Address)
	if err != nil {
		return nil, err
	}

	return c.answer(ctx, id)
}

/*
Update changes how the address replies and answers with it.
*/
func (c *EmailAddressController) Update(
	ctx context.Context, in *requests.UpdateEmailAddressRequest,
) (*EmailAddressOutput, error) {
	id, err := c.addressID(in.Address)
	if err != nil {
		return nil, err
	}

	b := in.Body
	update := email.UpdateEmailAddressInput{
		Membership: membership(ctx), ID: id, DisplayName: b.DisplayName,
		ReplyToMode: (*enums.ReplyToMode)(b.ReplyToMode), FromNameStyle: (*enums.FromNameStyle)(b.FromNameStyle),
	}

	if b.Settings != nil {
		update.QuotePrevious = b.Settings.QuotePrevious
		update.BulkMailHandling = (*enums.BulkMailHandling)(b.Settings.BulkMailHandling)
		update.Style = replyStyle(b.Settings.ReplyStyle)
	}

	update.IsDefault = b.IsDefault

	if b.InboxID != nil {
		inbox, err := c.IDs.Decode(string(models.PrefixInbox), *b.InboxID)
		if err != nil {
			return nil, exceptions.ErrInboxNotFound
		}

		update.InboxID = &inbox
	}

	row, err := c.Change.Handle(ctx, update)
	if err != nil {
		return nil, err
	}

	items, err := c.sending(ctx, c.guides(ctx, []resources.EmailAddress{c.shape(row)}))
	if err != nil {
		return nil, err
	}

	return &EmailAddressOutput{Body: fhttp.NewResource(items[0])}, nil
}

/*
sending fills in what each address sends as now.
*/
func (c *EmailAddressController) sending(
	ctx context.Context, items []resources.EmailAddress,
) ([]resources.EmailAddress, error) {
	if c.Senders == nil {
		return items, nil
	}

	states, err := c.Senders.WorkspaceSendingStates(ctx, membership(ctx).Workspace.ID)
	if err != nil {
		return nil, err
	}

	for i, item := range items {
		items[i] = item.WithSending(states.For(item.Address, item.ForwardingAddress))
	}

	return items, nil
}

/*
guides fills in each address's setup guides, the one that fits its
domain first.
*/
func (c *EmailAddressController) guides(ctx context.Context, items []resources.EmailAddress) []resources.EmailAddress {
	for i, item := range items {
		items[i] = item.WithGuides(ctx, c.Guides.Handle(ctx, item.Address))
	}

	return items
}

/*
Destroy disconnects the address.
*/
func (c *EmailAddressController) Destroy(ctx context.Context, in *requests.EmailAddressRequest) (*EmptyOutput, error) {
	id, err := c.addressID(in.Address)
	if err != nil {
		return nil, err
	}

	if err := c.Remove.Handle(ctx, membership(ctx), id); err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
StoreTest sends a forwarding test to the address's public address.
*/
func (c *EmailAddressController) StoreTest(
	ctx context.Context, in *requests.EmailAddressRequest,
) (*EmailAddressTestOutput, error) {
	id, err := c.addressID(in.Address)
	if err != nil {
		return nil, err
	}

	sent, err := c.Test.Handle(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return &EmailAddressTestOutput{Body: fhttp.NewResource(resources.EmailAddressTest{
		Object: "email_address_test", Status: "sent", SentTo: sent.SentTo, MessageID: sent.MessageID,
	})}, nil
}

/*
InboundEmails lists a page of what arrived at the address, newest
first; filter[status] narrows it.
*/
func (c *EmailAddressController) InboundEmails(
	ctx context.Context, in *requests.ListInboundEmailsRequest,
) (*InboundEmailsOutput, error) {
	id, err := c.addressID(in.Address)
	if err != nil {
		return nil, err
	}

	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), pagination.Allow{Filters: []string{"status"}})
	if err != nil {
		return nil, err
	}

	input, err := c.activityInput(ctx, id, query)
	if err != nil {
		return nil, err
	}

	page, err := c.Activity.Handle(ctx, input)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.InboundEmail]{Items: resources.NewInboundEmails(page.Emails), Limit: input.Limit}
	if page.More {
		last := page.Emails[len(page.Emails)-1]
		if out.NextCursor, err = c.Cursors.Encode(inboundPage{ReceivedAt: last.ReceivedAt, ID: last.ID}); err != nil {
			return nil, err
		}
	}

	return &InboundEmailsOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
activityInput turns the list query into the action's input.
*/
func (c *EmailAddressController) activityInput(
	ctx context.Context, id ids.UUID, query pagination.Query,
) (email.ListInboundEmailsInput, error) {
	in := email.ListInboundEmailsInput{Membership: membership(ctx), AddressID: id, Limit: query.Limit}

	if statuses := query.Filters["status"]; len(statuses) > 0 {
		if !enums.InboundEmailStatus(statuses[0]).Valid() {
			return in, pagination.ErrInvalidFilter
		}

		in.Status = &statuses[0]
	}

	if query.Cursor != "" {
		var start inboundPage
		if err := c.Cursors.Decode(query.Cursor, &start); err != nil {
			return in, err
		}

		in.Before, in.BeforeID = &start.ReceivedAt, &start.ID
	}

	return in, nil
}

/*
answer responds with the address as it now is.
*/
func (c *EmailAddressController) answer(ctx context.Context, id ids.UUID) (*EmailAddressOutput, error) {
	row, err := c.Show.Handle(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	items, err := c.sending(ctx, c.guides(ctx, []resources.EmailAddress{c.shape(row)}))
	if err != nil {
		return nil, err
	}

	return &EmailAddressOutput{Body: fhttp.NewResource(items[0])}, nil
}

/*
shape returns the resource of an address row.
*/
func (c *EmailAddressController) shape(row models.FindEmailAddressRow) resources.EmailAddress {
	return resources.NewEmailAddress(resources.EmailAddressRow{
		Address: row.EmailAddress, InboxID: models.PublicID(models.PrefixInbox, row.InboxID),
		InboxName: row.InboxName, ConnectionStatus: row.ConnectionStatus,
	}, c.Provider)
}

/*
addressID reads an address's public id.
*/
func (c *EmailAddressController) addressID(named string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixEmailAddress), named)
	if err != nil {
		return ids.Nil(), exceptions.ErrNotFound
	}

	return id, nil
}
