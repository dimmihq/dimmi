/*
|--------------------------------------------------------------------------
| Reading What Was Recorded
|--------------------------------------------------------------------------
|
| The harness marks each simulated conversation so it can be told apart
| from real ones, and reads its cost back from the runs the pipeline
| recorded: how many runs and replies, how many model calls, searches
| and other tool calls, the fresh, cached and written tokens, and the
| cost of each run and each reply.
|
*/

package costsim

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
mark tags the conversation as simulated, and signs a signed-in
scenario's customer in under the account, the way an identity token
would.
*/
func (h *Harness) mark(ctx context.Context, conv models.Conversation, sc Scenario, account string) error {
	return h.within(ctx, func(ctx context.Context) error {
		db := h.App.DB
		if _, err := db.Exec(ctx, `UPDATE conversations SET source = source || '{"simulation": true}'::jsonb
			WHERE workspace_id = $1 AND id = $2`, conv.WorkspaceID, conv.ID); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `UPDATE contacts SET locale = $3, kind = CASE WHEN $4 THEN 'user' ELSE kind END,
				external_id = CASE WHEN $4 THEN $5 ELSE external_id END
			WHERE workspace_id = $1 AND id = $2`, conv.WorkspaceID, conv.ContactID, sc.Locale, sc.Signed,
			account); err != nil {
			return err
		}
		_, err := db.Exec(ctx, `WITH tag AS (
				INSERT INTO tags (workspace_id, name, color) VALUES ($1, 'cost-simulation', 'gray')
				ON CONFLICT (workspace_id, name) DO UPDATE SET name = EXCLUDED.name RETURNING id)
			INSERT INTO conversation_tag (workspace_id, conversation_id, tag_id, created_by_type)
			SELECT $1, $2, id, 'system' FROM tag ON CONFLICT DO NOTHING`, conv.WorkspaceID, conv.ID)
		return err
	})
}

/*
channel moves the conversation to the connection.
*/
func (h *Harness) channel(ctx context.Context, conv models.Conversation, connection ids.UUID) error {
	return h.within(ctx, func(ctx context.Context) error {
		_, err := h.App.DB.Exec(ctx, `UPDATE conversations SET channel_connection_id = $3
			WHERE workspace_id = $1 AND id = $2`, conv.WorkspaceID, conv.ID, connection)
		return err
	})
}

/*
body is a message's text, "" when it cannot be read.
*/
func (h *Harness) body(ctx context.Context, conv models.Conversation, id ids.UUID) string {
	var text string
	_ = h.within(ctx, func(ctx context.Context) error {
		msg, err := models.New(h.App.DB).FindConversationMessage(ctx, models.FindConversationMessageParams{
			WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, ID: id,
		})
		if err == nil && msg.Body != nil {
			text = *msg.Body
		}
		return nil
	})
	return text
}

/*
read fills the result from the runs the pipeline recorded.
*/
func (h *Harness) read(ctx context.Context, conv models.Conversation, res *ConversationResult) error {
	return h.within(ctx, func(ctx context.Context) error {
		if err := h.readRuns(ctx, conv, res); err != nil {
			return err
		}
		if err := h.readSteps(ctx, conv, res); err != nil {
			return err
		}
		if err := h.readReplies(ctx, conv, res); err != nil {
			return err
		}
		runs, err := h.runIDs(ctx, `SELECT id FROM ai_runs WHERE workspace_id = $1 AND conversation_id = $2
			AND kind = 'conversation'`, conv.WorkspaceID, conv.ID)
		if err != nil {
			return fmt.Errorf("read run ids: %w", err)
		}
		if res.Layer, err = h.readLayer(ctx, conv.WorkspaceID, runs); err != nil {
			return err
		}
		if res.Ledger, err = h.readLedger(ctx, conv.WorkspaceID, runs); err != nil {
			return err
		}
		return h.readAnswer(ctx, conv, res)
	})
}

