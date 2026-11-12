package feedback_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
boards lists the help center's boards.
*/
func (b board) boards(t *testing.T) *testkit.Response {
	t.Helper()
	return b.d.Owner.GetJSON("/api/v1/help-centers/" + b.id + "/feedback-boards").AssertOK()
}

/*
newBoardNamed adds a board and returns its id.
*/
func (b board) newBoardNamed(t *testing.T, body testkit.JSON) string {
	t.Helper()
	return b.d.Owner.PostJSON("/api/v1/help-centers/"+b.id+"/feedback-boards", body).AssertCreated().
		String("data.id")
}

func TestFeedbackBoards(t *testing.T) {
	t.Parallel()

	t.Run("gives every help center a default board that holds its requests", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		b.post(t, testkit.JSON{"title": "Dark mode"})

		res := b.boards(t)

		res.AssertJSONPath("data.#", float64(1)).AssertJSONPath("data.0.name", "Feature requests").
			AssertJSONPath("data.0.visibility", "public").AssertJSONPath("data.0.post_count", float64(1))
		b.d.Owner.GetJSON("/api/v1/help-centers/"+b.id+"/feedback-posts").AssertOK().
			AssertJSONPath("data.0.board_id", res.String("data.0.id"))
	})

	t.Run("adds, changes, orders and lists boards, and posts on the one picked", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		first := b.boards(t).String("data.0.id")
		bugs := b.newBoardNamed(t, testkit.JSON{"name": "Bugs", "description": "Something broken?"})
		b.d.Owner.PostJSON("/api/v1/help-centers/"+b.id+"/feedback-boards", testkit.JSON{"name": "Bugs"}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "slug_taken")

		b.d.Owner.PatchJSON("/api/v1/feedback-boards/"+bugs, testkit.JSON{
			"name": "Bug reports", "approval_required": true,
		}).AssertOK().AssertJSONPath("data.name", "Bug reports").AssertJSONPath("data.slug", "bugs").
			AssertJSONPath("data.approval_required", true)
		order := testkit.JSON{"boards": []string{bugs, first}}
		b.d.Owner.PatchJSON("/api/v1/help-centers/"+b.id+"/feedback-boards", order).AssertOK().
			AssertJSONPath("data.0.id", bugs)
		b.post(t, testkit.JSON{"title": "Crash on login", "board_id": bugs})
		b.post(t, testkit.JSON{"title": "Dark mode", "board_id": first})

		b.d.Owner.GetJSON("/api/v1/help-centers/"+b.id+"/feedback-posts?board_id="+bugs).AssertOK().
			AssertJSONPath("meta.total", float64(1)).AssertJSONPath("data.0.title", "Crash on login")
		b.boards(t).AssertJSONPath("data.0.post_count", float64(1)).AssertJSONPath("data.1.post_count", float64(1))
	})

	t.Run("moves a request between boards", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		b.boards(t)
		bugs := b.newBoardNamed(t, testkit.JSON{"name": "Bugs"})
		id := b.post(t, testkit.JSON{"title": "Crash on login"})

		b.d.Owner.PatchJSON("/api/v1/feedback-posts/"+id, testkit.JSON{"board_id": bugs}).AssertOK().
			AssertJSONPath("data.board_id", bugs).AssertJSONPath("data.title", "Crash on login")
	})

	t.Run("moves a deleted board's requests onto the board named and keeps the last one", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		first := b.boards(t).String("data.0.id")
		bugs := b.newBoardNamed(t, testkit.JSON{"name": "Bugs"})
		ideas := b.newBoardNamed(t, testkit.JSON{"name": "Ideas"})
		b.post(t, testkit.JSON{"title": "Crash on login", "board_id": bugs})

		b.d.Owner.Delete("/api/v1/feedback-boards/" + bugs + "?move_to=" + ideas).AssertStatus(fhttp.StatusNoContent)
		b.d.Owner.Delete("/api/v1/feedback-boards/" + first).AssertStatus(fhttp.StatusNoContent)

		b.boards(t).AssertJSONPath("data.#", float64(1)).AssertJSONPath("data.0.post_count", float64(1))
		b.d.Owner.Delete("/api/v1/feedback-boards/"+ideas).AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "last_board")
	})

	t.Run("lets members read boards but only settings managers change them", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		member, _ := b.d.Join(t, enums.RoleMember)

		member.GetJSON("/api/v1/help-centers/" + b.id + "/feedback-boards").AssertOK()
		member.PostJSON("/api/v1/help-centers/"+b.id+"/feedback-boards", testkit.JSON{"name": "Bugs"}).
			AssertForbidden()
	})
}

