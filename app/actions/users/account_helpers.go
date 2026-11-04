/*
|--------------------------------------------------------------------------
| Account Helpers
|--------------------------------------------------------------------------
|
| What changes to a person's own account share: the workspaces to tell
| when teammates would see a change, and how an address is written where
| someone other than its owner might read it.
|
*/

package users

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
membershipsOf returns the workspaces the user works in, for telling their
teammates about a change.
*/
func membershipsOf(ctx context.Context, q *models.Queries, userID ids.UUID) ([]appevents.UserMembership, error) {
	rows, err := q.ListUserWorkspaces(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces of user: %w", err)
	}

	out := make([]appevents.UserMembership, 0, len(rows))
	for _, row := range rows {
		out = append(out, appevents.UserMembership{WorkspaceID: row.WorkspaceID, MemberID: row.MemberID})
	}

	return out, nil
}

/*
maskEmail keeps the first letter and the domain of an address, enough for
its owner to recognize it and for nobody else to learn it.
*/
func maskEmail(email string) string {
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" {
		return "***"
	}

	return string([]rune(local)[0]) + "***@" + domain
}
