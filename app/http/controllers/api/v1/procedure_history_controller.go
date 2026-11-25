/*
|--------------------------------------------------------------------------
| Procedure History Controller
|--------------------------------------------------------------------------
|
| GET /procedures/{procedure}/versions lists the published versions and
| .../versions/{version} reads one; POST .../restore puts one back;
| .../metrics is the procedure's scorecard; GET /reports/procedures
| lists every procedure's numbers side by side.
|
*/

package v1

import (
	"context"
	"strconv"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"

	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
ProcedureVersionsOutput is a page of a procedure's versions.
*/
type ProcedureVersionsOutput struct {
	Body fhttp.Collection[resources.ProcedureVersion]
}

/*
ProcedureVersionOutput is one version.
*/
type ProcedureVersionOutput struct {
	Body fhttp.Resource[resources.ProcedureVersion]
}

/*
ProcedureMetricsOutput is a procedure's scorecard.
*/
type ProcedureMetricsOutput struct {
	Body fhttp.Resource[resources.ProcedureMetrics]
}

/*
ProceduresReportOutput is every procedure's numbers.
*/
type ProceduresReportOutput struct {
	Body fhttp.Collection[resources.ProcedureReportRow]
}

/*
Versions lists a procedure's published versions, newest first.
*/
func (c *ProcedureController) Versions(ctx context.Context, in *requests.ProcedureVersionsRequest,
) (*ProcedureVersionsOutput, error) {
	id, err := c.decode(in.Procedure)
	if err != nil {
		return nil, err
	}

	limit := in.Limit
	if limit == 0 {
		limit = 25
	}

	page, err := c.Procedures.Versions(ctx, membership(ctx), id, procedures.Paging{Before: in.Before, Limit: limit})
	if err != nil {
		return nil, err
	}

	items := make([]resources.ProcedureVersion, len(page.Items))
	for i, v := range page.Items {
		items[i] = resources.NewProcedureVersion(v)
	}

	out := pagination.Page[resources.ProcedureVersion]{Items: items, Limit: int(limit)}
	if page.Next > 0 {
		out.NextCursor = strconv.Itoa(int(page.Next))
	}

	return &ProcedureVersionsOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
ShowVersion answers one published version.
*/
func (c *ProcedureController) ShowVersion(ctx context.Context, in *requests.ProcedureVersionRequest,
) (*ProcedureVersionOutput, error) {
	id, err := c.decode(in.Procedure)
	if err != nil {
		return nil, err
	}

	v, err := c.Procedures.ShowVersion(ctx, membership(ctx), id, in.Version)
	if err != nil {
		return nil, err
	}

	return &ProcedureVersionOutput{
		Body: fhttp.NewResource(resources.NewProcedureVersion(procedures.Version{ProcedureVersion: v})),
	}, nil
}

/*
RestoreVersion puts a version back as the working copy, or publishes
it.
*/
func (c *ProcedureController) RestoreVersion(ctx context.Context, in *requests.RestoreProcedureVersionRequest,
) (*ProcedureOutput, error) {
	id, err := c.decode(in.Procedure)
	if err != nil {
		return nil, err
	}

	restore := procedures.RestoreInput{Membership: membership(ctx), ID: id, Version: in.Version}
	if in.Body != nil {
		restore.Publish, restore.LockVersion = in.Body.Publish, in.Body.LockVersion
	}

	p, err := c.Procedures.Restore(ctx, restore)

	return procedureOutput(p, err)
}

/*
Metrics answers a procedure's scorecard.
*/
func (c *ProcedureController) Metrics(ctx context.Context, in *requests.ProcedureMetricsRequest,
) (*ProcedureMetricsOutput, error) {
	id, err := c.decode(in.Procedure)
	if err != nil {
		return nil, err
	}

	version := int32(0)
	if in.Version != "" && in.Version != "all" {
		n, err := strconv.ParseInt(in.Version, 10, 32)
		if err != nil || n < 1 {
			return nil, procedures.ErrVersionNotFound
		}
		version = int32(n)
	}

	m, err := c.Procedures.Metrics(ctx, membership(ctx), id, procedures.MetricsScope{Days: in.Days, Version: version})
	if err != nil {
		return nil, err
	}

	return &ProcedureMetricsOutput{Body: fhttp.NewResource(resources.NewProcedureMetrics(id, m))}, nil
}

/*
Report lists every procedure that ran in the range with its numbers.
*/
func (c *ProcedureController) Report(ctx context.Context, in *requests.ProceduresReportRequest,
) (*ProceduresReportOutput, error) {
	from, to := day(in.From), day(in.To)
	if !to.IsZero() {
		to = to.Add(24 * time.Hour)
	}

	rows, err := c.Procedures.Report(ctx, membership(ctx), from, to)
	if err != nil {
		return nil, err
	}

	items := make([]resources.ProcedureReportRow, len(rows))
	for i, r := range rows {
		items[i] = resources.NewProcedureReportRow(r)
	}

	return &ProceduresReportOutput{
		Body: fhttp.NewCollection(pagination.Page[resources.ProcedureReportRow]{Items: items, Limit: 500}),
	}, nil
}

/*
day reads a YYYY-MM-DD date in UTC, zero when empty or unreadable.
*/
func day(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}
	}

	return t
}
