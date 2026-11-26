/*
|--------------------------------------------------------------------------
| Tool Catalog: Migration Records
|--------------------------------------------------------------------------
|
| Sending a migration's records is not a tool: the records carry
| customer data, which never passes through an AI model. The dimmi CLI
| sends them, and the API document explains the operation all the same.
|
*/

package mcpserver

/*
unexposedDocsFromMigrationsRecordsStore returns the entry of
migrations.records.store.
*/
func unexposedDocsFromMigrationsRecordsStore() map[string]Doc {
	return map[string]Doc{
		"migrations.records.store": {
			Title: "Send migration records",
			Use: "Applies a batch of 1 to 100 records of one kind in dimmi's import format to the migration, " +
				"as quiet history: nothing is sent and no event goes out. Each record is kept under its " +
				"source_id, so sending it again updates what it became or skips it when unchanged, and a " +
				"record naming one not sent yet waits for it. It needs the settings:write ability and the " +
				"imports:manage permission.",
			Returns: "a result per record, in the order sent, and the waiting records this batch let through.",
		},
	}
}
