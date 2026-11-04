package privacy_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/erasures"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
requested opens a conversation, asks for its customer to be erased and
returns the erasure's id.
*/
func requested(t *testing.T, d tests.Desk) string {
	t.Helper()

	_, contact := customer(t, d)

	return d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertStatus(202).String("data.id")
}

/*
runErase runs the erase job's work on the request and returns its error.
*/
func runErase(t *testing.T, tc *tests.TestCase, erasure string) error {
	t.Helper()

	id, err := tc.App.IDs.Decode(string(models.PrefixContactErasure), erasure)
	require.NoError(t, err)

	_, err = tc.App.Actions.Inbox.EraseContact.Handle(t.Context(), erasures.EraseArgs{
		WorkspaceID: tc.Workspace.Workspace.ID, ErasureID: id,
	})

	return err
}

func TestErasureRefusals(t *testing.T) {
	t.Parallel()

	t.Run("refuses a membership that may not erase customers", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		acts := d.TC.App.Actions.Inbox
		none := authz.NewMembership(d.TC.Workspace.Workspace, d.TC.Workspace.Owner, []string{"contacts:write"})
		ctx := t.Context()

		_, err := acts.RequestErasure.Handle(ctx, erasures.RequestErasureInput{Membership: none})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.CancelErasure.Handle(ctx, erasures.CancelErasureInput{Membership: none})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.ListErasures.Handle(ctx, erasures.ErasureInput{Membership: none})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.ShowErasure.Handle(ctx, erasures.ErasureInput{Membership: none})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
	})

	t.Run("puts the job off while the request waits or is held, and ends it once done", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.Retention = holdAll{} }))
		inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
		require.NoError(t, err)
		d := tests.Desk{TC: tc, Inbox: inbox, Owner: tc.In(tc.Owner)}
		erasure := requested(t, d)
		id, err := tc.App.IDs.Decode(string(models.PrefixContactErasure), erasure)
		require.NoError(t, err)
		job := &jobs.EraseContact{Erase: tc.App.Actions.Inbox.EraseContact}
		args := erasures.EraseArgs{WorkspaceID: tc.Workspace.Workspace.ID, ErasureID: id}
		run := &queue.Job[erasures.EraseArgs]{Args: args}

		require.Error(t, job.Handle(t.Context(), run))
		tc.Clock.Travel(8 * 24 * time.Hour)
		require.Error(t, job.Handle(t.Context(), run))

		run.Args.ErasureID = ids.New()
		require.NoError(t, job.Handle(t.Context(), run))

		tc.BreakTable("contact_erasures")
		require.ErrorContains(t, job.Handle(t.Context(), run), "find erasure")
	})

	t.Run("leaves a request that is gone or closed alone", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		erasure := requested(t, d)
		d.Owner.PatchJSON("/api/v1/contact-erasures/"+erasure, testkit.JSON{"status": "canceled"}).AssertOK()

		require.NoError(t, runErase(t, d.TC, erasure))
	})
}

