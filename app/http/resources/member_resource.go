/*
|--------------------------------------------------------------------------
| Member Resources
|--------------------------------------------------------------------------
|
| A member as their teammates see them: their role and status in the
| workspace and who they are, by name and email, never anything about
| how they sign in, plus which conversations they see, which matters to
| whoever manages access. Those who manage members also see whether each
| one has a second factor and when they last used the workspace, so they
| can chase the stragglers. The caller's own membership, with the
| permissions they hold, is the separate member block of the me
| resource.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
WorkspaceMember is one member of a workspace, with their account. Teams
is present only when the request included the member's teams.
SignInLocked says whether failed guesses locked the member's account,
TwoFactorEnabled whether the account has a second factor, and
LastActiveAt when the member last used the workspace; all three are
null for callers who do not manage members.
*/
type WorkspaceMember struct {
	ID           string  `json:"id"`
	Object       string  `json:"object"`
	Role         string  `json:"role"`
	Status       string  `json:"status"`
	Availability string  `json:"availability"`
	JoinedVia    *string `json:"joined_via"`

	ConversationAccess           string   `json:"conversation_access"`
	ConversationAccessUnassigned bool     `json:"conversation_access_unassigned"`
	AccessTeamIDs                []string `json:"access_team_ids"`

	MemberAvailability

	User          Teammate       `json:"user"`
	Teams         *[]TeamSummary `json:"teams,omitempty"`
	DeactivatedAt *time.Time     `json:"deactivated_at"`
	CreatedAt     time.Time      `json:"created_at"`
	SignInLocked  *bool          `json:"sign_in_locked"`

	TwoFactorEnabled *bool      `json:"two_factor_enabled"`
	LastActiveAt     *time.Time `json:"last_active_at"`
}

/*
Teammate is the account behind a member, as other members see it.
*/
type Teammate struct {
	ID            string  `json:"id"`
	Object        string  `json:"object"`
	Name          string  `json:"name"`
	Email         string  `json:"email"`
	EmailVerified bool    `json:"email_verified"`
	AvatarURL     *string `json:"avatar_url"`
}

/*
NewWorkspaceMember shapes the member and their account.
*/
func NewWorkspaceMember(m models.WorkspaceMember, u models.User) WorkspaceMember {
	return WorkspaceMember{
		ID:           models.PublicID(models.PrefixMember, m.ID),
		Object:       "member",
		Role:         m.Role,
		Status:       m.Status,
		Availability: m.Availability,
		JoinedVia:    m.JoinedVia,

		ConversationAccess:           m.ConversationAccess,
		ConversationAccessUnassigned: m.ConversationAccessUnassigned,
		AccessTeamIDs:                []string{},

		MemberAvailability: NewMemberAvailability(m),

		User: Teammate{
			ID:            models.PublicID(models.PrefixUser, u.ID),
			Object:        "user",
			Name:          u.Name,
			Email:         u.Email,
			EmailVerified: u.IsVerified(),
			AvatarURL:     u.AvatarURL(),
		},
		DeactivatedAt:    m.DeactivatedAt,
		CreatedAt:        m.CreatedAt,
		TwoFactorEnabled: new(u.TwoFactorConfirmedAt != nil),
		LastActiveAt:     m.LastAccessedAt,
	}
}

/*
PublicTeamIDs returns the public ids of the teams, an empty list for
none.
*/
func PublicTeamIDs(teamIDs []ids.UUID) []string {
	out := make([]string, len(teamIDs))
	for i, id := range teamIDs {
		out[i] = models.PublicID(models.PrefixTeam, id)
	}

	return out
}

/*
ForTeammates returns the member without what only those who manage
members may see.
*/
func (w WorkspaceMember) ForTeammates() WorkspaceMember {
	w.SignInLocked, w.TwoFactorEnabled, w.LastActiveAt = nil, nil, nil

	return w
}

/*
NewWorkspaceMembers shapes a list of members, in the order given.
*/
func NewWorkspaceMembers(rows []models.ListWorkspaceMembersRow) []WorkspaceMember {
	out := make([]WorkspaceMember, 0, len(rows))
	for _, row := range rows {
		out = append(out, NewWorkspaceMember(row.WorkspaceMember, row.User))
	}

	return out
}

/*
MemberAvailability is how available a member is for new conversations:
whether customers' answers go back to the pool while they are away,
since when they are away, their own rule for taking conversations by
replying, and the most open conversations they hold, overall and per
channel type, null for no limit.
*/
type MemberAvailability struct {
	AwayReassign      bool           `json:"away_reassign"`
	AwaySince         *time.Time     `json:"away_since"`
	AssignOnReply     string         `json:"assign_on_reply"`
	Capacity          *int32         `json:"conversation_capacity"`
	CapacityByChannel map[string]int `json:"capacity_by_channel"`
}

/*
NewMemberAvailability shapes the member's availability.
*/
func NewMemberAvailability(m models.WorkspaceMember) MemberAvailability {
	byChannel := map[string]int{}
	_ = json.Unmarshal(m.CapacityByChannel, &byChannel)

	return MemberAvailability{
		AwayReassign: m.AwayReassign, AwaySince: m.AwaySince, AssignOnReply: m.AssignOnReply,
		Capacity: m.ConversationCapacity, CapacityByChannel: byChannel,
	}
}
