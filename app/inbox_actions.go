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
	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/inboxes"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/actions/tags"
)

/*
InboxActions holds the shared inbox's actions.
*/
type InboxActions struct {
	ListInboxes    *inboxes.ListInboxes
	ShowInbox      *inboxes.ShowInbox
	CreateInbox    *inboxes.CreateInbox
	UpdateInbox    *inboxes.UpdateInbox
	ListContacts   *contacts.ListContacts
	ShowContact    *contacts.ShowContact
	CreateContact  *contacts.CreateContact
	UpdateContact  *contacts.UpdateContact
	UpsertContact  *contacts.UpsertIdentifiedContact
	DeleteContact  *contacts.DeleteContact
	ListIdentities *contacts.ListIdentities
	DetachIdentity *contacts.DetachIdentity
	FindDuplicates *contacts.FindDuplicateContacts

	ListCompanies       *companies.ListCompanies
	ShowCompany         *companies.ShowCompany
	CreateCompany       *companies.CreateCompany
	IdentifyCompany     *companies.IdentifyCompany
	UpdateCompany       *companies.UpdateCompany
	DeleteCompany       *companies.DeleteCompany
	ListCompanyContacts *companies.ListCompanyContacts

	ListConversations       *conversations.ListConversations
	ShowConversation        *conversations.ShowConversation
	CreateConversation      *conversations.CreateConversation
	ApplyChanges            *conversations.ApplyConversationChanges
	ShowNeighbors           *conversations.ShowNeighbors
	CountConversations      *conversations.CountConversations
	ShowReadState           *conversations.ShowReadState
	UpdateReadState         *conversations.UpdateReadState
	Search                  *conversations.Search
	ListRecordConversations *conversations.ListRecordConversations
	ShowContext             *conversations.ShowContext

	ListTags          *tags.ListTags
	CreateTag         *tags.CreateTag
	UpdateTag         *tags.UpdateTag
	DeleteTag         *tags.DeleteTag
	TagConversation   *conversations.TagConversation
	UntagConversation *conversations.UntagConversation

	ListMessages      *messages.ListMessages
	Reply             *messages.ReplyToConversation
	AddNote           *messages.AddNote
	AddContactMessage *messages.AddContactMessage
}
