/*
|--------------------------------------------------------------------------
| Operator Settings
|--------------------------------------------------------------------------
|
| A workspace that never opened its Operator settings still has them:
| Operator on, nothing spent without a teammate asking, every change
| waiting for a teammate. Policies say per group of tools whether
| changes apply at once, wait for approval or are never offered; article
| and procedure changes always wait, since drafts are still reviewed,
| and so do the workspace's own actions, which cannot be undone.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Policy says what Operator may do with one group of tools.
*/
type Policy string

/*
The policies. Auto applies a change at once, still revertible; ask
waits for a teammate; off never offers the tools.
*/
const (
	PolicyAuto Policy = "auto"
	PolicyAsk  Policy = "ask"
	PolicyOff  Policy = "off"
)

/*
Group names a group of write tools a policy covers. It is a plain
string so a switch may handle only the groups it cares about.
*/
type Group = string

/*
The groups of write tools.
*/
const (
	GroupConversations Group = "conversations"
	GroupNotes         Group = "notes"
	GroupContacts      Group = "contacts"
	GroupArticles      Group = "articles"
	GroupProcedures    Group = "procedures"
	GroupActions       Group = "actions"
)

/*
Groups lists every group in the order settings show them.
*/
func Groups() []Group {
	return []Group{GroupConversations, GroupNotes, GroupContacts, GroupArticles, GroupProcedures, GroupActions}
}

/*
Settings are one workspace's Operator settings. Policies holds only the
groups an admin changed; the rest ask. Model is the model Operator
answers with on the agent's connection; empty means the agent's own.
*/
type Settings struct {
	Enabled         bool
	BriefOnOpen     bool
	TriageOnArrival bool
	Instructions    string
	Policies        map[Group]Policy
	Model           string
}

/*
DefaultSettings are the settings of a workspace that saved none.
*/
func DefaultSettings() Settings {
	return Settings{Enabled: true, Policies: map[Group]Policy{}}
}

/*
PolicyFor returns the policy of a group; groups that may never apply at
once ask even when saved as auto.
*/
func (s Settings) PolicyFor(g Group) Policy {
	p, ok := s.Policies[g]
	if !ok || !ValidPolicy(p) {
		return PolicyAsk
	}
	if p == PolicyAuto && !CanAuto(g) {
		return PolicyAsk
	}
	return p
}

/*
ValidPolicy reports whether p is a known policy.
*/
func ValidPolicy(p Policy) bool {
	return p == PolicyAuto || p == PolicyAsk || p == PolicyOff
}

/*
CanAuto reports whether a group's changes may apply without approval.
*/
func CanAuto(g Group) bool {
	return g != GroupArticles && g != GroupProcedures && g != GroupActions
}

/*
ValidGroup reports whether g names a group.
*/
func ValidGroup(g Group) bool {
	return slices.Contains(Groups(), g)
}

/*
Offers reports whether a connection can call a model: it lists the
model, or lists none and takes any.
*/
func Offers(key models.AiProviderKey, model string) bool {
	var listed []string
	_ = json.Unmarshal(key.Models, &listed)
	return len(listed) == 0 || slices.Contains(listed, model)
}

/*
LoadSettings reads the workspace's settings, the defaults when it saved
none.
*/
func LoadSettings(ctx context.Context, q *models.Queries, workspaceID ids.UUID) (Settings, error) {
	row, err := q.FindOperatorSettings(ctx, workspaceID)
	if errors.Is(err, database.ErrNotFound) {
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("find operator settings: %w", err)
	}
	return SettingsOf(row), nil
}

/*
SettingsOf reads a stored settings row.
*/
func SettingsOf(row models.OperatorSetting) Settings {
	s := Settings{
		Enabled: row.Enabled, BriefOnOpen: row.BriefOnOpen, TriageOnArrival: row.TriageOnArrival,
		Instructions: row.Instructions, Policies: map[Group]Policy{}, Model: deref(row.Model),
	}
	_ = json.Unmarshal(row.Policies, &s.Policies)
	return s
}
