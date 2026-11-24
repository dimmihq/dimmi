package procedure_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
refund is a procedure with chips and a branch: look the order up,
then decide in code whether it is inside the window; inside, refund
it, outside, offer store credit, which is only conditional.
*/
func refund() procedure.Definition {
	return procedure.Definition{
		Description: "The customer wants a refund.",
		Steps: []procedure.Step{
			{ID: "s1", Kind: procedure.KindInstruction, Text: "Run [[action:act_lookup]] with the order number."},
			{
				ID: "s2", Kind: procedure.KindBranch, Text: "Inside the window",
				Condition: &procedure.ConditionExpr{Mode: "all", Rules: []procedure.ConditionRule{
					{Field: "action.lookup_order.days_since_purchase", Operator: procedure.OpLte, Value: 30.0},
				}},
				ThenSteps: []procedure.Step{{
					ID: "s3", Kind: procedure.KindInstruction, Text: "Refund it with [[action:act_refund]].",
					Enforcement: &procedure.Enforcement{Requires: []string{"s1"}},
				}},
				ElseSteps: []procedure.Step{{
					ID: "s4", Kind: procedure.KindInstruction, Text: "Offer [[action:act_credit]].",
					Bindings: []procedure.Binding{
						{Kind: procedure.BindAction, Ref: "act_credit", Execution: "conditional"},
					},
				}},
			},
		},
		Handover: []string{},
	}
}

func tools() map[string]string {
	return map[string]string{"act_lookup": "lookup_order", "act_refund": "refund_order", "act_credit": "store_credit"}
}

func TestProcedureSteps(t *testing.T) {
	t.Parallel()

	t.Run("reads a legacy action step as a required action chip", func(t *testing.T) {
		t.Parallel()
		d := procedure.Normalize(signIn())
		require.Len(t, d.Steps[1].Bindings, 1)
		b := d.Steps[1].Bindings[0]
		assert.Equal(t, procedure.Binding{Kind: "action", Ref: "act_1", Execution: "required", OnMissing: "handoff"}, b)
		assert.Empty(t, d.Steps[0].Bindings)
	})

	t.Run("round-trips every new field and still refuses unknown ones", func(t *testing.T) {
		t.Parallel()
		raw, err := json.Marshal(refund())
		require.NoError(t, err)
		d, err := procedure.Parse(raw)
		require.NoError(t, err)
		assert.Equal(t, refund(), d)
		assert.Empty(t, procedure.Check(d))
		_, err = procedure.Parse(json.RawMessage(`{"steps":[{"id":"a","kind":"instruction","text":"x","wat":1}]}`))
		require.Error(t, err)
	})

	t.Run("makes actions under a condition in plain words conditional", func(t *testing.T) {
		t.Parallel()
		d := procedure.Normalize(procedure.Definition{Steps: []procedure.Step{{
			ID: "c", Kind: procedure.KindCondition, Text: "If they are upset, offer [[action:act_credit]]",
			ThenSteps: []procedure.Step{{ID: "t", Kind: procedure.KindInstruction, Text: "[[action:act_refund]]"}},
		}}})
		assert.Equal(t, "conditional", d.Steps[0].Bindings[0].Execution)
		assert.Equal(t, "conditional", d.Steps[0].ThenSteps[0].Bindings[0].Execution)
	})

	t.Run("lists every reference, nested ones included", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, []string{"act_lookup", "act_refund", "act_credit"}, procedure.ActionIDs(refund()))
		assert.Equal(t, "s3", procedure.StepWith(refund(), procedure.BindAction, "act_refund"))
	})

	t.Run("checks rules, nesting, chips and prerequisites", func(t *testing.T) {
		t.Parallel()
		d := refund()
		d.Steps[1].Condition.Rules = append(
			d.Steps[1].Condition.Rules,
			procedure.ConditionRule{Field: "x", Operator: "magic"},
		)
		d.Steps[0].Enforcement = &procedure.Enforcement{Requires: []string{"nope"}, MaxRetries: 9}
		d.Steps = append(d.Steps, procedure.Step{ID: "s9", Kind: procedure.KindBranch})
		d.Steps[1].ThenSteps[0].ThenSteps = []procedure.Step{{ID: "deep", Kind: "instruction", Text: "x"}}
		messages := map[string]bool{}
		for _, i := range procedure.Check(d) {
			messages[i.Message] = true
		}
		assert.True(t, messages["A rule compares in a way dimmi does not know."])
		assert.True(t, messages["Retry 1 to 3 times."])
		assert.True(t, messages["A step waits for an action this procedure does not take."])
		assert.True(t, messages["Write the rule this branch decides."])
		assert.True(t, messages["Only a condition can have then and else steps."])
	})
}

