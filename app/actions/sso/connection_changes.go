/*
|--------------------------------------------------------------------------
| Connection Changes
|--------------------------------------------------------------------------
|
| What an update changed, for the audit log and for deciding whether the
| connection must be tested again. Settings the identity provider checks
| send it back to draft; how attributes are read and who gets which role
| do not, since a wrong mapping shows up at the next sign-in and harms
| nobody's access. Secrets and certificates are never written out, only
| that they changed.
|
*/

package sso

import (
	"bytes"
	"encoding/json"
	"slices"

	"github.com/dimmihq/dimmi/app/models"
)

/*
sameProtocolSettings reports whether two settings talk to the identity
provider the same way, leaving the attribute map aside.
*/
func sameProtocolSettings(a, b Config) bool {
	a.AttributeMap, b.AttributeMap = AttributeMap{}, AttributeMap{}

	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)

	return bytes.Equal(left, right)
}

/*
fieldChanges returns, per field the update changed, what it was and what
it became, with only the fact of a change for settings and secrets.
*/
func fieldChanges(before, after models.SSOConnection) map[string]any {
	changes := map[string]any{}

	pair := func(field string, from, to any) {
		changes[field] = map[string]any{"from": from, "to": to}
	}

	if before.Name != after.Name {
		pair("name", before.Name, after.Name)
	}

	if before.Status != after.Status {
		pair("status", before.Status, after.Status)
	}

	if before.Enforced != after.Enforced {
		pair("enforced", before.Enforced, after.Enforced)
	}

	if before.JitEnabled != after.JitEnabled {
		pair("jit_enabled", before.JitEnabled, after.JitEnabled)
	}

	if before.DefaultRole != after.DefaultRole {
		pair("default_role", before.DefaultRole, after.DefaultRole)
	}

	if !slices.Equal(before.DefaultTeamIds, after.DefaultTeamIds) {
		changes["default_team_ids"] = map[string]any{"changed": true}
	}

	for field, values := range map[string][2][]byte{
		"config": {before.Config, after.Config}, "group_role_map": {before.GroupRoleMap, after.GroupRoleMap},
		"client_secret": {before.SecretEncrypted, after.SecretEncrypted},
	} {
		if !bytes.Equal(values[0], values[1]) {
			changes[field] = map[string]any{"changed": true}
		}
	}

	return changes
}
