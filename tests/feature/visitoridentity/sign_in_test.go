package visitoridentity_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
startConversation has the visitor write their first message and returns
the conversation's public id.
*/
func startConversation(t *testing.T, v *visitor, body string) string {
	t.Helper()
	res := v.client().PostJSON("/widget/v1/conversations", testkit.JSON{"body": body, "client_id": ids.New().String()}).
		AssertCreated()
	return res.String("data.id")
}

func TestVisitorSignIn(t *testing.T) {
	t.Parallel()

	t.Run("signs a visitor in as the customer the token names", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		res := d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{
			"sub": "user_42", "email": "ada@acme.test", "email_verified": true, "name": "Ada Lovelace",
			"company":           map[string]any{"id": "org_7", "name": "Acme"},
			"custom_attributes": map[string]any{"plan": "pro", "seats": 12},
		}), nil).AssertCreated()
		res.AssertJSONPath("data.kind", "identified").AssertJSONPath("data.identity.status", "verified")

		c := d.contact(res.String("data.contact.id"))
		assert.Equal(t, "user", c.Kind)
		assert.Equal(t, "user_42", *c.ExternalID)
		assert.Equal(t, "ada@acme.test", *c.Email)
		assert.True(t, c.EmailVerified)
		assert.Equal(t, "Ada Lovelace", *c.Name)
		assert.NotNil(t, c.CompanyID)
		assert.ElementsMatch(t, []string{"plan", "seats"}, c.TrustedAttributeKeys)
		s0 := d.session(res.String("data.id"))
		assert.Equal(t, "user_42", *s0.ExternalID)
		assert.Equal(t, d.tc.Clock.Now().Add(24*time.Hour).Unix(), s0.ExpiresAt.Unix())
	})

	t.Run("trusts a token's email only when it says it is verified", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		res := d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1", "email": "bob@acme.test"}), nil).
			AssertCreated()
		c := d.contact(res.String("data.contact.id"))
		assert.Equal(t, "bob@acme.test", *c.Email)
		assert.False(t, c.EmailVerified)
		d.inWorkspace(func(ctx context.Context, q *models.Queries) error {
			_, err := q.FindContactByIdentity(ctx, models.FindContactByIdentityParams{
				WorkspaceID: c.WorkspaceID, Type: "email", Value: "bob@acme.test",
			})
			assert.Error(t, err)
			return nil
		})
	})

	t.Run("makes one customer across sessions and widgets", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		a := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_5"}), nil)
		b := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_5"}), nil)
		assert.Equal(t, a.contact, b.contact)
		assert.NotEqual(t, a.id, b.id)
	})

	t.Run("turns the anonymous lead into the customer, keeping their conversation", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		lead := d.anonymous()
		conv := startConversation(t, lead, "where is my order")

		v := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_8"}), lead)
		assert.Equal(t, lead.contact, v.contact)
		assert.Equal(t, "user", d.contact(v.contact).Kind)
		v.client().GetJSON("/widget/v1/conversations").AssertOK().AssertJSONPath("data.0.id", conv)
		lead.client().GetJSON("/widget/v1/conversations").AssertUnauthorized()
		assert.Equal(t, "identity_switch", *d.session(lead.id).RevokeReason)
	})

	t.Run("merges a lead into a customer it signs in as", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		known := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_3"}), nil)
		before := startConversation(t, known, "first question")
		lead := d.anonymous()
		later := startConversation(t, lead, "second question")

		v := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_3"}), lead)
		assert.Equal(t, known.contact, v.contact)
		list := v.client().GetJSON("/widget/v1/conversations").AssertOK()
		var body struct {
			Data []struct{ ID string } `json:"data"`
		}
		list.Decode(&body)
		got := []string{}
		for _, c := range body.Data {
			got = append(got, c.ID)
		}
		assert.ElementsMatch(t, []string{before, later}, got)
		merged := d.contact(lead.contact)
		require.NotNil(t, merged.MergedIntoID)
	})

	t.Run("adds what a lead did to the customer it signs in as", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		known := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_4"}), nil)
		lead := d.anonymous()
		track := func(contact string, metadata testkit.JSON) {
			d.owner.PostJSON("/api/v1/contact-events", testkit.JSON{
				"name": "report.exported", "contact_id": contact, "metadata": metadata,
			}).AssertOK().AssertJSONPath("data.accepted", float64(1))
		}
		track(known.contact, testkit.JSON{"format": "csv"})
		d.tc.Clock.Advance(time.Hour)
		track(lead.contact, testkit.JSON{"format": "pdf"})
		track(lead.contact, nil)

		d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_4"}), lead)

		d.owner.GetJSON("/api/v1/contacts/"+known.contact+"/event-summaries").AssertOK().
			AssertJSONPath("data.#", float64(1)).AssertJSONPath("data.0.count", float64(3)).
			AssertJSONPath("data.0.last_7_days", float64(3))
		d.owner.GetJSON("/api/v1/contacts/"+lead.contact+"/event-summaries").AssertOK().
			AssertJSONPath("data.#", float64(0))
	})

	t.Run("extends a session already signed in as the same customer", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		v := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_2"}), nil)
		d.tc.Clock.Advance(time.Hour)
		d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_2"}), v).AssertOK().
			AssertJSONPath("data.id", v.id).AssertJSONMissing("data.token")
		assert.Equal(t, d.tc.Clock.Now().Add(24*time.Hour).Unix(), d.session(v.id).ExpiresAt.Unix())
	})

	t.Run("switches customers without ever merging one into another", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		first := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{
			"sub": "user_a", "email": "same@acme.test", "email_verified": true,
		}), nil)
		second := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{
			"sub": "user_b", "email": "same@acme.test", "email_verified": true,
		}), first)
		assert.NotEqual(t, first.contact, second.contact)
		assert.Nil(t, d.contact(first.contact).MergedIntoID)
		assert.Equal(t, "identity_switch", *d.session(first.id).RevokeReason)
	})

	t.Run("refreshes a signed-in session only for the same customer", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		v := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_4"}), nil)
		d.tc.Clock.Advance(25 * time.Hour)
		v.client().GetJSON("/widget/v1/conversations").AssertUnauthorized().AssertJSONPath("code", "session_expired")

		v.client().PatchJSON("/widget/v1/session", testkit.JSON{
			"user_jwt": s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_4", "name": "Grace"}),
		}).AssertOK().AssertJSONPath("data.id", v.id)
		v.client().GetJSON("/widget/v1/conversations").AssertOK()
		assert.Equal(t, "Grace", *d.contact(v.contact).Name)

		v.client().PatchJSON("/widget/v1/session", testkit.JSON{
			"user_jwt": s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "someone_else"}),
		}).AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "identity_mismatch")
	})

	t.Run("never resumes a signed-in session without a fresh token", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		v := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_6"}), nil)
		v.client().GetJSON("/widget/v1/session").AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "identity_proof_required")
	})

	t.Run("marks a signed-in visitor's messages verified for the agent", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		v := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_7"}), nil)
		startConversation(t, v, "refund please")
		var meta json.RawMessage
		require.NoError(t, d.tc.Operator().QueryRow(t.Context(),
			`SELECT metadata FROM messages WHERE workspace_id = $1 ORDER BY created_at DESC LIMIT 1`,
			d.tc.Workspace.Workspace.ID).Scan(&meta))
		var stamped struct {
			Widget struct {
				IdentityVerified bool `json:"identity_verified"`
			} `json:"widget"`
		}
		require.NoError(t, json.Unmarshal(meta, &stamped))
		assert.True(t, stamped.Widget.IdentityVerified)
	})
}
