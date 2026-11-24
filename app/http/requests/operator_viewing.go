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
the kind of page (article, procedure, conversation, help_center,
feature_request, changelog_entry, workflow or series), the id of what
it shows, and the id of the suggested change shown on it, when there is
one. Operator is given that record in full and works on it unless the
question says otherwise. A kind it does not know is ignored.
*/
type OperatorViewing struct {
	Page         string  `json:"page"                    validate:"required,max=64"`
	ID           string  `json:"id"                      validate:"required,max=64"`
	SuggestionID *string `json:"suggestion_id,omitempty" validate:"omitempty,max=64"`
}
