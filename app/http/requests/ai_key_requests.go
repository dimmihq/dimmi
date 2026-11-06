/*
|--------------------------------------------------------------------------
| AI Key Requests
|--------------------------------------------------------------------------
|
| A provider connection is created once, with its secret, and changed
| field by field afterwards; a secret is only ever written, never read.
|
*/

package requests

/*
AIKeyRequest names a connection in the path.
*/
type AIKeyRequest struct {
	Key string `path:"key"`
}

/*
AIStoreKeyRequest is the input of POST /ai/keys.
*/
type AIStoreKeyRequest struct {
	Body struct {
		Name     string         `json:"name"               validate:"required,max=60"`
		Provider string         `json:"provider"           validate:"required,max=40"`
		BaseURL  string         `json:"base_url,omitempty" validate:"omitempty,url,max=255"`
		APIKey   string         `json:"api_key,omitempty"  validate:"omitempty,max=4096"`
		Config   map[string]any `json:"config,omitempty"`
	}
}

/*
AIUpdateKeyRequest is the input of PATCH /ai/keys/{key}.
*/
type AIUpdateKeyRequest struct {
	Key  string `path:"key"`
	Body struct {
		Name    *string        `json:"name,omitempty"     validate:"omitempty,min=1,max=60"`
		BaseURL *string        `json:"base_url,omitempty" validate:"omitempty,url,max=255"`
		APIKey  *string        `json:"api_key,omitempty"  validate:"omitempty,max=4096"`
		Config  map[string]any `json:"config,omitempty"`
	}
}
