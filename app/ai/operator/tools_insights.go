/*
|--------------------------------------------------------------------------
| Insight Tools
|--------------------------------------------------------------------------
|
| The questions a support lead asks: how busy the team is, how fast it
| answers and closes, what customers write about, how the AI agent is
| doing and why it handed conversations over, which articles readers
| found useless. Reports count the whole workspace, so only teammates
| who see every conversation may read them. Why the AI agent did what it
| did in one conversation comes from its runs.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxReportDays is the longest range a report covers.
*/
const maxReportDays = 366

/*
insightTools are the tools that report on the workspace and the agent.
*/
func (e *Engine) insightTools() []tool {
	return []tool{
		{spec: contracts.ToolSpec{
			Name: "workspace_report",
			Description: "Report on support over a date range: conversations opened and closed per day and " +
				"channel, median minutes to first reply and to close, busiest hours (UTC), top tags, the AI " +
				"agent's outcomes, cost and handoff reasons, and the least helpful articles with readers' " +
				"comments. Dates are YYYY-MM-DD; the default is the last 30 days.",
			Schema: schema(`{"type":"object","properties":{"since":{"type":"string","format":"date"},` +
				`"until":{"type":"string","format":"date","description":"Exclusive end date."}}}`),
		}, handle: e.workspaceReport},
		{spec: contracts.ToolSpec{
			Name: "agent_runs", Schema: schema(numberSchema),
			Description: "Show what the AI agent did in one conversation: each run's outcome, handoff reason, " +
				"what it searched for and what it cost, newest first. Needs the conversation's number " +
				"unless this thread is about one; for the agent across the workspace use workspace_report.",
		}, handle: e.agentRuns},
	}
}

/*
span is a report's date range.
*/
type span struct {
	since, until time.Time
}

/*
spanOf reads a report's range, the last 30 days by default.
*/
func spanOf(now time.Time, since, until string) (span, error) {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	out := span{since: day.AddDate(0, 0, -29), until: day.AddDate(0, 0, 1)}
	var err error
	if until != "" {
		if out.until, err = time.Parse(time.DateOnly, until); err != nil {
			return out, fmt.Errorf("%w: until must be YYYY-MM-DD", errBadArguments)
		}
	}
	if since != "" {
		if out.since, err = time.Parse(time.DateOnly, since); err != nil {
			return out, fmt.Errorf("%w: since must be YYYY-MM-DD", errBadArguments)
		}
	}
	if !out.since.Before(out.until) || out.until.Sub(out.since) > maxReportDays*24*time.Hour {
		return out, fmt.Errorf("%w: the range must be 1 to %d days", errBadArguments, maxReportDays)
	}
	return out, nil
}

/*
workspaceReport reports on the workspace over a range.
*/
func (e *Engine) workspaceReport(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Since string `json:"since"`
		Until string `json:"until"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	if err := conversations.CanRead(s.m); err != nil {
		return result{}, err
	}
	if s.m.ConversationScope().Restricted() {
		return result{}, exceptions.ErrForbidden
	}
	r, err := spanOf(e.Clock.Now(), in.Since, in.Until)
	if err != nil {
		return result{}, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Report from %s to %s (UTC, end exclusive)\n", r.since.Format(time.DateOnly),
		r.until.Format(time.DateOnly))
	err = e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return reporter{q: models.New(tx), ws: s.m.Workspace.ID, r: r, b: &b}.report(ctx)
	})
	if err != nil {
		return result{}, fmt.Errorf("workspace report: %w", err)
	}
	s.tainted = true
	summary := fmt.Sprintf("Reported on %s to %s", r.since.Format(time.DateOnly), r.until.Format(time.DateOnly))
	return result{text: b.String(), step: Step{Tool: "workspace_report", Summary: summary}}, nil
}

/*
agentRuns lists the AI agent's runs on a conversation.
*/
func (e *Engine) agentRuns(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in numberArgs
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	r, err := ref(s, in.Number)
	if err != nil {
		return result{}, err
	}
	loaded, err := e.Services.Show.Handle(ctx, conversations.ShowConversationInput{Membership: s.m, Ref: r})
	if err != nil {
		return result{}, err
	}
	runs, err := e.Services.Runs.Handle(ctx, ai.ListRunsInput{Membership: s.m, ConversationID: loaded.Conversation.ID})
	if err != nil {
		return result{}, err
	}
	s.tainted = true
	var b strings.Builder
	shown := 0
	for _, run := range runs {
		if run.Kind != "conversation" || shown == 20 {
			continue
		}
		shown++
		fmt.Fprintf(&b, "- %s · %s · outcome %s", run.CreatedAt.UTC().Format("2006-01-02 15:04"), run.Status,
			deref(run.Outcome))
		writeRunDetails(&b, run)
	}
	if shown == 0 {
		b.WriteString("The AI agent has not run on this conversation.")
	}
	summary := fmt.Sprintf("Read the AI agent's runs on #%d", loaded.Conversation.Number)
	return result{text: b.String(), step: Step{Tool: "agent_runs", Summary: summary}}, nil
}

/*
writeRunDetails writes why a run ended as it did.
*/
func writeRunDetails(b *strings.Builder, run models.AiRun) {
	if run.HandoffReason != nil {
		fmt.Fprintf(b, " · handed off: %s", *run.HandoffReason)
	}
	if run.SkipReason != nil {
		fmt.Fprintf(b, " · skipped: %s", *run.SkipReason)
	}
	if run.ErrorCode != nil {
		fmt.Fprintf(b, " · error: %s", *run.ErrorCode)
	}
	if len(run.Flags) > 0 {
		fmt.Fprintf(b, " · flags: %s", strings.Join(run.Flags, ", "))
	}
	fmt.Fprintf(b, " · cost $%.4f\n", float64(run.CostMicros)/1e6)
	if run.QueryRewritten != nil {
		fmt.Fprintf(b, "  searched for: %s\n", untrusted(*run.QueryRewritten))
	}
	if len(run.Retrieval) > 2 {
		fmt.Fprintf(b, "  found: %s\n", truncate(string(run.Retrieval), 800))
	}
}
