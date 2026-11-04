/*
|--------------------------------------------------------------------------
| Audit Log Export
|--------------------------------------------------------------------------
|
| The audit log list answers with a CSV file when the client asks for
| one, by its Accept header or with format=csv, narrowed by the same
| filters. The file streams as it is read, so its status and headers
| go out before the first row: anything that refuses the export is
| checked and answered as JSON first.
|
*/

package v1

import (
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/log"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
csvType is the media type of an exported log.
*/
const csvType = "text/csv"

/*
ServeCSV is the middleware of the list route that answers a request
for CSV with the exported log, and hands every other request on.
*/
func (c *AuditLogController) ServeCSV(next fhttp.Handler) fhttp.Handler {
	return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
		if !wantsCSV(r) {
			next.ServeHTTP(w, r)

			return
		}

		c.export(w, r)
	})
}

/*
wantsCSV says whether the request asks for the log as CSV.
*/
func wantsCSV(r *fhttp.Request) bool {
	return r.URL.Query().Get("format") == "csv" || strings.Contains(r.Header.Get("Accept"), csvType)
}

/*
export streams the log as a file once the export is allowed. A stream
that breaks after its headers went out can only be logged.
*/
func (c *AuditLogController) export(w fhttp.ResponseWriter, r *fhttp.Request) {
	ctx := r.Context()

	query, err := pagination.Parse(r.URL.Query(), pagination.Allow{Filters: auditFilters()})
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	filter, err := c.filter(query.Filters)
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	export, err := c.Export.Start(ctx, audit.ExportAuditLogsInput{
		Membership: membership(ctx), Filter: filter, Asked: query.Filters,
	})
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	headers := w.Header()
	headers.Set("Content-Type", csvType+"; charset=utf-8")
	headers.Set("Content-Disposition", `attachment; filename="`+export.Filename+`"`)
	headers.Set("Cache-Control", "no-store")
	w.WriteHeader(fhttp.StatusOK)

	if err := c.Export.Write(ctx, export, streaming{w}, resources.NewAuditLogRow); err != nil {
		log.From(ctx).ErrorContext(ctx, "audit log export broke off", log.Err(err))
	}
}

/*
streaming hands every write to the client at once, so a long file
arrives as it is read rather than when it ends.
*/
type streaming struct {
	w fhttp.ResponseWriter
}

/*
Write sends the bytes and flushes them when the response can.
*/
func (s streaming) Write(p []byte) (int, error) {
	n, err := s.w.Write(p)
	if f, ok := s.w.(interface{ Flush() }); ok {
		f.Flush()
	}

	return n, err
}
