/*
|--------------------------------------------------------------------------
| Procedure Requests
|--------------------------------------------------------------------------
|
| The inputs of procedures. A definition travels as plain JSON; a save
| names the lock version the editor started from, so two editors never
| overwrite each other.
|
*/

package requests

import "encoding/json"

/*
ProcedureRequest names a procedure in the path.
*/
type ProcedureRequest struct {
	Procedure string `path:"procedure"`
}

/*
StoreProcedureRequest is the input of POST /procedures.
*/
type StoreProcedureRequest struct {
	Body struct {
		Name string `json:"name" maxLength:"80"`
	}
}

/*
UpdateProcedureRequest is the input of PATCH /procedures/{procedure}: a
save of the name or the working copy.
*/
type UpdateProcedureRequest struct {
	Procedure string `path:"procedure"`
	Body      struct {
		Name        *string         `json:"name,omitempty"         maxLength:"80"`
		Definition  json.RawMessage `json:"definition,omitempty"`
		LockVersion int32           `json:"lock_version,omitempty"`
	}
}
