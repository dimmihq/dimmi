/*
|--------------------------------------------------------------------------
| Workspace Initializers
|--------------------------------------------------------------------------
|
| The ordered list of setup steps every new workspace goes through. Parts
| of the product register their step while the application boots; the
| order of registration is the order of running, since some defaults
| build on others, such as a changelog on its help center.
|
*/

package workspaces

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Initializers holds the registered setup steps, in order.
*/
type Initializers struct {
	steps []contracts.WorkspaceInitializer
}

/*
NewInitializers returns an empty list of setup steps.
*/
func NewInitializers() *Initializers {
	return &Initializers{}
}

/*
Register adds steps after those already registered.
*/
func (i *Initializers) Register(steps ...contracts.WorkspaceInitializer) {
	i.steps = append(i.steps, steps...)
}

/*
Run runs every step on the workspace, in order, and stops at the first
that fails, naming it.
*/
func (i *Initializers) Run(ctx context.Context, tx *database.Tx, workspace models.Workspace) error {
	for _, step := range i.steps {
		if err := step.Initialize(ctx, tx, workspace); err != nil {
			return fmt.Errorf("initialize %s: %w", step.Name(), err)
		}
	}

	return nil
}