func TestErasureFailures(t *testing.T) {
	t.Parallel()

	t.Run("fails requests while the requests cannot be read or written", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		erasure := requested(t, d)
		_, contact := customer(t, tests.Desk{TC: d.TC, Inbox: d.Inbox, Owner: d.Owner})
		d.TC.BreakTable("contact_erasures")

		d.Owner.GetJSON("/api/v1/contact-erasures").AssertStatus(500)
		d.Owner.GetJSON("/api/v1/contact-erasures/" + erasure).AssertStatus(500)
		d.Owner.PatchJSON("/api/v1/contact-erasures/"+erasure, testkit.JSON{"status": "canceled"}).AssertStatus(500)
		d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertStatus(500)
		require.Error(t, runErase(t, d.TC, erasure))
	})

	for name, write := range map[string][2]string{
		"hiding the contact":     {"contacts", "UPDATE"},
		"freeing the identities": {"contact_identities", "DELETE"},
		"reading the identities": {"contact_identities", "SELECT"},
	} {
		t.Run("fails a request when "+name+" fails", func(t *testing.T) {
			t.Parallel()
			d := tests.NewDesk(t)
			_, contact := customer(t, d)
			if write[1] == "SELECT" {
				d.TC.BreakTable(write[0])
			} else {
				d.TC.FailWrites(write[0], write[1])
			}

			d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertStatus(500)
		})
	}

	for name, write := range map[string][2]string{
		"showing the contact again": {"contacts", "UPDATE"},
		"giving an identity back":   {"contact_identities", "INSERT"},
		"closing the request":       {"contact_erasures", "UPDATE"},
		"looking an identity up":    {"contact_identities", "SELECT"},
	} {
		t.Run("fails a cancel when "+name+" fails", func(t *testing.T) {
			t.Parallel()
			d := tests.NewDesk(t)
			erasure := requested(t, d)
			if write[1] == "SELECT" {
				d.TC.BreakTable(write[0])
			} else {
				d.TC.FailWrites(write[0], write[1])
			}

			d.Owner.PatchJSON("/api/v1/contact-erasures/"+erasure, testkit.JSON{"status": "canceled"}).AssertStatus(500)
		})
	}

	t.Run("fails a cancel whose sealed identities cannot be opened", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		erasure := requested(t, d)
		_, err := d.TC.App.DB.Exec(t.Context(), `UPDATE contact_erasures SET identities_stash = 'garbage'`)
		require.NoError(t, err)

		d.Owner.PatchJSON("/api/v1/contact-erasures/"+erasure, testkit.JSON{"status": "canceled"}).AssertStatus(500)
	})

	for name, write := range map[string][2]string{
		"reading the files":          {"attachments", "SELECT"},
		"deleting the files":         {"attachments", "DELETE"},
		"emptying the messages":      {"messages", "UPDATE"},
		"making the placeholder":     {"contacts", "INSERT"},
		"handing over conversations": {"conversations", "UPDATE"},
		"deleting the contact":       {"contacts", "DELETE"},
		"closing the request":        {"contact_erasures", "UPDATE"},
	} {
		t.Run("retries the erasure when "+name+" fails", func(t *testing.T) {
			t.Parallel()
			d := tests.NewDesk(t)
			erasure := requested(t, d)
			d.TC.Clock.Travel(8 * 24 * time.Hour)
			if write[1] == "SELECT" {
				d.TC.BreakTable(write[0])
			} else {
				d.TC.FailWrites(write[0], write[1])
			}

			require.Error(t, runErase(t, d.TC, erasure))
		})
	}

	t.Run("retries the erasure when a file's disk is unknown", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path, contact := customer(t, d)
		storeAttachment(t, d.TC, path)
		_, err := d.TC.App.DB.Exec(t.Context(), `UPDATE attachments SET disk = 'nowhere'`)
		require.NoError(t, err)
		erasure := d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertStatus(202).String("data.id")
		d.TC.Clock.Travel(8 * 24 * time.Hour)

		require.ErrorContains(t, runErase(t, d.TC, erasure), "open disk nowhere")
	})

	t.Run("retries the erasure when the disk refuses to delete a file", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		path, contact := customer(t, d)
		storeAttachment(t, d.TC, path)
		_, err := d.TC.App.DB.Exec(t.Context(), `UPDATE attachments SET path = '../outside.txt'`)
		require.NoError(t, err)
		erasure := d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertStatus(202).String("data.id")
		d.TC.Clock.Travel(8 * 24 * time.Hour)

		require.ErrorContains(t, runErase(t, d.TC, erasure), "delete file")
	})

	t.Run("fails a cancel whose sealed identities are not a list", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, contact := customer(t, d)
		erasure := d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertStatus(202).String("data.id")
		contactID, err := d.TC.App.IDs.Decode(string(models.PrefixContact), contact)
		require.NoError(t, err)
		scope := encryption.Scope{Kind: "workspace", ID: d.TC.Workspace.Workspace.ID.String()}
		sealed, err := d.TC.App.Keys.Encrypt(t.Context(), scope, []byte("not a list"),
			[]byte("contact_erasures.identities_stash:"+contactID.String()))
		require.NoError(t, err)
		_, err = d.TC.App.DB.Exec(t.Context(), `UPDATE contact_erasures SET identities_stash = $1`, sealed)
		require.NoError(t, err)

		d.Owner.PatchJSON("/api/v1/contact-erasures/"+erasure, testkit.JSON{"status": "canceled"}).AssertStatus(500)
	})

	t.Run("retries the erasure when the hold cannot be recorded", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.Retention = holdAll{} }))
		inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
		require.NoError(t, err)
		d := tests.Desk{TC: tc, Inbox: inbox, Owner: tc.In(tc.Owner)}
		erasure := requested(t, d)
		tc.Clock.Travel(8 * 24 * time.Hour)
		tc.FailWrites("contact_erasures", "UPDATE")

		require.Error(t, runErase(t, tc, erasure))
	})
}
