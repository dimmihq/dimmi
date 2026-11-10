/*
|--------------------------------------------------------------------------
| Matching A Page Of People
|--------------------------------------------------------------------------
|
| Whatever sends to an audience reads who matches a page at a time, so
| a large workspace never sits in memory and a send can resume after a
| crash from the last contact it saw. A saved audience, a rule written
| on the spot, or no rule at all, meaning everyone, are all read the
| same way.
|
*/

package audiences

import (
	"context"
	"encoding/json"
	"math"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
MatchInput is one page to read: the saved audience, or the rules when
none is named, and the last contact the previous page ended on.
*/
type MatchInput struct {
	WorkspaceID ids.UUID
	AudienceID  *ids.UUID
	Rules       json.RawMessage
	After       ids.UUID
	Size        int
}

/*
MatchPage is one page read: the contacts who match, the last contact
read, and whether the workspace has no more.
*/
type MatchPage struct {
	Contacts []models.Contact
	Last     ids.UUID
	Done     bool
}

/*
Match reads the next page of contacts and keeps those who match.
*/
func (a *Audiences) Match(ctx context.Context, q *models.Queries, in MatchInput) (MatchPage, error) {
	out := MatchPage{Last: in.After}

	program, err := a.programOf(ctx, q, in)
	if err != nil {
		return out, err
	}

	page, err := q.AudienceContactPage(ctx, models.AudienceContactPageParams{
		WorkspaceID: in.WorkspaceID, After: in.After, Fraction: 1, PageSize: int32Of(max(in.Size, 1)),
	})
	if err != nil {
		return out, wrap("read contacts", err)
	}

	out.Done = len(page) < max(in.Size, 1)
	if len(page) == 0 {
		return out, nil
	}

	out.Last = page[len(page)-1].ID
	if program == nil {
		out.Contacts = page

		return out, nil
	}

	env, err := a.env(ctx, q, in.WorkspaceID)
	if err != nil {
		return out, err
	}

	matched, err := judge{program: program, env: env}.matching(ctx, q, page)
	if err != nil {
		return out, err
	}

	keep := make(map[ids.UUID]bool, len(matched))
	for _, id := range matched {
		keep[id] = true
	}

	for _, c := range page {
		if keep[c.ID] {
			out.Contacts = append(out.Contacts, c)
		}
	}

	return out, nil
}

/*
programOf compiles the saved audience or the rules; nil means
everyone.
*/
func (a *Audiences) programOf(ctx context.Context, q *models.Queries, in MatchInput) (*audience.Program, error) {
	row := models.Audience{WorkspaceID: in.WorkspaceID, Rules: in.Rules}

	if in.AudienceID != nil {
		var err error
		if row, err = find(ctx, q, in.WorkspaceID, *in.AudienceID); err != nil {
			return nil, err
		}
	}

	if len(row.Rules) == 0 || string(row.Rules) == "null" {
		return nil, nil
	}

	program, err := audiences.Compile(row)
	if err != nil {
		return nil, ErrInvalid.With("reason", err.Error())
	}

	return program, nil
}

/*
int32Of bounds a count to what a query takes.
*/
func int32Of(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(n)
}
