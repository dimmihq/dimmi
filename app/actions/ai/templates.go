/*
|--------------------------------------------------------------------------
| Agent Templates
|--------------------------------------------------------------------------
|
| What a new agent can start from: an agent for a job real teams run,
| with its tone and guidance already written, or nothing at all. A
| template only fills the first draft; the agent is the team's own from
| then on. A template whose root system prompt leaves every business
| decision to the team starts with guidance that makes each of them,
| kept in its own file under resources/prompts/guidance.
|
*/

package ai

import (
	"slices"

	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
TemplateGroup sorts templates in the picker.
*/
type TemplateGroup string

/*
The template groups, in the order the picker lists them.
*/
const (
	TemplateGroupSupport  TemplateGroup = "support"
	TemplateGroupGrowth   TemplateGroup = "growth"
	TemplateGroupInternal TemplateGroup = "internal"
	TemplateGroupBlank    TemplateGroup = "blank"
)

/*
Template is a starting point for a new agent: its key, how the picker
shows it, the agent name it suggests, its role and the tone and
guidance its first draft gets. Widget says the agent usually lives on
a page of its own, such as a pricing page.
*/
type Template struct {
	Key         string
	Group       TemplateGroup
	Name        string
	Description string
	AgentName   string
	Role        string
	Tone        string
	Guidance    string
	Handles     string
	Widget      bool
}

/*
Definition is the first draft of an agent made from the template, on
top of a blank agent.
*/
func (t Template) Definition(blank agents.Definition) agents.Definition {
	def := blank
	if t.Tone != "" {
		def.Tone = t.Tone
	}
	def.Guidance = t.StartingGuidance()
	def.Description = t.Handles
	return def
}

/*
StartingGuidance is the guidance a new agent made from the template
starts with: the template's own guidance file when it has one, written
in full because its root system prompt holds none of the business's
decisions, and the short guidance written here otherwise.
*/
func (t Template) StartingGuidance() string {
	if written, ok := prompts.TemplateGuidance(t.Key); ok {
		return written
	}
	return t.Guidance
}

func (t Template) keyOrNil() *string {
	if t.Key == "" {
		return nil
	}
	key := t.Key
	return &key
}

/*
TemplateByKey returns the template with the key; an empty key is the
blank template.
*/
func TemplateByKey(key string) (Template, bool) {
	for _, t := range Templates() {
		if t.Key == key {
			return t, true
		}
	}
	return Template{}, false
}

/*
Templates lists every template, group by group.
*/
func Templates() []Template {
	return slices.Concat(supportTemplates(), growthTemplates(), otherTemplates())
}

/*
supportTemplates are the templates for answering customers.
*/
func supportTemplates() []Template {
	return []Template{
		{
			Key: "support", Group: TemplateGroupSupport, Name: "Customer support",
			Description: "A second support agent, for another brand or product.",
			AgentName:   "Support", Role: "support", Tone: "friendly",
			Handles: "Existing customers with questions about the product or a problem with their account.",
			Guidance: "## How you help\n\nAnswer from the help center and say which article the answer comes from. " +
				"When a question is vague, ask one short question before answering.\n\n" +
				"## When you don't know\n\nSay so plainly and offer to hand the conversation to the team.\n",
		},
		{
			Key: "ecommerce", Group: TemplateGroupSupport, Name: "Orders and returns",
			Description: "Tracks orders and runs returns inside your return window.",
			AgentName:   "Orders", Role: "ecommerce", Tone: "friendly",
			Handles: "Customers who have bought: where an order is, tracking, returns and exchanges.",
			Guidance: "## Orders first\n\nAsk for the order number before anything else, then look the order up.\n\n" +
				"## Returns\n\nOnly offer a return inside the return window. A parcel still in transit is never " +
				"refunded; tell the customer when it should arrive.\n\n" +
				"## Tone\n\nBe warm when a parcel is late or damaged. Never blame the carrier.\n",
		},
		{
			Key: "billing", Group: TemplateGroupSupport, Name: "Billing and retention",
			Description: "Invoices, refunds and cancellations, strictly by policy.",
			AgentName:   "Billing", Role: "billing", Tone: "professional",
			Handles: "Existing customers with questions about invoices, charges, refunds, plan changes or canceling.",
			Guidance: "## Before touching billing\n\nOnly discuss invoices, charges or plans with a signed-in " +
				"customer.\n\n## Credits\n\nNever promise a credit or refund the policy does not allow. " +
				"Duplicate charges go to the team.\n\n## Before someone leaves\n\nAsk why first. Offer a pause or a " +
				"smaller plan once, then respect the answer.\n",
		},
		{
			Key: "developer", Group: TemplateGroupSupport, Name: "Developer docs",
			Description: "Code-first answers from your docs and API reference.",
			AgentName:   "Developer docs", Role: "developer", Tone: "concise", Widget: true,
			Handles: "Developers building on the product: the API, SDKs, webhooks and integration errors.",
			Guidance: "## Answers\n\nLead with a runnable snippet from the docs, then explain it in a line " +
				"or two.\n\n## Accuracy\n\nNever invent an endpoint, a parameter or an SDK flag. " +
				"If the docs do not say, say so.\n",
		},
	}
}

/*
growthTemplates are the templates for selling and onboarding.
*/
func growthTemplates() []Template {
	return []Template{
		{
			Key: "sales", Group: TemplateGroupGrowth, Name: "Sales and lead qualification",
			Description: "Answers pricing questions, qualifies leads and books demos.",
			AgentName:   "Sales", Role: "sales", Tone: "professional", Widget: true,
			Handles: "People who have not bought yet: prices, which plan fits, trials and demos.",
		},
		{
			Key: "onboarding", Group: TemplateGroupGrowth, Name: "Onboarding coach",
			Description: "Walks new customers through setup, one step at a time.",
			AgentName:   "Onboarding", Role: "onboarding", Tone: "friendly",
			Handles: "New customers setting up for the first time: first steps, importing data and connecting tools.",
			Guidance: "## Coaching\n\nGive one step at a time in short, plain sentences, and suggest the " +
				"next step when one is done.\n\n## When they are stuck\n\nSingle sign-on, migrations and " +
				"anything that needs an admin go to the team.\n",
		},
	}
}

/*
otherTemplates are the internal helpdesk and the blank start.
*/
func otherTemplates() []Template {
	return []Template{
		{
			Key: "internal", Group: TemplateGroupInternal, Name: "IT and people helpdesk",
			Description: "Answers teammates in Slack from internal docs only.",
			AgentName:   "IT & People", Role: "internal", Tone: "concise",
			Handles: "Teammates with IT or people questions, answered from internal docs only.",
			Guidance: "## Who you talk to\n\nYour own colleagues. Be direct.\n\n## Sources\n\nAnswer only " +
				"from internal handbooks and say which one.\n\n## Requests\n\nHardware, access and " +
				"approvals go to the team with what was asked and by whom.\n",
		},
		{
			Key: "", Group: TemplateGroupBlank, Name: "Start from scratch",
			Description: "An empty draft with nothing in it.", Role: "custom",
		},
	}
}
