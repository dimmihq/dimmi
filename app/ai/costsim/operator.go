/*
|--------------------------------------------------------------------------
| Operator Runs
|--------------------------------------------------------------------------
|
| The same measurement for Operator, the team's own agent: each request
| is asked on a fresh thread by a teammate, through Operator's own
| engine, under the same spend guard, and its cost is read back from
| the task run Operator recorded.
|
*/

package costsim

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/auth/authz"
)

/*
RunOperator asks Operator each request, as the teammates in turn, and
reports what each run cost. Each request counts as one conversation of
kind operator.
*/
func (h *Harness) RunOperator(ctx context.Context, o Options, requests []string, team []authz.Membership,
) (*Report, error) {
	if len(requests) == 0 || len(team) == 0 {
		return nil, errors.New("no requests or no teammates")
	}
	if h.App.Actions.OperatorAgent == nil {
		return nil, errors.New("operator is not booted")
	}
	ctx, p, err := h.begin(ctx, &o)
	if err != nil {
		return nil, err
	}
	defer p.restore()

	report := h.newReport(o, p)
	for i := range o.Conversations {
		if !h.roomFor(p) {
			report.StoppedBy = "run spend limit"
			break
		}
		res := h.ask(ctx, team[i%len(team)], i, requests)
		report.Conversations = append(report.Conversations, res)
		o.Log("%3d %-28s calls %2d tools %2d  $%.4f  %s", i+1, res.Scenario, res.ModelCalls, res.ToolCalls,
			float64(res.CostMicros)/1e6, res.Outcome)
		if res.Outcome == "stopped_at_run_limit" {
			report.StoppedBy = "run spend limit"
			break
		}
	}
	return h.finish(report), nil
}

/*
ask asks Operator the i-th request on a fresh thread, as the teammate,
and reads back what it cost.
*/
func (h *Harness) ask(ctx context.Context, m authz.Membership, i int, requests []string) ConversationResult {
	a := h.App
	h.Budget.StartConversation()
	res := ConversationResult{
		Index: i + 1, Scenario: fmt.Sprintf("operator-%02d", i%len(requests)+1),
		Kind: "operator", Turns: 1, Models: map[string]int{}, Outcome: "answered",
	}
	since := a.Clock.Now()
	thread, err := a.Actions.OperatorAgent.Threads.Open(ctx, m, nil, true)
	if err == nil {
		_, err = a.Actions.OperatorAgent.Engine.Ask(ctx, operator.Question{
			Membership: m, ThreadID: thread.Thread.ID, Text: requests[i%len(requests)],
		})
	}
	if err != nil {
		res.Outcome, res.Error = "failed", err.Error()
	}
	if by := h.Budget.Refused(); by != "" {
		res.Outcome = "stopped_at_" + by + "_limit"
	}
	if err := h.readTask(ctx, m, since, &res); err != nil && res.Error == "" {
		res.Error = err.Error()
	}
	return res
}

/*
readTaskParts reads the tool layer's figures and the token ledger of
the teammate's task runs since the moment.
*/
func (h *Harness) readTaskParts(ctx context.Context, m authz.Membership, since time.Time,
	res *ConversationResult,
) error {
	ws := m.Workspace.ID
	runs, err := h.runIDs(ctx, `SELECT id FROM ai_runs WHERE workspace_id = $1 AND kind = 'task'
		AND actor_id = $2 AND created_at >= $3`, ws, m.Member.ID, since)
	if err != nil {
		return fmt.Errorf("read task run ids: %w", err)
	}
	if res.Layer, err = h.readLayer(ctx, ws, runs); err != nil {
		return err
	}
	res.Ledger, err = h.readLedger(ctx, ws, runs)
	return err
}

/*
readTask fills the result from the task runs the teammate started
since the moment.
*/
func (h *Harness) readTask(ctx context.Context, m authz.Membership, since time.Time, res *ConversationResult) error {
	return h.within(ctx, func(ctx context.Context) error {
		db := h.App.DB
		ws := m.Workspace.ID
		err := db.QueryRow(ctx, `SELECT count(*), coalesce(sum(cost_micros), 0), coalesce(sum(input_tokens), 0),
				coalesce(sum(cached_input_tokens), 0), coalesce(sum(output_tokens), 0)
			FROM ai_runs WHERE workspace_id = $1 AND kind = 'task' AND actor_id = $2 AND created_at >= $3`,
			ws, m.Member.ID, since).Scan(&res.Runs, &res.CostMicros, &res.NewTokens, &res.CachedTokens,
			&res.OutputTokens)
		if err != nil {
			return fmt.Errorf("read task runs: %w", err)
		}
		res.AIReplies = res.Runs
		res.ReplyCosts = []int64{res.CostMicros}
		rows, err := db.Query(ctx, `SELECT coalesce(s.model, ''), s.provider IS NOT NULL
			FROM ai_run_steps s JOIN ai_runs r ON r.id = s.run_id
			WHERE r.workspace_id = $1 AND r.kind = 'task' AND r.actor_id = $2 AND r.created_at >= $3`,
			ws, m.Member.ID, since)
		if err != nil {
			return fmt.Errorf("read task steps: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var model string
			var call bool
			if err := rows.Scan(&model, &call); err != nil {
				return err
			}
			if call {
				res.ModelCalls++
				res.Models[model]++
			} else {
				res.ToolCalls++
			}
		}
		rows.Close()
		return h.readTaskParts(ctx, m, since, res)
	})
}
