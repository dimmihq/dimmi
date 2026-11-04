/*
|--------------------------------------------------------------------------
| Contact Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on contacts. Every field of an update is
| optional and a missing one stays as it is; a field sent as null is
| cleared, which is why updates also keep the raw body, to tell null
| from missing.
|
*/

package requests

import "encoding/json"

/*
ShowContactRequest is the input of GET /contacts/{contact}.
*/
type ShowContactRequest struct {
	Contact string `path:"contact"`
}

/*
ContactBody is a contact's details as a request sends them.
*/
type ContactBody struct {
	ExternalID       *string        `json:"external_id,omitempty"       validate:"omitempty,min=1,max=255"`
	Email            *string        `json:"email,omitempty"             validate:"omitempty,email,max=320"`
	Name             *string        `json:"name,omitempty"              validate:"omitempty,max=255"`
	Phone            *string        `json:"phone,omitempty"             validate:"omitempty,max=40"`
	CustomAttributes map[string]any `json:"custom_attributes,omitempty"`
}

/*
StoreContactRequest is the input of POST /contacts.
*/
type StoreContactRequest struct {
	Body ContactBody
}

/*
UpdateContactRequest is the input of PATCH /contacts/{contact}.
*/
type UpdateContactRequest struct {
	Contact string `path:"contact"`
	Body    struct {
		ContactBody

		Blocked *bool `json:"blocked,omitempty"`
	}
	RawBody []byte
}

/*
Nulls returns the fields the raw body sends as null.
*/
func Nulls(raw []byte) map[string]bool {
	fields := map[string]json.RawMessage{}
	_ = json.Unmarshal(raw, &fields)

	out := map[string]bool{}
	for key, value := range fields {
		out[key] = string(value) == "null"
	}

	return out
}

/*
Sent returns the fields the raw body sends, null or not.
*/
func Sent(raw []byte) map[string]bool {
	fields := map[string]json.RawMessage{}
	_ = json.Unmarshal(raw, &fields)

	out := map[string]bool{}
	for key := range fields {
		out[key] = true
	}

	return out
}
