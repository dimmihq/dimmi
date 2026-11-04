/*
|--------------------------------------------------------------------------
| Workspace Behavior
|--------------------------------------------------------------------------
|
| Security settings of a workspace, read from and written to its settings
| document in one place, so the policy that enforces a setting and the
| endpoint that changes it always agree on where it lives. Settings nest
| by area, such as {"security": {"require_two_factor": true}}, and every
| key a change does not touch is kept as it was.
|
*/

package models

import "encoding/json"

/*
RequiresTwoFactor reports whether every member must protect their
account with a second factor to work in the workspace. Settings that do
not read as a document require nothing.
*/
func (w Workspace) RequiresTwoFactor() bool {
	return w.Security().RequireTwoFactor
}

/*
WithTwoFactorRequired returns the settings document with the
requirement of a second factor set to on, keeping every other key.
*/
func WithTwoFactorRequired(settings json.RawMessage, on bool) (json.RawMessage, error) {
	return withSecurityKeys(settings, map[string]any{"require_two_factor": on})
}
