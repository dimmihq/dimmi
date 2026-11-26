/*
|--------------------------------------------------------------------------
| Migration Routes
|--------------------------------------------------------------------------
|
| The API a client uses to bring a workspace's history in from another
| tool in dimmi's import format: start or resume a migration, follow
| it, upload its files, send its records in batches and undo it. Every
| route takes settings:write; the actions also check imports:manage,
| and imports:rollback to undo. Migrations count against a rate limit
| of their own, so one never slows the workspace's other integrations.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/migrations"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
migrationRoutes declares the migrations API.
*/
func migrationRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.Migrations == nil {
		return
	}

	c := &v1.MigrationController{Migrations: a.Actions.Migrations, IDs: a.IDs, Disks: a.Storage, Errors: a.Errors}
	one := "/migrations/{migration}"

	routing.Post(workspace, "/migrations", "migrations.store", c.Store).Ability("settings:write").
		Status(fhttp.StatusCreated)
	routing.Get(workspace, "/migrations", "migrations.index", c.Index).Ability("settings:write")
	routing.Get(workspace, one, "migrations.show", c.Get).Ability("settings:write")
	workspace.Handle(fhttp.MethodPost, one+"/files", "migrations.files.store", fhttp.HandlerFunc(c.Files)).
		Ability("settings:write").Use(middleware.BodyLimit(migrations.MaxMigrationFileBytes+uploadOverhead)).
		Query(
			routing.QueryParam{Name: "filename", Description: "Name of the uploaded file. Required."},
			routing.QueryParam{
				Name:        "source_id",
				Description: "Id of the file in the other tool; uploading it again returns the first copy.",
			},
		)
	routing.Post(workspace, one+"/records", "migrations.records.store", c.Records).Ability("settings:write").
		Use(middleware.BodyLimit(migrations.MaxBatchBytes)).Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/lookups", "migrations.lookups.store", c.Lookups).Ability("settings:write").
		Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/rollbacks", "migrations.rollbacks.store", c.Rollback).Ability("settings:write").
		Status(fhttp.StatusAccepted)
}
