/*
|--------------------------------------------------------------------------
| Operator Changeset Requests
|--------------------------------------------------------------------------
|
| The changeset a teammate decides on, and the memory they make
| Operator forget, named by public id.
|
*/

package requests

/*
OperatorChangesetRequest names one changeset.
*/
type OperatorChangesetRequest struct {
	Changeset string `path:"changeset"`
}

/*
OperatorMemoryRequest names one memory.
*/
type OperatorMemoryRequest struct {
	Memory string `path:"memory"`
}
