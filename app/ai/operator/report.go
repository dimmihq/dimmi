/*
|--------------------------------------------------------------------------
| Workspace Report
|--------------------------------------------------------------------------
|
| The sections of a workspace report, each one query over the range,
| written as compact text a model reads well. Test, spam and deleted
| conversations never count.
|
*/

package operator

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
reportRows is how many rows the ranked sections show.
*/
const reportRows = 10

/*
reporter writes one workspace's report over a range.
*/
type reporter struct {
	q  *models.Queries
	ws ids.UUID
	r  span
	b  *strings.Builder
}

/*
report writes every section of a workspace report.
*/
func (rp reporter) report(ctx context.Context) error {
	sections := []func(context.Context) error{
		rp.totals, rp.days, rp.channels, rp.hours, rp.tags, rp.agent, rp.articles,
	}
	for _, section := range sections {
		if err := section(ctx); err != nil {
			return err
		}
	}
	return nil
}

/*
totals writes the range's totals and medians.
*/
func (rp reporter) totals(ctx context.Context) error {
	q, ws, r, b := rp.q, rp.ws, rp.r, rp.b
	t, err := q.OperatorConversationTotals(ctx, models.OperatorConversationTotalsParams{
		WorkspaceID: ws, Since: r.since, Until: r.until,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(b, "\nConversations opened: %d; closed since: %d; still open: %d; replied to: %d\n",
		t.Opened, t.Closed, t.StillOpen, t.Replied)
	fmt.Fprintf(b, "Median first reply: %.0f minutes; median time to close: %.0f minutes\n",
		t.MedianFirstReplyMinutes, t.MedianResolutionMinutes)
	return nil
}

/*
days writes conversations opened and closed per day.
*/
func (rp reporter) days(ctx context.Context) error {
	q, ws, r, b := rp.q, rp.ws, rp.r, rp.b
	days, err := q.OperatorConversationsByDay(ctx, models.OperatorConversationsByDayParams{
		WorkspaceID: ws, Since: r.since, Until: r.until,
	})
	if err != nil {
		return err
	}
	b.WriteString("\nPer day (opened/closed):")
	for _, d := range days {
		fmt.Fprintf(b, " %s %d/%d;", d.Day.UTC().Format("01-02"), d.Opened, d.Closed)
	}
	b.WriteString("\n")
	return nil
}

/*
channels writes conversations opened per kind of channel.
*/
func (rp reporter) channels(ctx context.Context) error {
	q, ws, r, b := rp.q, rp.ws, rp.r, rp.b
	rows, err := q.OperatorConversationsByChannel(ctx, models.OperatorConversationsByChannelParams{
		WorkspaceID: ws, Since: r.since, Until: r.until,
	})
	if err != nil {
		return err
	}
	b.WriteString("By channel:")
	for _, c := range rows {
		fmt.Fprintf(b, " %s %d;", c.Channel, c.Opened)
	}
	b.WriteString("\n")
	return nil
}

/*
hours writes conversations opened per hour of the day.
*/
func (rp reporter) hours(ctx context.Context) error {
	q, ws, r, b := rp.q, rp.ws, rp.r, rp.b
	rows, err := q.OperatorBusiestHours(ctx, models.OperatorBusiestHoursParams{
		WorkspaceID: ws, Since: r.since, Until: r.until,
	})
	if err != nil {
		return err
	}
	b.WriteString("By hour opened (UTC):")
	for _, h := range rows {
		fmt.Fprintf(b, " %02d:00 %d;", h.Hour, h.Opened)
	}
	b.WriteString("\n")
	return nil
}

/*
tags writes the tags most often used.
*/
func (rp reporter) tags(ctx context.Context) error {
	q, ws, r, b := rp.q, rp.ws, rp.r, rp.b
	rows, err := q.OperatorTopTags(ctx, models.OperatorTopTagsParams{
		WorkspaceID: ws, Since: r.since, Until: r.until, RowLimit: reportRows,
	})
	if err != nil {
		return err
	}
	b.WriteString("Top tags:")
	for _, t := range rows {
		fmt.Fprintf(b, " %s %d;", t.Name, t.Conversations)
	}
	b.WriteString("\n")
	return nil
}

/*
agent writes how the AI agent's runs ended and why it handed off.
*/
func (rp reporter) agent(ctx context.Context) error {
	q, ws, r, b := rp.q, rp.ws, rp.r, rp.b
	outcomes, err := q.OperatorAiOutcomes(ctx, models.OperatorAiOutcomesParams{
		WorkspaceID: ws, Since: r.since, Until: r.until,
	})
	if err != nil {
		return err
	}
	b.WriteString("\nAI agent runs by outcome:")
	for _, o := range outcomes {
		fmt.Fprintf(b, " %s %d ($%.2f);", o.Outcome, o.Runs, float64(o.CostMicros)/1e6)
	}
	reasons, err := q.OperatorHandoffReasons(ctx, models.OperatorHandoffReasonsParams{
		WorkspaceID: ws, Since: r.since, Until: r.until,
	})
	if err != nil {
		return err
	}
	b.WriteString("\nHandoff reasons:")
	for _, h := range reasons {
		fmt.Fprintf(b, " %s %d;", deref(h.Reason), h.Runs)
	}
	b.WriteString("\n")
	return nil
}

/*
articles writes the articles readers found least helpful.
*/
func (rp reporter) articles(ctx context.Context) error {
	q, ws, r, b := rp.q, rp.ws, rp.r, rp.b
	rows, err := q.OperatorArticleHelpfulness(ctx, models.OperatorArticleHelpfulnessParams{
		WorkspaceID: ws, Since: r.since, Until: r.until, RowLimit: reportRows,
	})
	if err != nil {
		return err
	}
	b.WriteString("\nArticle ratings, least helpful first:\n")
	for _, a := range rows {
		fmt.Fprintf(b, "- %s (id %s): %d helpful, %d not", a.Title, models.PublicID(models.PrefixArticle, a.ID),
			a.Helpful, a.NotHelpful)
		for _, c := range a.Comments {
			fmt.Fprintf(b, "; reader: %s", untrusted(truncate(c, 300)))
		}
		b.WriteString("\n")
	}
	return nil
}
