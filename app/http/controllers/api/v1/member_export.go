/*
|--------------------------------------------------------------------------
| Member Export
|--------------------------------------------------------------------------
|
| The member list answers with a CSV file when the client asks for one,
| by its Accept header or with format=csv, narrowed by the same filters
| as the list. The file streams as it is read, so its status and headers
| go out before the first row: anything that refuses the export is
| checked and answered as JSON first.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/log"

	"github.com/dimmihq/dimmi/app/actions/members"
)

/*
ServeCSV is the middleware of the list route that answers a request
for CSV with the exported members, and hands every other request on.
*/
func (c *MemberController) ServeCSV(next fhttp.Handler) fhttp.Handler {
	return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
		if !wantsCSV(r) {
			next.ServeHTTP(w, r)

			return
		}

		c.serveExport(w, r)
	})
}

/*
serveExport streams the members as a file once the export is allowed. A
stream that breaks after its headers went out can only be logged.
*/
func (c *MemberController) serveExport(w fhttp.ResponseWriter, r *fhttp.Request) {
	ctx := r.Context()

	query, err := pagination.Parse(r.URL.Query(), pagination.Allow{Filters: memberFilters()})
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	list, err := c.listInput(ctx, query)
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	in := members.ExportMembersInput{List: list, Asked: query.Filters}

	filename, err := c.Export.Filename(in)
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	headers := w.Header()
	headers.Set("Content-Type", csvType+"; charset=utf-8")
	headers.Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	headers.Set("Cache-Control", "no-store")
	w.WriteHeader(fhttp.StatusOK)

	if err := c.Export.Write(ctx, in, streaming{w}); err != nil {
		log.From(ctx).ErrorContext(ctx, "members export broke off", log.Err(err))
	}
}
