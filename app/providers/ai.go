/*
|--------------------------------------------------------------------------
| AI Agent
|--------------------------------------------------------------------------
|
| The agent's wiring: today it binds the knowledge contract to its
| stand-in so every consumer already has something to ask; the
| provider manager and the pipeline join here as they land.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/knowledge"
)

/*
bootAI wires the agent's services.
*/
func bootAI(a *app.App) {
	if a.Knowledge == nil {
		a.Knowledge = knowledge.NewStub()
	}
}
