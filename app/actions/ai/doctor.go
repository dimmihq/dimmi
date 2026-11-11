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
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/app/support/aiprices"
	"github.com/dimmihq/dimmi/app/support/prompts"
	"github.com/dimmihq/dimmi/config"
)

/*
CheckStatus says how an AI check went.
*/
type CheckStatus string

/*
The outcomes of an AI check.
*/
const (
	CheckPass CheckStatus = "pass"
	CheckWarn CheckStatus = "warn"
	CheckFail CheckStatus = "fail"
)

/*
Check is one finding of the doctor.
*/
type Check struct {
	ID      string
	Status  CheckStatus
	Message string
}

/*
Doctor checks the install's agent plumbing.
*/
type Doctor struct {
	Config  config.AIConfig
	Prompts *prompts.Loader
	Prices  *aiprices.Table
}

/*
Handle runs every check.
*/
func (d *Doctor) Handle(_ context.Context) []Check {
	return []Check{
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
func (d *Doctor) managedCheck() Check {
	m := d.Config.Managed
	if m.Anthropic.APIKey != "" || m.OpenAI.APIKey != "" || m.OpenWeight.Configured() {
		return Check{ID: "ai.managed", Status: CheckPass, Message: "the built-in provider has an upstream"}
	}
	return Check{
		ID: "ai.managed", Status: CheckWarn,
		Message: "no managed upstream is configured; the built-in provider answers nothing until one is, " +
			"but connections a workspace adds itself still work",
	}
}

/*
promptsCheck renders one template per model family.
*/
func (d *Doctor) promptsCheck() Check {
	for _, family := range []enums.AIModelFamily{
		enums.AIModelFamilyAnthropic, enums.AIModelFamilyOpenAI, enums.AIModelFamilyOpenWeight,
	} {
		if _, err := d.Prompts.RenderFile(family, "answer.system.md", prompts.Vars{}); err != nil {
			return Check{
				ID: "ai.prompts", Status: CheckFail,
				Message: "the " + string(family) + " prompt templates do not render: " + err.Error(),
			}
		}
	}
	return Check{ID: "ai.prompts", Status: CheckPass, Message: "every family's prompt templates render"}
}

/*
pricesCheck passes when every cataloged model has a price.
*/
func (d *Doctor) pricesCheck() Check {
	if d.Prices == nil {
		return Check{ID: "ai.prices", Status: CheckWarn, Message: "the price table is not loaded"}
	}
	for _, e := range append(append(aimodels.Managed(), aimodels.Anthropic()...), aimodels.OpenAI()...) {
		if d.Prices.Find(providerForFamily(e.Family), e.ID) == nil {
			return Check{
				ID: "ai.prices", Status: CheckWarn,
				Message: "model " + e.ID + " has no price; its answers record no cost",
			}
		}
	}
	return Check{ID: "ai.prices", Status: CheckPass, Message: "every cataloged model has a price"}
}

/*
chatGPTCheck passes when the subscription connect's endpoints are all
present, or when it is off.
*/
func (d *Doctor) chatGPTCheck() Check {
	if !d.Config.ChatGPT.Enabled {
		return Check{ID: "ai.chatgpt", Status: CheckPass, Message: "the subscription connect is off"}
	}
	c := d.Config.ChatGPT
	if c.ClientID == "" || c.AuthorizeURL == "" || c.TokenURL == "" || c.APIBaseURL == "" {
		return Check{
			ID: "ai.chatgpt", Status: CheckFail,
			Message: "the subscription connect is enabled but an endpoint or the client id is missing",
		}
	}
	return Check{ID: "ai.chatgpt", Status: CheckPass, Message: "the subscription connect is configured"}
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
