/*
|--------------------------------------------------------------------------
| Managed Driver
|--------------------------------------------------------------------------
|
| The built-in provider fronts every family at once: one connection,
| no key of the workspace's own, and a model id alone decides whether
| a call lands on Anthropic, OpenAI or the open-weight host. The
| workspace's provider sees exactly one provider name — "dimmi" —
| while the drivers underneath stay ordinary. A model whose family the
| install has no upstream for is refused, never sent to another
| family's host under a name it does not know.
|
*/

package managed

import (
	"context"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/support/aimodels"
)

/*
Driver multiplexes calls to one upstream driver per family.
*/
type Driver struct {
	families map[enums.AIModelFamily]contracts.LLM
	models   []contracts.LLMModel
}

/*
New builds the multiplexer. Families maps each family to the upstream
driver built from the instance's managed credentials; models is the
curated list the connection offers.
*/
func New(families map[enums.AIModelFamily]contracts.LLM, models []contracts.LLMModel) *Driver {
	return &Driver{families: families, models: models}
}

/*
Provider names the managed provider.
*/
func (d *Driver) Provider() contracts.LLMProvider { return contracts.LLMProviderDimmi }

/*
Chat routes the call to the model's family driver.
*/
func (d *Driver) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	drv, err := d.forModel(req.Model)
	if err != nil {
		return nil, err
	}
	return drv.Chat(ctx, req)
}

/*
Stream routes the call to the model's family driver.
*/
func (d *Driver) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	drv, err := d.forModel(req.Model)
	if err != nil {
		return nil, err
	}
	return drv.Stream(ctx, req)
}

/*
ListModels returns the curated catalog.
*/
func (d *Driver) ListModels(_ context.Context) ([]contracts.LLMModel, error) {
	return d.models, nil
}

/*
Capabilities delegates to the model's family driver, and answers
nothing for a model no upstream serves.
*/
func (d *Driver) Capabilities(model string) contracts.LLMCapabilities {
	drv, err := d.forModel(model)
	if err != nil {
		return contracts.LLMCapabilities{}
	}
	return drv.Capabilities(model)
}

/*
forModel picks the driver for the model's family.
*/
func (d *Driver) forModel(model string) (contracts.LLM, error) {
	family := aimodels.FamilyOf(enums.AIProviderDimmi, model)
	if drv, ok := d.families[family]; ok {
		return drv, nil
	}
	return nil, &contracts.LLMError{
		Kind: contracts.LLMErrorModelNotFound, ProviderCode: "family_not_configured",
		Message: "the built-in provider has no upstream for " + model,
	}
}
