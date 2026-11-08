/*
|--------------------------------------------------------------------------
| Base Fields
|--------------------------------------------------------------------------
|
| The fields every surface can read: who the contact is, their company,
| their tags, what they did in the team's product, the session they
| came in on and the page they are on.
| Email and phone are only as trusted as their verification; custom
| attributes take their type from the value until definitions say
| otherwise.
|
*/

package audience

/*
Base returns the catalog of contact, company, session, visitor and page
fields, plus saved audiences.
*/
func Base() *Catalog {
	return NewCatalog(append(append(contactFields(), companyFields()...), clientFields()...)...)
}

/*
contactFields are the contact's own fields.
*/
func contactFields() []Field {
	return []Field{
		{Key: "contact.kind", Type: TypeList, Where: Server, Trust: Verified, Options: []string{"user", "lead"}},
		{Key: "contact.email", Type: TypeString, Where: Server, Trust: Claimed},
		{Key: "contact.email_domain", Type: TypeString, Where: Server, Trust: Claimed},
		{Key: "contact.email_verified", Type: TypeBool, Where: Server, Trust: Verified},
		{Key: "contact.phone", Type: TypeString, Where: Server, Trust: Claimed},
		{Key: "contact.name", Type: TypeString, Where: Server, Trust: Claimed},
		{Key: "contact.external_id", Type: TypeString, Where: Server, Trust: Verified},
		{Key: "contact.locale", Type: TypeString, Where: Server, Trust: Claimed},
		{Key: "contact.timezone", Type: TypeString, Where: Server, Trust: Claimed},
		{Key: "contact.country", Type: TypeString, Where: Server, Trust: Observed},
		{Key: "contact.region", Type: TypeString, Where: Server, Trust: Observed},
		{Key: "contact.city", Type: TypeString, Where: Server, Trust: Observed},
		{Key: "contact.created_at", Type: TypeDate, Where: Server, Trust: Observed},
		{Key: "contact.first_seen_at", Type: TypeDate, Where: Server, Trust: Observed},
		{Key: "contact.last_seen_at", Type: TypeDate, Where: Server, Trust: Observed},
		{Key: "contact.has_company", Type: TypeBool, Where: Server, Trust: Verified},
		{Key: "contact.tags", Type: TypeSet, Where: Server, Trust: Verified},
		{Key: "contact.custom.", Prefix: true, Type: TypeAuto, Where: Server, Trust: ByKey},
		{Key: "event.", Prefix: true, Type: TypeAuto, Where: Server, Trust: Verified},
		{Key: "audience", Type: TypeSaved, Where: Server, Trust: Verified, Ops: []string{OpIn, OpNotIn}},
	}
}

/*
companyFields are the fields of the contact's company. The browser can
never write them, so they are verified.
*/
func companyFields() []Field {
	return []Field{
		{Key: "company.name", Type: TypeString, Where: Server, Trust: Verified},
		{Key: "company.domain", Type: TypeString, Where: Server, Trust: Verified},
		{Key: "company.created_at", Type: TypeDate, Where: Server, Trust: Verified},
		{Key: "company.tags", Type: TypeSet, Where: Server, Trust: Verified},
		{Key: "company.custom.", Prefix: true, Type: TypeAuto, Where: Server, Trust: Verified},
	}
}

/*
clientFields are what the session and the browser report.
*/
func clientFields() []Field {
	return []Field{
		{Key: "session.verified", Type: TypeBool, Where: Server, Trust: Verified},
		{Key: "session.country", Type: TypeString, Where: Server, Trust: Observed},
		{Key: "session.browser", Type: TypeString, Where: Server, Trust: Claimed},
		{Key: "session.os", Type: TypeString, Where: Server, Trust: Claimed},
		{Key: "session.context.", Prefix: true, Type: TypeAuto, Where: Server, Trust: Claimed},
		{Key: "visitor.locale", Type: TypeString, Where: Client, Trust: Claimed},
		{
			Key: "visitor.device", Type: TypeList, Where: Client, Trust: Claimed,
			Options: []string{"desktop", "mobile", "tablet"},
		},
		{Key: "page.url", Type: TypeURL, Where: Client, Trust: Claimed},
		{Key: "page.path", Type: TypeURL, Where: Client, Trust: Claimed},
		{Key: "page.query", Type: TypeURL, Where: Client, Trust: Claimed},
		{Key: "page.host", Type: TypeURL, Where: Client, Trust: Claimed},
		{Key: "page.referrer", Type: TypeURL, Where: Client, Trust: Claimed},
	}
}
