/*
|--------------------------------------------------------------------------
| Agent Action Connection Requests
|--------------------------------------------------------------------------
|
| The inputs of the action connection endpoints. A secret is only ever
| written; fields left out stay as they are.
|
*/

package requests

import (
	"github.com/dimmihq/dimmi/app/ai/actionkit"
)

/*
AIActionConnectionRequest names a connection in the path.
*/
type AIActionConnectionRequest struct {
	Connection string `path:"connection"`
}

/*
AIDestroyActionConnectionRequest is the input of DELETE
/ai/action-connections/{connection}; force also deletes the actions
that use it.
*/
type AIDestroyActionConnectionRequest struct {
	Connection string `path:"connection"`
	Force      bool   `query:"force"`
}

/*
AIActionConnectionBody is a connection's fields. Token, Username and
Password are its secret and are never returned.
*/
type AIActionConnectionBody struct {
	Name           *string                   `json:"name,omitempty"            validate:"omitempty,max=60"`
	Kind           string                    `json:"kind,omitempty"            validate:"omitempty,oneof=http mcp"`
	BaseURL        *string                   `json:"base_url,omitempty"        validate:"omitempty,max=512"`
	AuthType       *string                   `json:"auth_type,omitempty"       validate:"omitempty,max=40"`
	AuthConfig     *actionkit.ConnectionAuth `json:"auth_config,omitempty"`
	Token          *string                   `json:"token,omitempty"           validate:"omitempty,max=4096"`
	Username       *string                   `json:"username,omitempty"        validate:"omitempty,max=256"`
	Password       *string                   `json:"password,omitempty"        validate:"omitempty,max=4096"`
	DefaultHeaders *map[string]string        `json:"default_headers,omitempty"`
	Variables      *map[string]string        `json:"variables,omitempty"`
	TimeoutMS      *int32                    `json:"timeout_ms,omitempty"`
	Status         *string                   `json:"status,omitempty"          validate:"omitempty,max=16"`
}

/*
AIStoreActionConnectionRequest is the input of POST
/ai/action-connections.
*/
type AIStoreActionConnectionRequest struct {
	Body AIActionConnectionBody
}

/*
AIUpdateActionConnectionRequest is the input of PATCH
/ai/action-connections/{connection}.
*/
type AIUpdateActionConnectionRequest struct {
	Connection string `path:"connection"`
	Body       AIActionConnectionBody
}
