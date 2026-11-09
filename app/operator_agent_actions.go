/*
|--------------------------------------------------------------------------
| Operator Agent Actions
|--------------------------------------------------------------------------
|
| Operator, the team's own agent: the engine that answers teammates and
| applies the changes they approve, and the actions the API serves for
| its settings, threads, changesets and memories.
|
*/

package app

import (
	"github.com/dimmihq/dimmi/app/actions/operatorchat"
	"github.com/dimmihq/dimmi/app/ai/operator"
)

/*
OperatorAgentActions groups Operator's engine and actions.
*/
type OperatorAgentActions struct {
	Engine     *operator.Engine
	Settings   *operatorchat.Settings
	Threads    *operatorchat.Threads
	Changesets *operatorchat.Changesets
	Memories   *operatorchat.Memories
}
