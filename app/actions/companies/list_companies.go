/*
|--------------------------------------------------------------------------
| Read Companies
|--------------------------------------------------------------------------
|
| Companies are read with contacts:read, a page at a time with their tags,
| one query per kind of row. A deleted company is simply not found.
|
*/

package companies

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Detail is a company with the tags on it.
*/
type Detail struct {
	Company models.Company
	Tags    []models.Tag
}

/*
LoadDetails loads the companies of one workspace, in the order given.
*/
func LoadDetails(ctx context.Context, q *models.Queries, rows []models.Company) ([]Detail, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	companyIDs := make([]ids.UUID, 0, len(rows))
	for _, c := range rows {
		companyIDs = append(companyIDs, c.ID)
	}

	tagged, err := q.ListCompanyTags(ctx, models.ListCompanyTagsParams{
		WorkspaceID: rows[0].WorkspaceID, CompanyIds: companyIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("read company tags: %w", err)
	}

	byCompany := map[ids.UUID][]models.Tag{}
	for _, row := range tagged {
		byCompany[row.CompanyID] = append(byCompany[row.CompanyID], row.Tag)
	}

	out := make([]Detail, len(rows))
	for i, c := range rows {
		out[i] = Detail{Company: c, Tags: byCompany[c.ID]}
	}

	return out, nil
}

/*
LoadDetail loads one company.
*/
func LoadDetail(ctx context.Context, q *models.Queries, c models.Company) (Detail, error) {
	details, err := LoadDetails(ctx, q, []models.Company{c})
	if err != nil {
		return Detail{}, err
	}

	return details[0], nil
}

/*
ListInput is who asks and which page: Search narrows by name or domain,
TagIDs to companies carrying every tag, and After continues after the
company with the time and id.
*/
type ListInput struct {
	Membership authz.Membership
	Search     *string
	TagIDs     []ids.UUID
	AfterAt    *time.Time
	AfterID    *ids.UUID
	Offset     int
	Limit      int
	Count      bool
}

/*
Page is a page of companies, whether more follow, how many people each
has and, when asked, how many companies match in all, up to TotalCap.
*/
type Page struct {
	Companies []Detail
	More      bool
	People    map[ids.UUID]int64
	Total     *int64
}

/*
TotalCap is the most a company count goes up to.
*/
const TotalCap = 10_000

/*
ListCompanies lists the workspace's companies, newest first.

Rules:
  - The caller needs contacts:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListCompanies struct {
	DB *database.DB
}

/*
Handle returns the page.
*/
func (a *ListCompanies) Handle(ctx context.Context, in ListInput) (Page, error) {
	if !in.Membership.Can("contacts:read") {
		return Page{}, exceptions.ErrForbidden
	}

	var page Page

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		rows, err := q.ListCompanies(ctx, models.ListCompaniesParams{
			WorkspaceID: in.Membership.Workspace.ID, Search: in.Search, TagIds: in.TagIDs, BeforeID: in.AfterID,
			BeforeCreatedAt: in.AfterAt, RowLimit: int64(in.Limit) + 1, RowOffset: int64(in.Offset),
		})
		if err != nil {
			return fmt.Errorf("list companies: %w", err)
		}

		if len(rows) > in.Limit {
			rows, page.More = rows[:in.Limit], true
		}

		if page.Companies, err = LoadDetails(ctx, q, rows); err != nil {
			return err
		}

		if page.People, err = people(ctx, q, rows); err != nil || !in.Count {
			return err
		}

		page.Total, err = count(ctx, q, in)

		return err
	})

	return page, err
}

/*
find returns the live company, or exceptions.ErrNotFound.
*/
func find(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Company, error) {
	c, err := q.FindCompany(ctx, models.FindCompanyParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) || (err == nil && c.DeletedAt != nil) {
		return c, exceptions.ErrNotFound
	}

	if err != nil {
		return c, fmt.Errorf("find company: %w", err)
	}

	return c, nil
}

/*
people counts the contacts of each company of the page.
*/
func people(ctx context.Context, q *models.Queries, rows []models.Company) (map[ids.UUID]int64, error) {
	out := map[ids.UUID]int64{}
	if len(rows) == 0 {
		return out, nil
	}

	companyIDs := make([]ids.UUID, 0, len(rows))
	for _, c := range rows {
		companyIDs = append(companyIDs, c.ID)
	}

	counted, err := q.CountCompanyPeople(ctx, models.CountCompanyPeopleParams{
		WorkspaceID: rows[0].WorkspaceID, CompanyIds: companyIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("count company people: %w", err)
	}

	for _, row := range counted {
		out[row.CompanyID] = row.People
	}

	return out, nil
}

/*
count counts the companies the input lists, up to TotalCap.
*/
func count(ctx context.Context, q *models.Queries, in ListInput) (*int64, error) {
	n, err := q.CountCompanies(ctx, models.CountCompaniesParams{
		WorkspaceID: in.Membership.Workspace.ID, Search: in.Search, TagIds: in.TagIDs, RowLimit: TotalCap,
	})
	if err != nil {
		return nil, fmt.Errorf("count companies: %w", err)
	}

	return &n, nil
}
