/*
|--------------------------------------------------------------------------
| Mentionables
|--------------------------------------------------------------------------
|
| Who a note on a conversation may mention: active teammates and, unless
| the workspace turned team mentions off, teams, at most ten of each
| whose name has a word starting with what was typed. Only names, avatars
| and availability show, so mentioning works with conversation access
| alone. Each teammate says whether they may see the conversation, so the
| writer is warned before mentioning someone who would not find it.
|
*/

package conversations

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxMentionables is the most teammates, and the most teams, listed.
*/
const maxMentionables = 10

/*
MentionablesInput is who asks, about which conversation, with what they
typed.
*/
type MentionablesInput struct {
	Membership authz.Membership
	Ref        Ref
	Query      string
}

/*
Mentionables are the teammates and teams a note may mention.
*/
type Mentionables struct {
	Members []models.ListMentionableMembersRow
	Teams   []models.ListMentionableTeamsRow
}

/*
ListMentionables returns who a note on the conversation may mention.

Rules:
  - The caller needs conversations:read and sees the conversation.
  - Teams are left out while the workspace has team mentions off.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ListMentionables struct {
	DB *database.DB
}

/*
Handle returns the mentionables.
*/
func (a *ListMentionables) Handle(ctx context.Context, in MentionablesInput) (Mentionables, error) {
	if err := CanRead(in.Membership); err != nil {
		return Mentionables{}, err
	}

	out := Mentionables{Members: []models.ListMentionableMembersRow{}, Teams: []models.ListMentionableTeamsRow{}}
	query := strings.TrimSpace(in.Query)

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := Find(ctx, q, in.Membership, in.Ref)
		if err != nil {
			return err
		}

		if out.Members, err = q.ListMentionableMembers(ctx, models.ListMentionableMembersParams{
			AssigneeMemberID: c.AssigneeMemberID, AssigneeTeamID: c.AssigneeTeamID, WorkspaceID: c.WorkspaceID,
			Query: query, RowLimit: maxMentionables,
		}); err != nil {
			return fmt.Errorf("list mentionable members: %w", err)
		}

		if !in.Membership.Workspace.TeamMentionsOn() {
			return nil
		}

		if out.Teams, err = q.ListMentionableTeams(ctx, models.ListMentionableTeamsParams{
			WorkspaceID: c.WorkspaceID, Query: query, RowLimit: maxMentionables,
		}); err != nil {
			return fmt.Errorf("list mentionable teams: %w", err)
		}

		return nil
	})

	return out, err
}
