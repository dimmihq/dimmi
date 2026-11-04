/*
|--------------------------------------------------------------------------
| Account Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on a person's own account. Every field of a
| profile update is optional and a missing one stays as it is; a language
| or time zone sent as null goes back to following the workspace.
|
*/

package requests

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
UpdateUserRequest is the input of PATCH /user. RawBody tells a field sent
as null from one not sent.
*/
type UpdateUserRequest struct {
	Body struct {
		Name     *string `json:"name,omitempty"     validate:"omitempty,max=120"`
		Locale   *string `json:"locale,omitempty"   validate:"omitempty,locale"`
		Timezone *string `json:"timezone,omitempty" validate:"omitempty,timezone"`
	}
	RawBody []byte
}

/*
Validate refuses a name made only of invisible characters, which would
be empty once cleaned.
*/
func (r *UpdateUserRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if r.Body.Name != nil && str.Clean(*r.Body.Name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	return nil
}

/*
UpdateUserPasswordRequest is the input of PATCH /user/password. A person
without a password leaves CurrentPassword out.
*/
type UpdateUserPasswordRequest struct {
	Body struct {
		CurrentPassword string `json:"current_password,omitempty" validate:"omitempty,max=256"`
		Password        string `json:"password,omitempty"         validate:"required,max=256,password_length,password"`
	}
}

/*
UpdateUserAvatarRequest is the input of PUT /user/avatar: a multipart form
with the picture in the avatar field.
*/
type UpdateUserAvatarRequest struct {
	RawBody fhttp.MultipartForm
}

/*
DestroyUserRequest is the input of DELETE /user: the address typed to
confirm.
*/
type DestroyUserRequest struct {
	Body struct {
		ConfirmEmail string `json:"confirm_email,omitempty" validate:"required,max=254"`
	}
}
