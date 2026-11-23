/*
|--------------------------------------------------------------------------
| Operator Viewing
|--------------------------------------------------------------------------
|
| What a teammate has open when they ask Operator something.
|
*/

package requests

/*
OperatorViewing says what the teammate has open when they ask Operator:
an article or a procedure, and the suggested change shown on it.
Operator explains and changes that unless the question says otherwise.
*/
type OperatorViewing struct {
	ArticleID   *string `json:"article_id,omitempty"   validate:"omitempty,max=64"`
	ProcedureID *string `json:"procedure_id,omitempty" validate:"omitempty,max=64"`
	ChangesetID *string `json:"changeset_id,omitempty" validate:"omitempty,max=64"`
}
