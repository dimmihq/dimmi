/*
|--------------------------------------------------------------------------
| Migration Requests
|--------------------------------------------------------------------------
|
| Inputs of the migrations API. A migration is started for an account of
| another tool, named by the tool's short name and the account's id
| there. Records arrive in batches of one kind, each in dimmi's import
| format; the JSON Schema of every kind is published with the docs.
|
*/

package requests

import (
	"encoding/json"
)

/*
StoreMigrationRequest is the input of POST /migrations.
*/
type StoreMigrationRequest struct {
	Body struct {
		Source        string  `json:"source"         validate:"required,max=40"  doc:"Short lowercase name of the tool the history comes from, such as intercom."`                 //nolint:lll // a struct tag cannot wrap
		SourceAccount string  `json:"source_account" validate:"required,max=255" doc:"Id of the account in that tool. Starting again for the same account resumes its migration."` //nolint:lll // a struct tag cannot wrap
		Name          *string `json:"name,omitempty" validate:"omitempty,max=120"`
	}
}

/*
MigrationRequest names a migration.
*/
type MigrationRequest struct {
	Migration string `path:"migration"`
}

/*
MigrationRecordsRequest is the input of POST /migrations/{migration}/records:
up to 100 records of one kind.
*/
type MigrationRecordsRequest struct {
	Migration string `path:"migration"`
	Body      struct {
		Kind    string            `json:"kind"    validate:"required" enum:"teammate,team,tag,attribute_definition,company,contact,contact_note,ticket_type,help_center,collection,article,snippet,saved_reply,schedule,workflow,conversation,ticket"` //nolint:lll // a struct tag cannot wrap
		Records []json.RawMessage `json:"records" validate:"required,min=1,max=100" maxItems:"100" minItems:"1" doc:"Records of the kind in the import format, version 1."`                                                                            //nolint:lll // a struct tag cannot wrap
	}
}

/*
MigrationLookupRequest is the input of POST /migrations/{migration}/lookups:
up to 100 source ids of one kind.
*/
type MigrationLookupRequest struct {
	Migration string `path:"migration"`
	Body      struct {
		Kind      string   `json:"kind"       validate:"required" enum:"teammate,team,tag,attribute_definition,company,contact,contact_note,ticket_type,help_center,collection,article,snippet,saved_reply,schedule,workflow,conversation,ticket"` //nolint:lll // a struct tag cannot wrap
		SourceIDs []string `json:"source_ids" validate:"required,min=1,max=100,dive,max=255" maxItems:"100" minItems:"1"`                                                                                                                          //nolint:lll // a struct tag cannot wrap
	}
}
