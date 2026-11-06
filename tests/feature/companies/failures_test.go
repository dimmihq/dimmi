package companies_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/tests"
)

func TestCompanyRefusals(t *testing.T) {
	t.Parallel()

	t.Run("refuses a membership without the contact permissions", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acts := d.TC.App.Actions.Inbox
		none := authz.NewMembership(d.TC.Workspace.Workspace, d.TC.Workspace.Owner, nil)
		ctx := t.Context()

		_, err := acts.ListCompanies.Handle(ctx, companies.ListInput{Membership: none, Limit: 1})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.ShowCompany.Handle(ctx, companies.ShowInput{Membership: none, CompanyID: ids.New()})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.CreateCompany.Handle(ctx, companies.CreateInput{Membership: none})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, _, err = acts.IdentifyCompany.Handle(ctx, companies.IdentifyInput{Membership: none})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.UpdateCompany.Handle(ctx, companies.UpdateInput{Membership: none, CompanyID: ids.New()})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.ListCompanyContacts.Handle(ctx, companies.ContactsInput{Membership: none, CompanyID: ids.New()})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
	})
}

func TestCompanyFailures(t *testing.T) {
	t.Parallel()

	for table, requests := range map[string][]string{
		"company_tag": {"GET /companies", "GET company", "PATCH tags", "POST upsert"},
		"contacts":    {"GET people", "DELETE company"},
		"tags":        {"PATCH tags"},
	} {
		t.Run("answers 500 when "+table+" cannot be used", func(t *testing.T) {
			t.Parallel()
			d := tests.NewDesk(t)
			path := "/api/v1/companies/" + d.Owner.PostJSON("/api/v1/companies", testkit.JSON{
				"name": "Acme", "external_id": "org_1",
			}).AssertCreated().String("data.id")
			d.TC.BreakTable(table)

			for _, request := range requests {
				status := send(d, path, request)
				assert.Equal(t, 500, status, request)
			}
		})
	}

	t.Run("answers 500 when companies cannot be written", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path := "/api/v1/companies/" + d.Owner.PostJSON("/api/v1/companies", testkit.JSON{
			"name": "Acme", "external_id": "org_1",
		}).AssertCreated().String("data.id")
		d.TC.FailWrites("companies", "INSERT", "UPDATE")

		for _, request := range []string{"POST company", "POST upsert", "PATCH name", "DELETE company"} {
			assert.Equal(t, 500, send(d, path, request), request)
		}
	})
}

/*
send makes one of the named requests against the company at the path
and returns the status it answered with.
*/
func send(d tests.Desk, path, request string) int {
	switch request {
	case "GET /companies":
		return d.Owner.GetJSON("/api/v1/companies").Status
	case "GET company":
		return d.Owner.GetJSON(path).Status
	case "GET people":
		return d.Owner.GetJSON(path + "/contacts").Status
	case "PATCH tags":
		return d.Owner.PatchJSON(path, testkit.JSON{"add_tags": []string{"vip"}}).Status
	case "PATCH name":
		return d.Owner.PatchJSON(path, testkit.JSON{"name": "Globex"}).Status
	case "POST company":
		return d.Owner.PostJSON("/api/v1/companies", testkit.JSON{"name": "Initech"}).Status
	case "POST upsert":
		return d.Owner.PostJSON("/api/v1/companies?upsert=true", testkit.JSON{"external_id": "org_1"}).Status
	default:
		return d.Owner.Delete(path).Status
	}
}
