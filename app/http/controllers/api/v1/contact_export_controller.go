/*
|--------------------------------------------------------------------------
| Contact Export Controllers
|--------------------------------------------------------------------------
|
| POST /contacts/{contact}/exports starts building a customer's export
| (202). GET .../exports/{export} says where it stands and, once ready,
| where it downloads from; GET .../download returns the zip itself.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ContactExportOutput is one export.
*/
type ContactExportOutput struct {
	Body fhttp.Resource[resources.ContactExport]
}

/*
ContactExportZipOutput is an export's zip.
*/
type ContactExportZipOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	CacheControl       string `header:"Cache-Control"`
	Body               []byte
}

/*
ContactExportController serves customers' exports.
*/
type ContactExportController struct {
	Request  *contacts.RequestContactExport
	Show     *contacts.ShowContactExport
	Download *contacts.DownloadContactExport
	IDs      *ids.Registry
}

/*
Store starts an export.
*/
func (c *ContactExportController) Store(
	ctx context.Context, in *requests.StoreContactExportRequest,
) (*ContactExportOutput, error) {
	contact, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	row, err := c.Request.Handle(ctx, contacts.ContactExportInput{
		Membership: membership(ctx), ContactID: contact, IncludeNotes: in.Body != nil && in.Body.IncludeNotes,
	})
	if err != nil {
		return nil, err
	}

	return &ContactExportOutput{Body: fhttp.NewResource(resources.NewContactExport(row))}, nil
}

/*
Get shows an export.
*/
func (c *ContactExportController) Get(
	ctx context.Context, in *requests.ContactExportRequest,
) (*ContactExportOutput, error) {
	input, err := c.input(ctx, in)
	if err != nil {
		return nil, err
	}

	row, err := c.Show.Handle(ctx, input)
	if err != nil {
		return nil, err
	}

	return &ContactExportOutput{Body: fhttp.NewResource(resources.NewContactExport(row))}, nil
}

/*
Zip returns a ready export's zip as a download.
*/
func (c *ContactExportController) Zip(
	ctx context.Context, in *requests.ContactExportRequest,
) (*ContactExportZipOutput, error) {
	input, err := c.input(ctx, in)
	if err != nil {
		return nil, err
	}

	zip, err := c.Download.Handle(ctx, input)
	if err != nil {
		return nil, err
	}

	return &ContactExportZipOutput{
		ContentType: "application/zip", CacheControl: "no-store",
		ContentDisposition: `attachment; filename="` + in.Export + `.zip"`, Body: zip,
	}, nil
}

/*
input decodes the contact and export the request names.
*/
func (c *ContactExportController) input(
	ctx context.Context, in *requests.ContactExportRequest,
) (contacts.ContactExportInput, error) {
	contact, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return contacts.ContactExportInput{}, exceptions.ErrNotFound
	}

	export, err := c.IDs.Decode(string(models.PrefixContactExport), in.Export)
	if err != nil {
		return contacts.ContactExportInput{}, exceptions.ErrNotFound
	}

	return contacts.ContactExportInput{Membership: membership(ctx), ContactID: contact, ExportID: export}, nil
}
