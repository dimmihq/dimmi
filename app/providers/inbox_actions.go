/*
|--------------------------------------------------------------------------
| Inbox Actions
|--------------------------------------------------------------------------
|
| The shared inbox's actions and its channel drivers. The core ships the
| api channel; other parts of the product register their own drivers on
| the same manager. Only the testing environment may add the fake
| driver, which records what it would have sent.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/inboxes"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/channels"
	"github.com/dimmihq/dimmi/app/drivers/channel/api"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootInbox registers the core channel drivers and builds the inbox's
actions.
*/
func bootInbox(a *app.App) error {
	a.Channels = channels.NewManager(a.Application.Config.App.Env == foundation.EnvTesting)
	if err := a.Channels.Register(api.Driver{}); err != nil {
		return err
	}

	delivery := conversations.Delivery{Channels: a.Channels, Queue: a.Queue}
	a.Actions.Inbox = &app.InboxActions{
		ListInboxes:   &inboxes.ListInboxes{DB: a.DB},
		ShowInbox:     &inboxes.ShowInbox{DB: a.DB},
		CreateInbox:   &inboxes.CreateInbox{DB: a.DB, Events: a.Events, Clock: a.Clock},
		UpdateInbox:   &inboxes.UpdateInbox{DB: a.DB, Events: a.Events, Clock: a.Clock},
		ListContacts:  &contacts.ListContacts{DB: a.DB},
		ShowContact:   &contacts.ShowContact{DB: a.DB},
		CreateContact: &contacts.CreateContact{DB: a.DB, Events: a.Events, Clock: a.Clock},
		UpdateContact: &contacts.UpdateContact{DB: a.DB, Events: a.Events, Clock: a.Clock},

		ListConversations: &conversations.ListConversations{DB: a.DB},
		ShowConversation:  &conversations.ShowConversation{DB: a.DB},
		CreateConversation: &conversations.CreateConversation{
			DB: a.DB, Events: a.Events, Clock: a.Clock, Delivery: delivery,
		},
		ApplyChanges:       &conversations.ApplyConversationChanges{DB: a.DB, Events: a.Events, Clock: a.Clock},
		ShowNeighbors:      &conversations.ShowNeighbors{DB: a.DB},
		CountConversations: &conversations.CountConversations{DB: a.DB},
		ShowReadState:      &conversations.ShowReadState{DB: a.DB},
		UpdateReadState:    &conversations.UpdateReadState{DB: a.DB, Events: a.Events, Clock: a.Clock},
		Search:             &conversations.Search{DB: a.DB, IDs: a.IDs},

		ListMessages: &messages.ListMessages{DB: a.DB},
		Reply: &messages.ReplyToConversation{
			DB: a.DB, Events: a.Events, Clock: a.Clock, Delivery: delivery,
		},
		AddNote:           &messages.AddNote{DB: a.DB, Events: a.Events, Clock: a.Clock},
		AddContactMessage: &messages.AddContactMessage{DB: a.DB, Events: a.Events, Clock: a.Clock},
	}

	return queue.Register(a.Queue, &jobs.SendChannelMessage{DB: a.DB, Clock: a.Clock, Channels: a.Channels})
}
