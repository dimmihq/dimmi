/*
|--------------------------------------------------------------------------
| Email Suppression Resource
|--------------------------------------------------------------------------
|
| An address the workspace will not mail: why, for which kind of mail,
| and until when when it lapses on its own.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
EmailSuppression is one suppressed address.
*/
type EmailSuppression struct {
	ID        string     `json:"id"`
	Object    string     `json:"object"`
	Email     string     `json:"email"`
	Reason    string     `json:"reason"    enum:"hard_bounce,soft_bounce,complaint,unsubscribed,manual,invalid"`
	Scope     string     `json:"scope"     enum:"all,notifications,changelog,feedback"`
	Source    string     `json:"source"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

/*
NewEmailSuppressions shapes a list of suppressions.
*/
func NewEmailSuppressions(rows []models.EmailSuppression) []EmailSuppression {
	out := make([]EmailSuppression, 0, len(rows))
	for _, r := range rows {
		out = append(out, EmailSuppression{
			ID: models.PublicID(models.PrefixEmailSuppression, r.ID), Object: "email_suppression", Email: r.Email,
			Reason: r.Reason, Scope: r.Scope, Source: r.Source, ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt,
		})
	}

	return out
}
