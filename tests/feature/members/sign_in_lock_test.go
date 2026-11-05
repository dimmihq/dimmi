package members_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
softLock locks an account against strangers after three failures.
*/
const softLock = "DIMMI_AUTH__LOCKOUT__PER_EMAIL_SOFT_LOCK=3"

/*
login is the body of a password sign-in.
*/
func login(email, password string) testkit.JSON {
	return testkit.JSON{"grant": "password", "email": email, "password": password}
}

/*
fail signs in with a wrong password the given number of times.
*/
func fail(t *testing.T, client *testkit.Client, email string, times int) {
	t.Helper()

	for range times {
		client.PostJSON("/api/v1/sessions", login(email, "wrong-password")).AssertValidationError("email")
	}
}

/*
lockPath is the path of the member's sign-in lock.
*/
func lockPath(id string) string {
	return "/api/v1/members/" + id + "/sign-in-lock"
}

func TestSignInLock(t *testing.T) {
	t.Parallel()

	t.Run("clearing a sign-in lock lets them try again", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(softLock))
		teammate, member := join(t, tc, enums.RoleMember)
		admin, adminMember := join(t, tc, enums.RoleAdmin)
		fail(t, tc.Client(), teammate.Email, 3)
		tc.Client().PostJSON("/api/v1/sessions", login(teammate.Email, factories.Password)).AssertStatus(429)
		client := in(tc, admin)

		client.GetJSON("/api/v1/members/"+memberID(member)).AssertOK().AssertJSONPath("data.sign_in_locked", true)
		client.Delete(lockPath(memberID(member))).AssertNoContent()

		client.GetJSON("/api/v1/members?limit=100").AssertOK().AssertJSONPath("data.#(id==\""+memberID(member)+
			"\").sign_in_locked", false)
		tc.Client().PostJSON("/api/v1/sessions", login(teammate.Email, factories.Password)).AssertCreated()
		tc.Table("audit_logs").AssertHas(testkit.Row{
			"event": "member.sign_in_lock_cleared", "actor_id": adminMember.ID, "subject_id": member.ID,
		})
	})

	t.Run("clearing also ends the run of failures of every address at the account", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__LOCKOUT__PER_IP_EMAIL=100"))
		teammate, member := join(t, tc, enums.RoleMember)
		fail(t, tc.Client(), teammate.Email, 5)
		tc.Client().PostJSON("/api/v1/sessions", login(teammate.Email, factories.Password)).AssertStatus(429)

		in(tc, tc.Owner).Delete(lockPath(memberID(member))).AssertNoContent()

		tc.Client().PostJSON("/api/v1/sessions", login(teammate.Email, factories.Password)).AssertCreated()
	})

	t.Run("a member sees no locks and cannot clear one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(softLock))
		teammate, member := join(t, tc, enums.RoleMember)
		other, _ := join(t, tc, enums.RoleMember)
		fail(t, tc.Client(), teammate.Email, 3)
		client := in(tc, other)

		client.GetJSON("/api/v1/members/"+memberID(member)).AssertOK().AssertJSONPath("data.sign_in_locked", nil)
		client.Delete(lockPath(memberID(member))).AssertForbidden()
	})

	t.Run("answers not found for a member of another workspace and a malformed id", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		elsewhere, err := factories.Workspace(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)
		owner := in(tc, tc.Owner)

		owner.Delete(lockPath(memberID(elsewhere.Owner))).AssertNotFound()
		owner.Delete(lockPath("mem_nope")).AssertNotFound()
	})

	t.Run("an access token cannot clear a lock", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)

		tc.ActingAsToken(tc.Workspace.Owner, "members:write").Delete(lockPath(memberID(member))).
			AssertForbidden().AssertJSONPath("code", "first_party_only")
	})
}
