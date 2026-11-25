package operator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/actions/workflowsims"
	engine "github.com/dimmihq/dimmi/app/workflows"
)

func TestSimulationTexts(t *testing.T) {
	t.Parallel()

	t.Run("reads a series walk in order, with what the person did before each step", func(t *testing.T) {
		t.Parallel()

		text := seriesTryText(&series.SimTrace{
			Entered: true,
			Steps: []series.SimStep{
				{NodeID: "entry", Kind: "entry", Result: "entered"},
				{NodeID: "welcome", Kind: "email", Result: "sent"},
				{NodeID: "check", Kind: "condition", Path: "else", Result: "else"},
			},
			Messages:  []series.SimMessage{{NodeID: "welcome", Kind: "email", Subject: "Welcome", Body: "Hi Ada"}},
			Decisions: []series.SimDecision{{Before: 2, What: `open the message "Welcome"`, Did: false}},
			End:       "finished",
		})

		assert.Contains(t, text, `sends "Welcome": Hi Ada`)
		assert.Less(t, strings.Index(text, "did not: open"), strings.Index(text, "check (condition)"))
		assert.Contains(t, text, "path else")
		assert.Contains(t, text, "Ended: finished. Goal reached: false.")
		assert.Contains(t, seriesTryText(&series.SimTrace{Reason: "entry_rules"}), "would not enter: entry_rules")
	})

	t.Run("reads a workflow try as the chat, the steps and what it would do", func(t *testing.T) {
		t.Parallel()

		text := workflowTryText(workflowsims.Result{
			Transcript: []workflowsims.Line{
				{From: "customer", Text: "Where is my parcel?"}, {From: "customer", Silent: true},
			},
			Trace: &engine.DryTrace{
				Verdict: engine.DryVerdict{Started: true},
				Steps: []engine.DryStep{
					{StepID: "s_sortsort", Kind: "classify", Outcome: "matched", PathLabel: "Shipping"},
				},
				Actions: []engine.DryAction{{Summary: "Assign to Shipping"}},
				End:     "completed",
			},
		})

		assert.Contains(t, text, "- customer: Where is my parcel?")
		assert.Contains(t, text, "(said nothing)")
		assert.Contains(t, text, "s_sortsort (classify): matched, path Shipping")
		assert.Contains(t, text, "- Assign to Shipping")
		assert.Contains(t, workflowTryText(workflowsims.Result{Trace: &engine.DryTrace{
			Verdict: engine.DryVerdict{Reason: "audience"},
		}}), "would not start: audience")
	})
}

func TestChatSkills(t *testing.T) {
	t.Parallel()

	chat := chatSkillList()
	for _, name := range []string{"design_workflow", "design_series", "improve_automation"} {
		assert.Contains(t, chat, "- "+name+": when ", "a chat is offered %s", name)
		assert.Contains(t, skillList(), "- "+name+": when ", "a routine is offered %s", name)
	}
	assert.NotContains(t, chat, "audit_articles", "routine-only skills stay out of a chat")
	assert.Contains(t, fixedTools(), "get_skill_by_name", "a chat always has the skill tool")
	assert.False(t, (&Engine{}).skillTool().routine, "the skill tool is offered beyond routines")
}

func TestBuildingBudget(t *testing.T) {
	t.Parallel()

	s := &session{}
	models, tools := s.limits()
	assert.Equal(t, []int{maxModelCalls, maxToolCalls}, []int{models, tools})

	_, err := (&Engine{}).readSkill(t.Context(), s, []byte(`{"skill_name":"audit_articles"}`))
	assert.NoError(t, err)
	assert.False(t, s.building, "a routine skill keeps a chat's bounds")

	_, err = (&Engine{}).readSkill(t.Context(), s, []byte(`{"skill_name":"design_workflow"}`))
	assert.NoError(t, err)
	models, tools = s.limits()
	assert.Equal(t, []int{buildingModelCalls, buildingToolCalls}, []int{models, tools})
}
