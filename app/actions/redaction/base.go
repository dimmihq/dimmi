/*
|--------------------------------------------------------------------------
| Redaction Rules
|--------------------------------------------------------------------------
|
| A workspace's own rules change what every later message stores, so
| reading them takes settings:read and changing them settings:write with
| redaction:manage. Old messages are never rewritten: a rule only acts on
| what arrives after it.
|
*/

package redaction

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/redaction"
)

/*
MaxRules is the most rules a workspace keeps.
*/
const MaxRules = 10

/*
canManage reports whether the caller may change the workspace's rules.
*/
func canManage(m authz.Membership) bool {
	return m.Can("settings:write") && m.Can("redaction:manage")
}

/*
base returns the event header for a change the member made.
*/
func base(m authz.Membership, now time.Time) appevents.Base {
	return appevents.Base{
		ID: ids.New(), WorkspaceID: m.Workspace.ID, OccurredAt: now,
		Actor: appevents.Actor{Type: "member", ID: m.Member.ID},
	}
}

/*
checkPattern refuses a pattern that does not compile.
*/
func checkPattern(pattern string) error {
	if _, err := redaction.Compile(pattern); err != nil {
		return exceptions.ErrInvalidRedactionPattern
	}

	return nil
}

/*
find returns the rule, or exceptions.ErrNotFound.
*/
func find(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.RedactionRule, error) {
	rule, err := q.FindRedactionRule(ctx, models.FindRedactionRuleParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return rule, exceptions.ErrNotFound
	}

	if err != nil {
		return rule, fmt.Errorf("find redaction rule: %w", err)
	}

	return rule, nil
}
