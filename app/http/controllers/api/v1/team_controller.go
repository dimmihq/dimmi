/*
|--------------------------------------------------------------------------
| Team Controller
|--------------------------------------------------------------------------
|
| GET /teams lists the live teams of the workspace in play a page at a
| time, by name, and GET /teams/{team} shows one; include=members adds
| who is in each. POST /teams creates a team, PATCH /teams/{team}
| renames it and DELETE /teams/{team} deletes it. Ids of deleted teams
| and of other workspaces' teams are simply not found.
|
*/

package v1

import (
	"context"
	"slices"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/teams"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TeamsOutput is a page of teams.
*/
type TeamsOutput struct {
	Body fhttp.Collection[resources.Team]
}

/*
TeamOutput is one team.
*/
type TeamOutput struct {
	Body fhttp.Resource[resources.Team]
}

/*
TeamController serves the teams of the workspace in play.
*/
type TeamController struct {
	ListTeams  *teams.ListTeams
	ShowTeam   *teams.ShowTeam
	CreateTeam *teams.CreateTeam
	UpdateTeam *teams.UpdateTeam
	DeleteTeam *teams.DeleteTeam
	IDs        *ids.Registry
	Cursors    *pagination.Cursors
}

/*
teamPage is where a page of teams starts, carried in its cursor.
*/
type teamPage struct {
	Offset int `json:"o"`
}

/*
TeamListing is what the team list accepts: its members embedded with
include=members.
*/
func TeamListing() pagination.Allow {
	return pagination.Allow{Includes: []string{"members"}}
}

/*
Index lists a page of teams by name.
*/
func (c *TeamController) Index(ctx context.Context, _ *requests.NoInput) (*TeamsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), TeamListing())
	if err != nil {
		return nil, err
	}

	in := teams.ListTeamsInput{
		Membership: membership(ctx), WithMembers: slices.Contains(query.Includes, "members"), Limit: query.Limit,
	}

	if query.Cursor != "" {
		var start teamPage
		if err := c.Cursors.Decode(query.Cursor, &start); err != nil {
			return nil, err
		}

		in.Offset = start.Offset
	}

	page, err := c.ListTeams.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.Team]{Items: resources.NewTeams(page.Teams), Limit: in.Limit}
	if page.More {
		if out.NextCursor, err = c.Cursors.Encode(teamPage{Offset: in.Offset + in.Limit}); err != nil {
			return nil, err
		}
	}

	return &TeamsOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
TeamShowing is what one team accepts: the relations it can embed, its
members and, for those who may delete it, how many conversations it
holds.
*/
func TeamShowing() pagination.Allow {
	return pagination.Allow{Includes: []string{"members", "conversation_counts"}}
}

/*
Show answers with one team.
*/
func (c *TeamController) Show(ctx context.Context, in *requests.ShowTeamRequest) (*TeamOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), TeamShowing())
	if err != nil {
		return nil, err
	}

	id, err := teamID(c.IDs, in.Team)
	if err != nil {
		return nil, err
	}

	team, err := c.ShowTeam.Handle(ctx, teams.ShowTeamInput{
		Membership: membership(ctx), TeamID: id, WithMembers: slices.Contains(query.Includes, "members"),
		WithConversationCounts: slices.Contains(query.Includes, "conversation_counts"),
	})
	if err != nil {
		return nil, err
	}

	return &TeamOutput{Body: fhttp.NewResource(resources.NewTeam(team))}, nil
}

/*
Store creates a team and answers with it.
*/
func (c *TeamController) Store(ctx context.Context, in *requests.StoreTeamRequest) (*TeamOutput, error) {
	team, err := c.CreateTeam.Handle(ctx, teams.CreateTeamInput{Membership: membership(ctx), Name: in.Body.Name})
	if err != nil {
		return nil, err
	}

	return &TeamOutput{Body: fhttp.NewResource(resources.NewTeam(team))}, nil
}

/*
Update renames a team and answers with it as it now is.
*/
func (c *TeamController) Update(ctx context.Context, in *requests.UpdateTeamRequest) (*TeamOutput, error) {
	id, err := teamID(c.IDs, in.Team)
	if err != nil {
		return nil, err
	}

	team, err := c.UpdateTeam.Handle(ctx, teams.UpdateTeamInput{
		Membership: membership(ctx), TeamID: id, Name: in.Body.Name,
	})
	if err != nil {
		return nil, err
	}

	return &TeamOutput{Body: fhttp.NewResource(resources.NewTeam(team))}, nil
}

/*
Destroy deletes a team, moving its conversations where the query says.
*/
func (c *TeamController) Destroy(ctx context.Context, in *requests.DestroyTeamRequest) (*EmptyOutput, error) {
	id, err := teamID(c.IDs, in.Team)
	if err != nil {
		return nil, err
	}

	toTeam, err := c.target(models.PrefixTeam, in.ReassignToTeam)
	if err != nil {
		return nil, err
	}

	toMember, err := c.target(models.PrefixMember, in.ReassignToMember)
	if err != nil {
		return nil, err
	}

	err = c.DeleteTeam.Handle(ctx, teams.DeleteTeamInput{
		Membership: membership(ctx), TeamID: id, ToTeam: toTeam, ToMember: toMember,
	})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
target reads where a deleted team's conversations go: nil when the query
names nothing, and an id that names nothing refused as an invalid
target.
*/
func (c *TeamController) target(prefix models.Prefix, named string) (*ids.UUID, error) {
	if named == "" {
		return nil, nil
	}

	id, err := c.IDs.Decode(string(prefix), named)
	if err != nil {
		return nil, exceptions.ErrReassignToInvalid
	}

	return &id, nil
}

/*
teamID returns the team a path names, refusing ids that name no team as
not found.
*/
func teamID(registry *ids.Registry, named string) (ids.UUID, error) {
	id, err := registry.Decode(string(models.PrefixTeam), named)
	if err != nil {
		return ids.Nil(), exceptions.ErrNotFound
	}

	return id, nil
}
