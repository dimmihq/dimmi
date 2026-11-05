/*
|--------------------------------------------------------------------------
| Contact Exporters
|--------------------------------------------------------------------------
|
| The exporters the parts of the product register at boot, each adding a
| section to a customer's export, in the order they were registered.
|
*/

package contacts

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Exporters holds the registered exporters, in order.
*/
type Exporters struct {
	steps []contracts.ContactExporter
}

/*
NewExporters returns an empty list of exporters.
*/
func NewExporters() *Exporters {
	return &Exporters{}
}

/*
Register adds exporters after those already registered.
*/
func (e *Exporters) Register(steps ...contracts.ContactExporter) {
	e.steps = append(e.steps, steps...)
}

/*
Run returns every exporter's section by name, stopping at the first
that fails.
*/
func (e *Exporters) Run(
	ctx context.Context, workspaceID, contactID ids.UUID,
) (map[string]contracts.ExportSection, error) {
	out := map[string]contracts.ExportSection{}

	for _, step := range e.steps {
		section, err := step.Export(ctx, workspaceID, contactID)
		if err != nil {
			return nil, fmt.Errorf("export %s: %w", step.Name(), err)
		}

		out[step.Name()] = section
	}

	return out, nil
}
