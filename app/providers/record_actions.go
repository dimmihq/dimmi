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
}
