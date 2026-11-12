/*
|--------------------------------------------------------------------------
| Incident Broadcasts
|--------------------------------------------------------------------------
|
| A suggested incident reaches everyone viewing the workspace at once,
| so whoever is online sees "12 conversations about failed sign-ins.
| Declare an incident?" without reloading. The broadcast names the
| suggestion; screens read it from the API.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
BroadcastData names the suggestion for open screens.
*/
func (e IncidentSuggested) BroadcastData() any {
	return map[string]string{"suggestion_id": ids.Encode("incs", e.SuggestionID)}
}
