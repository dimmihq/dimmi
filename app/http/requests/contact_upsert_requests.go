/*
|--------------------------------------------------------------------------
| Contact Create And Identity Requests
|--------------------------------------------------------------------------
|
| Creating a contact also serves a trusted server identifying a customer,
| with the company they work for, which is why it keeps the raw body, to
| tell a field cleared with null from one left out.
|
*/

package requests

/*
UpsertContactBody adds, to a contact's details, the company a trusted
caller names, and the ids of companies the contact belongs to. On a
plain create CompanyID names the default among them; on an upsert the
companies are added to those the contact has.
*/
type UpsertContactBody struct {
	ContactBody

	Company    *CompanyBody `json:"company,omitempty"`
	CompanyID  *string      `json:"company_id,omitempty"`
	CompanyIDs []string     `json:"company_ids,omitempty" validate:"omitempty,max=300,dive,min=1,max=64"`
}

/*
StoreContactRequest is the input of POST /contacts: a plain create, or
with upsert=true a find-or-add by external id, then email.
*/
type StoreContactRequest struct {
	Upsert  string `query:"upsert"`
	Body    UpsertContactBody
	RawBody []byte
}

/*
ContactIdentityRequest is the input of DELETE
/contacts/{contact}/identities/{identity}.
*/
type ContactIdentityRequest struct {
	Contact  string `path:"contact"`
	Identity string `path:"identity"`
}
