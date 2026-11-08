/*
|--------------------------------------------------------------------------
| Saved Audiences
|--------------------------------------------------------------------------
|
| A rule may say "is in" a saved audience. Membership is never stored:
| the saved audience's own rule is evaluated for the same person, so an
| edit applies everywhere at once. A saved audience whose rule no longer
| compiles matches nobody rather than everybody.
|
*/

package audiences

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
Resolver finds a workspace's saved audiences by public id, compiled
once each.
*/
type Resolver struct {
	rows     map[string]models.Audience
	compiled map[string]*audience.Program
}

/*
LoadResolver reads the workspace's saved audiences.
*/
func LoadResolver(ctx context.Context, q *models.Queries, ws ids.UUID) (*Resolver, error) {
	rows, err := q.ListAudiences(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("list audiences: %w", err)
	}

	return NewResolver(rows), nil
}

/*
NewResolver resolves among the given saved audiences.
*/
func NewResolver(rows []models.Audience) *Resolver {
	r := &Resolver{rows: map[string]models.Audience{}, compiled: map[string]*audience.Program{}}
	for _, row := range rows {
		r.rows[models.PublicID(models.PrefixAudience, row.ID)] = row
	}

	return r
}

/*
Saved returns the program of a saved audience. One that cannot be
compiled is reported missing, so it matches nobody.
*/
func (r *Resolver) Saved(id string) (*audience.Program, bool) {
	if p, ok := r.compiled[id]; ok {
		return p, p != nil
	}

	row, ok := r.rows[id]
	if !ok {
		return nil, false
	}

	p, err := Compile(row)
	if err != nil {
		p = nil
	}

	r.compiled[id] = p

	return p, p != nil
}

/*
Compile compiles a saved audience's rule.
*/
func Compile(row models.Audience) (*audience.Program, error) {
	a, err := audience.Parse(row.Rules)
	if err != nil {
		return nil, fmt.Errorf("parse audience: %w", err)
	}

	p, err := audience.Compile(a, audience.Base(), audience.SavedAudience())
	if err != nil {
		return nil, fmt.Errorf("compile audience: %w", err)
	}

	return p, nil
}

/*
CheckNamed checks the saved audiences messenger rules name: each must
exist, and those the start rule names, at any depth, must read only what
the server knows and trusts, as the start rule itself must.
*/
func (r *Resolver) CheckNamed(rules MessengerRules) []audience.Problem {
	var found []audience.Problem
	named := func(_ string, a *audience.Audience) []string {
		if a == nil {
			return nil
		}

		return audience.SavedIDs(a.Rules)
	}

	for _, id := range named("launcher", rules.Launcher) {
		found = append(found, r.exists("launcher", id)...)
	}

	for tab, a := range rules.Tabs {
		for _, id := range named("tabs."+tab, a) {
			found = append(found, r.exists("tabs."+tab, id)...)
		}
	}

	for _, id := range named("start", rules.Start) {
		found = append(found, r.trusted("start", id, 0)...)
	}

	return found
}

/*
exists refuses a saved audience the workspace does not have.
*/
func (r *Resolver) exists(path, id string) []audience.Problem {
	if _, ok := r.rows[id]; ok {
		return nil
	}

	return []audience.Problem{{
		Code: audience.CodeMissingReference, Path: path, Field: "audience",
		Message: "No saved audience has the id " + id + ".",
	}}
}

/*
trusted checks a saved audience the start rule names, and those inside
it: each must read only what the server knows and trusts.
*/
func (r *Resolver) trusted(path, id string, depth int) []audience.Problem {
	if missing := r.exists(path, id); missing != nil || depth > 3 {
		return missing
	}

	row := r.rows[id]

	a, err := audience.Parse(row.Rules)
	if err == nil {
		_, err = audience.Compile(a, audience.Base(), audience.MessengerStart())
	}

	if err != nil {
		return []audience.Problem{{
			Code: audience.CodeClaimedOnAccessGate, Path: path, Field: "audience",
			Message: "The saved audience " + row.Name + " reads what a visitor's browser could claim or the page " +
				"they are on, so it cannot decide who may start a conversation.",
		}}
	}

	var found []audience.Problem
	for _, inner := range audience.SavedIDs(a.Rules) {
		found = append(found, r.trusted(path, inner, depth+1)...)
	}

	return found
}
