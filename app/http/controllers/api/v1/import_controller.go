/*
|--------------------------------------------------------------------------
| Import Controllers
|--------------------------------------------------------------------------
|
| POST /imports keeps an uploaded CSV of customers or companies as a
| draft and answers with its header, first rows and suggested mapping.
| GET /imports lists the latest of a kind and GET /imports/{import}
| shows one with its progress. POST .../previews runs a mapping as a dry
| run, POST .../runs starts the import in the background, GET
| .../errors downloads the failed rows as a CSV, and POST .../rollbacks
| undoes a finished import in the background.
|
*/

package v1

import (
	"context"
	"fmt"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/imports"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ImportOutput is one import.
*/
type ImportOutput struct {
	Body fhttp.Resource[resources.Import]
}

/*
ImportsOutput is the latest imports of a kind.
*/
type ImportsOutput struct {
	Body fhttp.Collection[resources.ImportSummary]
}

/*
ImportPredictionOutput is a dry run of an import.
*/
type ImportPredictionOutput struct {
	Body fhttp.Resource[resources.ImportPrediction]
}

/*
ImportReportOutput is the CSV of an import's failed rows.
*/
type ImportReportOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	CacheControl       string `header:"Cache-Control"`
	Body               []byte
}

/*
ImportController serves imports of customers and companies.
*/
type ImportController struct {
	Imports *imports.Imports
	IDs     *ids.Registry
}

/*
Store keeps the uploaded file as a draft.
*/
func (c *ImportController) Store(ctx context.Context, in *requests.StoreImportRequest) (*ImportOutput, error) {
	defer func() { _ = in.RawBody.RemoveAll() }()

	uploads := in.RawBody.File["file"]
	if len(uploads) == 0 {
		return nil, imports.ErrUnreadable
	}

	file, err := uploads[0].Open()
	if err != nil {
		return nil, fmt.Errorf("open uploaded import: %w", err)
	}

	defer func() { _ = file.Close() }()

	v, err := c.Imports.Create(ctx, imports.CreateInput{
		Membership: membership(ctx), Kind: in.Kind(), FileName: uploads[0].Filename, File: file,
	})
	if err != nil {
		return nil, err
	}

	return &ImportOutput{Body: fhttp.NewResource(resources.NewImport(v))}, nil
}

/*
Index lists the latest imports of the kind.
*/
func (c *ImportController) Index(ctx context.Context, in *requests.ListImportsRequest) (*ImportsOutput, error) {
	rows, err := c.Imports.List(ctx, membership(ctx), in.Kind)
	if err != nil {
		return nil, err
	}

	items := resources.NewImportSummaries(rows)
	page := pagination.Page[resources.ImportSummary]{Items: items, Limit: len(items)}

	return &ImportsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Get shows an import.
*/
func (c *ImportController) Get(ctx context.Context, in *requests.ImportRequest) (*ImportOutput, error) {
	id, err := c.decode(in.Import)
	if err != nil {
		return nil, err
	}

	v, err := c.Imports.Show(ctx, imports.Input{Membership: membership(ctx), ID: id})
	if err != nil {
		return nil, err
	}

	return &ImportOutput{Body: fhttp.NewResource(resources.NewImport(v))}, nil
}

/*
Preview runs the plan as a dry run.
*/
func (c *ImportController) Preview(
	ctx context.Context, in *requests.ImportPlanRequest,
) (*ImportPredictionOutput, error) {
	input, err := c.plan(ctx, in)
	if err != nil {
		return nil, err
	}

	p, err := c.Imports.Predict(ctx, input)
	if err != nil {
		return nil, err
	}

	return &ImportPredictionOutput{Body: fhttp.NewResource(resources.NewImportPrediction(p))}, nil
}

/*
Run starts the import in the background.
*/
func (c *ImportController) Run(ctx context.Context, in *requests.ImportPlanRequest) (*ImportOutput, error) {
	input, err := c.plan(ctx, in)
	if err != nil {
		return nil, err
	}

	v, err := c.Imports.Start(ctx, input)
	if err != nil {
		return nil, err
	}

	return &ImportOutput{Body: fhttp.NewResource(resources.NewImport(v))}, nil
}

/*
Errors downloads the import's failed rows as a CSV.
*/
func (c *ImportController) Errors(ctx context.Context, in *requests.ImportRequest) (*ImportReportOutput, error) {
	id, err := c.decode(in.Import)
	if err != nil {
		return nil, err
	}

	data, name, err := c.Imports.Report(ctx, imports.Input{Membership: membership(ctx), ID: id})
	if err != nil {
		return nil, err
	}

	return &ImportReportOutput{
		ContentType: "text/csv; charset=utf-8", CacheControl: "no-store",
		ContentDisposition: `attachment; filename="` + name + `"`, Body: data,
	}, nil
}

/*
Rollback starts undoing the import in the background.
*/
func (c *ImportController) Rollback(ctx context.Context, in *requests.ImportRequest) (*ImportOutput, error) {
	id, err := c.decode(in.Import)
	if err != nil {
		return nil, err
	}

	v, err := c.Imports.Rollback(ctx, imports.Input{Membership: membership(ctx), ID: id})
	if err != nil {
		return nil, err
	}

	return &ImportOutput{Body: fhttp.NewResource(resources.NewImport(v))}, nil
}

/*
plan decodes the import and the plan a request sends.
*/
func (c *ImportController) plan(ctx context.Context, in *requests.ImportPlanRequest) (imports.Input, error) {
	id, err := c.decode(in.Import)
	if err != nil {
		return imports.Input{}, err
	}

	mapping := make([]imports.Mapping, 0, len(in.Body.Mapping))
	for _, m := range in.Body.Mapping {
		mapping = append(mapping, imports.Mapping{Column: m.Column, Field: m.Field})
	}

	return imports.Input{
		Membership: membership(ctx), ID: id, Plan: imports.Plan{Mapping: mapping, Mode: in.Body.Mode},
	}, nil
}

/*
decode reads an import's public id.
*/
func (c *ImportController) decode(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixImport), public)
	if err != nil {
		return id, exceptions.ErrNotFound
	}

	return id, nil
}
