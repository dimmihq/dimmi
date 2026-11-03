/*
|--------------------------------------------------------------------------
| OAuth Identity Resource
|--------------------------------------------------------------------------
|
| A provider account linked to the caller, as the security settings show
| it: which provider, which account there, and when it last signed in.
| The provider's own id for the account stays on the server.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
OAuthIdentity is a linked provider account. Email, Username and
AvatarURL are what the provider last reported; LastLoginAt is null
until the account was used to sign in.
*/
type OAuthIdentity struct {
	ID          string     `json:"id"`
	Object      string     `json:"object"`
	Provider    string     `json:"provider"`
	Email       *string    `json:"email"`
	Username    *string    `json:"username"`
	AvatarURL   *string    `json:"avatar_url"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

/*
NewOAuthIdentities shapes the linked provider accounts, in the order
given.
*/
func NewOAuthIdentities(identities []models.OauthIdentity) []OAuthIdentity {
	out := make([]OAuthIdentity, 0, len(identities))

	for _, i := range identities {
		var lastLogin *time.Time
		if i.LastLoginAt != nil {
			at := i.LastLoginAt.UTC()
			lastLogin = &at
		}

		out = append(out, OAuthIdentity{
			ID:          models.PublicID(models.PrefixOAuthIdentity, i.ID),
			Object:      "oauth_identity",
			Provider:    i.Provider,
			Email:       i.Email,
			Username:    i.Username,
			AvatarURL:   i.AvatarUrl,
			LastLoginAt: lastLogin,
			CreatedAt:   i.CreatedAt.UTC(),
		})
	}

	return out
}
