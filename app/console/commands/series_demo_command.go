//go:build dev

/*
|--------------------------------------------------------------------------
| Series Demo
|--------------------------------------------------------------------------
|
| Fills a development workspace with series a real team would run, so
| the builder and its results can be judged on believable data: an
| onboarding journey with a check and a split, a trial-ending series
| waiting on a date, and an account renewal that emails admins and
| posts in Slack. Each is built and published through the same actions
| the API uses and the workspace's people are enrolled for real, so the
| engine runs them. Run broadcasts:demo first for people and accounts.
| It writes only to the workspace it is given and never to a release
| build.
|
*/

package commands

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/series"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SeriesDemoCommand seeds a workspace with demo series.
*/
type SeriesDemoCommand struct {
	App *app.App
}

/*
Name returns series:demo.
*/
func (c SeriesDemoCommand) Name() string {
	return "series:demo"
}

/*
Description says what the command does.
*/
func (c SeriesDemoCommand) Description() string {
	return "Seed a development workspace with live series and the people on them"
}

/*
Flags names the workspace and the teammate who builds the series.
*/
func (c SeriesDemoCommand) Flags(flags *console.Flags) {
	flags.String("workspace", "", "the workspace's public id, ws_...")
	flags.String("member", "", "the teammate's public id, mem_...")
	flags.String("address", "", "the connected email address's public id, ema_...")
}

/*
Handle builds, publishes and fills the demo series.
*/
func (c SeriesDemoCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	m, err := c.membership(ctx, in.String("workspace"), in.String("member"))
	if err != nil {
		return err
	}

	address, _ := c.App.IDs.Decode(string(models.PrefixEmailAddress), in.String("address"))
	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	var demos []demoJourney
	if err := json.Unmarshal(demoSeriesJSON, &demos); err != nil {
		return fmt.Errorf("read demo series: %w", err)
	}

	for _, demo := range demos {
		row, err := c.build(ctx, m, demo, address)
		if err != nil {
			return fmt.Errorf("build %s: %w", demo.Name, err)
		}

		out.Line("published %s (%s)", demo.Name, models.PublicID(models.PrefixSeries, row.ID))
	}

	return nil
}

/*
build makes, saves, publishes and fills one demo series.
*/
func (c SeriesDemoCommand) build(ctx context.Context, m authz.Membership, demo demoJourney, address ids.UUID,
) (models.Series, error) {
	s := c.App.Actions.Series
	spec := demo.Spec

	row, err := s.Create(ctx, series.CreateInput{Membership: m, Name: demo.Name, Subject: demo.Subject, Spec: &spec})
	if err != nil {
		return row, err
	}

	save := series.CurrentSettings(row)
	save.Membership, save.SenderMemberID, save.ReplyMemberID = m, &m.Member.ID, &m.Member.ID
	if address != ids.Nil() {
		save.EmailAddressID = &address
	}

	if row, err = s.Save(ctx, save); err != nil {
		return row, err
	}

	published, err := s.Publish(ctx, series.PublishInput{Membership: m, ID: row.ID, Note: "Demo"})
	if err == nil || !hasSlack(spec) {
		return published, err
	}

	save = series.CurrentSettings(row)
	save.Membership, save.Spec = m, new(withoutSlack(spec))

	if row, err = s.Save(ctx, save); err != nil {
		return row, err
	}

	return s.Publish(ctx, series.PublishInput{Membership: m, ID: row.ID, Note: "Demo without Slack"})
}

/*
hasSlack reports whether a demo series posts in Slack.
*/
func hasSlack(spec series.Spec) bool {
	for _, n := range spec.Nodes {
		if n.Kind == series.KindSlack {
			return true
		}
	}

	return false
}

/*
withoutSlack drops a demo series' Slack steps, for a workspace with no
Slack connected, so the rest of it still goes live.
*/
func withoutSlack(spec series.Spec) series.Spec {
	gone := map[string]bool{}
	nodes := spec.Nodes[:0:0]

	for _, n := range spec.Nodes {
		if n.Kind == series.KindSlack {
			gone[n.ID] = true
			continue
		}

		nodes = append(nodes, n)
	}

	edges := spec.Edges[:0:0]
	for _, e := range spec.Edges {
		if !gone[e.From] && !gone[e.To] {
			edges = append(edges, e)
		}
	}

	spec.Nodes, spec.Edges = nodes, edges

	return spec
}

/*
membership reads the teammate the demo acts as, with an owner's rights.
*/
func (c SeriesDemoCommand) membership(ctx context.Context, workspace, member string) (authz.Membership, error) {
	ws, err := c.App.IDs.Decode(string(models.PrefixWorkspace), workspace)
	if err != nil {
		return authz.Membership{}, fmt.Errorf("--workspace: %w", err)
	}

	id, err := c.App.IDs.Decode(string(models.PrefixMember), member)
	if err != nil {
		return authz.Membership{}, fmt.Errorf("--member: %w", err)
	}

	ctx = tenancy.With(ctx, ws.String())
	q := models.New(c.App.DB)

	w, err := q.FindWorkspace(ctx, ws)
	if err != nil {
		return authz.Membership{}, fmt.Errorf("find workspace: %w", err)
	}

	mem, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{WorkspaceID: ws, ID: id})
	if err != nil {
		return authz.Membership{}, fmt.Errorf("find member: %w", err)
	}

	return authz.NewMembership(w, mem, appauth.RolePermissions(enums.RoleOwner)), nil
}

/*
demoSeriesJSON holds the series the demo builds, as the builder saves
them.
*/
//go:embed series_demo.json
var demoSeriesJSON []byte

/*
demoJourney is one demo series: its name, who travels it and its
definition.
*/
type demoJourney struct {
	Name    string      `json:"name"`
	Subject string      `json:"subject"`
	Spec    series.Spec `json:"spec"`
}
