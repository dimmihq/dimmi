/*
|--------------------------------------------------------------------------
| Team Resources
|--------------------------------------------------------------------------
|
| A team as the members of its workspace see it: its name and how many
| members it has, with the members themselves only when the request
| includes them. A member's teams appear on the member as summaries, and
| adding members to a team answers with one result per member asked
| for.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/teams"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Team is one team of a workspace. Members is present only when the
request included the members.
*/
type Team struct {
	ID          string             `json:"id"`
	Object      string             `json:"object"`
	Name        string             `json:"name"`
	MemberCount int64              `json:"member_count"`
	Members     *[]WorkspaceMember `json:"members,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

/*
TeamSummary names a team a member belongs to.
*/
type TeamSummary struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Name   string `json:"name"`
}

/*
TeamMemberResult is what adding one member to a team came to: added,
already_member, inactive or not_found. MemberID echoes the id as the
request sent it.
*/
type TeamMemberResult struct {
	MemberID string `json:"member_id"`
	Result   string `json:"result"`
}

/*
NewTeam shapes a team, with its members when they were loaded.
*/
func NewTeam(t teams.Team) Team {
	out := Team{
		ID:          models.PublicID(models.PrefixTeam, t.Team.ID),
		Object:      "team",
		Name:        t.Team.Name,
		MemberCount: t.MemberCount,
		CreatedAt:   t.Team.CreatedAt,
		UpdatedAt:   t.Team.UpdatedAt,
	}

	if t.Members == nil {
		return out
	}

	members := make([]WorkspaceMember, 0, len(t.Members))
	for _, row := range t.Members {
		members = append(members, NewWorkspaceMember(row.WorkspaceMember, row.User))
	}

	out.Members = &members

	return out
}

/*
NewTeams shapes a list of teams, in the order given.
*/
func NewTeams(list []teams.Team) []Team {
	out := make([]Team, 0, len(list))
	for _, t := range list {
		out = append(out, NewTeam(t))
	}

	return out
}

/*
NewTeamSummaries shapes the teams a member belongs to, an empty list for
none.
*/
func NewTeamSummaries(list []models.Team) []TeamSummary {
	out := make([]TeamSummary, 0, len(list))
	for _, t := range list {
		out = append(out, TeamSummary{ID: models.PublicID(models.PrefixTeam, t.ID), Object: "team", Name: t.Name})
	}

	return out
}
