/*
|--------------------------------------------------------------------------
| Actions
|--------------------------------------------------------------------------
|
| Every business action, built once at boot with exactly the services it
| uses. Controllers, jobs and commands call these, never build their own,
| so an action behaves the same whoever runs it.
|
*/

package app

import (
	"github.com/dimmihq/dimmi/app/actions/auth"
	"github.com/dimmihq/dimmi/app/actions/users"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
)

/*
Actions holds the business actions.
*/
type Actions struct {
	CreateUser       *users.CreateUser
	CreateWorkspace  *workspaces.CreateWorkspace
	StartSession     *auth.StartSession
	LoadMe           *auth.LoadMe
	AttemptLogin     *auth.AttemptLogin
	RegisterUser     *auth.RegisterUser
	ListSessions     *auth.ListSessions
	EndSession       *auth.EndSession
	EndOtherSessions *auth.EndOtherSessions
}
