/*
|--------------------------------------------------------------------------
| Skills
|--------------------------------------------------------------------------
|
| A skill is how Operator does one kind of recurring work well: which
| tools to call in which order, what to skip, and how to name what it
| suggests so it never suggests it twice. Skills ship with dimmi and are
| never shown to anyone; a routine's instructions stay a plain sentence,
| and Operator reads the skill that fits them before it starts. Every
| routine is offered every skill, whatever it was created from. A chat
| with a teammate is offered the skills that help one, such as building
| a workflow or a series, and reads them the same way.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
skill is one kind of work Operator knows how to do: its name, when to
read it, and whether a chat with a teammate is offered it too. Its
steps are the file of that name under
resources/prompts/tasks/operator/skills/.
*/
type skill struct {
	name, when string
	chat       bool
}

/*
skills lists every skill, in the order the prompt offers them.
*/
func skills() []skill {
	return append(supportSkills(), automationSkills()...)
}

/*
supportSkills are the skills about knowledge, conversations and the
AI agents.
*/
func supportSkills() []skill {
	return []skill{
		{
			name: "find_knowledge_gaps",
			when: "the routine asks for questions the AI agent or the team could not answer, missing " +
				"articles, or what the help center should cover",
		},
		{
			name: "audit_articles",
			when: "the routine asks to check help articles for contradictions, mistakes, or facts that went " +
				"stale",
		},
		{
			name: "sync_docs_with_pull_requests",
			when: "the routine asks to keep help articles in line with merged pull requests, releases or " +
				"product changes",
		},
		{
			name: "review_procedures",
			when: "the routine asks how procedures are doing, which steps fail, or to tune or test " +
				"procedures",
		},
		{
			name: "write_support_digest",
			when: "the routine asks for a digest, summary or report of support over a period",
		},
		{
			name: "watch_waiting_customers",
			when: "the routine asks who is waiting, who is unhappy or at risk of leaving, or which " +
				"conversations need a person now",
		},
		{
			name: "collect_feature_requests",
			when: "the routine asks what customers asked for, wished for or want the product to do",
		},
		{
			name: "review_ai_handovers",
			when: "the routine asks what the AI agent handed to the team, which of it the agent could " +
				"have answered, or why it handed over",
		},
		{
			name: "review_agent_simulations",
			when: "the routine asks why an AI agent's simulations fail or are flaky, or to check an agent's " +
				"guidance for instructions that contradict each other",
		},
		{
			name: "research_conversations",
			when: "the routine asks what customers are saying, asking or complaining about, or anything " +
				"else that is answered by reading conversations",
		},
	}
}

/*
automationSkills are the skills about workflows and series, offered in
a chat too.
*/
func automationSkills() []skill {
	return []skill{
		{
			name: "design_workflow",
			when: "a teammate wants a workflow built or changed, asks how to route, triage, follow up on or " +
				"protect conversations automatically, or describes a support problem a workflow could solve",
			chat: true,
		},
		{
			name: "design_series",
			when: "a teammate wants a series built or changed, or asks how to onboard, convert, retain, " +
				"renew or win back customers with messages over days or weeks",
			chat: true,
		},
		{
			name: "improve_automation",
			when: "the routine or a teammate asks how a workflow or series is doing, why it underperforms, " +
				"or what to change in one that is live",
			chat: true,
		},
	}
}

/*
skillNames lists the skills by name.
*/
func skillNames() []string {
	out := make([]string, 0, len(skills()))
	for _, s := range skills() {
		out = append(out, s.name)
	}
	return out
}

/*
skillList is the part of a routine's system prompt that offers the
skills.
*/
func skillList() string {
	return prompts.Task(taskSkills, prompts.Vars{"list": skillLines(func(skill) bool { return true })})
}

/*
chatSkillList is the part of a chat's system prompt that offers the
skills that help a teammate.
*/
func chatSkillList() string {
	return prompts.Task(taskChatSkills, prompts.Vars{"list": skillLines(func(s skill) bool { return s.chat })})
}

/*
skillLines writes one "- name: when ..." line per skill it keeps.
*/
func skillLines(keep func(skill) bool) string {
	lines := make([]string, 0, len(skills()))
	for _, s := range skills() {
		if !keep(s) {
			continue
		}
		lines = append(lines, "- "+s.name+": when "+s.when+".")
	}
	return strings.Join(lines, "\n")
}

/*
isChatSkill reports whether a chat is offered the skill.
*/
func isChatSkill(name string) bool {
	return slices.ContainsFunc(skills(), func(s skill) bool { return s.name == name && s.chat })
}

/*
skillBody is one skill's steps.
*/
func skillBody(name string) string {
	return prompts.Task("operator/skills/"+name, nil)
}

/*
readSkill hands the model one skill's steps.
*/
func (e *Engine) readSkill(_ context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Name string `json:"skill_name"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return result{}, errBadArguments
	}
	if slices.Contains(skillNames(), in.Name) {
		s.building = s.building || isChatSkill(in.Name)
		label := strings.ReplaceAll(in.Name, "_", " ")
		step := Step{Tool: "get_skill_by_name", Summary: "Read how to " + label}

		return result{text: skillBody(in.Name), step: step}, nil
	}
	return result{}, fmt.Errorf("%w: there is no skill %q; the skills are %s", errBadArguments, in.Name,
		strings.Join(skillNames(), ", "))
}

/*
skillTool is the tool that reads a skill.
*/
func (e *Engine) skillTool() tool {
	return tool{spec: contracts.ToolSpec{
		Name:        "get_skill_by_name",
		Description: "Read how to do one kind of work well, by the skill's name, before starting it.",
		Schema: schema(`{"type":"object","required":["skill_name"],"properties":{` +
			`"skill_name":{"type":"string","maxLength":100,"description":"The skill's name, such as ` +
			`audit_articles."}}}`),
	}, handle: e.readSkill, was: map[string]string{"name": "skill_name"}}
}
