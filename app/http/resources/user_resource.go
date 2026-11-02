/*
|--------------------------------------------------------------------------
| User Resource
|--------------------------------------------------------------------------
|
| A user as they see themselves. It never carries the password hash, the
| risk flags or any secret; has_password and two_factor_enabled are all a
| client needs to offer the right settings.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
defaultLocale is the locale shown for a user who never picked one.
*/
const defaultLocale = "en"

/*
User is a user's own profile.
*/
type User struct {
	ID               string    `json:"id"`
	Object           string    `json:"object"`
	Name             string    `json:"name"`
	Email            string    `json:"email"`
	EmailVerified    bool      `json:"email_verified"`
	TwoFactorEnabled bool      `json:"two_factor_enabled"`
	AvatarURL        *string   `json:"avatar_url"`
	Locale           string    `json:"locale"`
	Timezone         *string   `json:"timezone"`
	HasPassword      bool      `json:"has_password"`
	CreatedAt        time.Time `json:"created_at"`
}

/*
NewUser shapes the user.
*/
func NewUser(u models.User) User {
	locale := defaultLocale
	if u.Locale != nil {
		locale = *u.Locale
	}

	return User{
		ID:               models.PublicID(models.PrefixUser, u.ID),
		Object:           "user",
		Name:             u.Name,
		Email:            u.Email,
		EmailVerified:    u.IsVerified(),
		TwoFactorEnabled: u.TwoFactorEnabled(),
		Locale:           locale,
		Timezone:         u.Timezone,
		HasPassword:      u.HasPassword(),
		CreatedAt:        u.CreatedAt.UTC(),
	}
}
