package inbox_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
company is a company to store, by its attributes.
*/
type company = models.CreateCompanyParams

/*
createCompany stores the company, in the test's workspace unless it
names another, with a seat count and the clock's time.
*/
func createCompany(t *testing.T, tc *tests.TestCase, c company) (models.Company, error) {
	t.Helper()

	if c.WorkspaceID == ids.Nil() {
		c.WorkspaceID = tc.Workspace.Workspace.ID
	}

	c.CustomAttributes, c.Now = json.RawMessage(`{"seats":12}`), tc.Clock.Now()

	return query(t, tc, c.WorkspaceID, func(ctx context.Context, q *models.Queries) (models.Company, error) {
		return q.CreateCompany(ctx, c)
	})
}

/*
mustCompany stores the company and fails the test when it cannot.
*/
func mustCompany(t *testing.T, tc *tests.TestCase, c company) models.Company {
	t.Helper()

	created, err := createCompany(t, tc, c)
	require.NoError(t, err)

	return created
}

/*
companyNames lists the workspace's live companies matching the search,
the given number at a time, and returns their names in order.
*/
func companyNames(t *testing.T, tc *tests.TestCase, search *string, size int32) []string {
	t.Helper()

	workspace := tc.Workspace.Workspace.ID
	params := models.ListCompaniesParams{WorkspaceID: workspace, Search: search, RowLimit: size}

	var names []string

	for {
		page, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) ([]models.Company, error) {
			return q.ListCompanies(ctx, params)
		})
		require.NoError(t, err)

		if len(page) == 0 {
			return names
		}

		for _, c := range page {
			names = append(names, c.Name)
		}

		last := page[len(page)-1]
		params.BeforeID, params.BeforeCreatedAt = &last.ID, &last.CreatedAt
	}
}

/*
lead stores a lead with the name in the test's workspace.
*/
func lead(t *testing.T, tc *tests.TestCase, name string) {
	t.Helper()

	workspace := tc.Workspace.Workspace.ID
	_, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Contact, error) {
		return q.CreateContact(ctx, models.CreateContactParams{
			WorkspaceID: workspace, Kind: string(enums.ContactKindLead), Name: new(name),
			CustomAttributes: json.RawMessage(`{}`), Now: tc.Clock.Now(),
		})
	})
	require.NoError(t, err)
}

/*
companyContacts walks every page of the company's contacts, the given
number at a time, and returns their names in order.
*/
func companyContacts(t *testing.T, tc *tests.TestCase, company ids.UUID, size int32) []string {
	t.Helper()

	workspace := tc.Workspace.Workspace.ID
	params := models.ListCompanyContactsParams{WorkspaceID: workspace, CompanyID: company, RowLimit: size}

	var names []string

	for {
		page, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) ([]models.Contact, error) {
			return q.ListCompanyContacts(ctx, params)
		})
		require.NoError(t, err)

		if len(page) == 0 {
			return names
		}

		for _, c := range page {
			names = append(names, *c.Name)
		}

		last := page[len(page)-1]
		params.BeforeID, params.BeforeCreatedAt = &last.ID, &last.CreatedAt
	}
}

