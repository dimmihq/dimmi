package operator

import (
	"regexp"
	"slices"
	"strings"
)

/*
RoutineTool is one of Operator's tools as a routine's prompt names it:
its name, the tool group that gives it to a routine, and what it does.
Group is "routine" for the tools every routine has.
*/
type RoutineTool struct {
	Name        string
	Group       string
	Description string
}

/*
RoutineGroupAlways is the group of the tools every routine has.
*/
const RoutineGroupAlways = "routine"

/*
RoutineTools lists the tools a routine's prompt may name, by group then
name. The workspace's own actions are left out: they come and go with
the workspace, and the actions group gives them all.
*/
func RoutineTools() []RoutineTool {
	e := &Engine{}
	all := slices.Concat(e.conversationTools(), e.knowledgeTools(), e.insightTools(), e.procedureTools(),
		e.writeTools(), e.routineTools(), e.githubTools())
	out := make([]RoutineTool, 0, len(all))
	for _, t := range all {
		if t.spec.Name == "remember" || t.spec.Name == "draft_reply" {
			continue
		}
		out = append(out, RoutineTool{Name: t.spec.Name, Group: routineToolGroup(t), Description: t.spec.Description})
	}
	slices.SortStableFunc(out, func(a, b RoutineTool) int {
		return strings.Compare(a.Group+" "+a.Name, b.Group+" "+b.Name)
	})
	return out
}

/*
routineToolGroup is the group that gives a routine the tool.
*/
func routineToolGroup(t tool) string {
	switch {
	case t.group == RoutineGroupGitHub:
		return RoutineGroupGitHub
	case t.routine || t.spec.Name == "list_recent_changesets":
		return RoutineGroupAlways
	}
	return routineGroupOf(t)
}

/*
toolChip is a tool named in a routine's prompt, as the builder writes it.
*/
var toolChip = regexp.MustCompile(`\[\[tool:([a-z0-9_]+)\]\]`)

/*
spellTools writes each tool named in a prompt as its plain name, the
way the model calls it.
*/
func spellTools(prompt string) string {
	return toolChip.ReplaceAllString(prompt, "$1")
}
