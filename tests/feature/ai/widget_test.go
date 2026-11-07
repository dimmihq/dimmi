package ai_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
visitor is a messenger session on the workspace's first widget.
*/
type visitor struct {
	d     tests.Desk
	token string
}

/*
newVisitor starts an anonymous messenger session.
*/
func newVisitor(t *testing.T, d tests.Desk) visitor {
	t.Helper()

	res := d.TC.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": appOf(t, d)}).AssertCreated()

	return visitor{d: d, token: res.String("data.token")}
}

/*
appOf returns the public id of the workspace's first messenger.
*/
func appOf(t *testing.T, d tests.Desk) string {
	t.Helper()

	var app string
	d.TC.InWorkspace(func(ctx context.Context) error {
		rows, err := models.New(d.TC.App.DB).ListWidgetSettings(ctx, d.TC.Workspace.Workspace.ID)
		if err != nil {
			return err
		}
		require.NotEmpty(t, rows)
		app = models.PublicID(models.PrefixWidget, rows[0].ID)
		return nil
	})

	return app
}

/*
client returns the visitor's browser.
*/
func (v visitor) client() *testkit.Client {
	return v.d.TC.Client().WithHeader("Authorization", "Bearer "+v.token)
}

/*
start has the visitor open a conversation and returns its public and
internal ids.
*/
func (v visitor) start(t *testing.T) (string, ids.UUID) {
	t.Helper()

	public := v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
		"client_id": ids.New().String(), "body": "Where is my invoice?",
	}).AssertCreated().String("data.id")
	id, err := v.d.TC.App.IDs.Decode(string(models.PrefixConversation), public)
	require.NoError(t, err)

	return public, id
}

/*
aiState reads the conversation's agent state and how many messages the
agent wrote in it.
*/
func aiState(t *testing.T, d tests.Desk, conv ids.UUID) (string, int) {
	t.Helper()

	var state string
	var agentLines int
	err := d.TC.Operator().QueryRow(t.Context(), `
		SELECT c.ai_state, (SELECT count(*) FROM messages m WHERE m.conversation_id = c.id AND m.author_type = 'agent')
		FROM conversations c WHERE c.id = $1`, conv).Scan(&state, &agentLines)
	require.NoError(t, err)

	return state, agentLines
}

func TestAiWidgetControls(t *testing.T) {
	t.Parallel()

	t.Run("records that the agent's latest answer helped", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		agent := agentRow(t, d)
		v := newVisitor(t, d)
		conv, convID := v.start(t)
		older := seedRun(t, d, agent, run{
			conversation: &convID, outcome: "answered", at: d.TC.Clock.Now().Add(-time.Minute),
		})
		latest := seedRun(t, d, agent, run{conversation: &convID, outcome: "answered", at: d.TC.Clock.Now()})

		v.client().PostJSON("/widget/v1/conversations/"+conv+"/ai/helped", nil).AssertNoContent()
		v.client().PostJSON("/widget/v1/conversations/"+conv+"/ai/helped", nil).AssertNoContent()

		d.TC.Table("ai_runs").AssertCount(1, testkit.Row{"id": latest, "feedback": "helped"})
		d.TC.Table("ai_runs").AssertCount(1, testkit.Row{"id": older, "outcome": "answered"})
		d.TC.Table("ai_runs").AssertCount(0, testkit.Row{"id": older, "feedback": "helped"})
	})

	t.Run("does nothing when the agent never answered", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		v := newVisitor(t, d)
		conv, _ := v.start(t)

		v.client().PostJSON("/widget/v1/conversations/"+conv+"/ai/helped", nil).AssertNoContent()
	})

	t.Run("hands a conversation over once however often the visitor asks", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		agentRow(t, d)
		v := newVisitor(t, d)
		conv, convID := v.start(t)

		v.client().PostJSON("/widget/v1/conversations/"+conv+"/ai/handoff", nil).AssertNoContent()
		v.client().PostJSON("/widget/v1/conversations/"+conv+"/ai/handoff", nil).AssertNoContent()

		state, lines := aiState(t, d, convID)
		assert.Equal(t, "handed_off", state)
		assert.Equal(t, 1, lines)
	})

	t.Run("slows down a visitor pressing handoff over and over", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		agentRow(t, d)
		v := newVisitor(t, d)
		conv, _ := v.start(t)

		for range 5 {
			v.client().PostJSON("/widget/v1/conversations/"+conv+"/ai/handoff", nil).AssertNoContent()
		}
		v.client().PostJSON("/widget/v1/conversations/"+conv+"/ai/handoff", nil).AssertStatus(429)
	})

	t.Run("shows the sources an agent reply cites to the visitor and the team", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		agent := agentRow(t, d)
		v := newVisitor(t, d)
		conv, convID := v.start(t)
		runID := seedRun(t, d, agent, run{conversation: &convID, outcome: "answered", at: d.TC.Clock.Now()})
		_, err := d.TC.Operator().Exec(t.Context(), `
			INSERT INTO messages (workspace_id, conversation_id, type, author_type, author_id, body, metadata)
			VALUES ($1, $2, 'message', 'agent', $3, 'See the billing guide [1].', jsonb_build_object('ai',
				jsonb_build_object('run_id', $4::text, 'generated', true, 'outcome', 'answered',
					'sources', jsonb_build_array(jsonb_build_object('title', 'Billing', 'url', 'https://help.acme.com')))))`,
			d.TC.Workspace.Workspace.ID, convID, agent, runID.String())
		require.NoError(t, err)
		source := []testkit.JSON{{"title": "Billing", "url": "https://help.acme.com"}}

		visible := v.client().GetJSON("/widget/v1/conversations/" + conv + "/messages").AssertOK()
		visible.AssertJSONPath("data.#(author.type==\"agent\").ai.sources", source)
		visible.AssertJSONMissing("data.#(author.type==\"agent\").ai.run_id")
		visible.AssertJSONPath("data.#(author.type==\"agent\").ai.outcome", "answered")
		visible.AssertJSONMissing("data.#(author.type==\"contact\").ai")

		timeline := d.Owner.GetJSON("/api/v1/conversations/" + conv + "/messages").AssertOK()
		timeline.AssertJSONPath("data.#(author.type==\"agent\").ai.sources", source)
		timeline.AssertJSONPath("data.#(author.type==\"agent\").ai.run_id",
			models.PublicID(models.PrefixAiRun, runID))
	})

	t.Run("answers not found for another visitor's conversation", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		agent := agentRow(t, d)
		owner := newVisitor(t, d)
		conv, convID := owner.start(t)
		seedRun(t, d, agent, run{conversation: &convID, outcome: "answered", at: d.TC.Clock.Now()})
		stranger := newVisitor(t, d)

		stranger.client().PostJSON("/widget/v1/conversations/"+conv+"/ai/helped", nil).AssertNotFound()
		stranger.client().PostJSON("/widget/v1/conversations/"+conv+"/ai/handoff", nil).AssertNotFound()

		state, lines := aiState(t, d, convID)
		assert.NotEqual(t, "handed_off", state)
		assert.Equal(t, 0, lines)
		d.TC.Table("ai_runs").AssertCount(0, testkit.Row{"conversation_id": convID, "feedback": "helped"})
	})
}
