/*
|--------------------------------------------------------------------------
| Command List
|--------------------------------------------------------------------------
|
| The product's own console commands, handed to the framework with the
| application they work on: repairs an operator runs on a workspace, and
| help for users who cannot help themselves through the app. Each one
| calls the same action the app would, so it follows the same rules and
| leaves the same records.
|
*/

package commands

import (
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
)

/*
All returns the product's console commands.
*/
func All(a *app.App) []foundation.Command {
	return append([]foundation.Command{
		WorkspaceRepairCommand{App: a},
		&UserDisableTwoFactorCommand{App: a},
		&UserCreateCommand{App: a},
		&UserListCommand{App: a},
		&UserResetPasswordCommand{App: a},
		&UserVerifyEmailCommand{App: a},
		&UserLogoutCommand{App: a},
		WorkspaceClearAccessPolicyCommand{App: a},
		SSOVerifyDomainCommand{App: a},
		SSODisableCommand{App: a},
		EmailInboundDoctorCommand{App: a},
		EmailCloudflareWorkerCommand{App: a},
		EmailResendBackfillCommand{App: a},
		EmailDoctorCommand{App: a},
	}, append(operatorCommands(a), devCommands(a)...)...)
}
