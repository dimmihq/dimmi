/*
|--------------------------------------------------------------------------
| Report Controller
|--------------------------------------------------------------------------
|
| GET /reports lists the reports the caller may read. GET
| /reports/{report} answers one for a range and filters, with each
| number beside the period before when compare is set. GET
| /reports/{report}/conversations lists the conversations behind one of
| its numbers, up to ten thousand. GET, PUT and DELETE /report-digest
| read, set and stop the caller's daily or weekly digest by mail.
|
*/

package v1

import (
	"context"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/reports"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ReportTypesOutput lists the reports.
*/
type ReportTypesOutput struct {
	Body struct {
		Data []resources.ReportType `json:"data"`
	}
}

/*
ReportOutput is one report.
*/
type ReportOutput struct {
	Body fhttp.Resource[resources.ReportResult]
}

/*
ReportConversationsOutput is a page of the conversations behind a
number: how many there are, up to the cap, and whether there are more.
*/
type ReportConversationsOutput struct {
	Body struct {
		Data   []resources.ReportConversation `json:"data"`
		Total  int64                          `json:"total"`
		Capped bool                           `json:"capped"`
		Limit  int                            `json:"limit"`
		Offset int                            `json:"offset"`
	}
}

/*
ReportDigestOutput is the caller's digest.
*/
type ReportDigestOutput struct {
	Body fhttp.Resource[resources.ReportDigest]
}

/*
ReportController serves the reports.
*/
type ReportController struct {
	Reports *reports.Reports
	Digests *reports.Digests
	IDs     *ids.Registry
}

/*
Index lists the reports the caller may read.
*/
func (c *ReportController) Index(ctx context.Context, _ *requests.NoInput) (*ReportTypesOutput, error) {
	out := &ReportTypesOutput{}
	out.Body.Data = []resources.ReportType{}

	for _, d := range c.Reports.List(membership(ctx)) {
		out.Body.Data = append(out.Body.Data, resources.NewReportType(d))
	}

	return out, nil
}

/*
Show answers one report.
*/
func (c *ReportController) Show(ctx context.Context, in *requests.ShowReportRequest) (*ReportOutput, error) {
	f, err := c.filters(in.ReportFilters)
	if err != nil {
		return nil, err
	}

	r, err := c.Reports.Run(ctx, membership(ctx), in.Report, f)
	if err != nil {
		return nil, err
	}

	return &ReportOutput{Body: fhttp.NewResource(resources.NewReport(r))}, nil
}

/*
Conversations lists the conversations behind a number.
*/
func (c *ReportController) Conversations(
	ctx context.Context, in *requests.ReportConversationsRequest,
) (*ReportConversationsOutput, error) {
	f, err := c.filters(in.ReportFilters)
	if err != nil {
		return nil, err
	}

	m := membership(ctx)
	if _, err := c.Reports.Describe(m, in.Report); err != nil {
		return nil, err
	}

	limit := in.Limit
	if limit == 0 {
		limit = 25
	}

	page, err := c.Reports.Conversations(ctx, m, in.Metric, reports.DrillInput{
		Filters: f, Offset: in.Offset, Limit: limit,
	})
	if err != nil {
		return nil, err
	}

	out := &ReportConversationsOutput{}
	out.Body.Data = make([]resources.ReportConversation, len(page.Rows))
	for i, r := range page.Rows {
		out.Body.Data[i] = resources.NewReportConversation(r)
	}

	out.Body.Total, out.Body.Capped, out.Body.Limit, out.Body.Offset = page.Total, page.Capped, limit, in.Offset

	return out, nil
}

/*
ShowDigest answers the caller's digest.
*/
func (c *ReportController) ShowDigest(ctx context.Context, _ *requests.NoInput) (*ReportDigestOutput, error) {
	d, found, err := c.Digests.Show(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	var shown *reports.Digest
	if found {
		shown = &d
	}

	return &ReportDigestOutput{Body: fhttp.NewResource(resources.NewReportDigest(shown))}, nil
}

/*
UpdateDigest sets the caller's digest.
*/
func (c *ReportController) UpdateDigest(
	ctx context.Context, in *requests.UpdateReportDigestRequest,
) (*ReportDigestOutput, error) {
	b := in.Body

	inboxes, err := decodeReportIDs(c.IDs, models.PrefixInbox, b.Inboxes, "inbox_ids")
	if err != nil {
		return nil, err
	}

	teams, err := decodeReportIDs(c.IDs, models.PrefixTeam, b.Teams, "team_ids")
	if err != nil {
		return nil, err
	}

	d, err := c.Digests.Save(ctx, membership(ctx), b.Frequency, reports.Filters{
		Inboxes: inboxes, Teams: teams, Channels: b.Channels, BusinessHours: b.BusinessHours,
	})
	if err != nil {
		return nil, err
	}

	return &ReportDigestOutput{Body: fhttp.NewResource(resources.NewReportDigest(&d))}, nil
}

/*
DestroyDigest stops the caller's digest.
*/
func (c *ReportController) DestroyDigest(ctx context.Context, _ *requests.NoInput) (*struct{}, error) {
	return &struct{}{}, c.Digests.Delete(ctx, membership(ctx))
}

/*
filters reads the query string's range and filters.
*/
func (c *ReportController) filters(in requests.ReportFilters) (reports.Filters, error) {
	return reportFilters(c.IDs, in)
}

/*
reportFiltersOf reads a report's range and filters from a map of the
query string's names, as an export's selection holds them.
*/
func reportFiltersOf(reg *ids.Registry, m map[string]string) (reports.Filters, error) {
	return reportFilters(reg, requests.ReportFilters{
		From: m["from"], To: m["to"], BusinessHours: m["business_hours"] == "true", Inbox: m["inbox"],
		Team: m["team"], Member: m["member"], Channel: m["channel"], Tag: m["tag"], Priority: m["priority"],
		AI: m["ai"],
	})
}

/*
reportFilters reads a report's range and filters, refusing ids that name
nothing and priorities that are not levels.
*/
func reportFilters(reg *ids.Registry, in requests.ReportFilters) (reports.Filters, error) {
	f := reports.Filters{
		From: in.From, To: in.To, Compare: in.Compare, BusinessHours: in.BusinessHours, AI: in.AI,
		Channels: splitList(in.Channel),
	}

	lists := []struct {
		into   *[]ids.UUID
		prefix models.Prefix
		value  string
		field  string
	}{
		{&f.Inboxes, models.PrefixInbox, in.Inbox, "filter[inbox]"},
		{&f.Teams, models.PrefixTeam, in.Team, "filter[team]"},
		{&f.Members, models.PrefixMember, in.Member, "filter[member]"},
		{&f.Tags, models.PrefixTag, in.Tag, "filter[tag]"},
	}
	for _, l := range lists {
		decoded, err := decodeReportIDs(reg, l.prefix, splitList(l.value), l.field)
		if err != nil {
			return f, err
		}

		*l.into = decoded
	}

	for _, p := range splitList(in.Priority) {
		level, ok := enums.ConversationPriority(p).Level()
		if !ok {
			return f, exceptions.ErrNotFound.OnField("filter[priority]")
		}

		f.Priorities = append(f.Priorities, level)
	}

	return f, nil
}

/*
decodeReportIDs reads public ids, refusing one that names nothing.
*/
func decodeReportIDs(reg *ids.Registry, prefix models.Prefix, list []string, field string) ([]ids.UUID, error) {
	out := make([]ids.UUID, 0, len(list))
	for _, s := range list {
		id, err := reg.Decode(string(prefix), s)
		if err != nil {
			return nil, exceptions.ErrNotFound.OnField(field)
		}

		out = append(out, id)
	}

	return out, nil
}

/*
splitList reads a comma list, dropping empty entries.
*/
func splitList(s string) []string {
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}

	return out
}
