package contacts_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/tests"
)

func TestContactRefusals(t *testing.T) {
	t.Parallel()

	t.Run("refuses a membership without the contact permissions", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acts := d.TC.App.Actions.Inbox
		none := authz.NewMembership(d.TC.Workspace.Workspace, d.TC.Workspace.Owner, nil)
		reader := authz.NewMembership(d.TC.Workspace.Workspace, d.TC.Workspace.Owner, []string{"conversations:read"})
		show := contacts.ShowContactInput{Membership: none, ContactID: ids.New()}
		ctx := t.Context()

		require.ErrorIs(t, acts.DeleteContact.Handle(ctx, show), exceptions.ErrForbidden)
		_, err := acts.FindDuplicates.Handle(ctx, show)
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.ListIdentities.Handle(ctx, show)
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		err = acts.DetachIdentity.Handle(ctx, contacts.DetachIdentityInput{Membership: none, ContactID: ids.New()})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.UpsertContact.Handle(ctx, contacts.UpsertInput{Membership: none})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		for _, m := range []authz.Membership{none, reader} {
			ref := conversations.Ref{ID: ids.New()}
			_, err = acts.ShowContext.Handle(ctx, conversations.ContextInput{Membership: m, Ref: ref})
			require.ErrorIs(t, err, exceptions.ErrForbidden)
		}
	})

	t.Run("moves a known user to the company the upsert names", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		upsert := "/api/v1/contacts?upsert=true"
		id := d.Owner.PostJSON(upsert, testkit.JSON{"external_id": "user_1"}).AssertCreated().String("data.id")

		d.Owner.PostJSON(upsert, testkit.JSON{
			"external_id": "user_1", "company": testkit.JSON{"external_id": "org_1", "name": "Acme"},
		}).AssertOK().
			AssertJSONPath("data.id", id).
			AssertJSONPath("data.company.name", "Acme")
	})
}

func TestContactFailures(t *testing.T) {
	t.Parallel()

	for table, requests := range map[string][]string{
		"contact_tag":        {"GET contact", "PATCH tags", "POST upsert", "GET context"},
		"companies":          {"POST upsert company", "GET context"},
		"contact_identities": {"GET identities", "DELETE identity", "DELETE contact", "GET context"},
		"tags":               {"PATCH tags"},
		"conversations":      {"DELETE contact"},
	} {
		t.Run("answers 500 when "+table+" cannot be used", func(t *testing.T) {
			t.Parallel()
			f := newContactFixture(t)
			f.d.TC.BreakTable(table)

			for _, request := range requests {
				assert.Equal(t, 500, f.send(request), request)
			}
		})
	}

	for _, write := range []struct{ table, kind, request string }{
		{"contacts", "UPDATE", "POST upsert"},
		{"contacts", "UPDATE", "DELETE contact"},
		{"contact_tag", "INSERT", "PATCH tags"},
		{"contact_tag", "DELETE", "PATCH untag"},
		{"contact_identities", "DELETE", "DELETE identity"},
		{"contact_identities", "DELETE", "DELETE contact"},
	} {
		t.Run("answers 500 when "+write.table+" refuses "+write.kind+" on "+write.request, func(t *testing.T) {
			t.Parallel()
			f := newContactFixture(t)
			f.d.TC.FailWrites(write.table, write.kind)

			assert.Equal(t, 500, f.send(write.request), write.request)
		})
	}
}

/*
contactFixture is a desk with a tagged contact who has no conversation,
and a conversation of another customer.
*/
type contactFixture struct {
	d        tests.Desk
	path     string
	tag      string
	identity string
	conv     string
}

/*
newContactFixture adds the contact by external id and starts the other
customer's conversation.
*/
func newContactFixture(t *testing.T) contactFixture {
	t.Helper()

	d := tests.NewDesk(t)
	res := d.Owner.PostJSON("/api/v1/contacts?upsert=true", testkit.JSON{
		"external_id": "user_1", "email": "ann@acme.dev",
	}).AssertCreated()
	path := "/api/v1/contacts/" + res.String("data.id")
	tag := d.Owner.PatchJSON(path, testkit.JSON{"add_tags": []string{"vip"}}).AssertOK().String("data.tags.0.id")
	identity := d.Owner.GetJSON(path + "/identities").AssertOK().String("data.0.id")
	conv := d.Customer(t, "bob", "Hi").String("data.id")

	return contactFixture{d: d, path: path, tag: tag, identity: identity, conv: conv}
}

/*
send makes one of the named requests and returns its status.
*/
func (f contactFixture) send(request string) int {
	owner, upsert := f.d.Owner, "/api/v1/contacts?upsert=true"

	switch request {
	case "GET contact":
		return owner.GetJSON(f.path).Status
	case "GET identities":
		return owner.GetJSON(f.path + "/identities").Status
	case "GET context":
		return owner.GetJSON("/api/v1/conversations/" + f.conv + "/context").Status
	case "PATCH tags":
		return owner.PatchJSON(f.path, testkit.JSON{"add_tags": []string{"new"}}).Status
	case "PATCH untag":
		return owner.PatchJSON(f.path, testkit.JSON{"remove_tags": []string{f.tag}}).Status
	case "POST upsert":
		return owner.PostJSON(upsert, testkit.JSON{"external_id": "user_1", "name": "Ann"}).Status
	case "POST upsert company":
		return owner.PostJSON(upsert, testkit.JSON{
			"external_id": "user_1", "company": testkit.JSON{"external_id": "org_1", "name": "Acme"},
		}).Status
	case "DELETE identity":
		return owner.Delete(f.path + "/identities/" + f.identity).Status
	default:
		return owner.Delete(f.path).Status
	}
}
