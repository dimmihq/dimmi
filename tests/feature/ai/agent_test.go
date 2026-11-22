package ai_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestAiAgent(t *testing.T) {
	t.Parallel()

	t.Run("lists the default agent first, with public ids and never a secret", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-test")
		key := createKey(t, d, "Local")
		d.Owner.PostJSON("/api/v1/ai/keys", testkit.JSON{
			"name": "Anthropic", "provider": "anthropic", "api_key": "sk-ant-secret-value-1234",
		}).AssertCreated()
		path := agentPath(t, d)
		d.Owner.PatchJSON(path, testkit.JSON{"answer_key_id": key, "answer_model": "claude-test"}).AssertOK()

		list := d.Owner.GetJSON("/api/v1/ai/agents").AssertOK()
		list.AssertJSONPath("data.0.object", "ai_agent")
		list.AssertJSONPath("data.0.is_default", true)

		res := d.Owner.GetJSON(path).AssertOK()
		res.AssertJSONPath("data.agent.draft.answer_key_id", key)
		res.AssertJSONPath("data.keys.0.id", key)
		res.AssertJSONPath("data.keys.1.key_hint", "••••1234")
		assert.True(t, strings.HasPrefix(res.String("data.agent.id"), "aia_"))
		assert.NotEmpty(t, res.String("data.prompt_version"))
		body := string(res.Body)
		for _, leak := range []string{
			"sk-ant-secret", "credentials", "workspace_id", "created_by", d.TC.Workspace.Workspace.ID.String(),
		} {
			assert.NotContains(t, body, leak)
		}
	})

	t.Run("saves changes to the draft until the agent is published", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-test")
		key := createKey(t, d, "Anthropic")
		path := agentPath(t, d)

		res := d.Owner.PatchJSON(path, testkit.JSON{
			"name": "Ada", "answer_key_id": key, "answer_model": "claude-test", "guidance": "Be brief.",
			"enabled_channels": []string{"widget"},
		}).AssertOK()
		res.AssertJSONPath("data.agent.draft.name", "Ada")
		res.AssertJSONPath("data.agent.draft.guidance", "Be brief.")
		res.AssertJSONPath("data.agent.status", "draft")
		res.AssertJSONPath("data.agent.has_unpublished_changes", true)

		res = d.Owner.PostJSON(path+"/publications", testkit.JSON{"change_note": "First go"}).AssertOK()
		res.AssertJSONPath("data.agent.name", "Ada")
		res.AssertJSONPath("data.agent.guidance", "Be brief.")
		res.AssertJSONPath("data.agent.status", "live")
		res.AssertJSONPath("data.agent.enabled", true)
		res.AssertJSONPath("data.agent.live_version", 1)
		res.AssertJSONPath("data.agent.has_unpublished_changes", false)

		d.Owner.GetJSON(path+"/versions").AssertOK().
			AssertJSONPath("data.0.version", 1).
			AssertJSONPath("data.0.change_note", "First go").
			AssertJSONPath("data.0.definition.guidance", "Be brief.")
	})

	t.Run("drops a draft edited back to what is published", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-test")
		key := createKey(t, d, "Anthropic")
		path := agentPath(t, d)
		d.Owner.PatchJSON(path, testkit.JSON{"answer_key_id": key, "answer_model": "claude-test"}).AssertOK()
		d.Owner.PostJSON(path+"/publications", nil).AssertOK()

		d.Owner.PatchJSON(path, testkit.JSON{"tone": "concise"}).AssertOK().
			AssertJSONPath("data.agent.has_unpublished_changes", true)
		d.Owner.PatchJSON(path, testkit.JSON{"tone": "friendly"}).AssertOK()

		var draft []byte
		require.NoError(t, d.TC.Operator().QueryRow(t.Context(),
			"SELECT draft FROM ai_agents WHERE workspace_id = $1 AND is_default", d.TC.Workspace.Workspace.ID).
			Scan(&draft))
		assert.Nil(t, draft)
	})

	t.Run("puts an old version back as the draft, or publishes it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-test")
		key := createKey(t, d, "Anthropic")
		path := agentPath(t, d)
		d.Owner.PatchJSON(path, testkit.JSON{
			"answer_key_id": key, "answer_model": "claude-test", "guidance": "One.",
		}).AssertOK()
		d.Owner.PostJSON(path+"/publications", nil).AssertOK()
		d.Owner.PatchJSON(path, testkit.JSON{"guidance": "Two."}).AssertOK()
		d.Owner.PostJSON(path+"/publications", nil).AssertOK()

		d.Owner.PostJSON(path+"/versions/1/restore", nil).AssertOK().
			AssertJSONPath("data.agent.guidance", "Two.").
			AssertJSONPath("data.agent.draft.guidance", "One.")
		d.Owner.PostJSON(path+"/versions/1/restore", testkit.JSON{"publish": true}).AssertOK().
			AssertJSONPath("data.agent.guidance", "One.").
			AssertJSONPath("data.agent.live_version", 3)
		d.Owner.GetJSON(path + "/versions/9").AssertNotFound()
	})

	t.Run("turns an agent off and keeps its settings", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-test")
		key := createKey(t, d, "Anthropic")
		path := agentPath(t, d)
		d.Owner.PatchJSON(path, testkit.JSON{"answer_key_id": key, "answer_model": "claude-test"}).AssertOK()
		d.Owner.PostJSON(path+"/publications", nil).AssertOK()

		d.Owner.Delete(path+"/publications").AssertOK().
			AssertJSONPath("data.agent.status", "paused").
			AssertJSONPath("data.agent.enabled", false).
			AssertJSONPath("data.agent.answer_model", "claude-test")
		d.Owner.Delete(path + "/publications").AssertStatus(409)
	})

	t.Run("publishes only an agent that could answer", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-test")
		path := agentPath(t, d)

		d.Owner.PostJSON(path+"/publications", nil).AssertStatus(422)

		key := createKey(t, d, "Anthropic")
		d.Owner.PatchJSON(path, testkit.JSON{"answer_key_id": key, "answer_model": "not-offered"}).AssertOK()
		d.Owner.PostJSON(path+"/publications", nil).AssertStatus(422)

		d.Owner.PatchJSON(path, testkit.JSON{"answer_model": "claude-test"}).AssertOK().
			AssertJSONPath("data.ready", true)
		d.Owner.PostJSON(path+"/publications", nil).AssertOK().AssertJSONPath("data.agent.enabled", true)
	})

	t.Run("clears a nullable field sent as null or empty", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-test")
		key := createKey(t, d, "Anthropic")
		path := agentPath(t, d)
		d.Owner.PatchJSON(path, testkit.JSON{
			"fallback_key_id": key, "fallback_model": "claude-test", "product_name": "Acme",
		}).AssertOK()

		res := d.Owner.PatchJSON(path, testkit.JSON{
			"fallback_key_id": nil, "fallback_model": "", "product_name": nil,
		}).AssertOK()

		res.AssertJSONPath("data.agent.draft.fallback_key_id", nil)
		res.AssertJSONPath("data.agent.draft.fallback_model", nil)
		res.AssertJSONPath("data.agent.draft.product_name", nil)
	})

	t.Run("refuses values the agent does not understand", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := agentPath(t, d)
		otherTeam := models.PublicID(models.PrefixTeam, ids.New())
		otherKey := models.PublicID(models.PrefixAiProviderKey, ids.New())
		otherProcedure := models.PublicID(models.PrefixProcedure, ids.New())
		itself := models.PublicID(models.PrefixAiAgent, agentRow(t, d))

		cases := map[string]testkit.JSON{
			"include_contact_context": {"include_contact_context": "full"},
			"tone":                    {"tone": "angry"},
			"schedule":                {"schedule": "weekends"},
			"enabled_channels":        {"enabled_channels": []string{"widget", "fax"}},
			"languages":               {"languages": []string{"en", "not a language"}},
			"default_language":        {"default_language": "!!"},
			"handoff_team_id":         {"handoff_team_id": otherTeam},
			"answer_key_id":           {"answer_key_id": otherKey},
			"max_ai_replies":          {"max_ai_replies": 0},
			"link_allowlist":          {"link_allowlist": []string{"https://acme.com/path"}},
			"unknown_behavior":        {"unknown_behavior": "shrug"},
			"procedure_scope": {"procedure_scope": testkit.JSON{
				"mode": "selected", "ids": []string{otherProcedure},
			}},
			"transfer_agent_ids": {"transfer_agent_ids": []string{itself}},
		}
		for field, body := range cases {
			d.Owner.PatchJSON(path, body).AssertValidationError(field)
		}

		res := d.Owner.PatchJSON(path, testkit.JSON{"enabled_channels": []string{"fax"}})
		assert.Contains(t, string(res.Body), "widget, email, api, slack")
		assert.NotContains(t, string(res.Body), "{{")
	})

	t.Run("accepts every value the studio offers", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := agentPath(t, d)
		team, err := factories.Team(t.Context(), d.TC.App, factories.TeamOf{
			Workspace: d.TC.Workspace.Workspace, Name: "Billing",
		})
		require.NoError(t, err)
		teamID := models.PublicID(models.PrefixTeam, team.ID)

		res := d.Owner.PatchJSON(path, testkit.JSON{
			"assigned_behavior": "always", "include_contact_context": "none", "unknown_behavior": "handoff",
			"enabled_channels": []string{"widget", "email", "slack"}, "languages": []string{"en", "pt-BR"},
			"default_language": "de", "handoff_team_id": teamID, "raise_priority_on_handoff": true,
			"max_ai_replies": 8, "tone": "concise", "schedule": "outside_office_hours",
			"link_allowlist":  []string{"help.acme.com", "*.Acme.io"},
			"knowledge_scope": testkit.JSON{"mode": "selected"},
		}).AssertOK()

		res.AssertJSONPath("data.agent.draft.assigned_behavior", "always")
		res.AssertJSONPath("data.agent.draft.handoff_team_id", teamID)
		res.AssertJSONPath("data.agent.draft.raise_priority_on_handoff", true)
		res.AssertJSONPath("data.agent.draft.languages.1", "pt-BR")
		res.AssertJSONPath("data.agent.draft.max_ai_replies", 8)
		res.AssertJSONPath("data.agent.draft.link_allowlist", []string{"help.acme.com", "*.acme.io"})
		res.AssertJSONPath("data.agent.draft.knowledge_scope.mode", "selected")

		d.Owner.PatchJSON(path, testkit.JSON{"handoff_team_id": ""}).AssertOK().
			AssertJSONPath("data.agent.draft.handoff_team_id", nil).
			AssertJSONPath("data.agent.draft.raise_priority_on_handoff", true)
	})

	t.Run("saves the agent's role and the tags that always hand off", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := agentPath(t, d)
		legal := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Legal"}).AssertCreated().String("data.id")
		refunds := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Refunds"}).AssertCreated().String("data.id")

		d.Owner.GetJSON(path).AssertOK().
			AssertJSONPath("data.agent.handoff_mode", "full").
			AssertJSONPath("data.agent.always_handoff_tag_ids", []string{})

		d.Owner.PatchJSON(path, testkit.JSON{
			"handoff_mode": "triage_only", "always_handoff_tag_ids": []string{legal, refunds, legal},
		}).AssertOK().
			AssertJSONPath("data.agent.draft.handoff_mode", "triage_only").
			AssertJSONPath("data.agent.draft.always_handoff_tag_ids", []string{legal, refunds})

		d.Owner.PatchJSON(path, testkit.JSON{"always_handoff_tag_ids": nil}).AssertOK().
			AssertJSONPath("data.agent.draft.always_handoff_tag_ids", []string{})

		other := models.PublicID(models.PrefixTag, ids.New())
		d.Owner.PatchJSON(path, testkit.JSON{"handoff_mode": "answer_everything"}).
			AssertValidationError("handoff_mode")
		d.Owner.PatchJSON(path, testkit.JSON{"always_handoff_tag_ids": []string{legal, other}}).
			AssertValidationError("always_handoff_tag_ids")
	})

	t.Run("makes agents from templates and deletes all but the default", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		templates := d.Owner.GetJSON("/api/v1/ai/agent-templates").AssertOK()
		templates.AssertJSONPath("data.0.object", "ai_agent_template")

		sales := d.Owner.PostJSON("/api/v1/ai/agents", testkit.JSON{"name": "Sales", "template": "sales"}).
			AssertCreated()
		sales.AssertJSONPath("data.agent.status", "draft")
		sales.AssertJSONPath("data.agent.is_default", false)
		sales.AssertJSONPath("data.agent.role", "sales")
		sales.AssertJSONPath("data.agent.template", "sales")
		sales.AssertJSONPath("data.agent.draft.name", "Sales")
		assert.Contains(t, sales.String("data.agent.draft.guidance"), "Qualifying")

		d.Owner.PostJSON("/api/v1/ai/agents", testkit.JSON{"name": "sales"}).AssertValidationError("name")
		d.Owner.PostJSON("/api/v1/ai/agents", testkit.JSON{"name": "X", "template": "nope"}).
			AssertValidationError("template")
		d.Owner.GetJSON("/api/v1/ai/agents").AssertOK().AssertJSONPath("data.#", 2)

		d.Owner.Delete("/api/v1/ai/agents/" + sales.String("data.agent.id")).AssertNoContent()
		d.Owner.Delete(agentPath(t, d)).AssertStatus(409)
		d.Owner.GetJSON("/api/v1/ai/agents").AssertOK().AssertJSONPath("data.#", 1)
	})

	t.Run("puts a published agent on the channels it names", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-test")
		key := createKey(t, d, "Anthropic")
		sales := d.Owner.PostJSON("/api/v1/ai/agents", testkit.JSON{"name": "Sales", "template": "sales"}).
			AssertCreated().String("data.agent.id")
		var conn ids.UUID
		require.NoError(t, d.TC.Operator().QueryRow(t.Context(),
			"SELECT id FROM channel_connections WHERE workspace_id = $1 AND deleted_at IS NULL LIMIT 1",
			d.TC.Workspace.Workspace.ID).Scan(&conn))
		connID := models.PublicID(models.PrefixChannelConnection, conn)

		d.Owner.PatchJSON("/api/v1/ai/agents/"+sales, testkit.JSON{
			"answer_key_id": key, "answer_model": "claude-test",
			"placement": testkit.JSON{"channel_connection_ids": []string{connID}},
		}).AssertOK().AssertJSONPath("data.agent.draft.placement.channel_connection_ids", []string{connID})
		placed := func() *ids.UUID {
			var agent *ids.UUID
			require.NoError(t, d.TC.Operator().QueryRow(t.Context(),
				"SELECT ai_agent_id FROM channel_connections WHERE id = $1", conn).Scan(&agent))
			return agent
		}
		assert.Nil(t, placed(), "a draft places nothing")

		d.Owner.PostJSON("/api/v1/ai/agents/"+sales+"/publications", nil).AssertOK().
			AssertJSONPath("data.agent.placement.channel_connection_ids", []string{connID})
		require.NotNil(t, placed())
		assert.Equal(t, sales, models.PublicID(models.PrefixAiAgent, *placed()))
	})

	t.Run("keeps the workspace's spend cap and models in shared settings", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		agentRow(t, d)

		d.Owner.GetJSON("/api/v1/ai/settings").AssertOK().AssertJSONPath("data.object", "ai_settings")
		d.Owner.PatchJSON("/api/v1/ai/settings", testkit.JSON{"monthly_cap_usd": 12.5}).AssertOK().
			AssertJSONPath("data.monthly_spend_cap_micros", 12_500_000)
		d.Owner.PatchJSON("/api/v1/ai/settings", testkit.JSON{"monthly_cap_usd": 0}).AssertOK().
			AssertJSONPath("data.monthly_spend_cap_micros", nil)
		d.Owner.PatchJSON("/api/v1/ai/settings", testkit.JSON{
			"classifier_key_id": models.PublicID(models.PrefixAiProviderKey, ids.New()),
		}).AssertValidationError("classifier_key_id")
	})

	t.Run("lifts the month's pause when the cap rises above the spend", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		agentRow(t, d)
		_, err := d.TC.Operator().Exec(t.Context(), `
			INSERT INTO ai_spend_periods (workspace_id, period_start, cost_micros, capped_at)
			VALUES ($1, date_trunc('month', $2::timestamptz AT TIME ZONE 'UTC')::date, 3000000, $2)`,
			d.TC.Workspace.Workspace.ID, d.TC.Clock.Now())
		require.NoError(t, err)

		d.Owner.PatchJSON("/api/v1/ai/settings", testkit.JSON{"monthly_cap_usd": 50}).AssertOK()

		d.Owner.GetJSON("/api/v1/ai/usage").AssertOK().AssertJSONPath("data.capped_at", nil)
	})

	t.Run("lets members read agents and only admins change them", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := agentPath(t, d)
		member, _ := d.Join(t, enums.RoleMember)
		admin, _ := d.Join(t, enums.RoleAdmin)

		member.GetJSON(path).AssertOK()
		member.GetJSON("/api/v1/ai/agents").AssertOK()
		member.PatchJSON(path, testkit.JSON{"name": "Mallory"}).AssertForbidden()
		member.PostJSON("/api/v1/ai/agents", testkit.JSON{"name": "Mallory"}).AssertForbidden()
		member.PostJSON(path+"/publications", nil).AssertForbidden()
		member.PatchJSON("/api/v1/ai/settings", testkit.JSON{"monthly_cap_usd": 0}).AssertForbidden()
		admin.PatchJSON(path, testkit.JSON{"name": "Ada"}).AssertOK()
	})
}
