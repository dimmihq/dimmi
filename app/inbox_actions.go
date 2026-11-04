/*
|--------------------------------------------------------------------------
| Inbox Actions
|--------------------------------------------------------------------------
|
| The business actions of the shared inbox: its inboxes, contacts,
| conversations and their timelines. They are built once at boot like
| every other action, and kept together so the inbox's surface reads in
| one place.
|
*/

package app

import (
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/inboxes"
	"github.com/dimmihq/dimmi/app/actions/messages"
)

/*
InboxActions holds the shared inbox's actions.
*/
type InboxActions struct {
	ListInboxes   *inboxes.ListInboxes
	ShowInbox     *inboxes.ShowInbox
	CreateInbox   *inboxes.CreateInbox
	UpdateInbox   *inboxes.UpdateInbox
	ListContacts  *contacts.ListContacts
	ShowContact   *contacts.ShowContact
	CreateContact *contacts.CreateContact
	UpdateContact *contacts.UpdateContact

	ListConversations  *conversations.ListConversations
	ShowConversation   *conversations.ShowConversation
	CreateConversation *conversations.CreateConversation
	ApplyChanges       *conversations.ApplyConversationChanges
	ShowNeighbors      *conversations.ShowNeighbors
	CountConversations *conversations.CountConversations
	ShowReadState      *conversations.ShowReadState
	UpdateReadState    *conversations.UpdateReadState
	Search             *conversations.Search

	ListMessages      *messages.ListMessages
	ShowOriginal      *messages.ShowOriginal
	DownloadOriginal  *messages.DownloadOriginal
	Reply             *messages.ReplyToConversation
	RetryDelivery     *messages.RetryDelivery
	AddNote           *messages.AddNote
	AddContactMessage *messages.AddContactMessage
}
