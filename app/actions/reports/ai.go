/*
|--------------------------------------------------------------------------
| dimmi AI
|--------------------------------------------------------------------------
|
| What the AI agent did for the conversations started in the range: how
| many it answered in, how many it resolved, confirmed by the customer
| or assumed from a close or a day of silence, how many it handed to the
| team and why, and what each resolution cost. Questions it could not
| answer and searches that found no article show where knowledge is
| missing.
|
*/

package reports

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
listed is how many questions and gaps a table shows.
*/
const listed = 15

/*
AI is the dimmi AI report.
*/
type AI struct{}

/*
Describe says what the report is.
*/
func (AI) Describe() Description {
	return Description{
		Key: "ai", Title: "dimmi AI", Order: 70,
		Summary: "How often the AI answers and resolves, why it hands off, and what it costs.",
	}
}

/*
Metrics returns the headline numbers.
*/
func (AI) Metrics(ctx context.Context, tx *database.Tx, q Query) ([]Metric, error) {
	r := reader{tx: tx, q: q}

	v, err := r.startedFigures(ctx, "")
	if err != nil {
		return nil, err
	}

	s := only(v, 6)
	started, involved, handed, resolved, confirmed := s.count(0), s.count(1), s.count(2), s.count(3), s.count(4)

	spend, err := models.New(tx).ReportAiSpend(ctx, models.ReportAiSpendParams{
		WorkspaceID: q.Membership.Workspace.ID, Since: q.Start, Until: q.End,
	})
	if err != nil {
		return nil, err
	}

	var perResolution *float64
	if resolved > 0 {
		perResolution = num(float64(s.count(5)) / float64(resolved))
	}

	return []Metric{
		rateMetric("ai_involvement", "AI involvement", involved, started),
		rateMetric("ai_resolution", "AI resolution rate", resolved, involved),
		countMetric("ai_confirmed", "Confirmed resolutions", confirmed),
		countMetric("ai_assumed", "Assumed resolutions", resolved-confirmed),
		rateMetric("ai_handoff", "Hand-off rate", handed, involved),
		{
			Key: "ai_cost_per_resolution", Label: "Cost per resolution", Unit: UnitMicros, Value: perResolution,
			Sample: resolved, Drill: "ai_resolution",
		},
		{Key: "ai_spend", Label: "AI spend", Unit: UnitMicros, Value: num(spend.CostMicros), Sample: spend.Runs},
	}, nil
}

/*
Charts returns the charts.
*/
func (AI) Charts(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
	r := reader{tx: tx, q: q}

	byDay, err := r.startedFigures(ctx, groupDay)
	if err != nil {
		return nil, err
	}

	reasonSeries, err := r.handoffReasons(ctx)
	if err != nil {
		return nil, err
	}

	questions, err := r.unanswered(ctx)
	if err != nil {
		return nil, err
	}

	return []Chart{
		{
			Key: "ai_daily", Title: "AI involvement and resolution", Kind: ChartLine, Unit: UnitPercent, Axis: AxisDay,
			Series: []Series{
				daily(q, Series{Key: "involvement", Label: "Involvement", Drill: "ai_involvement"},
					byDay, percentOf(1, 0)),
				daily(q, Series{Key: "resolution", Label: "Resolution", Drill: "ai_resolution"},
					byDay, percentOf(3, 1)),
			},
		},
		{
			Key: "ai_outcomes", Title: "What the AI did", Kind: ChartStacked, Unit: UnitCount, Axis: AxisDay,
			Series: []Series{
				daily(q, Series{Key: "confirmed", Label: "Confirmed", Drill: "ai_confirmed"}, byDay, at(4)),
				daily(q, Series{Key: "assumed", Label: "Assumed", Drill: "ai_assumed"}, byDay, func(v values) *float64 {
					return num(v.count(3) - v.count(4))
				}),
				daily(q, Series{Key: "handed_off", Label: "Handed off", Drill: "ai_handoff"}, byDay, at(2)),
			},
		},
		{
			Key: "handoff_reasons", Title: "Why the AI handed off", Kind: ChartBar, Unit: UnitCount, Axis: AxisCategory,
			Drill: "ai_handoff", Series: []Series{reasonSeries},
		},
		questions,
	}, nil
}

/*
handoffReasons counts the hand-offs by their reason.
*/
func (r reader) handoffReasons(ctx context.Context) (Series, error) {
	reasons, err := r.read(ctx, figures{
		from: fromFacts, dated: "f.started_at", cols: "count(*)::float8",
		extra: "f.ai_handed_off_at IS NOT NULL",
	}, "COALESCE(f.ai_handoff_reason, 'unknown')", 1)
	if err != nil {
		return Series{}, err
	}

	out := Series{Key: "handoffs", Label: "Hand-offs"}
	for _, v := range reasons {
		out.Points = append(out.Points, Point{X: humanize(v.key), Y: v.at(0)})
	}

	return out, nil
}

/*
unanswered lists the questions the AI could not answer, most asked
first.
*/
func (r reader) unanswered(ctx context.Context) (Chart, error) {
	rows, err := models.New(r.tx).ReportUnansweredQuestions(ctx, models.ReportUnansweredQuestionsParams{
		WorkspaceID: r.q.Membership.Workspace.ID, Since: r.q.Start, Until: r.q.End, RowLimit: listed,
	})
	if err != nil {
		return Chart{}, err
	}

	out := Chart{
		Key: "unanswered_questions", Title: "Questions the AI could not answer", Kind: ChartTable,
		Axis: AxisCategory, Columns: []Column{{Key: "asked", Label: "Asked", Unit: UnitCount}},
	}
	for _, q := range rows {
		out.Rows = append(out.Rows, Row{Key: q.Question, Label: q.Question, Values: []*float64{num(q.Asked)}})
	}

	return out, nil
}

/*
humanize writes a stored reason as words: "asked_for_human" reads
"Asked for human".
*/
func humanize(key string) string {
	words := strings.ReplaceAll(key, "_", " ")
	if words == "" {
		return words
	}

	return strings.ToUpper(words[:1]) + words[1:]
}
