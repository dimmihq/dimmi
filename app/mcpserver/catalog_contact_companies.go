package mcpserver

/*
contactCompanyDocs returns the entries that add, remove and pick the
default among the companies a contact belongs to.
*/
func contactCompanyDocs() map[string]Doc {
	return map[string]Doc{
		"contacts.companies.store": {
			Title: "Add a company to a contact",
			Use: "Links a contact to one more company. A contact can belong to several; one is the default, " +
				"which conversations and tickets take when they start. The first company becomes the default.",
			Returns: "the contact as it now is, with all its companies.",
			Params: map[string]string{
				"company_id": "Id of the company (cmp_...) to add.",
				"default":    "true also makes it the contact's default company.",
			},
			Hint: "company_limit_reached: the contact is already in 300 companies; remove one first.",
		},
		"contacts.companies.destroy": {
			Title: "Remove a company from a contact",
			Use: "Unlinks a contact from one company. When it was the default, the company linked longest " +
				"becomes the default, or the contact has none left.",
			Returns: "the contact as it now is, with all its companies.",
			Params: map[string]string{
				"company": "Id of the company (cmp_...) to remove.",
			},
		},
		"contacts.companies.default.store": {
			Title: "Make a company a contact's default",
			Use: "Makes one of the contact's companies its default: company_id and company on the contact " +
				"then name it, and new conversations and tickets take it. Past ones keep theirs.",
			Returns: "the contact as it now is, with all its companies.",
			Params: map[string]string{
				"company": "Id of a company (cmp_...) the contact already belongs to.",
			},
		},
	}
}

/*
schemaTextsFromContactCompany returns the texts of a contact's link to
one company.
*/
func schemaTextsFromContactCompany() map[string]map[string]string {
	return map[string]map[string]string{
		"ContactCompany": {
			"name":       "The company's name.",
			"domain":     "The company's web domain, such as acme.com. Null when unknown.",
			"is_default": "true for the contact's default company, the one company_id and company name.",
			"added_at":   "When the contact was linked to the company.",
		},
	}
}

/*
sharedTextsFromContactCompanies returns the texts of the request fields
that pick a company among a contact's companies.
*/
func sharedTextsFromContactCompanies() map[string]string {
	return map[string]string{
		"UpdateConversationRequestBody.company_id": "Id of the company (cmp_...) the conversation is " +
			"about, one of its contact's companies. New conversations take the contact's default.",
	}
}
