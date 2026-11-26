/*
|--------------------------------------------------------------------------
| Migration Catalog
|--------------------------------------------------------------------------
|
| Entries for migrations from other tools: start or resume one, follow
| it and undo it. Sending records and uploading files are left to the
| dimmi CLI, so the customer data they carry never passes
| through an AI model; an agent drives the CLI and reads progress here.
|
*/

package mcpserver

/*
migrationDocs returns the entries of migrations.
*/
func migrationDocs() map[string]Doc {
	return map[string]Doc{
		"migrations.store": {
			Title: "Start or resume a migration",
			Use: "Starts a migration of history from another tool for one account there, or returns the one " +
				"under way for the same source and account. Records are sent by the dimmi CLI. Needs " +
				"imports:manage.",
			Returns: "the migration with its progress.",
			Next:    []string{"dimmi_get_migration_by_id: follow its progress"},
			Risk:    new(RiskWrite),
		},
		"migrations.index": {
			Title:   "List migrations",
			Use:     "Lists the workspace's migrations from other tools with where each stands.",
			Returns: "up to 100 migrations, newest first.",
		},
		"migrations.show": {
			Title: "Get a migration",
			Use: "Reads one migration: how many records of each kind were applied, wait for another record, " +
				"failed or are not supported yet, and the latest problems with why.",
			Returns: "the migration with counts per kind and its latest problems.",
		},
		"migrations.lookups.store": {
			Name:  "look_up_migration_records",
			Title: "Look up records of a migration",
			Use: "Says where each record named by its id in the other tool stands: applied with the dimmi id it " +
				"became, deferred, failed, unsupported, or missing when the migration never received it. Use it " +
				"to check a sample after a run.",
			Returns: "one entry per source id, in the order asked.",
		},
		"migrations.rollbacks.store": {
			Name:  "roll_back_migration",
			Title: "Undo a migration",
			Use: "Undoes everything the migration created, in the background: conversations, contacts, " +
				"articles and tickets go to the trash, tags, notes and replies are deleted. Records others now " +
				"use are kept. Needs imports:rollback.",
			Returns: "the migration with status rolling_back; poll it until rolled_back.",
			Next:    []string{"dimmi_get_migration_by_id: follow the undo until status is rolled_back"},
			Risk:    new(RiskDanger),
		},
	}
}
