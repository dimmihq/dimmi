/*
|--------------------------------------------------------------------------
| Two-Factor Reset Resource
|--------------------------------------------------------------------------
|
| A link an admin mailed a teammate to turn their second factor off. The
| admin learns which member it went to and until when it works, never
| the link itself: only the teammate's mailbox holds that.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TwoFactorReset is a reset link: pending once mailed, completed once
used.
*/
type TwoFactorReset struct {
	Object    string    `json:"object"`
	ID        string    `json:"id"`
	MemberID  string    `json:"member_id"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}

/*
NewTwoFactorReset shapes a reset link in the status given.
*/
func NewTwoFactorReset(r members.TwoFactorReset, status string) TwoFactorReset {
	return TwoFactorReset{
		Object: "two_factor_reset", ID: models.PublicID(models.PrefixOneTimeToken, r.ID),
		MemberID: models.PublicID(models.PrefixMember, r.MemberID), Status: status, ExpiresAt: r.ExpiresAt.UTC(),
	}
}