func TestProcedureRules(t *testing.T) {
	t.Parallel()

	facts := procedure.Facts{
		Lookup: func(field string) (any, bool) {
			v, ok := map[string]any{
				"contact.attr.plan": "Pro", "contact.email": "ada@example.com", "conversation.channel": "widget",
				"contact.tags": []string{"vip"}, "contact.created_at": "2026-01-02T00:00:00Z",
			}[field]
			return v, ok
		},
		Outputs: map[string]any{"lookup_order": map[string]any{
			"days_since_purchase": 12.0, "items": []any{map[string]any{"sku": "A-1"}}, "status": "shipped",
		}},
	}
	rule := func(mode string, rules ...procedure.ConditionRule) procedure.Verdict {
		return procedure.Evaluate(procedure.ConditionExpr{Mode: mode, Rules: rules}, facts)
	}
	r := func(field, op string, v any) procedure.ConditionRule {
		return procedure.ConditionRule{Field: field, Operator: op, Value: v}
	}

	cases := []struct {
		name string
		got  procedure.Verdict
		want bool
	}{
		{"eq ignores case", rule("all", r("contact.attr.plan", "eq", "pro")), true},
		{"neq", rule("all", r("contact.attr.plan", "neq", "free")), true},
		{"numbers from results", rule("all", r("action.lookup_order.days_since_purchase", "lte", 30)), true},
		{"numbers as text", rule("all", r("action.lookup_order.days_since_purchase", "gt", "20")), false},
		{"paths into lists", rule("all", r("action.lookup_order.items.0.sku", "eq", "A-1")), true},
		{"in", rule("all", r("conversation.channel", "in", []any{"email", "widget"})), true},
		{"not in a comma list", rule("all", r("conversation.channel", "not_in", "email, slack")), true},
		{"contains text", rule("all", r("contact.email", "contains", "@EXAMPLE")), true},
		{"contains in a list", rule("all", r("contact.tags", "contains", "vip")), true},
		{"exists", rule("all", r("contact.email", "exists", nil)), true},
		{"not exists", rule("all", r("contact.attr.missing", "not_exists", nil)), true},
		{"dates", rule("all", r("contact.created_at", "lt", "2026-06-01")), true},
		{"any", rule("any", r("contact.attr.plan", "eq", "free"), r("conversation.channel", "eq", "widget")), true},
		{
			"all fails on one",
			rule("all", r("contact.attr.plan", "eq", "pro"), r("conversation.channel", "eq", "email")),
			false,
		},
	}
	for _, c := range cases {
		assert.True(t, c.got.Known, c.name)
		assert.Equal(t, c.want, c.got.Value, c.name)
	}

	waits := rule("all", r("contact.attr.plan", "eq", "pro"), r("action.refund_order.id", "exists", nil))
	assert.False(t, waits.Known, "a rule on an action not called yet waits")
	assert.Equal(t, []string{"refund_order"}, waits.Needs)
	assert.True(t, rule("all", r("contact.attr.plan", "eq", "free"), r("action.refund_order.id", "exists", nil)).Known,
		"unless another rule already decides it")
	assert.True(t, rule("any", r("contact.attr.plan", "eq", "pro"), r("action.refund_order.id", "exists", nil)).Value)
}

