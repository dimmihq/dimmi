/*
|--------------------------------------------------------------------------
| Audience Pages
|--------------------------------------------------------------------------
|
| The dashboard's table of audiences reads them a page at a time, by
| name, searched by what the name contains, with how many match counted
| up to a cap so the count stays cheap however many a workspace keeps.
|
*/

package audiences

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TotalCap is the most an audience count goes up to.
*/
const TotalCap = 10_000

/*
PageInput is one page of audiences: Search narrows to names containing
it, Offset skips that many and Count asks how many match in all.
*/
type PageInput struct {
	Membership authz.Membership
	Search     *string
	Limit      int
	Offset     int
	Count      bool
}

/*
AudiencePage is one page of audiences and, when asked, how many match.
*/
type AudiencePage struct {
	Audiences []models.Audience
	Total     *int64
}

/*
Page returns one page of the workspace's saved audiences by name.
*/
func (a *Audiences) Page(ctx context.Context, in PageInput) (AudiencePage, error) {
	if !in.Membership.Can("audiences:read") {
		return AudiencePage{}, exceptions.ErrForbidden
	}

	var out AudiencePage

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		var err error

		out.Audiences, err = q.ListAudiencePage(ctx, models.ListAudiencePageParams{
			WorkspaceID: in.Membership.Workspace.ID, Search: in.Search,
			RowLimit: int64(in.Limit), RowOffset: int64(in.Offset),
		})
		if err != nil || !in.Count {
			return wrap("list audiences", err)
		}

		n, err := q.CountAudiences(ctx, models.CountAudiencesParams{
			WorkspaceID: in.Membership.Workspace.ID, Search: in.Search, RowLimit: TotalCap,
		})
		out.Total = &n

		return wrap("count audiences", err)
	})

	return out, err
}
