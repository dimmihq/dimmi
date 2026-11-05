/*
|--------------------------------------------------------------------------
| Forwarding Test Controller
|--------------------------------------------------------------------------
|
| Sends a forwarding test to an address and reads it back while it waits
| for its mail. The settings page follows a test live and reads it here
| when a live update arrives or the live connection is down.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EmailAddressTestOutput is one forwarding test.
*/
type EmailAddressTestOutput struct {
	Body fhttp.Resource[resources.EmailAddressTest]
}

/*
EmailAddressTestController serves the forwarding tests of addresses.
*/
type EmailAddressTestController struct {
	Send *email.SendEmailAddressTest
	Show *email.ShowEmailAddressTest
	IDs  *ids.Registry
}

/*
Store sends a forwarding test to the address's public address.
*/
func (c *EmailAddressTestController) Store(
	ctx context.Context, in *requests.EmailAddressRequest,
) (*EmailAddressTestOutput, error) {
	id, err := c.decode(models.PrefixEmailAddress, in.Address)
	if err != nil {
		return nil, err
	}

	sent, err := c.Send.Handle(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return c.output(ctx, sent), nil
}

/*
ShowTest answers with one forwarding test of the address.
*/
func (c *EmailAddressTestController) ShowTest(
	ctx context.Context, in *requests.EmailAddressTestRequest,
) (*EmailAddressTestOutput, error) {
	addressID, err := c.decode(models.PrefixEmailAddress, in.Address)
	if err != nil {
		return nil, err
	}

	testID, err := c.decode(models.PrefixEmailAddressTest, in.Test)
	if err != nil {
		return nil, err
	}

	found, err := c.Show.Handle(ctx, membership(ctx), addressID, testID)
	if err != nil {
		return nil, err
	}

	return c.output(ctx, found), nil
}

/*
output wraps a test in the response.
*/
func (c *EmailAddressTestController) output(ctx context.Context, t email.AddressTest) *EmailAddressTestOutput {
	return &EmailAddressTestOutput{Body: fhttp.NewResource(resources.NewEmailAddressTest(ctx, t.Test, t.SentTo,
		t.MessageID))}
}

/*
decode reads a public id of the prefix, or answers not found.
*/
func (c *EmailAddressTestController) decode(prefix models.Prefix, named string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), named)
	if err != nil {
		return ids.Nil(), exceptions.ErrNotFound
	}

	return id, nil
}
