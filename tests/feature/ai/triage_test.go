package ai_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
triageCall is a scripted triage_conversation call.
*/
func triageCall(args map[string]any) contracts.ToolCall {
	data, _ := json.Marshal(args)
	return call("triage_conversation", string(data))
}

/*
conversationTags lists the conversation's tag ids and who added each.
*/
func (h *harness) conversationTags(conv ids.UUID) map[ids.UUID]string {
	h.t.Helper()
	out := map[ids.UUID]string{}
	h.d.TC.InWorkspace(func(ctx context.Context) error {
		rows, err := models.New(h.d.TC.App.DB).ListConversationTagAuthors(ctx, models.ListConversationTagAuthorsParams{
			WorkspaceID: h.d.TC.Workspace.Workspace.ID, ConversationID: conv,
		})
		for _, r := range rows {
			out[r.TagID] = r.CreatedByType
		}
		return err
	})
	return out
}

/*
TestAiTriage lets the agent keep the inbox's view of a conversation
current while a teammate's choices always win.
*/
func TestAiTriage(t *testing.T) {
	t.Parallel()

	t.Run("sets the title, tags, team and priority as the agent", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		billing := h.tag("billing")
		team := h.team("Billing")
		h.llm.answer.Script(tools(triageCall(map[string]any{
			"title": "Double charge refund", "tags": []string{"Billing", "vip"}, "team": "billing", "priority": true,
		})), reply("Let me look into that."))

		conv, trigger := h.conversation("I was charged twice")
		h.operator("UPDATE conversations SET subject = NULL WHERE id = $1", conv)
		res := h.run(conv, trigger)
		require.Equal(t, "greeted", res.Outcome)

		s := h.load(conv, res.RunID)
		assert.Equal(t, "Double charge refund", str(s.conv.Subject))
		require.NotNil(t, s.conv.AssigneeTeamID)
		assert.Equal(t, team.ID, *s.conv.AssigneeTeamID)
		assert.Positive(t, s.conv.Priority)
		assert.Equal(t, map[ids.UUID]string{billing.ID: "agent"}, h.conversationTags(conv))

		var result map[string]any
		require.NoError(t, json.Unmarshal([]byte(h.llm.answer.Sent()[1].Messages[2].Parts[0].Text), &result))
		assert.Equal(t, []any{"tag vip"}, result["ignored"])

		sent := h.llm.answer.Sent()[0]
		for _, spec := range sent.Tools {
			if spec.Name == "triage_conversation" {
				assert.Contains(t, spec.Description, "billing")
				assert.Contains(t, spec.Description, "Billing")
			}
		}
	})

	t.Run("keeps a teammate's tags and subject and replaces only its own tags", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		billing, urgent, sales := h.tag("billing"), h.tag("urgent"), h.tag("sales")

		conv, first := h.conversation("I was charged twice")
		h.operator(`INSERT INTO conversation_tag (workspace_id, conversation_id, tag_id, created_by_type, created_at)
			VALUES ($1, $2, $3, 'member', now()), ($1, $2, $4, 'agent', now())`,
			h.d.TC.Workspace.Workspace.ID, conv, urgent.ID, sales.ID)
		h.operator("UPDATE conversations SET subject = 'Refund request' WHERE id = $1", conv)

		h.llm.answer.Script(tools(triageCall(map[string]any{"title": "Billing question", "tags": []string{"billing"}})),
			reply("Let me check."))
		res := h.run(conv, first)

		assert.Equal(t, map[ids.UUID]string{urgent.ID: "member", billing.ID: "agent"}, h.conversationTags(conv))
		s := h.load(conv, res.RunID)
		assert.Equal(t, "Refund request", str(s.conv.Subject))

		result := h.llm.answer.Sent()[1].Messages[2].Parts[0].Text
		assert.Contains(t, result, "tag urgent added by a teammate")
		assert.Contains(t, result, "title set by a teammate or the customer")
	})

	t.Run("never overrides a teammate's assignment", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.operator("ALTER TABLE ai_agents DROP CONSTRAINT IF EXISTS ai_agents_assigned_behavior_check")
		h.operator("UPDATE ai_agents SET assigned_behavior = 'always' WHERE workspace_id = $1",
			h.d.TC.Workspace.Workspace.ID)
		h.team("Billing")
		member := h.d.TC.Workspace.Owner.ID

		conv, trigger := h.conversation("I was charged twice")
		h.operator("UPDATE conversations SET assignee_member_id = $2 WHERE id = $1", conv, member)
		h.llm.answer.Script(tools(triageCall(map[string]any{"team": "Billing"})), reply("Let me check."))
		res := h.run(conv, trigger)

		s := h.load(conv, res.RunID)
		assert.Nil(t, s.conv.AssigneeTeamID)
		assert.Contains(t, h.llm.answer.Sent()[1].Messages[2].Parts[0].Text, "assignment made by a teammate")
	})
}