func TestProcedurePath(t *testing.T) {
	t.Parallel()

	in := func(outputs map[string]any, called ...string) procedure.Input {
		c := map[string]bool{}
		for _, id := range called {
			c[id] = true
		}
		return procedure.Input{
			Facts: procedure.Facts{Outputs: outputs}, Called: c, Tools: tools(),
			Reads: map[string]bool{"act_lookup": true}, Marked: true,
		}
	}

	t.Run("moves on from a step that takes no action only once dimmi says it is finished", func(t *testing.T) {
		t.Parallel()
		talk := procedure.Definition{
			Description: "The customer wants to cancel.",
			Steps: []procedure.Step{
				{ID: "ask", Kind: procedure.KindInstruction, Text: "Ask why."},
				{ID: "explain", Kind: procedure.KindInstruction, Text: "Explain what canceling does."},
			},
			Handover: []string{},
		}
		i := in(nil)
		text := procedure.RenderPlan("Cancel", talk, procedure.Resolve(talk, i), i)
		assert.Contains(t, text, "1. Ask why. [ACTIVE]")
		assert.NotContains(t, text, "2. Explain what canceling does. [ACTIVE]")

		i.Done = map[string]bool{"ask": true}
		text = procedure.RenderPlan("Cancel", talk, procedure.Resolve(talk, i), i)
		assert.Contains(t, text, "1. Ask why. [DONE]")
		assert.Contains(t, text, "2. Explain what canceling does. [ACTIVE]")
	})

	t.Run("leaves the side of a plain-words condition that was not taken", func(t *testing.T) {
		t.Parallel()
		choice := procedure.Definition{
			Description: "The customer wants their money back.",
			Steps: []procedure.Step{
				{ID: "ask", Kind: procedure.KindInstruction, Text: "Ask where they want the money."},
				{
					ID: "card", Kind: procedure.KindCondition, Text: "They want it on their card",
					ThenSteps: []procedure.Step{{ID: "amount", Kind: procedure.KindInstruction, Text: "Ask how much."}},
					ElseSteps: []procedure.Step{{ID: "keep", Kind: procedure.KindInstruction, Text: "Say it stays."}},
				},
			},
			Handover: []string{},
		}
		i := in(nil)
		i.Done = map[string]bool{"ask": true}
		plan := procedure.Resolve(choice, i)
		keep, _ := plan.Find("keep")
		assert.NotEqual(t, procedure.StatusEvaluated, keep.Status, "both sides stay until one is begun")

		i.Done["amount"] = true
		plan = procedure.Resolve(choice, i)
		keep, _ = plan.Find("keep")
		assert.Equal(t, procedure.StatusEvaluated, keep.Status, "the other side is no longer to do")
		assert.Contains(t, procedure.RenderPlan("Refund", choice, plan, i), "Say it stays. [NOT TAKEN]")
		for _, n := range plan.Nodes {
			assert.NotEqual(t, procedure.StatusActive, n.Status, "nothing is left to do")
		}
	})

	t.Run("stops asking for an action once the customer turned its step down", func(t *testing.T) {
		t.Parallel()
		i := in(nil)
		require.Len(t, procedure.Resolve(refund(), i).Missing(i.Called, i.Tools), 1)

		i.Done = map[string]bool{"s1": true}
		plan := procedure.Resolve(refund(), i)
		assert.Empty(t, plan.Missing(i.Called, i.Tools), "the lookup is no longer required of the reply")
		assert.Equal(t, procedure.StatusDone, plan.Nodes[0].Status)
		assert.NotContains(t, procedure.RenderPlan("Refund", refund(), plan, i), "order number. [ACTIVE]")
	})

	t.Run("locks a branch until the action it reads has run", func(t *testing.T) {
		t.Parallel()
		i := in(nil)
		plan := procedure.Resolve(refund(), i)
		text := procedure.RenderPlan("Refund", refund(), plan, i)
		assert.Contains(t, text, "1. Run the lookup_order tool (required) with the order number. [ACTIVE]")
		assert.Contains(t, text, "[LOCKED: call lookup_order first to unlock the next steps]")
		assert.NotContains(t, text, "refund_order")
		assert.NotContains(t, text, "store_credit")
		missing := plan.Missing(i.Called, i.Tools)
		require.Len(t, missing, 1)
		assert.Equal(t, "act_lookup", missing[0].ActionID)
	})

	t.Run("keeps only the side code decided and enforces only its actions", func(t *testing.T) {
		t.Parallel()
		inside := in(map[string]any{"lookup_order": map[string]any{"days_since_purchase": 3.0}}, "act_lookup")
		plan := procedure.Resolve(refund(), inside)
		text := procedure.RenderPlan("Refund", refund(), plan, inside)
		assert.Contains(t, text, "is true, so follow these steps")
		assert.Contains(t, text, "2.1. Refund it with the refund_order tool (required). [ACTIVE]")
		assert.NotContains(t, text, "store_credit", "the else side is never shown")
		assert.Contains(t, text, "1. Run the lookup_order tool (required) with the order number. [DONE]")
		missing := plan.Missing(inside.Called, inside.Tools)
		require.Len(t, missing, 1)
		assert.Equal(t, "act_refund", missing[0].ActionID)

		outside := in(map[string]any{"lookup_order": map[string]any{"days_since_purchase": 90.0}}, "act_lookup")
		plan = procedure.Resolve(refund(), outside)
		text = procedure.RenderPlan("Refund", refund(), plan, outside)
		assert.Contains(t, text, "store_credit tool (only when it applies)")
		assert.NotContains(t, text, "refund_order")
		assert.Empty(t, plan.Missing(outside.Called, outside.Tools), "a conditional action never blocks")
	})

	t.Run("names the prerequisites an action waits for", func(t *testing.T) {
		t.Parallel()
		i := in(map[string]any{"lookup_order": map[string]any{"days_since_purchase": 3.0}})
		plan := procedure.Resolve(refund(), i)
		assert.Equal(t, []string{"act_lookup"}, plan.Prerequisites("act_refund", i.Called, i.Tools))
		assert.Empty(t, plan.Prerequisites("act_refund", map[string]bool{"act_lookup": true}, i.Tools))
		assert.Empty(t, plan.Prerequisites("act_other", i.Called, i.Tools), "an action never named waits for nothing")
		locked := procedure.Resolve(refund(), in(nil))
		assert.Equal(t, []string{"act_lookup"}, locked.Prerequisites("act_refund", nil, i.Tools),
			"an action under a branch that waits still waits for its prerequisites")
		delete(i.Tools, "act_lookup")
		assert.Empty(t, plan.Prerequisites("act_refund", i.Called, i.Tools), "nor for an action it cannot be offered")
	})

	t.Run("does not enforce an action it cannot offer, and says what to do", func(t *testing.T) {
		t.Parallel()
		i := in(nil)
		delete(i.Tools, "act_lookup")
		i.Unavailable = map[string]string{"act_lookup": "identity_unverified"}
		plan := procedure.Resolve(refund(), i)
		assert.Empty(t, plan.Missing(i.Called, i.Tools))
		require.Len(t, plan.Unavailable(i.Tools), 1)
		text := procedure.RenderPlan("Refund", refund(), plan, i)
		assert.Contains(t, text, "not available right now because identity unverified: hand over instead")
	})

	t.Run("expands sub-procedures in place under scoped ids and stops at a loop", func(t *testing.T) {
		t.Parallel()
		sub := procedure.Definition{Description: "Verify", Steps: []procedure.Step{
			{ID: "check_1", Kind: procedure.KindInstruction, Text: "Verify with [[action:act_lookup]]."},
			{ID: "check_2", Kind: procedure.KindSubProcedure, SubProcedure: "proc_parent"},
		}}
		parent := procedure.Definition{Description: "Parent", Steps: []procedure.Step{
			{ID: "p1", Kind: procedure.KindSubProcedure, SubProcedure: "proc_sub"},
			{ID: "p2", Kind: procedure.KindInstruction, Text: "Then [[action:act_refund]]."},
		}}
		i := in(nil)
		i.Subs = map[string]procedure.Sub{
			"proc_sub":    {Name: "Verify", Def: sub},
			"proc_parent": {Name: "Parent", Def: parent},
		}
		i.Root = "proc_parent"
		plan := procedure.Resolve(parent, i)
		node, ok := plan.Find("p1/check_1")
		require.True(t, ok)
		assert.Equal(t, "1.1", node.Label)
		assert.Equal(t, "proc_sub", node.Sub)
		assert.Equal(t, "act_lookup", plan.Actions[0].ActionID)
		assert.Len(t, plan.Nodes, 4, "the loop back to the parent is never expanded")
	})

	t.Run("shows both sides of a condition in plain words", func(t *testing.T) {
		t.Parallel()
		d := procedure.Definition{Description: "x", Steps: []procedure.Step{{
			ID: "c", Kind: procedure.KindCondition, Text: "Is the customer upset?",
			ThenSteps: []procedure.Step{
				{ID: "t", Kind: procedure.KindInstruction, Text: "Offer [[action:act_credit]]."},
			},
			ElseSteps: []procedure.Step{{ID: "e", Kind: procedure.KindInstruction, Text: "Go on."}},
		}}}
		i := in(nil)
		plan := procedure.Resolve(d, i)
		text := procedure.RenderPlan("x", d, plan, i)
		assert.Contains(t, text, "If so:\n   1a.1. Offer the store_credit tool (only when it applies).")
		assert.Contains(t, text, "If not:\n   1b.1. Go on.")
		assert.Empty(t, plan.Missing(i.Called, i.Tools))

		d.Steps[0].ThenSteps[0].Bindings = []procedure.Binding{
			{Kind: "action", Ref: "act_credit", Execution: "required"},
		}
		assert.Len(t, procedure.Resolve(d, i).Missing(i.Called, i.Tools), 1, "unless the team made it required")
	})
}

