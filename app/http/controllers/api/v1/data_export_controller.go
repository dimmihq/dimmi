/*
|--------------------------------------------------------------------------
| Data Export Controllers
|--------------------------------------------------------------------------
|
| POST /data-exports starts an export (202); GET /data-exports lists the
| caller's own; GET and DELETE /data-exports/{data_export} show and
| remove one, and GET .../download returns its file. A selection by
| filter reads the same filters the conversation list and the customer
| directory read, so an export covers exactly what the member was
| looking at.
|
*/

package v1

import (
	"context"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/dataexports"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DataExportOutput is one export.
*/
type DataExportOutput struct {
	Body fhttp.Resource[resources.DataExport]
}

/*
DataExportsOutput is the caller's exports.
*/
type DataExportsOutput struct {
	Body fhttp.Collection[resources.DataExport]
}

/*
DataExportFileOutput is an export's file.
*/
type DataExportFileOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	CacheControl       string `header:"Cache-Control"`
	Body               []byte
}

/*
DataExportController serves members' data exports.
*/
type DataExportController struct {
	Request  *dataexports.RequestDataExport
	List     *dataexports.ListDataExports
	Show     *dataexports.ShowDataExport
	Download *dataexports.DownloadDataExport
	Remove   *dataexports.DeleteDataExport
	IDs      *ids.Registry
	Clock    clock.Clock
}

/*
Store starts an export.
*/
func (c *DataExportController) Store(
	ctx context.Context, in *requests.StoreDataExportRequest,
) (*DataExportOutput, error) {
	input := dataexports.RequestInput{
		Membership: membership(ctx), Kind: in.Body.Kind, Format: in.Body.Format, IncludeNotes: in.Body.IncludeNotes,
	}

	var err error
	if in.Body.Kind == dataexports.KindContacts {
		input.Contacts, err = c.contactSelection(ctx, in.Body.Selection)
	} else {
		input.Conversations, err = c.conversationSelection(in.Body.Selection)
	}

	if err != nil {
		return nil, err
	}

	row, err := c.Request.Handle(ctx, input)
	if err != nil {
		return nil, err
	}

	return &DataExportOutput{Body: fhttp.NewResource(resources.NewDataExport(row))}, nil
}

/*
Index lists the caller's exports.
*/
func (c *DataExportController) Index(ctx context.Context, _ *requests.NoInput) (*DataExportsOutput, error) {
	rows, err := c.List.Handle(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.DataExport]{Items: resources.NewDataExports(rows), Limit: len(rows)}

	return &DataExportsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Get shows an export.
*/
func (c *DataExportController) Get(ctx context.Context, in *requests.DataExportRequest) (*DataExportOutput, error) {
	input, err := c.input(ctx, in)
	if err != nil {
		return nil, err
	}

	row, err := c.Show.Handle(ctx, input)
	if err != nil {
		return nil, err
	}

	return &DataExportOutput{Body: fhttp.NewResource(resources.NewDataExport(row))}, nil
}

/*
File returns a ready export's file as a download.
*/
func (c *DataExportController) File(
	ctx context.Context, in *requests.DataExportRequest,
) (*DataExportFileOutput, error) {
	input, err := c.input(ctx, in)
	if err != nil {
		return nil, err
	}

	file, err := c.Download.Handle(ctx, input)
	if err != nil {
		return nil, err
	}

	return &DataExportFileOutput{
		ContentType: file.ContentType, CacheControl: "no-store",
		ContentDisposition: `attachment; filename="` + file.Name + `"`, Body: file.Data,
	}, nil
}

/*
Destroy deletes an export and its file.
*/
func (c *DataExportController) Destroy(ctx context.Context, in *requests.DataExportRequest) (*EmptyOutput, error) {
	input, err := c.input(ctx, in)
	if err != nil {
		return nil, err
	}

	if err := c.Remove.Handle(ctx, input); err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
input decodes the export the request names.
*/
func (c *DataExportController) input(
	ctx context.Context, in *requests.DataExportRequest,
) (dataexports.ExportInput, error) {
	id, err := c.IDs.Decode(string(models.PrefixDataExport), in.Export)
	if err != nil {
		return dataexports.ExportInput{}, exceptions.ErrNotFound
	}

	return dataexports.ExportInput{Membership: membership(ctx), ExportID: id}, nil
}

/*
conversationSelection reads the conversations picked, or the list's
filters, a saved view standing for filter[view].
*/
func (c *DataExportController) conversationSelection(sel requests.ExportSelection) (conversations.Selection, error) {
	if len(sel.IDs) > 0 {
		picked, err := decodeAll(c.IDs, models.PrefixConversation, sel.IDs)

		return conversations.Selection{IDs: picked}, err
	}

	query := selectionQuery(sel)
	if sel.ViewID != "" {
		query.Filters["view"] = []string{sel.ViewID}
	}

	lq, err := parseListQuery(c.IDs, query)
	if err != nil {
		return conversations.Selection{}, err
	}

	return conversations.Selection{Filters: &lq.filters}, nil
}

/*
contactSelection reads the customers picked, or the directory's filters.
*/
func (c *DataExportController) contactSelection(
	ctx context.Context, sel requests.ExportSelection,
) (dataexports.ContactSelection, error) {
	if len(sel.IDs) > 0 {
		picked, err := decodeAll(c.IDs, models.PrefixContact, sel.IDs)

		return dataexports.ContactSelection{IDs: picked}, err
	}

	in, err := (&ContactController{IDs: c.IDs}).listInput(ctx, selectionQuery(sel))
	if err != nil {
		return dataexports.ContactSelection{}, err
	}

	conditions, members, err := conditionsOf(sel.Conditions, c.IDs, c.Clock)
	if err != nil {
		return dataexports.ContactSelection{}, err
	}

	return dataexports.ContactSelection{Filters: &dataexports.ContactFilters{
		Kind: in.Kind, Email: in.Email, ExternalID: in.ExternalID, Blocked: in.Blocked, CompanyID: in.CompanyID,
		TagIDs: in.TagIDs, SeenAfter: in.Seen.From, SeenBefore: in.Seen.To, CreatedAfter: in.Created.From,
		CreatedBefore: in.Created.To, Custom: in.Custom, Conditions: conditions, Audiences: members,
		Search: in.Search,
	}}, nil
}

/*
selectionQuery turns a selection's filters and search into the list
query the list endpoints read.
*/
func selectionQuery(sel requests.ExportSelection) pagination.Query {
	query := pagination.Query{Filters: map[string][]string{}, Search: sel.Q}
	for name, value := range sel.Filter {
		query.Filters[name] = []string{value}
	}

	return query
}

/*
decodeAll decodes picked public ids, refusing the selection when one is
not of the kind.
*/
func decodeAll(registry *ids.Registry, prefix models.Prefix, public []string) ([]ids.UUID, error) {
	out := make([]ids.UUID, 0, len(public))

	for _, p := range public {
		id, err := registry.Decode(string(prefix), p)
		if err != nil {
			return nil, exceptions.ErrInvalidSelection
		}

		out = append(out, id)
	}

	return out, nil
}
