/*
|--------------------------------------------------------------------------
| Migration Controllers
|--------------------------------------------------------------------------
|
| POST /migrations starts a migration from an account of another tool,
| or resumes the one under way for it. GET /migrations lists them and
| GET /migrations/{migration} shows one with its progress. POST
| .../files uploads one attachment or picture as the raw body and
| answers with its file id; POST .../records applies a batch of records
| of one kind, answering per record; POST .../rollbacks undoes
| everything the migration created, in the background.
|
*/

package v1

import (
	"context"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/actions/migrations"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MigrationOutput is one migration.
*/
type MigrationOutput struct {
	Status int
	Body   fhttp.Resource[resources.Migration]
}

/*
MigrationsOutput is the workspace's migrations.
*/
type MigrationsOutput struct {
	Body fhttp.Collection[resources.MigrationSummary]
}

/*
MigrationBatchOutput is what a batch of records came to.
*/
type MigrationBatchOutput struct {
	Body fhttp.Resource[resources.MigrationBatch]
}

/*
MigrationLookupsOutput is where the records looked up stand.
*/
type MigrationLookupsOutput struct {
	Body fhttp.Collection[resources.MigrationLookup]
}

/*
MigrationController serves migrations from other tools.
*/
type MigrationController struct {
	Migrations *migrations.Migrations
	IDs        *ids.Registry
	Disks      messages.Disks
	Errors     fhttp.Errors
}

/*
Store starts the migration, 201, or resumes the one under way for the
same account, 200.
*/
func (c *MigrationController) Store(ctx context.Context, in *requests.StoreMigrationRequest) (*MigrationOutput, error) {
	m, created, err := c.Migrations.Start(ctx, migrations.StartInput{
		Membership: membership(ctx), Source: in.Body.Source, SourceAccount: in.Body.SourceAccount, Name: in.Body.Name,
	})
	if err != nil {
		return nil, err
	}

	status := fhttp.StatusOK
	if created {
		status = fhttp.StatusCreated
	}

	return &MigrationOutput{Status: status, Body: fhttp.NewResource(resources.NewMigration(m))}, nil
}

/*
Index lists the workspace's latest migrations.
*/
func (c *MigrationController) Index(ctx context.Context, _ *struct{}) (*MigrationsOutput, error) {
	rows, err := c.Migrations.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := resources.NewMigrationSummaries(rows)
	page := pagination.Page[resources.MigrationSummary]{Items: items, Limit: len(items)}

	return &MigrationsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Get shows the migration with its progress.
*/
func (c *MigrationController) Get(ctx context.Context, in *requests.MigrationRequest) (*MigrationOutput, error) {
	id, err := c.decode(in.Migration)
	if err != nil {
		return nil, err
	}

	m, err := c.Migrations.Show(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return &MigrationOutput{Status: fhttp.StatusOK, Body: fhttp.NewResource(resources.NewMigration(m))}, nil
}

/*
Records applies a batch of records.
*/
func (c *MigrationController) Records(
	ctx context.Context, in *requests.MigrationRecordsRequest,
) (*MigrationBatchOutput, error) {
	id, err := c.decode(in.Migration)
	if err != nil {
		return nil, err
	}

	b, err := c.Migrations.Apply(ctx, migrations.BatchInput{
		Membership: membership(ctx), MigrationID: id, Kind: in.Body.Kind, Records: in.Body.Records,
	})
	if err != nil {
		return nil, err
	}

	return &MigrationBatchOutput{Body: fhttp.NewResource(resources.NewMigrationBatch(in.Body.Kind, b))}, nil
}

/*
Lookups answers where each record named stands.
*/
func (c *MigrationController) Lookups(
	ctx context.Context, in *requests.MigrationLookupRequest,
) (*MigrationLookupsOutput, error) {
	id, err := c.decode(in.Migration)
	if err != nil {
		return nil, err
	}

	found, err := c.Migrations.Lookup(ctx, migrations.LookupInput{
		Membership: membership(ctx), MigrationID: id, Kind: in.Body.Kind, SourceIDs: in.Body.SourceIDs,
	})
	if err != nil {
		return nil, err
	}

	items := resources.NewMigrationLookups(in.Body.Kind, found)
	page := pagination.Page[resources.MigrationLookup]{Items: items, Limit: len(items)}

	return &MigrationLookupsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Rollback starts undoing the migration in the background.
*/
func (c *MigrationController) Rollback(ctx context.Context, in *requests.MigrationRequest) (*MigrationOutput, error) {
	id, err := c.decode(in.Migration)
	if err != nil {
		return nil, err
	}

	m, err := c.Migrations.Rollback(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return &MigrationOutput{Status: fhttp.StatusAccepted, Body: fhttp.NewResource(resources.NewMigration(m))}, nil
}

/*
Files keeps the raw body as a file of the migration and answers with
it: 201 for a new file, 200 for one uploaded before under the same
source id.
*/
func (c *MigrationController) Files(w fhttp.ResponseWriter, r *fhttp.Request) {
	ctx := r.Context()

	id, err := c.decode(r.PathValue("migration"))
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	filename := strings.TrimSpace(r.URL.Query().Get("filename"))
	if filename == "" || r.ContentLength <= 0 {
		c.Errors.Render(w, r, exceptions.ErrAttachmentSizeMismatch)

		return
	}

	row, replayed, err := c.Migrations.Upload(ctx, migrations.FileInput{
		Membership: membership(ctx), MigrationID: id, SourceID: r.URL.Query().Get("source_id"),
		Filename: filename, ContentType: contentType(r), SizeBytes: r.ContentLength,
		Body: fhttp.MaxBytesReader(w, r.Body, migrations.MaxMigrationFileBytes+1),
	})
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	status := fhttp.StatusCreated
	if replayed {
		status = fhttp.StatusOK
	}

	file := resources.NewAttachment(messages.PresentFile(ctx, c.Disks, row))
	fhttp.WriteJSON(w, status, fhttp.NewResource(file))
}

/*
decode reads a migration's public id, a missing migration for one that
is not.
*/
func (c *MigrationController) decode(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixMigration), public)
	if err != nil {
		return id, migrations.ErrNotFound
	}

	return id, nil
}
