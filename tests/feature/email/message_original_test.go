package email_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
originalPath names a message's original on the API.
*/
func originalPath(m models.Message) string {
	return fmt.Sprintf("/api/v1/conversations/%s/messages/%s/original",
		models.PublicID(models.PrefixConversation, m.ConversationID), models.PublicID(models.PrefixMessage, m.ID))
}

func TestMessageOriginal(t *testing.T) {
	t.Parallel()

	htmlMail := func(tc *tests.TestCase, html string) models.Message {
		address := defaultAddress(tc)
		deliver(tc, []byte("From: Bob <bob@example.com>\r\nTo: "+address.ForwardingAddress+"\r\nSubject: Help\r\n"+
			"Message-ID: <"+unique("id")+">\r\nMIME-Version: 1.0\r\nContent-Type: text/html\r\n\r\n"+html), "")

		return messages(tc, conversations(tc)[0].ID)[0]
	}

	t.Run("returns the whole text and html a reader may safely see", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		m := htmlMail(tc, `<div onclick="x()">New words</div><script>alert(1)</script>`+
			`<img src="https://tracker.example.com/open.gif" alt="Logo"><img src="cid:a@b">`+
			`<a href="https://example.com">Site</a>`+
			`<div class="gmail_quote">On Mon wrote:<blockquote>Old words</blockquote></div>`)

		res := tc.ActingAsOwner().GetJSON(originalPath(m)).AssertOK().
			AssertJSONPath("data.object", "message_original").
			AssertJSONPath("data.raw_available", true).
			AssertJSONPath("data.html_too_large", false)

		assert.Equal(t, "New words", strings.TrimSpace(strings.Split(*m.Body, "\n")[0]))
		assert.Contains(t, res.String("data.text"), "Old words")

		html := res.String("data.html")
		assert.Contains(t, html, "Old words")
		assert.Contains(t, html, "[Image: Logo]")
		assert.Contains(t, html, `target="_blank"`)
		assert.Contains(t, html, `rel="noopener noreferrer"`)
		assert.NotContains(t, html, "tracker.example.com")
		assert.NotContains(t, html, "cid:")
		assert.NotContains(t, html, "<script")
		assert.NotContains(t, html, "onclick")
	})

	t.Run("keeps embedded images, which load from nowhere", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		m := htmlMail(tc, `<p>See</p><img src="data:image/png;base64,AAAA" alt="Shot">`)

		assert.Contains(t, tc.ActingAsOwner().GetJSON(originalPath(m)).AssertOK().String("data.html"), "data:image/png")
	})

	t.Run("answers null html for plain text mail", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		m := loneMessage(t, tc, "Hello")

		tc.ActingAsOwner().GetJSON(originalPath(m)).AssertOK().
			AssertJSONPath("data.text", "Hello").
			AssertJSONPath("data.html", nil)
	})

	t.Run("offers the whole of a body too long to show", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		long := strings.Repeat(strings.Repeat("word ", 20)+"\n\n", 1000) + "The last words"
		m := loneMessage(t, tc, long)
		path := "/api/v1/conversations/" + models.PublicID(models.PrefixConversation, m.ConversationID)
		owner := tc.ActingAsOwner()

		owner.GetJSON(path+"/messages").AssertOK().
			AssertJSONPath("data.0.truncated", true).
			AssertJSONPath("data.0.has_original", true)
		assert.NotContains(t, *m.Body, "The last words")
		assert.Contains(t, owner.GetJSON(originalPath(m)).AssertOK().String("data.text"), "The last words")
	})

	t.Run("finds nothing for a note, a deleted message or a message of another conversation", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		m := loneMessage(t, tc, "Hello")
		owner := tc.ActingAsOwner()
		path := "/api/v1/conversations/" + models.PublicID(models.PrefixConversation, m.ConversationID)
		note := owner.PostJSON(path+"/notes", testkit.JSON{"body": "Private"}).AssertCreated().String("data.id")

		owner.GetJSON(path + "/messages/" + note + "/original").AssertNotFound()
		owner.GetJSON(path + "/messages/msg_nope/original").AssertNotFound()
		owner.GetJSON(path + "/messages/msg_nope/original/raw").AssertNotFound()

		second := owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": models.PublicID(models.PrefixInbox, conversations(tc)[0].InboxID), "author": "contact",
			"subject": "Other", "body": "Other", "contact": testkit.JSON{"email": "ann@example.com"},
		}).AssertCreated().String("data.id")
		owner.GetJSON("/api/v1/conversations/" + second + "/messages/" +
			models.PublicID(models.PrefixMessage, m.ID) + "/original").AssertNotFound()

		_, err := tc.App.DB.Exec(t.Context(), "UPDATE messages SET deleted_at = now() WHERE id = $1", m.ID)
		require.NoError(t, err)

		owner.GetJSON(originalPath(m)).AssertNotFound()
		owner.GetJSON(originalPath(m) + "/raw").AssertNotFound()
		owner.GetJSON(path+"/messages").AssertOK().
			AssertJSONPath("data.0.has_original", false).
			AssertJSONPath("data.0.body", nil)
	})

	t.Run("refuses another workspace's message", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		m := loneMessage(t, tc, "Hello")

		other, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)

		stranger := tc.ActingAsOwner().WithHeader("Dimmi-Workspace",
			models.PublicID(models.PrefixWorkspace, other.Workspace.ID))
		stranger.GetJSON(originalPath(m)).AssertNotFound()
		stranger.GetJSON(originalPath(m) + "/raw").AssertNotFound()

		outsider := tc.CreateUser()
		tc.ActingAs(outsider).WithHeader("Dimmi-Workspace",
			models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)).
			GetJSON(originalPath(m)).AssertStatus(404)
	})

	t.Run("downloads the raw email as an inert attachment", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		m := htmlMail(tc, `<script>alert(1)</script><div style="display:none">Hidden</div><p>Visible</p>`)

		res := tc.ActingAsOwner().GetJSON(originalPath(m)+"/raw").AssertOK().
			AssertHeader("Content-Type", "application/octet-stream").
			AssertHeader("X-Content-Type-Options", "nosniff").
			AssertHeader("Cache-Control", "no-store").
			AssertHeader("Content-Security-Policy", "default-src 'none'; sandbox").
			AssertHeader("Content-Disposition",
				`attachment; filename="`+models.PublicID(models.PrefixMessage, m.ID)+`.eml"`)

		assert.Contains(t, string(res.Body), "Hidden")
		assert.Contains(t, string(res.Body), "Subject: Help")
	})

	t.Run("keeps raw mail from a member who may read the conversation but not raw mail", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		m := loneMessage(t, tc, "Hello")
		user := tc.CreateUser()
		_, err := factories.Member(t.Context(), tc.App, factories.Membership{
			Workspace: tc.Workspace.Workspace, User: user, Role: enums.RoleMember,
		})
		require.NoError(t, err)
		member := tc.ActingAs(user).WithHeader("Dimmi-Workspace",
			models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID))

		member.GetJSON(originalPath(m)).AssertOK().
			AssertJSONPath("data.text", "Hello").
			AssertJSONPath("data.raw_available", false)
		member.GetJSON(originalPath(m) + "/raw").AssertForbidden()
	})

	t.Run("needs a token with the right to read conversations", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		m := loneMessage(t, tc, "Hello")
		user := tc.CreateUser()
		member, err := factories.Member(t.Context(), tc.App, factories.Membership{
			Workspace: tc.Workspace.Workspace, User: user, Role: enums.RoleAdmin,
		})
		require.NoError(t, err)

		tc.ActingAsToken(member, "contacts:read").GetJSON(originalPath(m)).AssertForbidden()
		tc.ActingAsToken(member, "contacts:read").GetJSON(originalPath(m) + "/raw").AssertForbidden()
		tc.ActingAsToken(member, "conversations:read").GetJSON(originalPath(m)).AssertOK().
			AssertJSONPath("data.raw_available", false)
		tc.ActingAsToken(member, "conversations:read").GetJSON(originalPath(m) + "/raw").AssertForbidden()
		tc.ActingAsToken(member, "conversations:read", "settings:read").GetJSON(originalPath(m) + "/raw").AssertOK()
	})

	t.Run("says the raw email is gone once retention purged it", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		m := loneMessage(t, tc, "Hello")
		_, err := tc.App.DB.Exec(t.Context(), "UPDATE inbound_emails SET raw_path = NULL, raw_deleted_at = now()")
		require.NoError(t, err)
		owner := tc.ActingAsOwner()

		owner.GetJSON(originalPath(m)).AssertOK().AssertJSONPath("data.raw_available", false)
		owner.GetJSON(originalPath(m) + "/raw").AssertNotFound()
	})

	t.Run("says the raw email is gone when its file is", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		m := loneMessage(t, tc, "Hello")
		_, err := tc.App.DB.Exec(t.Context(), "UPDATE inbound_emails SET raw_path = 'nowhere/gone.eml'")
		require.NoError(t, err)

		tc.ActingAsOwner().GetJSON(originalPath(m) + "/raw").AssertNotFound()
	})
}
