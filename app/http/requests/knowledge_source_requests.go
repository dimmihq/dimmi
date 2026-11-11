/*
|--------------------------------------------------------------------------
| Knowledge Source Requests
|--------------------------------------------------------------------------
|
| The inputs of the knowledge sources API. A source's settings are the
| JSON object its driver's schema describes, secrets included; secrets
| are never sent back. Patterns choose which documents count, and the
| help center is a public id.
|
*/

package requests

/*
KnowledgeSourceRequest names a source in the path.
*/
type KnowledgeSourceRequest struct {
	Source string `path:"source"`
}

/*
KnowledgeSourceBody is a source to create or a change to one. Type and
mode are read only on create.
*/
type KnowledgeSourceBody struct {
	Type         string         `json:"type,omitempty"           maxLength:"40"`
	Name         string         `json:"name,omitempty"           maxLength:"80"`
	Mode         string         `json:"mode,omitempty"           enum:"sync,import,"`
	Targets      []string       `json:"targets,omitempty"        maxItems:"3"`
	HelpCenterID *string        `json:"help_center_id,omitempty"`
	Settings     map[string]any `json:"settings,omitempty"`
	Include      []string       `json:"include,omitempty"        maxItems:"50"`
	Exclude      []string       `json:"exclude,omitempty"        maxItems:"50"`
	Schedule     string         `json:"schedule,omitempty"       enum:"manual,hourly,daily,weekly,"`
	SyncNow      bool           `json:"sync_now,omitempty"`
}

/*
StoreKnowledgeSourceRequest is the input of POST /knowledge/sources.
*/
type StoreKnowledgeSourceRequest struct {
	Body KnowledgeSourceBody
}

/*
PreviewKnowledgeSourceRequest is the input of POST
/knowledge/sources/previews: settings to check and count before saving.
*/
type PreviewKnowledgeSourceRequest struct {
	Body KnowledgeSourceBody
}

/*
UpdateKnowledgeSourceRequest is the input of PATCH
/knowledge/sources/{source}.
*/
type UpdateKnowledgeSourceRequest struct {
	Source string `path:"source"`
	Body   KnowledgeSourceBody
}
