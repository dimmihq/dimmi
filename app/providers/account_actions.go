/*
|--------------------------------------------------------------------------
| Account Actions
|--------------------------------------------------------------------------
|
| The actions on a person's own account: their profile, password, picture
| and address, and deleting it all. They are kept together, and the worker
| that takes old pictures out of storage is registered with them.
|
*/

package providers

import (
	"fmt"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/actions/users"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootAccount builds the actions on a person's own account, and those an
admin uses to help a teammate back into theirs.
*/
func bootAccount(a *app.App) error {
	bootSignInHelp(a)

	disk, err := a.Storage.Default()
	if err != nil {
		return fmt.Errorf("avatar disk: %w", err)
	}

	acts, window := a.Actions, a.Config.Auth.PasswordConfirmation.TTL

	acts.UpdateProfile = &users.UpdateProfile{DB: a.DB, Clock: a.Clock, Events: a.Events}
	acts.UpdatePassword = &users.UpdatePassword{
		DB: a.DB, Clock: a.Clock, Events: a.Events, Hash: a.Hash, Mail: a.Mail, Links: a.Links,
		LoginPolicy: a.LoginPolicy, Lockout: a.Lockout, Sessions: a.Session, AppGrants: a.AppGrants,
		SecurityLog: a.SecurityLog, ConfirmWindow: window,
	}
	acts.UpdateAvatar = &users.UpdateAvatar{DB: a.DB, Clock: a.Clock, Events: a.Events, Disk: disk, Queue: a.Queue}
	acts.DeleteAvatar = &users.DeleteAvatar{DB: a.DB, Clock: a.Clock, Events: a.Events, Queue: a.Queue}
	acts.RequestEmailChange = &users.RequestEmailChange{
		DB: a.DB, Mail: a.Mail, Limiter: a.Limiter, Links: a.Links, Tokens: a.Secrets, SecurityLog: a.SecurityLog,
	}
	acts.ConfirmEmailChange = &users.ConfirmEmailChange{
		DB: a.DB, Clock: a.Clock, Events: a.Events, Mail: a.Mail, Limiter: a.Limiter, IDs: a.IDs, Links: a.Links,
		Tokens: a.Secrets, SecurityLog: a.SecurityLog,
	}
	acts.PreviewAccountDeletion = &members.PreviewAccountDeletion{DB: a.DB}
	acts.DeleteAccount = &users.DeleteAccount{
		DB: a.DB, Clock: a.Clock, Events: a.Events, Mail: a.Mail, Queue: a.Queue, Sessions: a.Session,
		AppGrants: a.AppGrants, ConfirmWindow: window,
		Workspaces: &members.LeaveEveryWorkspace{Events: a.Events, Clock: a.Clock, Trash: acts.TrashWorkspace},
	}

	return queue.Register(a.Queue, &jobs.DeleteUserAvatar{Disk: disk})
}
