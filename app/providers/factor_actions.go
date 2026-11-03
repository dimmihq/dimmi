/*
|--------------------------------------------------------------------------
| Second Factor Actions
|--------------------------------------------------------------------------
|
| The actions behind two-factor authentication: answering the challenge
| at sign-in, and a user managing their own factors. They are built
| after the sign-in actions, since answering the challenge ends in
| starting a session like every other sign-in.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/auth"
	"github.com/dimmihq/dimmi/app/actions/factors"
)

/*
bootFactors builds the actions that answer the second-factor challenge
and manage auth factors.
*/
func bootFactors(a *app.App) {
	acts := a.Actions
	cfg := a.Config.Auth.Factors

	acts.CompleteTwoFactorChallenge = &auth.CompleteTwoFactorChallenge{
		DB: a.DB, Clock: a.Clock, Mail: a.Mail, Limiter: a.Limiter, Links: a.Links, Lockout: a.Lockout,
		Tokens: a.Secrets, Factors: a.Factors, SecurityLog: a.SecurityLog, StartSession: acts.StartSession,
		LoadMe: acts.LoadMe,
	}

	acts.DisableTwoFactor = &factors.DisableTwoFactor{
		Clock: a.Clock, Mail: a.Mail, Links: a.Links, AppGrants: a.AppGrants, SecurityLog: a.SecurityLog,
	}
	acts.RegenerateRecoveryCodes = &factors.RegenerateRecoveryCodes{
		DB: a.DB, Clock: a.Clock, Factors: a.Factors, SecurityLog: a.SecurityLog,
	}
	acts.ListAuthFactors = &factors.ListAuthFactors{DB: a.DB, Clock: a.Clock, Factors: a.Factors, Config: cfg}
	acts.AddAuthFactor = &factors.AddAuthFactor{
		DB: a.DB, Clock: a.Clock, Limiter: a.Limiter, Factors: a.Factors, Config: cfg,
		Regenerate: acts.RegenerateRecoveryCodes,
	}
	acts.ActivateAuthFactor = &factors.ActivateAuthFactor{
		DB: a.DB, Clock: a.Clock, Mail: a.Mail, Limiter: a.Limiter, Links: a.Links, Factors: a.Factors,
		Config: cfg, SecurityLog: a.SecurityLog,
	}
	acts.UpdateAuthFactor = &factors.UpdateAuthFactor{DB: a.DB, Clock: a.Clock, SecurityLog: a.SecurityLog}
	acts.RemoveAuthFactor = &factors.RemoveAuthFactor{
		DB: a.DB, Clock: a.Clock, Factors: a.Factors, Disable: acts.DisableTwoFactor, SecurityLog: a.SecurityLog,
	}
	acts.RemoveAllAuthFactors = &factors.RemoveAllAuthFactors{DB: a.DB, Disable: acts.DisableTwoFactor}
}
