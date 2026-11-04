package workspaces_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

const patPath = "/api/v1/tokens"

func TestPersonalAccessTokens(t *testing.T) {
	t.Parallel()

	t.Run("mints a token, shows its secret once and keeps it out of caches", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := in(tc, tc.Owner)

		res := browser.PostJSON(patPath, testkit.JSON{
			"name": "  CI deploy ", "abilities": []string{"members:read", "conversations:read"},
		}).
			AssertCreated().
			AssertHeader("Cache-Control", "no-store").
			AssertJSONPath("data.object", "personal_access_token").
			AssertJSONPath("data.name", "CI deploy").
			AssertJSONPath("data.abilities", []string{"conversations:read", "members:read"}).
			AssertJSONPath("data.expires_at", nil).
			AssertJSONPath("data.last_used_at", nil).
			AssertJSONPath("data.member.id", memberID(tc.Workspace.Owner)).
			AssertJSONPath("data.member.name", tc.Owner.Name)

		plain := res.String("data.plain_text_token")
		assert.True(t, strings.HasPrefix(plain, "dmi_live_"))
		assert.Len(t, plain, 47)
		assert.Equal(t, plain[:13], res.String("data.prefix"))

		browser.GetJSON(patPath).AssertOK().
			AssertJSONPath("data.0.id", res.String("data.id")).
			AssertJSONMissing("data.0.plain_text_token")
		browser.GetJSON(patPath + "/" + res.String("data.id")).AssertOK().
			AssertJSONMissing("data.plain_text_token")

		tc.Client().WithHeader("Authorization", "Bearer "+plain).GetJSON("/api/v1/me").AssertOK().
			AssertJSONPath("data.auth.type", "personal_access_token").
			AssertJSONPath("data.auth.token.id", res.String("data.id")).
			AssertJSONPath("data.auth.token.name", "CI deploy").
			AssertJSONPath("data.auth.token.prefix", plain[:13]).
			AssertJSONPath("data.auth.abilities", []string{"conversations:read", "members:read"})
	})

	t.Run("mints a test token in a test workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		exec(t, tc, "UPDATE workspaces SET test_mode = true WHERE id = $1", tc.Workspace.Workspace.ID)

		plain := in(tc, tc.Owner).PostJSON(patPath, testkit.JSON{"name": "Sandbox", "abilities": []string{"*"}}).
			AssertCreated().
			AssertJSONPath("data.abilities", []string{"*"}).
			String("data.plain_text_token")

		assert.True(t, strings.HasPrefix(plain, "dmi_test_"))
		tc.Client().WithHeader("Authorization", "Bearer "+plain).GetJSON("/api/v1/members").AssertOK()
	})

	t.Run("lets the token reach workspace routes with only its abilities", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		token := bearer(t, tc, in(tc, tc.Owner), "members:read")

		token.GetJSON("/api/v1/members").AssertOK()
		token.GetJSON("/api/v1/workspace").AssertOK()
		token.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Renamed"}).
			AssertForbidden().
			AssertJSONPath("code", "missing_ability")
	})

	t.Run("keeps tokens away from routes only the user may reach", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := in(tc, tc.Owner)
		id := browser.PostJSON(patPath, testkit.JSON{"name": "CI", "abilities": []string{"*"}}).String("data.id")
		token := bearer(t, tc, browser, "*")

		for _, res := range []*testkit.Response{
			token.GetJSON(patPath),
			token.PostJSON(patPath, testkit.JSON{"name": "Escalate", "abilities": []string{"*"}}),
			token.GetJSON(patPath + "/abilities"),
			token.Delete(patPath + "/" + id),
			token.GetJSON("/api/v1/sessions"),
			token.GetJSON("/api/v1/user/auth-factors"),
		} {
			res.AssertForbidden().AssertJSONPath("code", "first_party_only")
		}
	})

	t.Run("refuses abilities the role holds nothing of, naming them", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, _ := join(t, tc, enums.RoleMember)

		res := in(tc, user).PostJSON(patPath, testkit.JSON{
			"name": "Too much", "abilities": []string{"conversations:read", "billing:read", "made:up", "billing:read"},
		}).
			AssertValidationError("abilities").
			AssertJSONPath("code", "ability_not_allowed")

		assert.JSONEq(t, `["billing:read","made:up"]`, res.String("abilities"))
		tc.Table("personal_access_tokens").AssertCount(0)
	})

	t.Run("lists every ability group, allowed only where the role holds a permission", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, _ := join(t, tc, enums.RoleMember)

		res := in(tc, user).GetJSON(patPath+"/abilities").AssertOK().
			AssertJSONPath("data.0.object", "token_ability").
			AssertJSONPath("data.0.ability", "conversations:read").
			AssertJSONPath("data.0.allowed", true)

		var body struct {
			Data []struct {
				Ability     string   `json:"ability"`
				Permissions []string `json:"permissions"`
				Allowed     bool     `json:"allowed"`
			} `json:"data"`
		}
		res.Decode(&body)

		allowed := map[string]bool{}
		for _, group := range body.Data {
			allowed[group.Ability] = group.Allowed
		}

		assert.False(t, allowed["billing:read"])
		assert.False(t, allowed["members:write"])
		assert.True(t, allowed["members:read"])
		assert.NotEmpty(t, body.Data[0].Permissions)
	})

	t.Run("shrinks a token the moment its member is demoted", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleAdmin)
		token := bearer(t, tc, in(tc, user), "settings:write")

		token.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Acme"}).AssertOK()
		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "member"}).AssertOK()

		token.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Acme again"}).AssertForbidden()
	})

	t.Run("refuses an expiry in the past or more than five years away", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := in(tc, tc.Owner)

		for _, at := range []time.Time{tests.Epoch(), tests.Epoch().AddDate(5, 1, 0)} {
			browser.PostJSON(patPath, testkit.JSON{"name": "CI", "abilities": []string{"*"}, "expires_at": at}).
				AssertValidationError("expires_at").
				AssertJSONPath("code", "invalid_expiry")
		}

		expires := tests.Epoch().Add(90 * 24 * time.Hour)
		browser.PostJSON(patPath, testkit.JSON{"name": "CI", "abilities": []string{"*"}, "expires_at": expires}).
			AssertCreated().
			AssertJSONPath("data.expires_at", expires.Format(time.RFC3339))
	})

	t.Run("refuses a token past the member's limit, revoked ones not counted", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_TOKENS__MAX_PER_MEMBER=2"))
		browser := in(tc, tc.Owner)
		mint := func() *testkit.Response {
			return browser.PostJSON(patPath, testkit.JSON{"name": "CI", "abilities": []string{"*"}})
		}

		first := mint().AssertCreated().String("data.id")
		mint().AssertCreated()
		mint().AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "token_limit_reached")

		browser.Delete(patPath + "/" + first).AssertNoContent()
		mint().AssertCreated()
	})

	t.Run("needs a recent confirmation to mint", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := in(tc, tc.Owner)
		tc.Clock.Advance(tc.App.Config.Auth.PasswordConfirmation.TTL + time.Second)

		browser.PostJSON(patPath, testkit.JSON{"name": "CI", "abilities": []string{"*"}}).
			AssertStatus(fhttp.StatusLocked).
			AssertJSONPath("code", "password_confirmation_required")
	})

	t.Run("refuses a name that is blank once cleaned, or no abilities", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := in(tc, tc.Owner)

		browser.PostJSON(patPath, testkit.JSON{"name": "\u200b ", "abilities": []string{"*"}}).
			AssertValidationError("name")
		browser.PostJSON(patPath, testkit.JSON{"name": "CI", "abilities": []string{}}).
			AssertValidationError("abilities")
	})

	t.Run("mails the owner and writes their security log", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		created := record[appevents.TokenCreated](tc)

		id := in(tc, tc.Owner).PostJSON(patPath, testkit.JSON{"name": "CI", "abilities": []string{"members:read"}}).
			AssertCreated().String("data.id")

		mails, messages := tests.Sent[appmail.TokenCreatedMail](tc, fake)
		require.Len(t, mails, 1)
		assert.Equal(t, "CI", mails[0].Token)
		assert.Equal(t, []string{"members:read"}, mails[0].Abilities)
		assert.Equal(t, []string{tc.Owner.Email}, tests.SentTo(messages[0]))
		assert.Contains(t, messages[0].Message.HTML, "/d/settings/tokens")
		assert.NotContains(t, messages[0].Message.HTML, "dmi_live_")

		tc.Table("security_events").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "type": "token.created"})
		require.Len(t, created.all(), 1)
		assert.Equal(t, id, models.PublicID(models.PrefixPersonalToken, created.all()[0].TokenID))
	})

	t.Run("renames a token without changing what it may do", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := in(tc, tc.Owner)
		id := browser.PostJSON(patPath, testkit.JSON{"name": "CI", "abilities": []string{"members:read"}}).
			String("data.id")

		browser.PatchJSON(patPath+"/"+id, testkit.JSON{"name": "Deploy bot"}).AssertOK().
			AssertJSONPath("data.name", "Deploy bot").
			AssertJSONPath("data.abilities", []string{"members:read"})
		browser.PatchJSON(patPath+"/"+id, testkit.JSON{"name": " "}).AssertValidationError("name")
	})

	t.Run("revokes a token, which stops working at once and stays listed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := in(tc, tc.Owner)
		revoked := record[appevents.TokenRevoked](tc)
		res := browser.PostJSON(patPath, testkit.JSON{"name": "CI", "abilities": []string{"*"}}).AssertCreated()
		token := tc.Client().WithHeader("Authorization", "Bearer "+res.String("data.plain_text_token"))
		token.GetJSON("/api/v1/me").AssertOK()

		browser.Delete(patPath + "/" + res.String("data.id")).AssertNoContent()
		browser.Delete(patPath + "/" + res.String("data.id")).AssertNoContent()

		token.GetJSON("/api/v1/me").AssertUnauthorized().AssertJSONPath("code", "token_revoked")
		browser.GetJSON(patPath).AssertOK().
			AssertJSONPath("data.0.revoked_at", tests.Epoch().Format(time.RFC3339)).
			AssertJSONPath("data.0.last_used_at", tests.Epoch().Format(time.RFC3339))
		tc.Table("personal_access_tokens").AssertHas(testkit.Row{"revoked_by_member_id": tc.Workspace.Owner.ID})
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "type": "token.revoked"})
		assert.Len(t, revoked.all(), 1)

		tc.Clock.Advance(91 * 24 * time.Hour)
		browser = in(tc, tc.Owner)
		browser.GetJSON(patPath).AssertOK().AssertJSONPath("data", []any{})
	})

	t.Run("lets members manage only their own tokens and admins everyone's", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		owner, teammate := in(tc, tc.Owner), in(tc, user)
		ownerToken := owner.PostJSON(patPath, testkit.JSON{"name": "Owner CI", "abilities": []string{"*"}}).
			String("data.id")
		memberToken := teammate.PostJSON(patPath, testkit.JSON{"name": "Mine", "abilities": []string{"*"}}).
			String("data.id")

		teammate.GetJSON(patPath).AssertOK().
			AssertJSONPath("data.#", float64(1)).
			AssertJSONPath("data.0.id", memberToken)
		teammate.GetJSON(patPath + "?filter[member]=all").AssertForbidden()
		teammate.GetJSON(patPath + "?filter[member]=" + memberID(tc.Workspace.Owner)).AssertForbidden()
		teammate.GetJSON(patPath + "/" + ownerToken).AssertNotFound()
		teammate.PatchJSON(patPath+"/"+ownerToken, testkit.JSON{"name": "Mine now"}).AssertNotFound()
		teammate.Delete(patPath + "/" + ownerToken).AssertNotFound()

		owner.GetJSON(patPath).AssertOK().AssertJSONPath("data.#", float64(1))
		owner.GetJSON(patPath+"?filter[member]=all").AssertOK().AssertJSONPath("data.#", float64(2))
		owner.GetJSON(patPath+"?filter[member]="+memberID(member)).AssertOK().
			AssertJSONPath("data.0.member.id", memberID(member))
		owner.GetJSON(patPath + "?filter[member]=nope").AssertStatus(fhttp.StatusBadRequest)
		owner.Delete(patPath + "/" + memberToken).AssertNoContent()
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": user.ID, "type": "token.revoked"})
	})

	t.Run("pages through tokens newest first", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := in(tc, tc.Owner)
		for _, name := range []string{"one", "two", "three"} {
			browser.PostJSON(patPath, testkit.JSON{"name": name, "abilities": []string{"*"}}).AssertCreated()
			tc.Clock.Advance(time.Second)
		}

		first := browser.GetJSON(patPath+"?limit=2").AssertOK().
			AssertJSONPath("data.0.name", "three").
			AssertJSONPath("data.1.name", "two")

		browser.GetJSON(patPath+"?limit=2&cursor="+first.String("meta.next_cursor")).AssertOK().
			AssertJSONPath("data.#", float64(1)).
			AssertJSONPath("data.0.name", "one")
		browser.GetJSON(patPath + "?cursor=forged").AssertStatus(fhttp.StatusBadRequest)
	})

	t.Run("does not find tokens of other workspaces or ids that are not tokens", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other := tests.New(t)
		foreign, _ := other.MintToken(other.Workspace.Owner, "*")
		browser := in(tc, tc.Owner)

		browser.GetJSON(patPath + "/" + models.PublicID(models.PrefixPersonalToken, foreign.ID)).AssertNotFound()
		browser.Delete(patPath + "/" + memberID(tc.Workspace.Owner)).AssertNotFound()
	})

	t.Run("revokes the tokens of a member who is deactivated", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		token := bearer(t, tc, in(tc, user), "*")

		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"status": "deactivated"}).
			AssertOK()

		token.GetJSON("/api/v1/me").AssertUnauthorized()
		tc.Table("personal_access_tokens").AssertHas(testkit.Row{
			"owner_id": member.ID, "revoked_at": tests.Epoch(), "revoked_by_member_id": tc.Workspace.Owner.ID,
		})
	})

	t.Run("warns owners a week before a token expires, once", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		browser := in(tc, tc.Owner)
		mint := func(name string, expires time.Duration) string {
			return browser.PostJSON(patPath, testkit.JSON{
				"name": name, "abilities": []string{"*"}, "expires_at": tests.Epoch().Add(expires),
			}).AssertCreated().String("data.id")
		}
		mint("Soon", 5*24*time.Hour)
		mint("Later", 30*24*time.Hour)
		revoked := mint("Revoked", 3*24*time.Hour)
		browser.Delete(patPath + "/" + revoked).AssertNoContent()
		tests.Sent[appmail.TokenCreatedMail](tc, fake)

		for range 2 {
			require.NoError(t, tc.App.Queue.Push(t.Context(), jobs.NotifyExpiringTokensArgs{}))
			tc.Drain()
		}

		mails, messages := tests.Sent[appmail.TokenExpiringMail](tc, fake)
		require.Len(t, mails, 1)
		assert.Equal(t, "Soon", mails[0].Token)
		assert.Equal(t, tc.Workspace.Workspace.Name, mails[0].Workspace)
		assert.Equal(t, "7 November 2026", mails[0].ExpiresOn)
		assert.Equal(t, []string{tc.Owner.Email}, tests.SentTo(messages[0]))
		tc.Table("personal_access_tokens").AssertHas(testkit.Row{"name": "Soon", "expiry_notified_at": tests.Epoch()})
		tc.Table("personal_access_tokens").AssertHas(testkit.Row{"name": "Later", "expiry_notified_at": nil})
	})

	t.Run("turns away a mistyped token without looking it up", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, plain := tc.MintToken(tc.Workspace.Owner, "*")
		typo := plain[:20] + string(plain[20]^1) + plain[21:]

		withTokensLocked(t, tc, func() {
			tc.Client().WithHeader("Authorization", "Bearer "+typo).GetJSON("/api/v1/members").
				AssertUnauthorized().
				AssertJSONPath("code", "invalid_token")
		})
	})
}

/*
bearer mints a token through the API for the browser's user and returns
a client sending it.
*/
func bearer(t *testing.T, tc *tests.TestCase, browser *testkit.Client, abilities ...string) *testkit.Client {
	t.Helper()

	plain := browser.PostJSON(patPath, testkit.JSON{"name": "Test", "abilities": abilities}).
		AssertCreated().
		String("data.plain_text_token")

	return tc.Client().WithHeader("Authorization", "Bearer "+plain)
}

/*
withTokensLocked runs fn while another transaction holds the token table
exclusively, so any query on it would wait. Finishing well within the
wait proves the function never touched the table.
*/
func withTokensLocked(t *testing.T, tc *tests.TestCase, fn func()) {
	t.Helper()

	err := tc.App.DB.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
		if _, err := tx.Exec(ctx, "LOCK TABLE personal_access_tokens IN ACCESS EXCLUSIVE MODE"); err != nil {
			return err
		}

		done := make(chan struct{})

		go func() {
			defer close(done)

			fn()
		}()

		wait, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		select {
		case <-done:
		case <-wait.Done():
			t.Error("the request waited on the token table")
		}

		return nil
	})
	require.NoError(t, err)
}
