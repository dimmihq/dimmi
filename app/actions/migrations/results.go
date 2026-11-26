/*
|--------------------------------------------------------------------------
| Record Results
|--------------------------------------------------------------------------
|
| What a client reads back about each record: the records it names,
| warnings about fields left out, conflicts with what teammates changed
| here, and why a record could not be applied.
|
*/

package migrations

/*
Ref names another record of the migration by its kind and source id.
*/
type Ref struct {
	Kind     string `json:"kind"`
	SourceID string `json:"source_id"`
}

/*
Notice is a warning about a field: what was left out and why.
*/
type Notice struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

/*
Conflict is a field a teammate changed here after the migration wrote
it, which a later sync leaves as the teammate set it: the value here,
and the one the other tool sent.
*/
type Conflict struct {
	Field  string `json:"field"`
	Dimmi  any    `json:"dimmi"`
	Source any    `json:"source"`
}

/*
Problem is why a record could not be applied.
*/
type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}
