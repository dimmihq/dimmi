/*
|--------------------------------------------------------------------------
| AI Doctor
|--------------------------------------------------------------------------
|
| The install's answer health: whether a managed upstream can carry
| the built-in provider, whether every family's prompts render, the
| price table covers the catalog, and the subscription connect has
| its endpoints.
|
*/

package ai

import (
	"context"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/support/aiprices"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/app/support/prompts"
	"github.com/dimmihq/dimmi/config"
)

/*
AICheckStatus says how an AI check went.
*/
type AICheckStatus string

/*
The outcomes of an AI check.
*/
const (
	AICheckPass AICheckStatus = "pass"
	AICheckWarn AICheckStatus = "warn"
	AICheckFail AICheckStatus = "fail"
)

/*
AICheck is one finding of the doctor.
*/
type AICheck struct {
	ID      string
	Status  AICheckStatus
	Message string
}

/*
AiDoctor checks the install's agent plumbing.
*/
type AiDoctor struct {
	Config  config.AIConfig
	Prompts *prompts.Loader
	Prices  *aiprices.Table
}

/*
Handle runs every check.
*/
func (d *AiDoctor) Handle(ctx context.Context) []AICheck {
	return []AICheck{
		d.managedCheck(),
		d.promptsCheck(),
		d.pricesCheck(),
		d.chatGPTCheck(),
	}
}

/*
managedCheck passes when at least one upstream backs the built-in
provider.
*/
func (d *AiDoctor) managedCheck() AICheck {
	m := d.Config.Managed
	if m.Anthropic.APIKey != "" || m.OpenAI.APIKey != "" || m.OpenWeight.APIKey != "" {
		return AICheck{ID: "ai.managed", Status: AICheckPass, Message: "the built-in provider has an upstream"}
	}
	return AICheck{ID: "ai.managed", Status: AICheckWarn,
		Message: "no managed upstream is configured; the built-in provider answers nothing until one is, " +
			"but connections a workspace adds itself still work"}
}

/*
promptsCheck renders one template per model family.
*/
func (d *AiDoctor) promptsCheck() AICheck {
	for _, family := range []enums.AIModelFamily{
		enums.AIModelFamilyAnthropic, enums.AIModelFamilyOpenAI, enums.AIModelFamilyOpenWeight,
	} {
		if _, err := d.Prompts.RenderFile(family, "answer.system.md", prompts.Vars{}); err != nil {
			return AICheck{ID: "ai.prompts", Status: AICheckFail,
				Message: "the " + string(family) + " prompt templates do not render: " + err.Error()}
		}
	}
	return AICheck{ID: "ai.prompts", Status: AICheckPass, Message: "every family's prompt templates render"}
}

/*
pricesCheck passes when every cataloged model has a price.
*/
func (d *AiDoctor) pricesCheck() AICheck {
	if d.Prices == nil {
		return AICheck{ID: "ai.prices", Status: AICheckWarn, Message: "the price table is not loaded"}
	}
	for _, e := range append(append(aimodels.Managed(), aimodels.Anthropic()...), aimodels.OpenAI()...) {
		if d.Prices.Find(providerForFamily(e.Family), e.ID) == nil {
			return AICheck{ID: "ai.prices", Status: AICheckWarn,
				Message: "model " + e.ID + " has no price; its answers record no cost"}
		}
	}
	return AICheck{ID: "ai.prices", Status: AICheckPass, Message: "every cataloged model has a price"}
}

/*
chatGPTCheck passes when the subscription connect's endpoints are all
present, or when it is off.
*/
func (d *AiDoctor) chatGPTCheck() AICheck {
	if !d.Config.ChatGPT.Enabled {
		return AICheck{ID: "ai.chatgpt", Status: AICheckPass, Message: "the subscription connect is off"}
	}
	c := d.Config.ChatGPT
	if c.ClientID == "" || c.AuthorizeURL == "" || c.TokenURL == "" || c.APIBaseURL == "" {
		return AICheck{ID: "ai.chatgpt", Status: AICheckFail,
			Message: "the subscription connect is enabled but an endpoint or the client id is missing"}
	}
	return AICheck{ID: "ai.chatgpt", Status: AICheckPass, Message: "the subscription connect is configured"}
}

/*
providerForFamily maps a family to the provider key its prices live
under.
*/
func providerForFamily(family enums.AIModelFamily) string {
	switch family {
	case enums.AIModelFamilyAnthropic:
		return "anthropic"
	case enums.AIModelFamilyOpenAI:
		return "openai"
	case enums.AIModelFamilyOpenWeight:
		return "engy"
	}
	return ""
}
