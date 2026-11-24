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

import "slices"

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
	TemplateVoice          = "tpl_voice_of_customer"
	TemplateRequests       = "tpl_feature_request_roundup"
	TemplateUnhappy        = "tpl_unhappy_customer_watch"
	TemplateWaiting        = "tpl_waiting_conversations"
	TemplateHandovers      = "tpl_ai_handover_review"
	TemplateSimulations    = "tpl_simulation_review"
	TemplateCustom         = "tpl_custom"
)

/*
RoutineTemplates lists every template, the custom one last.
*/
func RoutineTemplates() []RoutineTemplate {
	return append(slices.Concat(knowledgeTemplates(), conversationTemplates(), agentTemplates()), RoutineTemplate{
		Key: TemplateCustom, Name: "Start from scratch",
		Description: "Say what Operator should do, in your own words, and pick when.",
		Cadence:     "weekly", Expr: "0 9 * * 1", NotifyPolicy: "on_findings",
	})
}

/*
knowledgeTemplates are the routines that keep the help center and the
procedures right.
*/
func knowledgeTemplates() []RoutineTemplate {
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
	}
}

/*
conversationTemplates are the routines that read what customers wrote:
what they ask for, who is unhappy and who is waiting.
*/
func conversationTemplates() []RoutineTemplate {
	return []RoutineTemplate{
		{
			Key: TemplateVoice, Name: "Voice of the customer",
			Description: "Every Monday, says what customers asked about and complained about most last " +
				"week, with the conversations behind each.",
			Prompt: "Tell the team what customers asked about most and complained about most since the " +
				"last run, with how many conversations were about each and a few to read.",
			Cadence: "weekly", Expr: "0 10 * * 1", NotifyPolicy: "always",
			Requires: []string{RequiresConversations},
		},
		{
			Key: TemplateRequests, Name: "Feature request roundup",
			Description: "Every Thursday, lists the features customers asked for, how many asked for each " +
				"and who.",
			Prompt: "List the features customers asked for since the last run, how many customers asked " +
				"for each, and the conversations where they asked.",
			Cadence: "weekly", Expr: "0 9 * * 4", NotifyPolicy: "on_findings",
			Requires: []string{RequiresConversations},
		},
		{
			Key: TemplateUnhappy, Name: "Unhappy customer watch",
			Description: "Every morning, finds open conversations where the customer is frustrated or " +
				"talks of leaving, and says which need a person today.",
			Prompt: "Find the open conversations where the customer is frustrated, has asked more than " +
				"once, or talks of leaving, and tell the team which need a person today and why.",
			Cadence: "daily", Expr: "0 9 * * *", NotifyPolicy: "on_findings",
			Requires: []string{RequiresConversations},
		},
		{
			Key: TemplateWaiting, Name: "Waiting too long",
			Description: "Every morning, finds open conversations the team has not answered for over a " +
				"day, and says what each is waiting on.",
			Prompt: "Find the open conversations where the customer has waited more than a day for the " +
				"team to reply, and say what each one is waiting on.",
			Cadence: "daily", Expr: "0 8 * * *", NotifyPolicy: "on_findings",
			Requires: []string{RequiresConversations},
		},
	}
}

/*
agentTemplates are the routines that look at how the AI agents are
doing: what they passed on that they could have answered, and why their
simulations fail.
*/
func agentTemplates() []RoutineTemplate {
	return []RoutineTemplate{
		{
			Key: TemplateHandovers, Name: "AI handover review",
			Description: "Every Tuesday, reads what the AI agent passed to the team and says which it could " +
				"have answered, and what it was missing.",
			Prompt: "Read the conversations the AI agent handed to the team since the last run, say which " +
				"it could have answered itself, and draft the help articles it was missing.",
			Cadence: "weekly", Expr: "0 9 * * 2", NotifyPolicy: "on_findings",
			Requires: []string{RequiresConversations, RequiresHelpCenter},
		},
		{
			Key: TemplateSimulations, Name: "Simulation check-up",
			Description: "Every Wednesday, reads the simulations that fail or pass only sometimes and says " +
				"why, with the lines of guidance that cause it.",
			Prompt: "Find the agent simulations that fail or pass only sometimes, and say why each one " +
				"does, quoting the lines of guidance or the checks that cause it.",
			Cadence: "weekly", Expr: "0 10 * * 3", NotifyPolicy: "on_findings",
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
