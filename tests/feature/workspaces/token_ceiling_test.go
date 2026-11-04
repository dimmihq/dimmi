package workspaces_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

func TestTokenCeiling(t *testing.T) {
	t.Parallel()

	t.Run("writes out everything a member may do when asked for it, never widening on promotion", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		res := in(tc, user).PostJSON(patPath, testkit.JSON{"name": "All I can", "abilities": []string{"*"}}).
			AssertCreated()
		res.AssertJSONMissing(`data.abilities.#(=="*")`)
		token := tc.Client().WithHeader("Authorization", "Bearer "+res.String("data.plain_text_token"))
		token.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Mine now"}).AssertForbidden()

		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "admin"}).AssertOK()

		in(tc, user).PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Mine by right"}).AssertOK()
		token.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Mine now"}).AssertForbidden()
	})

	t.Run("never widens a token that named its groups when its member is promoted", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		res := in(tc, user).PostJSON(patPath, testkit.JSON{"name": "Mine", "abilities": []string{"settings:write"}}).
			AssertCreated()
		token := tc.Client().WithHeader("Authorization", "Bearer "+res.String("data.plain_text_token"))

		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "admin"}).AssertOK()

		token.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Mine now"}).AssertForbidden()
	})

	t.Run("gives back what a token was minted with when a demoted member returns to that role", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleAdmin)
		res := in(tc, user).PostJSON(patPath, testkit.JSON{"name": "Mine", "abilities": []string{"*"}}).AssertCreated()
		token := tc.Client().WithHeader("Authorization", "Bearer "+res.String("data.plain_text_token"))
		path := "/api/v1/members/" + memberID(member)

		in(tc, tc.Owner).PatchJSON(path, testkit.JSON{"role": "member"}).AssertOK()
		token.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Mine now"}).AssertForbidden()

		in(tc, tc.Owner).PatchJSON(path, testkit.JSON{"role": "admin"}).AssertOK()
		token.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Mine again"}).AssertOK()
	})

	t.Run("gives an owner's token asked for everything every group", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		res := in(tc, tc.Owner).PostJSON(patPath, testkit.JSON{"name": "All", "abilities": []string{"*"}}).
			AssertCreated()

		res.AssertJSONMissing(`data.abilities.#(=="*")`)
		tc.Client().WithHeader("Authorization", "Bearer "+res.String("data.plain_text_token")).
			PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Still mine"}).AssertOK()
	})
}