func TestFeedbackBoardsOnThePortal(t *testing.T) {
	t.Parallel()

	t.Run("shows a tab per board and narrows the roadmap and list to one", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		b.boards(t)
		bugs := b.newBoardNamed(t, testkit.JSON{"name": "Bugs"})
		b.post(t, testkit.JSON{"title": "Crash on login", "board_id": bugs})
		b.post(t, testkit.JSON{"title": "Dark mode"})

		all := string(b.page("/feedback").AssertOK().Body)
		assert.Contains(t, all, `aria-label="Boards"`)
		assert.Contains(t, all, "Crash on login")
		assert.Contains(t, all, "Dark mode")
		one := string(b.page("/feedback?board=bugs").AssertOK().Body)
		assert.Contains(t, one, "Crash on login")
		assert.NotContains(t, one, "Dark mode")
		listed := string(b.page("/feedback?board=bugs&sort=new").AssertOK().Body)
		assert.Contains(t, listed, "Crash on login")
		assert.NotContains(t, listed, "Dark mode")
		b.page("/feedback?board=nowhere").AssertNotFound()
	})

	t.Run("keeps internal boards from customers and private ones from visitors", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		b.boards(t)
		team := b.newBoardNamed(t, testkit.JSON{"name": "Team only", "visibility": "internal"})
		private := b.newBoardNamed(t, testkit.JSON{"name": "Customers", "visibility": "private"})
		b.post(t, testkit.JSON{"title": "Rewrite billing", "board_id": team})
		b.post(t, testkit.JSON{"title": "Priority support", "board_id": private})

		page := string(b.page("/feedback").AssertOK().Body)
		assert.NotContains(t, page, "Rewrite billing")
		assert.NotContains(t, page, "Priority support")
		b.page("/feedback/1-rewrite-billing").AssertNotFound()
		b.page("/feedback/2-priority-support").AssertNotFound()
		b.page("/feedback?board=team-only").AssertNotFound()

		ana := b.reader(t, "ana@example.org")
		signed := string(b.as(ana, "/feedback").AssertOK().Body)
		assert.Contains(t, signed, "Priority support")
		assert.NotContains(t, signed, "Rewrite billing")
		b.as(ana, "/feedback/2-priority-support").AssertOK()
		b.as(ana, "/feedback/1-rewrite-billing").AssertNotFound()
		b.form(ana, "/feedback/1-rewrite-billing/votes", nil).AssertStatus(fhttp.StatusSeeOther)
		b.d.Owner.GetJSON("/api/v1/help-centers/"+b.id+"/feedback-posts?board_id="+team).AssertOK().
			AssertJSONPath("data.0.vote_count", float64(0))
	})

	t.Run("posts a customer's idea on the board they pick, held when that board asks", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		b.boards(t)
		bugs := b.newBoardNamed(t, testkit.JSON{"name": "Bugs", "approval_required": true})
		ana := b.reader(t, "ana@example.org")

		form := string(b.as(ana, "/feedback/new?board=bugs").AssertOK().Body)
		assert.Contains(t, form, `<select name="board"`)
		b.form(ana, "/feedback", url.Values{"title": {"Crash on login"}, "board": {"bugs"}}).
			AssertHeader("Location", "/hc/"+b.subdomain+"/feedback?submitted=pending")
		b.form(ana, "/feedback", url.Values{"title": {"Dark mode"}}).
			AssertHeader("Location", "/hc/"+b.subdomain+"/feedback/2-dark-mode")
		b.form(ana, "/feedback", url.Values{"title": {"Nowhere"}, "board": {"nowhere"}}).AssertNotFound()

		b.d.Owner.GetJSON("/api/v1/help-centers/"+b.id+"/feedback-posts?board_id="+bugs+"&moderation=pending").
			AssertOK().AssertJSONPath("data.0.title", "Crash on login")
	})
}
