/*
|--------------------------------------------------------------------------
| Migration Wiring
|--------------------------------------------------------------------------
|
| Migrations from other tools write through the actions everyone uses,
| so they are built after tickets and the help center. The actions that
| announce what they do get copies built with a silent dispatcher, so
| nothing a migration writes sets anything off. Undoing one runs in a
| job.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/migrations"
	"github.com/dimmihq/dimmi/app/actions/teams"
)

/*
bootMigrations builds the migrations API's actions and registers its
undo.
*/
func bootMigrations(a *app.App) error {
	acts := a.Actions
	silent := migrations.Silent()
	delivery := conversations.Delivery{Channels: a.Channels, Queue: a.Queue}

	kit := migrations.Kit{
		Events:        silent,
		Conversations: &conversations.CreateConversation{DB: a.DB, Events: silent, Clock: a.Clock, Delivery: delivery},
		Tickets:       acts.Tickets, CreateTeam: &teams.CreateTeam{DB: a.DB, Events: silent, Clock: a.Clock},
		AddTeamMembers: &teams.AddTeamMembers{DB: a.DB, Events: silent, Clock: a.Clock},
		Attributes:     acts.Attributes, HelpCenters: acts.HelpCenters, Collections: acts.Collections,
		Articles: acts.Articles, Images: acts.HelpImages, Schedules: acts.Schedules, Disks: a.Storage,
	}

	if acts.Inbox != nil {
		kit.Upload = acts.Inbox.UploadFile
		kit.CreateSavedReply, kit.UpdateSavedReply = acts.Inbox.CreateSavedReply, acts.Inbox.UpdateSavedReply
	}

	m := &migrations.Migrations{DB: a.DB, Clock: a.Clock, Queue: a.Queue, IDs: a.IDs, Kit: kit}
	acts.Migrations = m

	return queue.Register(a.Queue, &migrations.RollbackWorker{Migrations: m})
}
