/*
|--------------------------------------------------------------------------
| Workspace Initializer
|--------------------------------------------------------------------------
|
| A new workspace is ready to use the moment it exists: each part of the
| product that needs a default, such as a first inbox, adds it inside the
| transaction that creates the workspace, so a failure leaves no
| workspace at all. The same step runs again when an operator repairs a
| workspace, so every initializer creates only what is missing.
|
*/

package contracts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
WorkspaceInitializer sets up the defaults one part of the product needs
in a workspace.
*/
type WorkspaceInitializer interface {
	/*
		Name says which defaults the initializer creates, such as
		"default_inbox", for errors and repair reports.
	*/
	Name() string

	/*
		Initialize creates the defaults the workspace lacks, inside the
		transaction, which is scoped to the workspace. Running it on a
		workspace that has them changes nothing.
	*/
	Initialize(ctx context.Context, tx *database.Tx, workspace models.Workspace) error
}
