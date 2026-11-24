/*
|--------------------------------------------------------------------------
| Series Simulation Requests
|--------------------------------------------------------------------------
|
| A dry simulation of one person through a series: nothing is sent and
| no wait holds them.
|
*/

package requests

/*
SimulateSeriesPerson is the made-up person a series' dry simulation
walks: a name, an email and custom attributes.
*/
type SimulateSeriesPerson struct {
	Name       string         `json:"name,omitempty"       maxLength:"200"`
	Email      string         `json:"email,omitempty"      maxLength:"320"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

/*
RunSeriesSimulationBody is a dry simulation of a series: the copy
(draft by default, or live), the contact who travels it or a made-up
person, the events they would do, and what they would do with its
messages.
*/
type RunSeriesSimulationBody struct {
	Version   string                `json:"version,omitempty"    enum:"draft,live,"`
	ContactID string                `json:"contact_id,omitempty" maxLength:"64"`
	Person    *SimulateSeriesPerson `json:"person,omitempty"`
	Events    []string              `json:"events,omitempty"     maxItems:"20"`
	Engaged   []string              `json:"engaged,omitempty"    maxItems:"3"`
}

/*
RunSeriesSimulationRequest is the input of POST
/series/{series}/simulations/run.
*/
type RunSeriesSimulationRequest struct {
	Series string `path:"series"`
	Body   RunSeriesSimulationBody
}
