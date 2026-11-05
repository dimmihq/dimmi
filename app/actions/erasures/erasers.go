/*
|--------------------------------------------------------------------------
| Contact Erasers
|--------------------------------------------------------------------------
|
| The erasers the parts of the product register at boot, run in the order
| they were registered. Every one runs on every attempt; the erasure
| completes only once all of them succeed in one attempt.
|
*/

package erasures

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Erasers holds the registered erasers, in order.
*/
type Erasers struct {
	steps []contracts.ContactEraser
}

/*
NewErasers returns an empty list of erasers.
*/
func NewErasers() *Erasers {
	return &Erasers{}
}

/*
Register adds erasers after those already registered.
*/
func (e *Erasers) Register(steps ...contracts.ContactEraser) {
	e.steps = append(e.steps, steps...)
}

/*
Run runs every eraser on the contact and stops at the first that fails,
naming it.
*/
func (e *Erasers) Run(ctx context.Context, workspaceID, contactID ids.UUID) error {
	for _, step := range e.steps {
		if err := step.Erase(ctx, workspaceID, contactID); err != nil {
			return fmt.Errorf("erase %s: %w", step.Name(), err)
		}
	}

	return nil
}
