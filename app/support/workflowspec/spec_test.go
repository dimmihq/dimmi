package workflowspec_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
routeByTopic is a first-message workflow with buttons, a branch and
actions.
*/
const routeByTopic = `{
  "schema": 1,
  "trigger": "conversation.first_message",
  "channels": ["widget", "email"],
  "audience": {"visitors": "identified", "rules": {"field": "company.custom.plan", "op": "in", "value": ["pro"]}},
  "schedule": {"mode": "any"},
  "reentry": {"mode": "once_per_conversation"},
  "exclusive": false,
  "steps": [
    {"id": "s_ask123", "kind": "buttons", "settings": {"text": "Hi {{contact.first_name | fallback: \"there\"}}!"},
     "paths": [
       {"id": "p_billing", "label": "Billing", "steps": [
         {"id": "s_assign1", "kind": "assign", "settings": {"mode": "team", "team_id": "team_billing"}},
         {"id": "s_tag0001", "kind": "tag", "settings": {"tag_ids": ["tag_billing"]}}
       ]},
       {"id": "p_other1", "label": "Something else", "steps": [
         {"id": "s_branch1", "kind": "branch", "paths": [
           {"id": "p_vipvip", "label": "VIP",
            "condition": {"field": "conversation.priority", "op": "eq", "value": "urgent"},
            "steps": [{"id": "s_note001", "kind": "note",
              "settings": {"text": "VIP!", "mention_member_ids": ["mem_lea"]}}]},
           {"id": "p_else01", "outcome": "else", "steps": []}
         ]}
       ]},
       {"id": "p_typed1", "outcome": "typed", "steps": []}
     ]}
  ]
}`

/*
parse reads a definition or fails the test.
*/
func parse(t *testing.T, raw string) workflowspec.Spec {
	t.Helper()

	s, err := workflowspec.Parse(json.RawMessage(raw))
	require.NoError(t, err)

	return s
}

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("reads a definition", func(t *testing.T) {
		t.Parallel()

		s := parse(t, routeByTopic)
		assert.Equal(t, workflowspec.TriggerFirstMessage, s.Trigger)
		assert.Len(t, s.Steps[0].Paths, 3)
	})

	t.Run("refuses unknown keys", func(t *testing.T) {
		t.Parallel()

		_, err := workflowspec.Parse(json.RawMessage(`{"schema":1,"trigger":"reusable","stpes":[]}`))

		var invalid *workflowspec.InvalidError
		require.ErrorAs(t, err, &invalid)
		assert.Equal(t, workflowspec.CodeSchema, invalid.Errors[0].Code)
		assert.Contains(t, err.Error(), "workflow invalid: schema")
	})

	t.Run("refuses definitions over the size limit", func(t *testing.T) {
		t.Parallel()

		_, err := workflowspec.Parse(make(json.RawMessage, workflowspec.MaxBytes+1))
		require.Error(t, err)
	})

	t.Run("gives a definition without steps an empty list", func(t *testing.T) {
		t.Parallel()

		assert.NotNil(t, parse(t, `{"schema":1,"trigger":"reusable"}`).Steps)
	})
}

func TestCanonical(t *testing.T) {
	t.Parallel()

	t.Run("hashes the same whatever the key order", func(t *testing.T) {
		t.Parallel()

		a := parse(t, `{"schema":1,"trigger":"reusable","steps":[
			{"id":"s_aaaaaa","kind":"assign","settings":{"mode":"team","team_id":"team_x"}}]}`)
		b := parse(t, `{"steps":[{"settings":{"team_id":"team_x","mode":"team"},"kind":"assign","id":"s_aaaaaa"}],
			"trigger":"reusable","schema":1}`)

		ha, err := workflowspec.Hash(a)
		require.NoError(t, err)
		hb, err := workflowspec.Hash(b)
		require.NoError(t, err)
		assert.Equal(t, ha, hb)
		assert.Len(t, ha, 64)
	})

	t.Run("treats empty settings as absent", func(t *testing.T) {
		t.Parallel()

		a := parse(t, `{"schema":1,"trigger":"reusable","trigger_settings":{},"steps":[{"id":"s_cccccc",
			"kind":"close","settings":{}}]}`)
		b := parse(t, `{"schema":1,"trigger":"reusable","steps":[{"id":"s_cccccc","kind":"close"}]}`)

		ha, _ := workflowspec.Hash(a)
		hb, _ := workflowspec.Hash(b)
		assert.Equal(t, ha, hb)
	})

	t.Run("keeps large numbers exact", func(t *testing.T) {
		t.Parallel()

		s := parse(t, `{"schema":1,"trigger":"reusable","trigger_settings":{"n":12345678901234567890}}`)
		raw, err := workflowspec.Canonical(s)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "12345678901234567890")
	})

	t.Run("canonicalizes nested conditions", func(t *testing.T) {
		t.Parallel()

		raw, err := workflowspec.Canonical(parse(t, routeByTopic))
		require.NoError(t, err)
		assert.Contains(t, string(raw), `"condition":{"field":"conversation.priority","op":"eq","value":"urgent"}`)
	})

	t.Run("refuses broken raw parts", func(t *testing.T) {
		t.Parallel()

		bad := json.RawMessage(`{`)
		for _, s := range []workflowspec.Spec{
			{TriggerSettings: bad},
			{Audience: bad},
			{Steps: []workflowspec.Step{{ID: "s_x", Settings: bad}}},
			{Steps: []workflowspec.Step{{ID: "s_x", Paths: []workflowspec.Path{{ID: "p_x", Condition: bad}}}}},
			{Steps: []workflowspec.Step{{ID: "s_x", Paths: []workflowspec.Path{{
				ID:    "p_x",
				Steps: []workflowspec.Step{{ID: "s_y", Settings: bad}},
			}}}}},
		} {
			_, err := workflowspec.Hash(s)
			require.Error(t, err)
		}
	})
}

