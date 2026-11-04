/*
|--------------------------------------------------------------------------
| Away Reassign
|--------------------------------------------------------------------------
|
| A member who is away and asked for their conversations to go back
| should not leave a customer waiting on them. When the customer writes
| again, the conversation leaves the member and stays with its team, or
| becomes unassigned, so whoever is working picks it up. Only a person's
| own message moves it: automatic mail does not, and the member coming
| back later does not bring anything back.
|
*/

package conversations

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ReasonAwayReassign is why a conversation left an away member.
*/
const ReasonAwayReassign = "away_reassign"

/*
handBack takes the conversation from its member when the member is away
and asked for their conversations to go back, and reports whether it
did. The team keeps it.
*/
func (w *Writer) handBack(ctx context.Context, c models.Conversation) (models.Conversation, bool, error) {
	if c.AssigneeMemberID == nil || c.Spam {
		return c, false, nil
	}

	m, err := w.Q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
		WorkspaceID: c.WorkspaceID, ID: *c.AssigneeMemberID,
	})
	if err != nil {
		return c, false, fmt.Errorf("find assignee: %w", err)
	}

	if !m.AwayReassign || m.Availability != string(enums.AvailabilityAway) {
		return c, false, nil
	}

	return w.AssignFor(ctx, c, Assignment{Team: c.AssigneeTeamID}, ReasonAwayReassign)
}
