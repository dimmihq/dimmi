/*
|--------------------------------------------------------------------------
| Routine Templates
|--------------------------------------------------------------------------
|
| A template is a routine filled in ahead and nothing more: a name, a
| plain sentence of instructions and the cadence that suits it. Picking
| one only fills the builder. A routine written from scratch with the
| same words runs exactly the same, because how the work is done lives
| in Operator's skills, which every routine is offered.
|
*/

package operator

/*
RoutineTemplate is one template. Requires names what the workspace
needs for the routine to have anything to do.
*/
type RoutineTemplate struct {
	Key          string
	Name         string
	Description  string
	Prompt       string
	Cadence      string
	Expr         string
	NotifyPolicy string
	Requires     []string
}

/*
The connections a template may need.
*/
const (
	RequiresGitHub        = "github"
	RequiresHelpCenter    = "help_center"
	RequiresConversations = "conversations"
	RequiresProcedures    = "procedures"
)

/*
The template keys.
*/
const (
	TemplateKnowledgeGaps  = "tpl_knowledge_gap_finder"
	TemplateGitHubDocsSync = "tpl_github_pr_docs_sync"
	TemplateStaleness      = "tpl_knowledge_staleness_audit"
	TemplateQADigest       = "tpl_qa_procedure_digest"
	TemplateCustom         = "tpl_custom"
)

/*
RoutineTemplates lists every template, the custom one last.
*/
func RoutineTemplates() []RoutineTemplate {
	return []RoutineTemplate{
		{
			Key: TemplateKnowledgeGaps, Name: "Knowledge gap finder",
			Description: "Every Monday, finds questions the AI agent could not answer and drafts the " +
				"articles that would have answered them.",
			Prompt: "Find the questions the AI agent could not answer since the last run, and draft the " +
				"help articles that would have answered them.",
			Cadence: "weekly", Expr: "0 9 * * 1", NotifyPolicy: "on_findings",
			Requires: []string{RequiresConversations, RequiresHelpCenter},
		},
		{
			Key: TemplateGitHubDocsSync, Name: "Pull requests to docs",
			Description: "Every morning, reads the pull requests merged the day before and suggests edits to " +
				"the help articles they made out of date.",
			Prompt: "Read the pull requests merged since the last run, and suggest edits to any help article " +
				"they made out of date.",
			Cadence: "daily", Expr: "30 8 * * *", NotifyPolicy: "on_findings",
			Requires: []string{RequiresGitHub, RequiresHelpCenter},
		},
		{
			Key: TemplateStaleness, Name: "Help center audit",
			Description: "Every Wednesday, reads a batch of help articles and suggests fixes where they " +
				"contradict each other or have gone stale.",
			Prompt: "Check the help center for articles that contradict each other or have gone stale, " +
				"and suggest fixes.",
			Cadence: "weekly", Expr: "0 9 * * 3", NotifyPolicy: "on_findings",
			Requires: []string{RequiresHelpCenter},
		},
		{
			Key: TemplateQADigest, Name: "Weekly digest and procedure tune-up",
			Description: "Every Friday, writes a short digest of the week's support and suggests fixes to " +
				"procedures that kept going wrong.",
			Prompt: "Write a short digest of this week's support, then suggest fixes for any procedure " +
				"that kept going wrong.",
			Cadence: "weekly", Expr: "0 15 * * 5", NotifyPolicy: "always",
			Requires: []string{RequiresConversations, RequiresProcedures},
		},
		{
			Key: TemplateCustom, Name: "Start from scratch",
			Description: "Say what Operator should do, in your own words, and pick when.",
			Cadence:     "weekly", Expr: "0 9 * * 1", NotifyPolicy: "on_findings",
		},
	}
}

/*
RoutineTemplateByKey returns one template.
*/
func RoutineTemplateByKey(key string) (RoutineTemplate, bool) {
	for _, t := range RoutineTemplates() {
		if t.Key == key {
			return t, true
		}
	}
	return RoutineTemplate{}, false
}
