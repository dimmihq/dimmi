/*
|--------------------------------------------------------------------------
| Session Resource
|--------------------------------------------------------------------------
|
| A browser the user is signed in on, as they see it in their list: how
| it signed in, where it was last used from and when it ends. The device
| id that ties sessions together stays on the server.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/auth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Session is one of the user's sessions. Client, Platform and AppVersion
describe the user's own apps and are null for browsers.
*/
type Session struct {
	ID              string    `json:"id"`
	Object          string    `json:"object"`
	Kind            string    `json:"kind"`
	Current         bool      `json:"current"`
	AuthMethod      string    `json:"auth_method"`
	AuthMethods     []string  `json:"auth_methods"`
	TwoFactorUsed   bool      `json:"two_factor_used"`
	AuthenticatedAt time.Time `json:"authenticated_at"`
	DeviceLabel     *string   `json:"device_label"`
	Client          *string   `json:"client"`
	Platform        *string   `json:"platform"`
	AppVersion      *string   `json:"app_version"`
	IP              *string   `json:"ip"`
	Country         *string   `json:"country"`
	LastActivityAt  time.Time `json:"last_activity_at"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at"`
}

/*
NewSessions shapes the user's browser sessions, in the order given.
*/
func NewSessions(sessions []auth.BrowserSession) []Session {
	out := make([]Session, 0, len(sessions))

	for _, s := range sessions {
		r := s.Record
		out = append(out, Session{
			ID:              models.PublicID(models.PrefixSession, r.ID),
			Object:          "session",
			Kind:            "browser",
			Current:         s.Current,
			AuthMethod:      s.SignIn.Method,
			AuthMethods:     nonNil(s.SignIn.Methods),
			TwoFactorUsed:   s.SignIn.UsedSecondFactor(),
			AuthenticatedAt: r.AuthenticatedAt.UTC(),
			DeviceLabel:     optional(s.SignIn.DeviceLabel),
			IP:              optional(r.IP),
			Country:         optional(r.Country),
			LastActivityAt:  r.LastActivityAt.UTC(),
			CreatedAt:       r.CreatedAt.UTC(),
			ExpiresAt:       r.ExpiresAt.UTC(),
		})
	}

	return out
}

func optional(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
