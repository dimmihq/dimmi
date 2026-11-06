/*
|--------------------------------------------------------------------------
| Managed Driver
|--------------------------------------------------------------------------
|
| The built-in provider fronts every family at once: one connection,
| no key of the workspace's own, and a model id alone decides whether
| a call lands on Anthropic, OpenAI or the open-weight host. The
| workspace's provider sees exactly one provider name — "dimmi" —
| while the drivers underneath stay ordinary.
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
	fallback contracts.LLM
	models   []contracts.LLMModel
}

/*
New builds the multiplexer. Families maps each family to the upstream
driver built from the instance's managed credentials; models is the
curated list the connection offers. Requests for a family with no
driver land on the first available one.
*/
func New(families map[enums.AIModelFamily]contracts.LLM, models []contracts.LLMModel) *Driver {
	var fallback contracts.LLM
	for _, d := range families {
		fallback = d
		break
	}
	return &Driver{families: families, fallback: fallback, models: models}
}

/*
Provider names the managed provider.
*/
func (d *Driver) Provider() contracts.LLMProvider { return contracts.LLMProviderDimmi }

/*
Chat routes the call to the model's family driver.
*/
func (d *Driver) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	return d.forModel(req.Model).Chat(ctx, req)
}

/*
Stream routes the call to the model's family driver.
*/
func (d *Driver) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	return d.forModel(req.Model).Stream(ctx, req)
}

/*
ListModels returns the curated catalog.
*/
func (d *Driver) ListModels(_ context.Context) ([]contracts.LLMModel, error) {
	return d.models, nil
}

/*
Capabilities delegates to the model's family driver.
*/
func (d *Driver) Capabilities(model string) contracts.LLMCapabilities {
	return d.forModel(model).Capabilities(model)
}

/*
forModel picks the driver for the model's family.
*/
func (d *Driver) forModel(model string) contracts.LLM {
	family := aimodels.FamilyOf(enums.AIProviderDimmi, model)
	if drv, ok := d.families[family]; ok {
		return drv
	}
	return d.fallback
}
