/*
|--------------------------------------------------------------------------
| Two-Factor Challenge Resource
|--------------------------------------------------------------------------
|
| The answer to a first sign-in step that needs a second one: which
| factors may answer, the preferred first, and until when. The secret
| that ties the browser to the challenge travels in a cookie scripts
| cannot read, never in this body. Each method is an object, so a later
| kind of factor can carry what its prompt needs in its options.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/auth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TwoFactorChallenge is a sign-in waiting on its second factor.
*/
type TwoFactorChallenge struct {
	Object    string            `json:"object"`
	ExpiresAt time.Time         `json:"expires_at"`
	Methods   []ChallengeMethod `json:"methods"`
}

/*
ChallengeMethod is one factor that may answer: its kind, the factor,
its name, whether it is the preferred one, and the options its prompt
needs, null for codes.
*/
type ChallengeMethod struct {
	Method    string  `json:"method"`
	FactorID  string  `json:"factor_id"`
	Name      string  `json:"name"`
	Preferred bool    `json:"preferred"`
	Options   *string `json:"options"`
}

/*
NewTwoFactorChallenge shapes the challenge.
*/
func NewTwoFactorChallenge(c auth.TwoFactorChallenge) TwoFactorChallenge {
	methods := make([]ChallengeMethod, 0, len(c.Methods))
	for _, m := range c.Methods {
		methods = append(methods, ChallengeMethod{
			Method: m.Method, FactorID: models.PublicID(models.PrefixAuthFactor, m.FactorID), Name: m.Name,
			Preferred: m.Preferred,
		})
	}

	return TwoFactorChallenge{Object: "two_factor_challenge", ExpiresAt: c.ExpiresAt.UTC(), Methods: methods}
}