func TestCompanyQueries(t *testing.T) {
	t.Parallel()

	t.Run("finds a company by id, external id and domain in its workspace only", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		acme := mustCompany(t, tc, company{Name: "Acme", ExternalID: new("org_7"), Domain: new("acme.dev")})

		found, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) ([]models.Company, error) {
			byID, err := q.FindCompany(ctx, models.FindCompanyParams{WorkspaceID: workspace, ID: acme.ID})
			if err != nil {
				return nil, err
			}

			byExternal, err := q.FindCompanyByExternalID(ctx, models.FindCompanyByExternalIDParams{
				WorkspaceID: workspace, ExternalID: new("org_7"),
			})
			if err != nil {
				return nil, err
			}

			byDomain, err := q.FindCompanyByDomain(ctx, models.FindCompanyByDomainParams{
				WorkspaceID: workspace, Domain: new("ACME.dev"),
			})

			return []models.Company{byID, byExternal, byDomain}, err
		})
		require.NoError(t, err)
		other, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)
		_, err = query(t, tc, other.Workspace.ID, func(ctx context.Context, q *models.Queries) (models.Company, error) {
			return q.FindCompany(ctx, models.FindCompanyParams{WorkspaceID: workspace, ID: acme.ID})
		})

		assert.Equal(t, []models.Company{acme, acme, acme}, found)
		assert.JSONEq(t, `{"seats":12}`, string(acme.CustomAttributes))
		require.ErrorIs(t, err, database.ErrNotFound)
	})

	t.Run("keeps one company per external id in a workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		mustCompany(t, tc, company{Name: "Acme", ExternalID: new("org_7")})
		other, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)

		_, duplicate := createCompany(t, tc, company{Name: "Acme again", ExternalID: new("org_7")})
		_, elsewhere := createCompany(t, tc, company{
			WorkspaceID: other.Workspace.ID, Name: "Acme", ExternalID: new("org_7"),
		})

		assert.True(t, database.IsUniqueViolation(duplicate, "companies_workspace_id_external_id_key"))
		require.NoError(t, elsewhere)
	})

	t.Run("lists live companies newest first, searched by name or domain", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		for _, c := range []struct{ name, domain string }{
			{"Acme", "acme.dev"}, {"Globex", "globex.com"}, {"Initech", "acme-labs.io"}, {"Umbrella", "umbrella.co"},
		} {
			mustCompany(t, tc, company{Name: c.name, Domain: new(c.domain)})
			tc.Clock.Travel(time.Minute)
		}

		assert.Equal(t, []string{"Umbrella", "Initech", "Globex", "Acme"}, companyNames(t, tc, nil, 3))
		assert.Equal(t, []string{"Initech", "Acme"}, companyNames(t, tc, new("ACME"), 1))
		assert.Equal(t, []string{"Globex"}, companyNames(t, tc, new("lob"), 10))
		assert.Empty(t, companyNames(t, tc, new("nobody"), 10))
	})

	t.Run("updates a live company and deletes it once, detaching its contacts", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		acme := mustCompany(t, tc, company{Name: "Acme", Domain: new("acme.dev")})
		th := seedThread(t, tc, workspace)
		exec(t, tc, "UPDATE contacts SET company_id = $1 WHERE id = $2", acme.ID, th.contact.ID)

		updated, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Company, error) {
			return q.UpdateCompany(ctx, models.UpdateCompanyParams{
				WorkspaceID: workspace, ID: acme.ID, ExternalID: new("org_9"), Name: "Acme Inc",
				Domain: acme.Domain, CustomAttributes: json.RawMessage(`{}`), Now: tc.Clock.Now(),
			})
		})
		require.NoError(t, err)
		remove := func(ctx context.Context, q *models.Queries) (int64, error) {
			return q.DeleteCompany(ctx, models.DeleteCompanyParams{
				WorkspaceID: workspace, ID: acme.ID, Now: tc.Clock.Now(),
			})
		}
		deleted, err := query(t, tc, workspace, remove)
		require.NoError(t, err)
		again, err := query(t, tc, workspace, remove)
		require.NoError(t, err)
		detached, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (int64, error) {
			return q.DetachCompanyContacts(ctx, models.DetachCompanyContactsParams{
				WorkspaceID: workspace, CompanyID: acme.ID, Now: tc.Clock.Now(),
			})
		})
		require.NoError(t, err)
		changes := []int64{deleted, again, detached}
		_, err = query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Company, error) {
			return q.UpdateCompany(ctx, models.UpdateCompanyParams{
				WorkspaceID: workspace, ID: acme.ID, Name: "Ghost", CustomAttributes: json.RawMessage(`{}`),
				Now: tc.Clock.Now(),
			})
		})

		assert.Equal(t, "Acme Inc", updated.Name)
		assert.Equal(t, "org_9", *updated.ExternalID)
		assert.Equal(t, []int64{1, 0, 1}, changes)
		require.ErrorIs(t, err, database.ErrNotFound)
		assert.Empty(t, companyNames(t, tc, nil, 10))
		tc.Table("contacts").AssertHas(testkit.Row{"id": th.contact.ID, "company_id": nil})
	})

	t.Run("lists the live contacts of a company, newest first, a page at a time", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		acme := mustCompany(t, tc, company{Name: "Acme"})
		for _, name := range []string{"Ada", "Grace", "Linus", "Merged"} {
			lead(t, tc, name)
			tc.Clock.Travel(time.Minute)
		}
		exec(t, tc, "UPDATE contacts SET company_id = $1 WHERE workspace_id = $2", acme.ID, workspace)
		exec(t, tc, "UPDATE contacts SET merged_into_id = id WHERE name = 'Merged'")

		listed := companyContacts(t, tc, acme.ID, 2)

		assert.Equal(t, []string{"Linus", "Grace", "Ada"}, listed)
	})
}
