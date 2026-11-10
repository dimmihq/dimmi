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

	t.Run("shows the agent with public ids and never a secret", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-test")
		key := createKey(t, d, "Local")
		d.Owner.PostJSON("/api/v1/ai/keys", testkit.JSON{
			"name": "Anthropic", "provider": "anthropic", "api_key": "sk-ant-secret-value-1234",
		}).AssertCreated()
		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"answer_key_id": key, "answer_model": "claude-test"}).
			AssertOK()

		res := d.Owner.GetJSON("/api/v1/ai/agent").AssertOK()

		res.AssertJSONPath("data.agent.object", "ai_agent")
		res.AssertJSONPath("data.agent.answer_key_id", key)
		res.AssertJSONPath("data.keys.0.id", key)
		res.AssertJSONPath("data.keys.1.key_hint", "••••1234")
		assert.True(t, strings.HasPrefix(res.String("data.agent.id"), "aia_"))
		assert.True(t, strings.HasPrefix(key, "aik_"))
		assert.NotEmpty(t, res.String("data.prompt_version"))
		body := string(res.Body)
		for _, leak := range []string{
			"sk-ant-secret", "credentials", "workspace_id", "created_by", d.TC.Workspace.Workspace.ID.String(),
		} {
			assert.NotContains(t, body, leak)
		}
	})

	t.Run("changes only the fields a save sends", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-test")
		key := createKey(t, d, "Anthropic")
		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{
			"answer_key_id": key, "answer_model": "claude-test", "fallback_key_id": key,
			"fallback_model": "claude-test", "guidance": "Be brief.",
		}).AssertOK()

		res := d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"name": "Ada"}).AssertOK()

		res.AssertJSONPath("data.agent.name", "Ada")
		res.AssertJSONPath("data.agent.answer_key_id", key)
		res.AssertJSONPath("data.agent.answer_model", "claude-test")
		res.AssertJSONPath("data.agent.fallback_key_id", key)
		res.AssertJSONMissing("data.agent.classifier_key_id")
		res.AssertJSONPath("data.agent.guidance", "Be brief.")
	})

	t.Run("clears a nullable field sent as null or empty", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-test")
		key := createKey(t, d, "Anthropic")
		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{
			"fallback_key_id": key, "fallback_model": "claude-test", "product_name": "Acme",
		}).AssertOK()

		res := d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{
			"fallback_key_id": nil, "fallback_model": "", "product_name": nil,
		}).AssertOK()

		res.AssertJSONPath("data.agent.fallback_key_id", nil)
		res.AssertJSONPath("data.agent.fallback_model", nil)
		res.AssertJSONPath("data.agent.product_name", nil)
	})

	t.Run("treats a cap of zero or null as no cap", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"monthly_cap_usd": 12.5}).AssertOK().
			AssertJSONPath("data.agent.monthly_spend_cap_micros", 12_500_000)
		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"monthly_cap_usd": 0}).AssertOK().
			AssertJSONPath("data.agent.monthly_spend_cap_micros", nil)
		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"monthly_cap_usd": 5}).AssertOK()
		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"monthly_cap_usd": nil}).AssertOK().
			AssertJSONPath("data.agent.monthly_spend_cap_micros", nil)
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

		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"monthly_cap_usd": 50}).AssertOK()

		d.Owner.GetJSON("/api/v1/ai/usage").AssertOK().AssertJSONPath("data.capped_at", nil)
	})

	t.Run("refuses values the agent does not understand", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		otherTeam := models.PublicID(models.PrefixTeam, ids.New())
		otherKey := models.PublicID(models.PrefixAiProviderKey, ids.New())

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
		}
		for field, body := range cases {
			d.Owner.PatchJSON("/api/v1/ai/agent", body).AssertValidationError(field)
		}

		res := d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"enabled_channels": []string{"fax"}})
		assert.Contains(t, string(res.Body), "widget, email, api, slack")
		assert.NotContains(t, string(res.Body), "{{")
	})

	t.Run("accepts every value the settings page offers", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		team, err := factories.Team(t.Context(), d.TC.App, factories.TeamOf{
			Workspace: d.TC.Workspace.Workspace, Name: "Billing",
		})
		require.NoError(t, err)
		teamID := models.PublicID(models.PrefixTeam, team.ID)

		res := d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{
			"assigned_behavior": "always", "include_contact_context": "none", "unknown_behavior": "handoff",
			"enabled_channels": []string{"widget", "email", "slack"}, "languages": []string{"en", "pt-BR"},
			"default_language": "de", "handoff_team_id": teamID, "raise_priority_on_handoff": true,
			"max_ai_replies": 8, "tone": "concise", "schedule": "outside_office_hours",
			"link_allowlist": []string{"help.acme.com", "*.Acme.io"},
		}).AssertOK()

		res.AssertJSONPath("data.agent.assigned_behavior", "always")
		res.AssertJSONPath("data.agent.handoff_team_id", teamID)
		res.AssertJSONPath("data.agent.raise_priority_on_handoff", true)
		res.AssertJSONPath("data.agent.languages.1", "pt-BR")
		res.AssertJSONPath("data.agent.max_ai_replies", 8)
		res.AssertJSONPath("data.agent.link_allowlist", []string{"help.acme.com", "*.acme.io"})

		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"unknown_behavior": "say_unknown"}).AssertOK().
			AssertJSONPath("data.agent.unknown_behavior", "say_unknown")

		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"handoff_team_id": ""}).AssertOK().
			AssertJSONPath("data.agent.handoff_team_id", nil).
			AssertJSONPath("data.agent.raise_priority_on_handoff", true)
	})

	t.Run("saves the agent's role and the tags that always hand off", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		legal := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Legal"}).AssertCreated().String("data.id")
		refunds := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Refunds"}).AssertCreated().String("data.id")

		d.Owner.GetJSON("/api/v1/ai/agent").AssertOK().
			AssertJSONPath("data.agent.handoff_mode", "full").
			AssertJSONPath("data.agent.always_handoff_tag_ids", []string{})

		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{
			"handoff_mode": "triage_only", "always_handoff_tag_ids": []string{legal, refunds, legal},
		}).AssertOK().
			AssertJSONPath("data.agent.handoff_mode", "triage_only").
			AssertJSONPath("data.agent.always_handoff_tag_ids", []string{legal, refunds})

		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"name": "Ada", "raise_priority_on_handoff": true}).
			AssertOK()
		d.Owner.GetJSON("/api/v1/ai/agent").AssertOK().
			AssertJSONPath("data.agent.handoff_mode", "triage_only").
			AssertJSONPath("data.agent.always_handoff_tag_ids", []string{legal, refunds}).
			AssertJSONPath("data.agent.raise_priority_on_handoff", true)

		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{
			"handoff_mode": "full", "always_handoff_tag_ids": []string{refunds},
		}).AssertOK().
			AssertJSONPath("data.agent.handoff_mode", "full").
			AssertJSONPath("data.agent.always_handoff_tag_ids", []string{refunds})

		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"always_handoff_tag_ids": nil}).AssertOK().
			AssertJSONPath("data.agent.always_handoff_tag_ids", []string{})
	})

	t.Run("refuses a role or a tag the workspace does not have", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		legal := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Legal"}).AssertCreated().String("data.id")
		other := models.PublicID(models.PrefixTag, ids.New())
		team := models.PublicID(models.PrefixTeam, ids.New())

		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"handoff_mode": "answer_everything"}).
			AssertValidationError("handoff_mode")
		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"always_handoff_tag_ids": []string{legal, other}}).
			AssertValidationError("always_handoff_tag_ids")
		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"always_handoff_tag_ids": []string{team}}).
			AssertValidationError("always_handoff_tag_ids")
		tooMany := make([]string, 51)
		for i := range tooMany {
			tooMany[i] = legal
		}
		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"always_handoff_tag_ids": tooMany}).
			AssertValidationError("always_handoff_tag_ids")

		d.Owner.GetJSON("/api/v1/ai/agent").AssertOK().
			AssertJSONPath("data.agent.always_handoff_tag_ids", []string{})
	})

	t.Run("switches on only an agent that could answer", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		answering(d, "claude-test")

		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{"enabled": true}).AssertValidationError("enabled")

		key := createKey(t, d, "Anthropic")
		d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{
			"enabled": true, "answer_key_id": key, "answer_model": "not-offered",
		}).AssertValidationError("enabled")

		res := d.Owner.PatchJSON("/api/v1/ai/agent", testkit.JSON{
			"enabled": true, "answer_key_id": key, "answer_model": "claude-test",
			"enabled_channels": []string{"widget"},
		}).AssertOK()
		res.AssertJSONPath("data.agent.enabled", true)
		res.AssertJSONPath("data.ready", true)
	})

	t.Run("lets members read the agent and only admins change it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		member, _ := d.Join(t, enums.RoleMember)
		admin, _ := d.Join(t, enums.RoleAdmin)

		member.GetJSON("/api/v1/ai/agent").AssertOK()
		member.PatchJSON("/api/v1/ai/agent", testkit.JSON{"name": "Mallory"}).AssertForbidden()
		member.PatchJSON("/api/v1/ai/agent", testkit.JSON{"monthly_cap_usd": 0}).AssertForbidden()
		admin.PatchJSON("/api/v1/ai/agent", testkit.JSON{"name": "Ada"}).AssertOK()
	})
}
