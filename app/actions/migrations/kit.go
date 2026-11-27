/*
|--------------------------------------------------------------------------
| The Writers A Migration Uses
|--------------------------------------------------------------------------
|
| A migration writes through the same actions as everyone else, each
| built with a dispatcher no listener hears, so whatever an action would
| announce goes nowhere: no workflow, notification, SLA, webhook or AI
| agent reacts to history. Conversations replay through their writer's
| imported mode, which dispatches nothing to begin with.
|
*/

package migrations

import (
	"github.com/dimmihq/dimmi/framework/events"

	"github.com/dimmihq/dimmi/app/actions/attachments"
	"github.com/dimmihq/dimmi/app/actions/attributes"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/actions/savedreplies"
	"github.com/dimmihq/dimmi/app/actions/schedules"
	"github.com/dimmihq/dimmi/app/actions/teams"
	"github.com/dimmihq/dimmi/app/actions/tickets"
)

/*
Kit is the actions a migration writes through. Events is the silent
dispatcher every one of them was built with.
*/
type Kit struct {
	Events           *events.Dispatcher
	Conversations    *conversations.CreateConversation
	Tickets          *tickets.Tickets
	CreateTeam       *teams.CreateTeam
	AddTeamMembers   *teams.AddTeamMembers
	Attributes       *attributes.Definitions
	HelpCenters      *helpcenters.HelpCenters
	Collections      *helpcenters.Collections
	Articles         *helpcenters.Articles
	Images           *helpcenters.Images
	CreateSavedReply *savedreplies.CreateSavedReply
	UpdateSavedReply *savedreplies.UpdateSavedReply
	Schedules        *schedules.Schedules
	Upload           *attachments.UploadFile
	Disks            messages.Disks
}

/*
Silent returns a dispatcher with no listeners, for the actions of a
migration.
*/
func Silent() *events.Dispatcher {
	return events.New(nil)
}

/*
kit returns the actions the step writes through.
*/
func (s *step) kit() *Kit {
	return &s.runner.svc.Kit
}
