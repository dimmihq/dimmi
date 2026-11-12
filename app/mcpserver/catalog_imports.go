/*
|--------------------------------------------------------------------------
| Import Catalog
|--------------------------------------------------------------------------
|
| Entries for imports of customers and companies from a CSV: upload,
| dry run, start, follow and the report of failed rows. Uploading the
| file is a form, which agents do not send, so it has no entry.
|
*/

package mcpserver

/*
importDocs returns the entries of imports.
*/
func importDocs() map[string]Doc {
	return map[string]Doc{
		"imports.index": {
			Title: "List imports",
			Use: "Lists the latest imports of customers or companies with where each stands. " +
				"Imports need imports:manage.",
			Returns: "up to 20 imports, newest first.",
		},
		"imports.show": {
			Title:   "Get an import",
			Use:     "Reads one import: its columns, mapping, progress, counts and first failed rows.",
			Returns: "the import with status draft, queued, running, completed or failed.",
		},
		"imports.previews.store": {
			Title: "Dry run an import",
			Use: "Runs a mapping over every row without writing anything and counts what would be created, " +
				"updated, skipped or refused.",
			Returns: "the counts and the first refused rows with why.",
			Next:    []string{"dimmi_imports_runs_create: start the import with the same mapping"},
			Params: map[string]string{
				"mapping": "A list of {column, field}: column is the index in the header from 0; field is " +
					"email, name, external_id, phone, company_name, company_domain or tags for contacts, " +
					"name, domain, external_id or tags for companies, or attribute.<key> for a custom attribute.",
				"mode": "update changes records already there; skip leaves them alone.",
			},
		},
		"imports.runs.store": {
			Title:   "Start an import",
			Use:     "Starts a draft import in the background with the mapping and mode given. An import runs once.",
			Returns: "the import with status queued; poll it for progress.",
			Next:    []string{"dimmi_imports_get: follow progress until completed"},
			Risk:    new(RiskDanger),
			Params: map[string]string{
				"mapping": "The same mapping as the dry run.",
				"mode":    "update or skip.",
			},
		},
		"imports.errors.show": {
			Title:   "Download an import's failed rows",
			Use:     "Fetches a CSV of the rows that failed, each with its line in the file and why.",
			Returns: "the CSV.",
			Risk:    new(RiskSensitive),
		},
	}
}