/*
readRuns reads the runs' sums and the conversation's own total.
*/
func (h *Harness) readRuns(ctx context.Context, conv models.Conversation, res *ConversationResult) error {
	err := h.App.DB.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE answer_message_id IS NOT NULL),
			coalesce(sum(cost_micros), 0), coalesce(sum(input_tokens), 0), coalesce(sum(cached_input_tokens), 0),
			coalesce(sum(output_tokens), 0),
			(SELECT ai_cost_micros FROM conversations WHERE workspace_id = $1 AND id = $2)
		FROM ai_runs WHERE workspace_id = $1 AND conversation_id = $2 AND kind = 'conversation'`,
		conv.WorkspaceID, conv.ID).Scan(&res.Runs, &res.AIReplies, &res.CostMicros, &res.NewTokens,
		&res.CachedTokens, &res.OutputTokens, &res.ConvCost)
	if err != nil {
		return fmt.Errorf("read runs: %w", err)
	}
	return nil
}

/*
readSteps counts the runs' model calls by model, their searches and
their other tool calls.
*/
func (h *Harness) readSteps(ctx context.Context, conv models.Conversation, res *ConversationResult) error {
	rows, err := h.App.DB.Query(ctx, `SELECT s.kind, coalesce(s.model, ''), s.provider IS NOT NULL
		FROM ai_run_steps s JOIN ai_runs r ON r.id = s.run_id
		WHERE r.workspace_id = $1 AND r.conversation_id = $2 AND r.kind = 'conversation'`, conv.WorkspaceID, conv.ID)
	if err != nil {
		return fmt.Errorf("read steps: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind, model string
		var call bool
		if err := rows.Scan(&kind, &model, &call); err != nil {
			return err
		}
		countStep(res, kind, model, call)
	}
	return rows.Err()
}

/*
countStep counts one step: a model call, a search or another tool call.
*/
func countStep(res *ConversationResult, kind, model string, call bool) {
	switch {
	case call:
		res.ModelCalls++
		res.Models[model]++
	case kind == string(enums.AIRunStepKindToolCall) && model == "search_knowledge":
		res.Searches++
	case kind == string(enums.AIRunStepKindToolCall):
		res.ToolCalls++
	}
}

/*
readAnswer reads the agent's last reply and the addresses of the
passages its runs kept.
*/
func (h *Harness) readAnswer(ctx context.Context, conv models.Conversation, res *ConversationResult) error {
	err := h.App.DB.QueryRow(ctx, `SELECT coalesce((SELECT body FROM messages WHERE workspace_id = $1
			AND conversation_id = $2 AND author_type = 'agent' AND type = 'message'
			ORDER BY created_at DESC LIMIT 1), '')`,
		conv.WorkspaceID, conv.ID).Scan(&res.Reply)
	if err != nil {
		return fmt.Errorf("read reply: %w", err)
	}
	return h.App.DB.QueryRow(ctx, `SELECT coalesce(array_agg(DISTINCT s ->> 'url') FILTER (WHERE s ->> 'url' <> ''),
			'{}') FROM ai_runs r, jsonb_array_elements(coalesce(r.retrieval, '[]')) s
		WHERE r.workspace_id = $1 AND r.conversation_id = $2`, conv.WorkspaceID, conv.ID).Scan(&res.Retrieved)
}

/*
readReplies reads what each run that replied cost.
*/
func (h *Harness) readReplies(ctx context.Context, conv models.Conversation, res *ConversationResult) error {
	rows, err := h.App.DB.Query(ctx, `SELECT cost_micros FROM ai_runs WHERE workspace_id = $1 AND conversation_id = $2
		AND kind = 'conversation' AND answer_message_id IS NOT NULL ORDER BY created_at`, conv.WorkspaceID, conv.ID)
	if err != nil {
		return fmt.Errorf("read reply costs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c int64
		if err := rows.Scan(&c); err != nil {
			return err
		}
		res.ReplyCosts = append(res.ReplyCosts, c)
	}
	return rows.Err()
}
