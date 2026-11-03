/*
|--------------------------------------------------------------------------
| Mailed Secret Resources
|--------------------------------------------------------------------------
|
| Answers about secrets sent by mail. Asking for one answers with a
| plain message that reads the same whether or not anything was sent.
| Redeeming an email verification answers with the address proven.
| Confirming who you are answers with how, and until when it holds.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/auth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Message is a plain answer for a person to read.
*/
type Message struct {
	Message string `json:"message"`
}

/*
EmailVerification is a verified address: which account, which address
and when it was first proven.
*/
type EmailVerification struct {
	Object     string    `json:"object"`
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	Status     string    `json:"status"`
	VerifiedAt time.Time `json:"verified_at"`
	UserID     string    `json:"user_id"`
}

/*
NewEmailVerification shapes a verification.
*/
func NewEmailVerification(v auth.Verification) EmailVerification {
	return EmailVerification{
		Object:     "email_verification",
		ID:         models.PublicID(models.PrefixOneTimeToken, v.ID),
		Email:      v.Email,
		Status:     "verified",
		VerifiedAt: v.VerifiedAt.UTC(),
		UserID:     models.PublicID(models.PrefixUser, v.UserID),
	}
}

/*
Confirmation is how the caller last proved who they are, and until when
sensitive steps need no new proof.
*/
type Confirmation struct {
	Object         string    `json:"object"`
	Method         string    `json:"method"`
	ConfirmedUntil time.Time `json:"confirmed_until"`
}

/*
NewConfirmation shapes a confirmation.
*/
func NewConfirmation(c auth.Confirmation) Confirmation {
	return Confirmation{Object: "confirmation", Method: c.Method, ConfirmedUntil: c.Until.UTC()}
}
