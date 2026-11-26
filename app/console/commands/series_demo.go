//go:build dev

/*
|--------------------------------------------------------------------------
| Series Demo Helpers
|--------------------------------------------------------------------------
|
| Builds and publishes the seeded series definitions embedded in
| series_demo.json, dropping Slack steps when a workspace has no Slack
| connection so the rest of each series still goes live.
|
*/

package commands

import (
	"context"
	_ "embed"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
demoSeriesJSON holds the series the seed builds, as the builder saves them.
*/
//go:embed series_demo.json
var demoSeriesJSON []byte

/*
demoJourney is one seeded series: its name, who travels it and its
definition.
*/
type demoJourney struct {
	Name    string      `json:"name"`
	Subject string      `json:"subject"`
	Spec    series.Spec `json:"spec"`
}

/*
demoSeriesInput holds the owner, series definition and sender email address.
*/
type demoSeriesInput struct {
	Owner   authz.Membership
	Demo    demoJourney
	Address ids.UUID
}

/*
buildDemoSeries makes, saves and publishes one seeded series.
*/
func buildDemoSeries(ctx context.Context, a *app.App, in demoSeriesInput) (models.Series, error) {
	s := a.Actions.Series
	m, demo := in.Owner, in.Demo
	spec := demo.Spec

	row, err := s.Create(ctx, series.CreateInput{Membership: m, Name: demo.Name, Subject: demo.Subject, Spec: &spec})
	if err != nil {
		return row, err
	}

	save := series.CurrentSettings(row)
	save.Membership, save.SenderMemberID, save.ReplyMemberID = m, &m.Member.ID, &m.Member.ID
	if in.Address != ids.Nil() {
		save.EmailAddressID = &in.Address
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
