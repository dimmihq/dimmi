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
	conversationviews "github.com/dimmihq/dimmi/app/actions/conversation_views"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/drafts"
	"github.com/dimmihq/dimmi/app/actions/erasures"
	"github.com/dimmihq/dimmi/app/actions/inboxes"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/actions/redaction"
	"github.com/dimmihq/dimmi/app/actions/retention"
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
	DeleteInbox    *inboxes.DeleteInbox
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
	ShowPreferences         *conversations.ShowPreferences
	UpdatePreferences       *conversations.UpdatePreferences

	ListTags          *tags.ListTags
	CreateTag         *tags.CreateTag
	UpdateTag         *tags.UpdateTag
	DeleteTag         *tags.DeleteTag
	TagConversation   *conversations.TagConversation
	UntagConversation *conversations.UntagConversation

	ListViews    *conversationviews.ListViews
	ShowView     *conversationviews.ShowView
	CreateView   *conversationviews.CreateView
	UpdateView   *conversationviews.UpdateView
	DeleteView   *conversationviews.DeleteView
	ReorderViews *conversationviews.ReorderViews

	ListSnoozeLists   *conversations.ListSnoozeLists
	UpdateSnoozeList  *conversations.UpdateSnoozeList
	ShowSnoozeOptions *conversations.ShowSnoozeOptions

	ListDrafts  *drafts.ListDrafts
	SaveDraft   *drafts.SaveDraft
	UpdateDraft *drafts.UpdateDraft
	DeleteDraft *drafts.DeleteDraft

	ListMessages      *messages.ListMessages
	Reply             *messages.ReplyToConversation
	AddNote           *messages.AddNote
	AddContactMessage *messages.AddContactMessage
	DeleteMessage     *messages.DeleteMessage
	EditNote          *messages.EditNote

	ListRedactionRules      *redaction.ListRules
	UpdateRedactionSettings *redaction.UpdateSettings
	CreateRedactionRule     *redaction.CreateRule
	UpdateRedactionRule     *redaction.UpdateRule
	DeleteRedactionRule     *redaction.DeleteRule
	TryRedaction            *redaction.TryRedaction

	RequestErasure *erasures.RequestErasure
	CancelErasure  *erasures.CancelErasure
	ListErasures   *erasures.ListErasures
	ShowErasure    *erasures.ShowErasure
	EraseContact   *erasures.EraseContact

	RequestContactExport  *contacts.RequestContactExport
	ShowContactExport     *contacts.ShowContactExport
	DownloadContactExport *contacts.DownloadContactExport
	BuildContactExport    *contacts.BuildContactExport
	ExpireContactExports  *contacts.ExpireContactExports

	ApplyRetention   *retention.ApplyConversationRetention
	PreviewRetention *retention.PreviewRetention
}
