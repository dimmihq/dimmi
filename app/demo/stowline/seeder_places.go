/*
|--------------------------------------------------------------------------
| Seeding The Team's Places And Vocabulary
|--------------------------------------------------------------------------
|
| The teams with their people, an inbox per team with a connection for
| every channel, the tags and saved replies the team works with, and the
| attributes its customers carry, each through the dashboard's own
| actions.
|
*/

package stowline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/savedreplies"
	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/actions/teams"
	"github.com/dimmihq/dimmi/app/models"
)

/*
places adds the teams with their people, an inbox per team with its
channels, and points each inbox at its team.
*/
func (r *run) places(ctx context.Context) error {
	r.teams, r.inboxes, r.channels = map[string]ids.UUID{}, map[string]ids.UUID{}, map[string]map[string]ids.UUID{}
	for _, t := range Teams() {
		if err := r.makeTeam(ctx, t.Name); err != nil {
			return err
		}
	}
	def, err := models.New(r.s.App.DB).FindDefaultInbox(ctx, r.ws.ID)
	if err != nil {
		return fmt.Errorf("find default inbox: %w", err)
	}
	for _, t := range Teams() {
		if err := r.makeInbox(ctx, t.Name, t.Inbox, def.ID); err != nil {
			return err
		}
	}
	return nil
}

/*
makeTeam creates the team with the teammates who sit on it.
*/
func (r *run) makeTeam(ctx context.Context, name string) error {
	a := r.s.App
	made, err := a.Actions.CreateTeam.Handle(ctx, teams.CreateTeamInput{Membership: r.owner, Name: name})
	if err != nil {
		return fmt.Errorf("create team %s: %w", name, err)
	}
	r.teams[name] = made.Team.ID
	var members []ids.UUID
	for i, m := range r.plan.Team {
		if m.Team == name {
			members = append(members, r.team[i].Member.ID)
		}
	}
	if len(members) == 0 {
		return nil
	}
	if _, err := a.Actions.AddTeamMembers.Handle(ctx, teams.AddTeamMembersInput{
		Membership: r.owner, TeamID: made.Team.ID, MemberIDs: members,
	}); err != nil {
		return fmt.Errorf("fill team %s: %w", name, err)
	}
	return nil
}

/*
makeInbox names the team's inbox, the workspace's default one for
Support, points it at the team and connects its channels.
*/
func (r *run) makeInbox(ctx context.Context, team, name string, support ids.UUID) error {
	a := r.s.App
	inbox := support
	if name != "Support" {
		made, err := models.New(a.DB).CreateInbox(ctx, models.CreateInboxParams{
			WorkspaceID: r.ws.ID, Name: name, Settings: json.RawMessage(`{}`), Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("create inbox %s: %w", name, err)
		}
		inbox = made.ID
	}
	r.inboxes[name] = inbox
	if _, err := a.DB.Exec(ctx, `UPDATE inboxes SET name = $3, default_team_id = $4
		WHERE workspace_id = $1 AND id = $2`, r.ws.ID, inbox, name, r.teams[team]); err != nil {
		return fmt.Errorf("point inbox at team: %w", err)
	}
	return r.connect(ctx, name, inbox)
}

/*
connect gives an inbox a connection for every channel the planned
conversations arrive on, reusing the API connection it already has.
*/
func (r *run) connect(ctx context.Context, name string, inbox ids.UUID) error {
	r.channels[name] = map[string]ids.UUID{}
	names := map[string]string{
		"widget": "Stowline messenger", "email": strings.ToLower(name) + "@" + Domain,
		"api": "API", "slack": "Shared Slack channels",
	}
	for _, kind := range []string{"widget", "email", "api", "slack"} {
		var id ids.UUID
		err := r.s.App.DB.QueryRow(ctx, `SELECT id FROM channel_connections
			WHERE workspace_id = $1 AND inbox_id = $2 AND type = $3 AND deleted_at IS NULL ORDER BY created_at LIMIT 1`,
			r.ws.ID, inbox, kind).Scan(&id)
		if errors.Is(err, database.ErrNotFound) {
			err = r.s.App.DB.QueryRow(ctx, `INSERT INTO channel_connections (workspace_id, inbox_id, type, name, status,
					config, last_activity_at)
				VALUES ($1, $2, $3, $4, 'active', '{}', now()) RETURNING id`,
				r.ws.ID, inbox, kind, names[kind]).Scan(&id)
		}
		if err != nil {
			return fmt.Errorf("connect %s to %s: %w", kind, name, err)
		}
		r.channels[name][kind] = id
	}
	return nil
}

/*
vocabulary adds the tags, the team's saved replies and the attributes
the customers carry.
*/
func (r *run) vocabulary(ctx context.Context) error {
	a := r.s.App
	r.tags = map[string]ids.UUID{}
	for _, t := range Tags() {
		made, err := a.Actions.Inbox.CreateTag.Handle(ctx, tags.CreateTagInput{
			Membership: r.owner, Name: t[0], Color: new(t[1]),
		})
		if err != nil {
			return fmt.Errorf("create tag %s: %w", t[0], err)
		}
		r.tags[t[0]] = made.ID
	}
	for i, sr := range SavedReplies() {
		by := r.owner
		if i%3 == 2 && len(r.team) > 1 {
			by = r.team[1+i%(len(r.team)-1)]
		}
		if _, err := a.Actions.Inbox.CreateSavedReply.Handle(ctx, savedreplies.CreateSavedReplyInput{
			Membership: by, Fields: savedreplies.Fields{
				Title: new(sr[0]), Body: new(sr[1]), Shortcut: new(sr[2]),
				Shared: new(i%3 != 2),
			},
		}); err != nil {
			return fmt.Errorf("create saved reply %s: %w", sr[0], err)
		}
	}
	for _, d := range attributeDefinitions() {
		if _, err := a.DB.Exec(ctx, `INSERT INTO attribute_definitions (workspace_id, object, key, label, description,
				data_type, options)
			VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb) ON CONFLICT DO NOTHING`,
			r.ws.ID, d[0], d[1], d[2], d[3], d[4], d[5]); err != nil {
			return fmt.Errorf("define attribute %s: %w", d[1], err)
		}
	}
	return nil
}
