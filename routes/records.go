/*
|--------------------------------------------------------------------------
| Customer Record Routes
|--------------------------------------------------------------------------
|
| What the team keeps about customers and what they talk about: the tag
| vocabulary and the tags on conversations, contacts' identities and
| possible duplicates, and companies. Every route works inside the
| workspace the request names, behind the same guards as the inbox.
| Adding a tag is part of working a conversation; renaming and deleting
| one change what every record means, so they are a settings change, as
| are the workspace's redaction rules.
|
*/

package routes

import (
	"net/http"

	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
recordRoutes declares the customer record routes.
*/
func recordRoutes(r *routing.Router, a *app.App) {
	r.Middleware("csrf", "session", "auth", "workspace")

	tagRoutes(r, a)
	contextRoutes(r, a)
	contactDetailRoutes(r, a)
	companyRoutes(r, a)
	redactionRoutes(r, a)
}

/*
redactionRoutes declares the routes on the workspace's redaction rules,
a settings change since they alter what every later message stores.
*/
func redactionRoutes(r *routing.Router, a *app.App) {
	acts := a.Actions.Inbox
	c := &v1.RedactionController{
		List: acts.ListRedactionRules, Settings: acts.UpdateRedactionSettings, Create: acts.CreateRedactionRule,
		Change: acts.UpdateRedactionRule, Remove: acts.DeleteRedactionRule, Try: acts.TryRedaction, IDs: a.IDs,
	}
	path := "/redaction-rules/{redaction_rule}"

	routing.Get(r, "/redaction-rules", "redaction-rules.index", c.Index).Ability("settings:read")
	routing.Patch(r, "/redaction-rules", "redaction-rules.settings", c.UpdateSettings).Ability("settings:write")
	routing.Post(r, "/redaction-rules", "redaction-rules.store", c.Store).Ability("settings:write")
	routing.Patch(r, path, "redaction-rules.update", c.Update).Ability("settings:write")
	routing.Delete(r, path, "redaction-rules.destroy", c.Destroy).Ability("settings:write")
	routing.Post(r, "/redaction-tests", "redaction-tests.store", c.Test).Ability("settings:write").
		Status(http.StatusOK)
}

/*
contextRoutes declares what the panel beside a conversation shows about
its customer.
*/
func contextRoutes(r *routing.Router, a *app.App) {
	c := &v1.ConversationContextController{Show: a.Actions.Inbox.ShowContext, IDs: a.IDs}

	routing.Get(r, "/conversations/{conversation}/context", "conversations.context.show", c.Get).
		Ability("conversations:read", "contacts:read")
}

/*
contactDetailRoutes declares the routes around a contact, beside the
inbox's own contact routes, and the requests to erase customers.
*/
func contactDetailRoutes(r *routing.Router, a *app.App) {
	acts := a.Actions.Inbox
	cursors := pagination.NewCursors(a.DerivedKey(v1.CursorKeyPurpose))
	contacts := &v1.ContactController{Conversations: acts.ListRecordConversations, IDs: a.IDs, Cursors: cursors}
	removal := &v1.ErasureController{
		Remove: acts.DeleteContact, Request: acts.RequestErasure, Cancel: acts.CancelErasure,
		List: acts.ListErasures, Show: acts.ShowErasure, IDs: a.IDs,
	}
	details := &v1.ContactIdentityController{
		List: acts.ListIdentities, Detach: acts.DetachIdentity, Duplicates: acts.FindDuplicates, IDs: a.IDs,
	}
	path := "/contacts/{contact}"

	routing.Delete(r, path, "contacts.destroy", removal.Destroy).Ability("contacts:write")
	exports := &v1.ContactExportController{
		Request: acts.RequestContactExport, Show: acts.ShowContactExport, Download: acts.DownloadContactExport,
		IDs: a.IDs,
	}
	routing.Post(r, path+"/exports", "contacts.exports.store", exports.Store).Ability("contacts:write").
		Status(http.StatusAccepted)
	routing.Get(r, path+"/exports/{export}", "contacts.exports.show", exports.Get).Ability("contacts:write")
	routing.Get(r, path+"/exports/{export}/download", "contacts.exports.download", exports.Zip).
		Ability("contacts:write")
	routing.Get(r, "/contact-erasures", "contact-erasures.index", removal.Index).Ability("contacts:write")
	routing.Get(r, "/contact-erasures/{contact_erasure}", "contact-erasures.show", removal.Get).
		Ability("contacts:write")
	routing.Patch(r, "/contact-erasures/{contact_erasure}", "contact-erasures.update", removal.Update).
		Ability("contacts:write")
	routing.Get(r, path+"/conversations", "contacts.conversations.index", contacts.ConversationsIndex).
		Ability("conversations:read")
	routing.Get(r, path+"/identities", "contacts.identities.index", details.Index).Ability("contacts:read")
	routing.Delete(r, path+"/identities/{identity}", "contacts.identities.destroy", details.Destroy).
		Ability("contacts:write")
	routing.Get(r, path+"/duplicates", "contacts.duplicates.index", details.DuplicatesIndex).Ability("contacts:read")
}

/*
companyRoutes declares the routes on companies.
*/
func companyRoutes(r *routing.Router, a *app.App) {
	acts := a.Actions.Inbox
	c := &v1.CompanyController{
		List: acts.ListCompanies, Show: acts.ShowCompany, Create: acts.CreateCompany, Identify: acts.IdentifyCompany,
		Change: acts.UpdateCompany, Remove: acts.DeleteCompany, People: acts.ListCompanyContacts,
		Conversations: acts.ListRecordConversations, IDs: a.IDs,
		Cursors: pagination.NewCursors(a.DerivedKey(v1.CursorKeyPurpose)),
	}
	path := "/companies/{company}"

	routing.Get(r, "/companies", "companies.index", c.Index).Ability("contacts:read")
	routing.Post(r, "/companies", "companies.store", c.Store).Ability("contacts:write")
	routing.Get(r, path, "companies.show", c.Get).Ability("contacts:read")
	routing.Patch(r, path, "companies.update", c.Update).Ability("contacts:write")
	routing.Delete(r, path, "companies.destroy", c.Destroy).Ability("contacts:write")
	routing.Get(r, path+"/contacts", "companies.contacts.index", c.Contacts).Ability("contacts:read")
	routing.Get(r, path+"/conversations", "companies.conversations.index", c.ConversationsIndex).
		Ability("conversations:read")
}

/*
tagRoutes declares the routes on tags.
*/
func tagRoutes(r *routing.Router, a *app.App) {
	acts := a.Actions.Inbox
	tags := &v1.TagController{
		List: acts.ListTags, Create: acts.CreateTag, Change: acts.UpdateTag, Remove: acts.DeleteTag, IDs: a.IDs,
	}
	onConversation := &v1.ConversationTagController{
		Tag: acts.TagConversation, Untag: acts.UntagConversation, IDs: a.IDs,
	}

	routing.Get(r, "/tags", "tags.index", tags.Index).Ability("conversations:read")
	routing.Post(r, "/tags", "tags.store", tags.Store).Ability("conversations:write")
	routing.Patch(r, "/tags/{tag}", "tags.update", tags.Update).Ability("settings:write")
	routing.Delete(r, "/tags/{tag}", "tags.destroy", tags.Destroy).Ability("settings:write")

	routing.Post(r, "/conversations/{conversation}/tags", "conversations.tags.store", onConversation.Store).
		Ability("conversations:write").Status(http.StatusOK)
	routing.Delete(r, "/conversations/{conversation}/tags/{tag}", "conversations.tags.destroy",
		onConversation.Destroy).Ability("conversations:write")
}
