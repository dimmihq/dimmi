/*
|--------------------------------------------------------------------------
| Confirmed Resolution
|--------------------------------------------------------------------------
|
| A conversation counts as resolved only when the customer says so. An
| answer that asks for confirmation ends with Prompt; the decision model
| judges whether the customer's reply confirms it (see
| app/ai/pipeline/resolution.go).
|
*/

package resolution

/*
Prompt is the line an answer ends with when the customer is asked to
confirm it.
*/
const Prompt = "Did that solve it? Reply yes if it did."
