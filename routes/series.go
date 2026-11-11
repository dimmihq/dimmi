/*
|--------------------------------------------------------------------------
| Series Routes
|--------------------------------------------------------------------------
|
| The API of series. Reading takes series:read and building drafts
| series:write; anything that reaches customers, publishing, pausing,
| archiving, adding or removing people, also needs the publish
| permission the action checks.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
seriesRoutes declares the series API.
*/
func seriesRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.Series == nil {
		return
	}

	sc := &v1.SeriesController{Series: a.Actions.Series, IDs: a.IDs}
	one := "/series/{series}"

	routing.Get(workspace, "/series", "series.index", sc.Index).Ability("series:read")
	routing.Post(workspace, "/series", "series.store", sc.Store).Ability("series:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, one, "series.show", sc.Show).Ability("series:read")
	routing.Patch(workspace, one, "series.update", sc.Update).Ability("series:write")
	routing.Delete(workspace, one, "series.destroy", sc.Destroy).Ability("series:write").Status(fhttp.StatusNoContent)
	routing.Post(workspace, one+"/check", "series.check.store", sc.Check).Ability("series:read").Status(fhttp.StatusOK)
	routing.Get(workspace, one+"/versions", "series.versions.index", sc.Versions).Ability("series:read")
	routing.Post(workspace, one+"/versions/{version}/restore", "series.versions.restore.store", sc.Restore).
		Ability("series:write").Status(fhttp.StatusOK)
	routing.Get(workspace, one+"/report", "series.report.show", sc.Report).Ability("series:read")
	routing.Post(workspace, one+"/nodes/{node}/winner", "series.nodes.winner.store", sc.Winner).
		Ability("series:write").Status(fhttp.StatusOK)

	seriesActionRoutes(workspace, sc, one)
	seriesJourneyRoutes(workspace, sc, one)

	routing.Get(workspace, "/contacts/{contact}/series", "contacts.series.index", sc.ContactJourneys).
		Ability("series:read")
	routing.Get(workspace, "/companies/{company}/series", "companies.series.index", sc.CompanyJourneys).
		Ability("series:read")
}

/*
seriesActionRoutes declares what can be done to one series.
*/
func seriesActionRoutes(workspace *routing.Router, sc *v1.SeriesController, one string) {
	routing.Post(workspace, one+"/publications", "series.publications.store", sc.Publish).
		Ability("series:write").Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/pause", "series.pause.store", sc.Pause).Ability("series:write").Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/resume", "series.resume.store", sc.Resume).
		Ability("series:write").Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/archive", "series.archive.store", sc.Archive).
		Ability("series:write").Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/duplicate", "series.duplicate.store", sc.Duplicate).
		Ability("series:write").Status(fhttp.StatusCreated)
}

/*
seriesJourneyRoutes declares the people on one series.
*/
func seriesJourneyRoutes(workspace *routing.Router, sc *v1.SeriesController, one string) {
	journey := one + "/enrollments/{enrollment}"

	routing.Get(workspace, one+"/enrollments", "series.enrollments.index", sc.Enrollments).Ability("series:read")
	routing.Post(workspace, one+"/enrollments", "series.enrollments.store", sc.Enroll).
		Ability("series:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, journey, "series.enrollments.show", sc.Trace).Ability("series:read")
	routing.Delete(workspace, journey, "series.enrollments.destroy", sc.Remove).
		Ability("series:write").Status(fhttp.StatusNoContent)
	routing.Post(workspace, journey+"/skip", "series.enrollments.skip.store", sc.Skip).
		Ability("series:write").Status(fhttp.StatusNoContent)
	routing.Post(workspace, one+"/tests", "series.tests.store", sc.Try).
		Ability("series:write").Status(fhttp.StatusCreated)
}
