/*
|--------------------------------------------------------------------------
| Knowledge Option Requests
|--------------------------------------------------------------------------
|
| Asking a connected service which values a source setting can take,
| such as the repositories an account reaches, with the settings filled
| so far.
|
*/

package requests

/*
KnowledgeOptionsBody names a setting to list the values of, with the
settings filled so far.
*/
type KnowledgeOptionsBody struct {
	Type     string         `json:"type"               maxLength:"40"`
	Settings map[string]any `json:"settings,omitempty"`
	Field    string         `json:"field"              maxLength:"60"`
}

/*
KnowledgeOptionsRequest is the input of POST /knowledge/sources/options.
*/
type KnowledgeOptionsRequest struct {
	Body KnowledgeOptionsBody
}