func TestProcedureCycles(t *testing.T) {
	t.Parallel()

	graph := map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a"}, "x": {"y"}, "y": {}}
	refs := func(id string) []string { return graph[id] }
	assert.Equal(t, []string{"a", "b", "c", "a"}, procedure.Cycle("a", refs))
	assert.Nil(t, procedure.Cycle("x", refs))

	deep := map[string][]string{"1": {"2"}, "2": {"3"}, "3": {"4"}, "4": {"5"}}
	assert.NotNil(t, procedure.Cycle("1", func(id string) []string { return deep[id] }), "too deep")
}

func TestProcedureAssertions(t *testing.T) {
	t.Parallel()

	o := procedure.Observed{
		Procedures: []string{"proc_1"},
		Calls: []procedure.ObservedCall{
			{ActionID: "act_refund", Tool: "refund_order", Blocked: true},
			{
				ActionID:  "act_lookup",
				Tool:      "lookup_order",
				Args:      map[string]any{"order_id": "ORD-102"},
				Succeeded: true,
			},
			{ActionID: "act_refund", Tool: "refund_order", Succeeded: true},
		},
		Branches:  map[string]string{"s2": "then"},
		HandedOff: true, Team: "team_1", Priority: "high",
		Replies: []string{"Your refund is on its way within 30 days."},
	}
	no := false
	as := []procedure.Assertion{
		{Type: "procedure_matched", Procedure: "proc_1"},
		{Type: "action_called", Action: "lookup_order", Args: map[string]any{"order_id": "ORD-102"}},
		{Type: "action_not_called", Action: "act_credit"},
		{Type: "action_order", Action: "lookup_order", Before: "refund_order"},
		{Type: "branch_taken", Step: "s2", Branch: "then"},
		{Type: "handed_off", Team: "team_1", Priority: "high"},
		{Type: "reply_contains", Text: "30 DAYS"},
		{Type: "reply_not_contains", Text: "sorry"},
		{Type: "llm_rubric", Rubric: "Polite"},
	}
	rs := procedure.Assert(as, o, "", nil)
	for _, r := range rs[:8] {
		assert.True(t, r.Passed, r.Type+": "+r.Message)
	}
	assert.False(t, rs[8].Passed, "a check nobody could judge does not pass")
	assert.False(t, rs[8].Skipped)
	require.ErrorIs(t, rs[8].Err, procedure.ErrNoJudge)
	assert.False(t, procedure.Passed(rs))

	met := func(string, string) (procedure.Judgment, error) {
		return procedure.Judgment{Passed: true, Message: "Met."}, nil
	}
	judged := procedure.Assert(as, o, "", met)
	assert.True(t, procedure.Passed(judged))

	unsure := procedure.Assert([]procedure.Assertion{{Type: procedure.AssertLLMRubric, Rubric: "polite"}}, o, "",
		func(string, string) (procedure.Judgment, error) {
			return procedure.Judgment{Unsure: true, Message: "The judge could not tell."}, nil
		})
	assert.False(t, unsure[0].Passed)
	assert.True(t, unsure[0].Unsure)
	assert.True(t, procedure.Passed(unsure), "a check the judge was unsure of does not fail the run")

	fails := procedure.Assert([]procedure.Assertion{
		{Type: "procedure_matched", Matched: &no},
		{Type: "action_called", Action: "lookup_order", Args: map[string]any{"order_id": "ORD-1"}},
		{Type: "branch_taken", Step: "s2", Branch: "else"},
		{Type: "handed_off", Team: "team_2"},
		{Type: "llm_rubric", Rubric: "x"},
	}, o, "", func(string, string) (procedure.Judgment, error) {
		return procedure.Judgment{Message: "rude"}, nil
	})
	for _, r := range fails {
		assert.False(t, r.Passed, r.Type)
	}
	assert.False(t, procedure.Passed(fails))
	assert.NotEmpty(t, procedure.CheckAssertions([]procedure.Assertion{{Type: "nope"}, {Type: "action_order"}}))
}
