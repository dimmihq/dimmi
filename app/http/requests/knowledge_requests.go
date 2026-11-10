/*
|--------------------------------------------------------------------------
| Knowledge Settings Requests
|--------------------------------------------------------------------------
|
| A member reads the workspace's embedding model and picks another:
| a provider key, or none for the built-in embedder, with a model and
| its size, each defaulted when left out.
|
*/

package requests

/*
KnowledgeSettingsRequest is the input of GET /knowledge/settings.
*/
type KnowledgeSettingsRequest struct{}

/*
UpdateKnowledgeSettingsRequest is the input of PATCH
/knowledge/settings. EmbeddingKeyID is a provider key's public id,
null or left out for the built-in embedder.
*/
type UpdateKnowledgeSettingsRequest struct {
	Body struct {
		EmbeddingKeyID *string `json:"embedding_key_id,omitempty"`
		Model          string  `json:"model,omitempty"            validate:"max=200"`
		Dims           int32   `json:"dims,omitempty"             validate:"omitempty,min=64,max=4096"`
	}
}
