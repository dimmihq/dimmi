/*
|--------------------------------------------------------------------------
| Series Tools
|--------------------------------------------------------------------------
|
| Operator reads series the way the builder shows them: who enters, what
| makes them leave, the goal, and every step with where each of its
| paths leads, with what publishing would refuse. It explains what
| series did with a person (explain_series.go). Its changes are
| suggestions on the working copy (propose_automation.go).
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
seriesListed is how many series one listing shows.
*/
const seriesListed = 100

/*
seriesTools are the tools that read series.
*/
func (e *Engine) seriesTools() []tool {
	if e.Services.Series == nil {
		return nil
	}
	return []tool{
		{spec: contracts.ToolSpec{
			Name: "list_series",
			Description: "List the workspace's series, most recently changed first, each with its id, who it is " +
				"for, its status and its link.",
			Schema: schema(`{"type":"object","properties":{"status":{"type":"string","enum":["draft","live",` +
				`"paused","archived"],"description":"Only series in this status."}}}`),
		}, handle: e.listSeries},
		{spec: contracts.ToolSpec{
			Name: "get_series_by_id",
			Description: "Read one series: who enters, what makes them leave, the goal, every step with its " +
				"settings and where each path leads, what publishing would refuse, and the step kinds with " +
				"their settings and paths.",
			Schema: schema(`{"type":"object","required":["series_id"],"properties":{` +
				`"series_id":{"type":"string","description":"Series id (srs_...)."}}}`),
		}, handle: e.getSeries, was: map[string]string{"series": "series_id", "id": "series_id"}},
		{spec: contracts.ToolSpec{
			Name: "get_series_reference",
			Description: "Read how series are built: every step kind with its settings and paths, and the " +
				"shapes of entry, exit and goal. Read it before drafting a new series.",
			Schema: schema(`{"type":"object","properties":{}}`),
		}, handle: e.getSeriesReference},
		e.explainSeriesTool(),
	}
}

/*
listSeries lists the series.
*/
func (e *Engine) listSeries(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Status string `json:"status"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	list := series.ListInput{Membership: s.m, Limit: seriesListed}
	if in.Status != "" {
		list.Status = &in.Status
	}
	rows, err := e.Services.Series.List(ctx, list)
	if err != nil {
		return result{}, err
	}
	var b strings.Builder
	for _, row := range rows {
		fmt.Fprintf(&b, "- [%s](%s) · series_id %s · for %ss · %s\n", row.Name,
			e.link(s, linkSeries, models.PrefixSeries, row.ID), models.PublicID(models.PrefixSeries, row.ID),
			row.Subject, row.Status)
	}
	if len(rows) == 0 {
		b.WriteString("No series found.")
	}
	return result{text: b.String(), step: Step{Tool: "list_series", Summary: fmt.Sprintf("Listed %d series",
		len(rows))}}, nil
}

/*
getSeries reads one series with its outline, problems and reference.
*/
func (e *Engine) getSeries(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	id, err := e.idArg(args, "series_id", models.PrefixSeries)
	if err != nil {
		return result{}, err
	}
	row, err := e.Services.Series.Show(ctx, s.m, id)
	if err != nil {
		return result{}, err
	}
	text, err := e.seriesText(s, row)
	if err != nil {
		return result{}, err
	}
	return result{text: text, step: Step{Tool: "get_series_by_id", Summary: "Read the series " + row.Name}}, nil
}

/*
getSeriesReference reads how series are built.
*/
func (e *Engine) getSeriesReference(_ context.Context, _ *session, _ json.RawMessage) (result, error) {
	return result{text: seriesReference(), step: Step{
		Tool: "get_series_reference", Summary: "Read how series are built",
	}}, nil
}

/*
seriesText is everything about one series the model edits by. A
suggestion open on it shows its working copy as suggested instead.
*/
func (e *Engine) seriesText(s *session, row models.Series) (string, error) {
	spec, err := series.SpecOf(row)
	if err != nil {
		return "", err
	}
	if suggested, ok := s.suggestedSeries(row.ID); ok {
		spec = suggested
	}
	public := models.PublicID(models.PrefixSeries, row.ID)
	var b strings.Builder
	fmt.Fprintf(&b, "Series %q\nseries_id: %s\nFor: %ss\nStatus: %s\nLink: %s\n", row.Name, public, row.Subject,
		row.Status, e.link(s, linkSeries, models.PrefixSeries, row.ID))
	if d := strings.TrimSpace(row.Description); d != "" {
		fmt.Fprintf(&b, "Description: %s\n", d)
	}
	b.WriteString("\nWorking copy:\n" + seriesOutline(spec))
	b.WriteString("\n" + problemText(series.Check(spec, row.Subject)))
	fmt.Fprintf(&b, "\nTo change it, call propose_series_edit with series_id %s.\n\n", public)
	b.WriteString(seriesReference())
	return truncate(b.String(), 40000), nil
}

/*
problemText writes what a series misses before it can go live.
*/
func problemText(problems []series.Problem) string {
	if len(problems) == 0 {
		return "Check: nothing stops it from being published.\n"
	}
	var b strings.Builder
	b.WriteString("Check, what stops it from being published:\n")
	for _, p := range problems {
		fmt.Fprintf(&b, "- %s: %s", p.Code, p.Message)
		var where []string
		if p.NodeID != "" {
			where = append(where, "step "+p.NodeID)
		}
		if p.Field != "" {
			where = append(where, "setting "+p.Field)
		}
		if len(where) > 0 {
			fmt.Fprintf(&b, " (%s)", strings.Join(where, ", "))
		}
		b.WriteString("\n")
	}
	return b.String()
}
