/*
|--------------------------------------------------------------------------
| Request Step Settings
|--------------------------------------------------------------------------
|
| The typed settings of the step that calls another service: where it
| sends the request, what it sends and which response values it keeps.
|
*/

package workflowspec

/*
HTTPRequestSettings calls the customer's own system. With ConnectionID
the URL is a path under the connection's base URL and the connection's
sealed secret authenticates the call; without one it is an absolute
URL called without credentials. URL, header values and Body hold
variables. Save copies values of a JSON response into run variables or
contact attributes.
*/
type HTTPRequestSettings struct {
	ConnectionID string         `json:"connection_id,omitempty"`
	URL          string         `json:"url,omitempty"`
	Method       string         `json:"method,omitempty"`
	Headers      []HTTPHeader   `json:"headers,omitempty"`
	Body         string         `json:"body,omitempty"`
	Timeout      string         `json:"timeout,omitempty"`
	Save         []ResponseSave `json:"save,omitempty"`
}

/*
HTTPHeader is one header a request sends.
*/
type HTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

/*
ResponseSave copies the value at Path, a plain dotted path into the
response, to Field: run.vars.<key> or contact.custom.<key>.
*/
type ResponseSave struct {
	Path  string `json:"path"`
	Field string `json:"field"`
}
