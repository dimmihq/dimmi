/*
|--------------------------------------------------------------------------
| Customer Record Actions
|--------------------------------------------------------------------------
|
| The actions on what the team keeps about customers: the tag vocabulary,
| contacts' identities and duplicates, and companies. They sit beside the
| inbox's own actions, built once at boot.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/redaction"
	"github.com/dimmihq/dimmi/app/actions/savedreplies"
	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
tagPayload is a tag as tag events carry it to open screens.
*/
func tagPayload(tag models.Tag) any {
	return resources.NewTag(tag)
}

/*
bootRecords builds the customer record actions.
*/
func bootRecords(a *app.App) {
	acts := a.Actions.Inbox

	acts.UpsertContact = &contacts.UpsertIdentifiedContact{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.DeleteContact = &contacts.DeleteContact{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.ListIdentities = &contacts.ListIdentities{DB: a.DB}
	acts.DetachIdentity = &contacts.DetachIdentity{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.FindDuplicates = &contacts.FindDuplicateContacts{DB: a.DB}
	acts.ListRecordConversations = &conversations.ListRecordConversations{DB: a.DB}
	acts.ShowContext = &conversations.ShowContext{DB: a.DB}

	acts.ListTags = &tags.ListTags{DB: a.DB}
	acts.CreateTag = &tags.CreateTag{DB: a.DB, Events: a.Events, Clock: a.Clock, Payload: tagPayload}
	acts.UpdateTag = &tags.UpdateTag{DB: a.DB, Events: a.Events, Clock: a.Clock, Payload: tagPayload}
	acts.DeleteTag = &tags.DeleteTag{DB: a.DB, Events: a.Events, Clock: a.Clock, Payload: tagPayload}
	acts.TagConversation = &conversations.TagConversation{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.UntagConversation = &conversations.UntagConversation{DB: a.DB, Events: a.Events, Clock: a.Clock}

	acts.ListCompanies = &companies.ListCompanies{DB: a.DB}
	acts.ShowCompany = &companies.ShowCompany{DB: a.DB}
	acts.CreateCompany = &companies.CreateCompany{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.IdentifyCompany = &companies.IdentifyCompany{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.UpdateCompany = &companies.UpdateCompany{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.DeleteCompany = &companies.DeleteCompany{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.ListCompanyContacts = &companies.ListCompanyContacts{DB: a.DB}

	bootRedaction(a)
	bootSavedReplies(a)
}

/*
bootRedaction builds the actions on the workspace's redaction rules.
*/
func bootRedaction(a *app.App) {
	acts := a.Actions.Inbox
	acts.ListRedactionRules = &redaction.ListRules{DB: a.DB}
	acts.UpdateRedactionSettings = &redaction.UpdateSettings{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.CreateRedactionRule = &redaction.CreateRule{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.UpdateRedactionRule = &redaction.UpdateRule{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.DeleteRedactionRule = &redaction.DeleteRule{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.TryRedaction = &redaction.TryRedaction{DB: a.DB}
}

/*
bootSavedReplies builds the actions on saved replies and macros.
*/
func bootSavedReplies(a *app.App) {
	acts := a.Actions.Inbox
	acts.ListSavedReplies = &savedreplies.ListSavedReplies{DB: a.DB}
	acts.ShowSavedReply = &savedreplies.ShowSavedReply{DB: a.DB}
	acts.CreateSavedReply = &savedreplies.CreateSavedReply{DB: a.DB, Clock: a.Clock}
	acts.UpdateSavedReply = &savedreplies.UpdateSavedReply{DB: a.DB, Clock: a.Clock}
	acts.DeleteSavedReply = &savedreplies.DeleteSavedReply{DB: a.DB}
	acts.ImportSavedReplies = &savedreplies.ImportSavedReplies{DB: a.DB, Clock: a.Clock}
}
