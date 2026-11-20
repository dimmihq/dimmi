/*
|--------------------------------------------------------------------------
| Tool Catalog: Customer Notes
|--------------------------------------------------------------------------
|
| The entries for what the team writes down about a customer or a
| company, as separate notes, one of which can be pinned to show first.
|
*/

package mcpserver

/*
noteDocs returns the entries for customer and company notes.
*/
func noteDocs() map[string]Doc {
	return map[string]Doc{
		"contacts.notes.index": {
			Title:   "List a customer's notes",
			Use:     "Lists what the team wrote about a customer, the pinned note first, then newest first.",
			Returns: "the notes with their author and when they were written.",
		},
		"contacts.notes.store": {
			Title: "Add a note about a customer",
			Use:   "Writes a note only the team sees about a customer, such as context for the next teammate.",
		},
		"companies.notes.index": {
			Title:   "List a company's notes",
			Use:     "Lists what the team wrote about a company, the pinned note first, then newest first.",
			Returns: "the notes with their author and when they were written.",
		},
		"companies.notes.store": {
			Title: "Add a note about a company",
			Use:   "Writes a note only the team sees about a company.",
		},
		"contact-notes.update": {
			Title: "Change a note",
			Use:   "Changes a customer or company note's text, or pins it so it shows first.",
		},
		"contact-notes.destroy": {
			Title: "Delete a note",
			Use:   "Deletes a customer or company note.",
		},
	}
}
