/*
|--------------------------------------------------------------------------
| User Avatar Controller
|--------------------------------------------------------------------------
|
| PUT /user/avatar replaces the caller's picture from a multipart upload,
| and DELETE /user/avatar takes it down.
|
*/

package v1

import (
	"context"
	"fmt"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/users"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
UserAvatarController serves the caller's picture.
*/
type UserAvatarController struct {
	UpdateAvatar *users.UpdateAvatar
	DeleteAvatar *users.DeleteAvatar
}

/*
Update stores the picture and answers with the profile as it now is.
*/
func (c *UserAvatarController) Update(
	ctx context.Context, in *requests.UpdateUserAvatarRequest,
) (*UserOutput, error) {
	defer func() { _ = in.RawBody.RemoveAll() }()

	uploads := in.RawBody.File["avatar"]
	if len(uploads) == 0 {
		return nil, exceptions.ErrInvalidAvatar
	}

	file, err := uploads[0].Open()
	if err != nil {
		return nil, fmt.Errorf("open uploaded avatar: %w", err)
	}

	defer func() { _ = file.Close() }()

	user, err := c.UpdateAvatar.Handle(ctx, caller(ctx).ID(), file)
	if err != nil {
		return nil, err
	}

	return &UserOutput{Body: fhttp.NewResource(resources.NewUser(user))}, nil
}

/*
Destroy takes the picture down.
*/
func (c *UserAvatarController) Destroy(ctx context.Context, _ *requests.NoInput) (*EmptyOutput, error) {
	if err := c.DeleteAvatar.Handle(ctx, caller(ctx).ID()); err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}
