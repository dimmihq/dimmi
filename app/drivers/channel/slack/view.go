/*
|--------------------------------------------------------------------------
| Submitted Forms
|--------------------------------------------------------------------------
|
| A form the app opened comes back with what it was opened with and what
| the person entered, each input under its block. Nothing in it is
| trusted beyond naming what the person meant to act on.
|
*/

package slack

/*
View is a submitted form: its callback id, what it was opened with,
and the values entered, by block and input.
*/
type View struct {
	CallbackID      string `json:"callback_id"`
	PrivateMetadata string `json:"private_metadata"`
	State           struct {
		Values map[string]map[string]struct {
			Value string `json:"value"`
		} `json:"values"`
	} `json:"state"`
}

/*
Value returns what was entered in the input of the block.
*/
func (v View) Value(block, input string) string {
	return v.State.Values[block][input].Value
}
