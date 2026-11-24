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
| routine is offered every skill, whatever it was created from.
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
skill is one kind of work Operator knows how to do: its name and when
to read it. Its steps are the file of that name under
resources/prompts/tasks/operator/skills/.
*/
type skill struct {
	name, when string
}

/*
skills lists every skill, in the order the prompt offers them.
*/
func skills() []skill {
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
			name: "research_conversations",
			when: "the routine asks what customers are saying, asking or complaining about, or anything " +
				"else that is answered by reading conversations",
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
	lines := make([]string, 0, len(skills()))
	for _, s := range skills() {
		lines = append(lines, "- "+s.name+": when "+s.when+".")
	}
	return prompts.Task(taskSkills, prompts.Vars{"list": strings.Join(lines, "\n")})
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
func (e *Engine) readSkill(_ context.Context, _ *session, args json.RawMessage) (result, error) {
	var in struct {
		Name string `json:"skill_name"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return result{}, errBadArguments
	}
	if slices.Contains(skillNames(), in.Name) {
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
		Description: "Read how to do one kind of routine work well, by the skill's name, before starting it.",
		Schema: schema(`{"type":"object","required":["skill_name"],"properties":{` +
			`"skill_name":{"type":"string","maxLength":100,"description":"The skill's name, such as ` +
			`audit_articles."}}}`),
	}, handle: e.readSkill, routine: true, was: map[string]string{"name": "skill_name"}}
}
