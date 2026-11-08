/*
|--------------------------------------------------------------------------
| Messenger Rule Requests
|--------------------------------------------------------------------------
|
| The input of saving who sees a messenger's launcher, who may start a
| conversation and who sees each tab. Each rule is an audience in the
| rule language; a missing one means everyone.
|
*/

package requests

import "encoding/json"

/*
SaveWidgetAudiencesRequest is the input of PATCH
/widget-settings/{widget}/audiences. A rule left out stays as it is and
null removes it.
*/
type SaveWidgetAudiencesRequest struct {
	Widget string `path:"widget"`
	Body   struct {
		Launcher json.RawMessage            `json:"launcher,omitempty"`
		Start    json.RawMessage            `json:"start,omitempty"`
		Tabs     map[string]json.RawMessage `json:"tabs,omitempty"`
	}
}