func TestCompile(t *testing.T) {
	t.Parallel()

	t.Run("collects references and customer-facing steps", func(t *testing.T) {
		t.Parallel()

		c := workflowspec.Compile(parse(t, routeByTopic))
		assert.Equal(t, workflowspec.KindCustomerFacing, c.Kind)
		assert.Equal(t, 5, c.Steps)
		assert.Equal(t, []string{"s_ask123"}, c.CustomerFacingSteps)
		assert.Equal(t, []string{"team_billing"}, c.References.Teams)
		assert.Equal(t, []string{"tag_billing"}, c.References.Tags)
		assert.Equal(t, []string{"mem_lea"}, c.References.Members)
	})

	t.Run("a team trigger with only actions is background", func(t *testing.T) {
		t.Parallel()

		s := parse(t, `{"schema":1,"trigger":"note.created","schedule":{"mode":"open","schedule_id":"sch_a"},
			"steps":[{"id":"s_wait01","kind":"wait","settings":{"until":{"kind":"office_hours_open",
			"schedule_id":"sch_b"}}},
			{"id":"s_call01","kind":"run_workflow","settings":{"workflow_id":"wf_x"}}]}`)
		c := workflowspec.Compile(s)
		assert.Equal(t, workflowspec.KindBackground, c.Kind)
		assert.ElementsMatch(t, []string{"sch_a", "sch_b"}, c.References.Schedules)
		assert.Equal(t, []string{"wf_x"}, c.References.Workflows)
	})

	t.Run("a close asking for a rating speaks to the customer", func(t *testing.T) {
		t.Parallel()

		s := parse(t, `{"schema":1,"trigger":"reusable","steps":[{"id":"s_close1","kind":"close",
			"settings":{"ask_rating":true}}]}`)
		assert.Equal(t, workflowspec.KindCustomerFacing, workflowspec.WorkflowKind(s))
	})

	t.Run("assigns missing ids", func(t *testing.T) {
		t.Parallel()

		s := parse(t, `{"schema":1,"trigger":"reusable","steps":[{"kind":"branch","paths":[{"label":"A"},
			{"outcome":"else"}]}]}`)
		workflowspec.AssignIDs(&s)

		assert.Regexp(t, `^s_[a-z0-9]{10}$`, s.Steps[0].ID)
		assert.Regexp(t, `^p_[a-z0-9]{10}$`, s.Steps[0].Paths[1].ID)
		assert.NotNil(t, s.Steps[0].Paths[0].Steps)
	})
}

func TestIndex(t *testing.T) {
	t.Parallel()

	ix := workflowspec.NewIndex(parse(t, routeByTopic))

	t.Run("finds the first step and moves along a path", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "s_ask123", ix.First())
		assert.Equal(t, "s_assign1", ix.Enter("s_ask123", "p_billing"))
		assert.Equal(t, "s_tag0001", ix.Next("s_assign1"))
		assert.Empty(t, ix.Next("s_tag0001"))
		assert.Empty(t, ix.Next("s_ask123"))
	})

	t.Run("an empty or unknown path leads nowhere", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, ix.Enter("s_ask123", "p_typed1"))
		assert.Empty(t, ix.Enter("s_ask123", "p_nope"))
		assert.Empty(t, ix.Enter("s_nope", "p_typed1"))
		assert.Empty(t, ix.Next("s_nope"))

		_, ok := ix.Step("s_note001")
		assert.True(t, ok)
	})

	t.Run("an empty definition has no first step", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, workflowspec.NewIndex(workflowspec.Spec{}).First())
	})
}

func TestDurations(t *testing.T) {
	t.Parallel()

	cases := map[string]time.Duration{"30s": 30 * time.Second, "10m": 10 * time.Minute, "3d": 72 * time.Hour}
	for in, want := range cases {
		got, err := workflowspec.ParseDuration(in)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}

	for _, bad := range []string{"", "-1d", "xd", "-5m", "soon"} {
		_, err := workflowspec.ParseDuration(bad)
		require.ErrorIs(t, err, workflowspec.ErrDuration, bad)
	}

	t.Run("knows the variables it can fill", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{"contact.first_name", "run.vars.order"},
			workflowspec.Variables(`Hi {{ contact.first_name | fallback: "x" }} {{run.vars.order}}`))
		assert.True(t, workflowspec.KnownVariable("contact.custom.plan"))
		assert.False(t, workflowspec.KnownVariable("contact.custom."))
		assert.False(t, workflowspec.KnownVariable("contact.frist_name"))
	})
}
