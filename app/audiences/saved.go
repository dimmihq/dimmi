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
