package user_test

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth/webauthn"
	"github.com/dimmihq/dimmi/framework/auth/webauthn/webauthntest"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/auth/factors"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
device is a passkey on a software authenticator: the authenticator and
the public id of the factor it activated.
*/
type device struct {
	key *webauthntest.Authenticator
	id  string
}

/*
newDevice returns a software authenticator answering for the install's
own origin.
*/
func newDevice(tc *tests.TestCase) *webauthntest.Authenticator {
	return webauthntest.New(tc.App.Links.Base)
}

/*
raw returns the JSON at the path of the response, such as the options a
passkey prompt needs.
*/
func raw(res *testkit.Response, path string) json.RawMessage {
	return json.RawMessage(res.String(path))
}

/*
startPasskey adds a pending passkey with the name for the browser's user
and returns its public id and creation options.
*/
func startPasskey(browser *testkit.Client, name string) (string, json.RawMessage) {
	res := browser.PostJSON(factorsPath, testkit.JSON{"type": "passkey", "name": name}).AssertCreated()

	return res.String("data.id"), raw(res, "data.setup.options")
}

/*
addPasskey registers a passkey with the name on a new device for the
browser's user and returns the device and the activation answer.
*/
func addPasskey(t *testing.T, tc *tests.TestCase, browser *testkit.Client, name string) (device, *testkit.Response) {
	t.Helper()

	id, options := startPasskey(browser, name)
	key := newDevice(tc)

	answer, err := key.Create(options)
	require.NoError(t, err)

	res := browser.PatchJSON(factorsPath+"/"+id, testkit.JSON{"credential": answer}).AssertOK()

	return device{key: key, id: id}, res
}

/*
signInWithPassword signs the user in with their password and returns
the browser holding the second-factor challenge, with the challenge.
*/
func signInWithPassword(tc *tests.TestCase, email string) (*testkit.Client, *testkit.Response) {
	res := tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
		"grant": "password", "email": email, "password": factories.Password,
	}).AssertOK().AssertJSONPath("data.object", "two_factor_challenge")

	return tc.Client().WithCookie(res.Cookie(tc.App.Cookies.TwoFactorName())), res
}

/*
methodOf returns the index of the factor among the challenge's methods.
*/
func methodOf(t *testing.T, challenge *testkit.Response, factorID string) int {
	t.Helper()

	var body struct {
		Data struct {
			Methods []struct {
				FactorID string `json:"factor_id"`
			} `json:"methods"`
		} `json:"data"`
	}
	challenge.Decode(&body)

	for i, m := range body.Data.Methods {
		if m.FactorID == factorID {
			return i
		}
	}

	t.Fatalf("challenge does not offer %s: %s", factorID, challenge.Body)

	return -1
}

/*
signPasskey answers the challenge's passkey prompt with the device.
*/
func signPasskey(t *testing.T, challenge *testkit.Response, key *webauthntest.Authenticator) json.RawMessage {
	t.Helper()

	signed, err := key.Get(raw(challenge, "data.methods.#(method==\"passkey\").options"))
	require.NoError(t, err)

	return signed
}

/*
answerPasskey sends the signed answer to the browser's challenge.
*/
func answerPasskey(browser *testkit.Client, signed json.RawMessage) *testkit.Response {
	return browser.PostJSON("/api/v1/sessions", testkit.JSON{
		"grant": "two_factor", "method": "passkey", "credential": signed,
	})
}

/*
storedPasskey is what the table keeps of a passkey.
*/
type storedPasskey struct {
	credential  webauthn.Credential
	lookupHash  []byte
	lastCounter int64
	lastUsedAt  *time.Time
}

/*
factorRow returns what the table keeps of the passkey with the public
id.
*/
func factorRow(t *testing.T, tc *tests.TestCase, id string) storedPasskey {
	t.Helper()

	uuid, err := tc.App.IDs.Decode(string(models.PrefixAuthFactor), id)
	require.NoError(t, err)

	var (
		out     storedPasskey
		encoded []byte
	)

	err = tc.App.DB.QueryRow(t.Context(), `SELECT credential, lookup_hash, last_counter, last_used_at
		FROM user_auth_factors WHERE id = $1`, uuid).Scan(&encoded, &out.lookupHash, &out.lastCounter, &out.lastUsedAt)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(encoded, &out.credential))

	return out
}

/*
protectedBrowser returns a new user signed in with a browser session,
who then turned two-factor authentication on with an authenticator app.
*/
func protectedBrowser(t *testing.T, tc *tests.TestCase) (factories.TwoFactorAccount, *testkit.Client) {
	t.Helper()

	user := tc.CreateUser()
	browser := tc.ActingAs(user)

	account, err := factories.WithTwoFactor(t.Context(), tc.App, user)
	require.NoError(t, err)

	return account, browser
}

/*
withChallenge returns the options with another challenge, as options
from a ceremony the server never began would carry.
*/
func withChallenge(t *testing.T, options json.RawMessage, challenge string) json.RawMessage {
	t.Helper()

	var opts map[string]any
	require.NoError(t, json.Unmarshal(options, &opts))
	opts["challenge"] = challenge

	out, err := json.Marshal(opts)
	require.NoError(t, err)

	return out
}

/*
encodeID returns a credential id as WebAuthn JSON carries it.
*/
func encodeID(id []byte) string {
	return base64.RawURLEncoding.EncodeToString(id)
}

/*
path returns the JSON path of a field of the challenge's method at the
index.
*/
func path(at int, field string) string {
	return "data.methods." + strconv.Itoa(at) + "." + field
}

/*
excludedIDs returns the credential ids creation options exclude.
*/
func excludedIDs(t *testing.T, options json.RawMessage) []string {
	t.Helper()

	var opts map[string][]map[string]any
	_ = json.Unmarshal(options, &opts)

	ids := []string{}
	for _, c := range opts["excludeCredentials"] {
		id, _ := c["id"].(string)
		ids = append(ids, id)
	}

	return ids
}

func TestPasskeyEnrollment(t *testing.T) {
	t.Parallel()

	t.Run("starts a passkey with creation options shown once", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := tc.ActingAsOwner().PostJSON(factorsPath, testkit.JSON{"type": "passkey", "name": "Work laptop"}).
			AssertCreated()

		res.AssertHeader("Cache-Control", "no-store").
			AssertJSONPath("data.type", "passkey").
			AssertJSONPath("data.name", "Work laptop").
			AssertJSONPath("data.status", "pending").
			AssertJSONPath("data.setup.options.rp.id", "localhost").
			AssertJSONPath("data.setup.options.rp.name", "dimmi").
			AssertJSONPath("data.setup.options.user.name", tc.Owner.Email).
			AssertJSONPath("data.setup.options.authenticatorSelection.userVerification", "preferred").
			AssertJSONPath("data.setup.options.attestation", "none").
			AssertJSONMissing("data.setup.secret")
		assert.NotEmpty(t, res.String("data.setup.options.challenge"))
		tc.ActingAsOwner().GetJSON(factorsPath).AssertOK().AssertJSONMissing("data.0.setup")
	})

	t.Run("activates with the device's answer and turns two-factor on", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		laptop, res := addPasskey(t, tc, tc.ActingAsOwner(), "Work laptop")

		res.AssertJSONPath("data.status", "active").
			AssertJSONPath("data.preferred", true).
			AssertJSONPath("data.recovery_codes.type", "recovery_code")
		stored := factorRow(t, tc, laptop.id)
		assert.Equal(t, factors.LookupHash(laptop.key.ID()), stored.lookupHash)
		assert.Equal(t, int64(0), stored.lastCounter)
		assert.Equal(t, laptop.key.ID(), stored.credential.ID)
		assert.Equal(t, []string{"internal"}, stored.credential.Transports)
		assert.True(t, stored.credential.UserVerified)
		rotated(tc, res).GetJSON("/api/v1/me").AssertJSONPath("data.user.two_factor_enabled", true)
	})

	t.Run("refuses an answer to another registration and counts it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		id, options := startPasskey(browser, "Laptop")

		answer, err := newDevice(tc).Create(withChallenge(t, options, "c29tZSBvdGhlciBjaGFsbGVuZ2U"))
		require.NoError(t, err)

		browser.PatchJSON(factorsPath+"/"+id, testkit.JSON{"credential": answer}).
			AssertValidationError("credential").AssertJSONPath("code", "invalid_credential")
		browser.PatchJSON(factorsPath+"/"+id, testkit.JSON{"credential": "nope"}).AssertValidationError("credential")
		tc.Table("user_auth_factors").AssertHas(testkit.Row{"type": "passkey", "status": "pending"})
	})

	t.Run("refuses a device answering for another site", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		id, options := startPasskey(browser, "Laptop")

		answer, err := webauthntest.New("https://evil.example").Create(options)
		require.NoError(t, err)

		browser.PatchJSON(factorsPath+"/"+id, testkit.JSON{"credential": answer}).
			AssertJSONPath("code", "invalid_credential")
	})

	t.Run("holds several passkeys, each excluded from the next registration", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		laptop, res := addPasskey(t, tc, tc.ActingAsOwner(), "Laptop")
		browser := rotated(tc, res)

		id, options := startPasskey(browser, "Phone")
		assert.Equal(t, []string{encodeID(laptop.key.ID())}, excludedIDs(t, options))

		phone := newDevice(tc)
		answer, err := phone.Create(options)
		require.NoError(t, err)
		browser.PatchJSON(factorsPath+"/"+id, testkit.JSON{"credential": answer}).AssertOK().
			AssertJSONPath("data.preferred", false).
			AssertJSONMissing("data.recovery_codes")

		browser.GetJSON(factorsPath).AssertOK().
			AssertJSONPath("data.#(type==\"passkey\")#.name", []string{"Laptop", "Phone"})
	})

	t.Run("adds a passkey beside an authenticator app without new recovery codes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		account, browser := protectedBrowser(t, tc)

		_, res := addPasskey(t, tc, browser, "Security key")

		res.AssertJSONPath("data.preferred", false).AssertJSONMissing("data.recovery_codes")
		tc.Table("user_auth_factors").AssertHas(testkit.Row{"id": account.Recovery.ID, "status": "active"})
	})
}

func TestPasskeySignIn(t *testing.T) {
	t.Parallel()

	t.Run("offers the passkey with request options and signs in with it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		laptop, _ := addPasskey(t, tc, tc.ActingAsOwner(), "Laptop")
		browser, challenge := signInWithPassword(tc, tc.Owner.Email)

		at := methodOf(t, challenge, laptop.id)
		challenge.AssertJSONPath(path(at, "method"), "passkey").
			AssertJSONPath(path(at, "name"), "Laptop").
			AssertJSONPath(path(at, "options.rpId"), "localhost").
			AssertJSONPath(path(at, "options.allowCredentials.0.id"), encodeID(laptop.key.ID()))

		res := answerPasskey(browser, signPasskey(t, challenge, laptop.key)).AssertCreated()

		res.AssertJSONPath("data.auth.auth_methods", []string{"password", "passkey"}).
			AssertJSONPath("data.auth.two_factor_used", true)
		stored := factorRow(t, tc, laptop.id)
		assert.Equal(t, int64(1), stored.lastCounter)
		assert.Equal(t, tests.Epoch(), stored.lastUsedAt.UTC())
	})

	t.Run("refuses an answer signed for another challenge", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		laptop, _ := addPasskey(t, tc, tc.ActingAsOwner(), "Laptop")
		_, first := signInWithPassword(tc, tc.Owner.Email)
		browser, _ := signInWithPassword(tc, tc.Owner.Email)

		answerPasskey(browser, signPasskey(t, first, laptop.key)).
			AssertValidationError("credential").AssertJSONPath("code", "invalid_credential")

		tc.Table("one_time_tokens").AssertHas(testkit.Row{"purpose": "two_factor_challenge", "attempts": 1})
		tc.Table("sessions").AssertCount(1)
	})

	t.Run("never takes the same signed answer twice", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		laptop, _ := addPasskey(t, tc, tc.ActingAsOwner(), "Laptop")
		browser, challenge := signInWithPassword(tc, tc.Owner.Email)
		signed := signPasskey(t, challenge, laptop.key)
		answerPasskey(browser, signed).AssertCreated()

		answerPasskey(browser, signed).AssertUnauthorized()
		again, _ := signInWithPassword(tc, tc.Owner.Email)
		answerPasskey(again, signed).AssertJSONPath("code", "invalid_credential")
	})

	t.Run("refuses a copied key whose counter falls behind", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		laptop, _ := addPasskey(t, tc, tc.ActingAsOwner(), "Laptop")
		browser, challenge := signInWithPassword(tc, tc.Owner.Email)
		answerPasskey(browser, signPasskey(t, challenge, laptop.key)).AssertCreated()
		copied := laptop.key.Clone()
		browser, challenge = signInWithPassword(tc, tc.Owner.Email)
		answerPasskey(browser, signPasskey(t, challenge, laptop.key)).AssertCreated()

		browser, challenge = signInWithPassword(tc, tc.Owner.Email)
		answerPasskey(browser, signPasskey(t, challenge, copied)).AssertJSONPath("code", "invalid_credential")

		assert.Equal(t, int64(2), factorRow(t, tc, laptop.id).lastCounter)
	})

	t.Run("signs in with either of two passkeys", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		laptop, res := addPasskey(t, tc, tc.ActingAsOwner(), "Laptop")
		phone, _ := addPasskey(t, tc, rotated(tc, res), "Phone")
		browser, challenge := signInWithPassword(tc, tc.Owner.Email)

		assert.JSONEq(t, challenge.String(path(methodOf(t, challenge, laptop.id), "options")),
			challenge.String(path(methodOf(t, challenge, phone.id), "options")))

		answerPasskey(browser, signPasskey(t, challenge, phone.key)).AssertCreated()
		assert.Nil(t, factorRow(t, tc, laptop.id).lastUsedAt)
	})

	t.Run("offers a passkey beside an authenticator app, either one signing in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		account, protected := protectedBrowser(t, tc)
		key, _ := addPasskey(t, tc, protected, "Security key")
		browser, challenge := signInWithPassword(tc, account.User.Email)

		challenge.AssertJSONPath("data.methods.#.method", []string{"totp", "passkey", "recovery_code"}).
			AssertJSONPath("data.methods.0.options", nil)

		browser.PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "two_factor", "method": "totp", "credential": signPasskey(t, challenge, key.key),
		}).AssertValidationError("credential")
		answerPasskey(browser, signPasskey(t, challenge, key.key)).AssertCreated()
	})
}

func TestPasskeyRemoval(t *testing.T) {
	t.Parallel()

	t.Run("removes one passkey and keeps two-factor on with the other", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		laptop, res := addPasskey(t, tc, tc.ActingAsOwner(), "Laptop")
		browser := rotated(tc, res)
		phone, _ := addPasskey(t, tc, browser, "Phone")

		browser.Delete(factorsPath + "/" + laptop.id).AssertNoContent()

		tc.Table("user_auth_factors").AssertMissing(testkit.Row{"type": "passkey", "name": "Laptop"})
		browser.GetJSON("/api/v1/me").AssertJSONPath("data.user.two_factor_enabled", true)
		challengeBrowser, challenge := signInWithPassword(tc, tc.Owner.Email)
		assert.Equal(t, 0, methodOf(t, challenge, phone.id))
		answerPasskey(challengeBrowser, signPasskey(t, challenge, phone.key)).AssertCreated()
	})

	t.Run("turns two-factor off with the last passkey", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		laptop, res := addPasskey(t, tc, tc.ActingAsOwner(), "Laptop")
		browser := rotated(tc, res)

		browser.Delete(factorsPath + "/" + laptop.id).AssertNoContent()

		tc.Table("user_auth_factors").AssertCount(0)
		browser.GetJSON("/api/v1/me").AssertJSONPath("data.user.two_factor_enabled", false)
	})

	t.Run("keeps the last passkey while a workspace requires a second factor", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		laptop, res := addPasskey(t, tc, tc.ActingAsOwner(), "Laptop")
		_, err := tc.App.DB.Exec(t.Context(), `UPDATE workspaces
			SET settings = jsonb_set(settings, '{security}', '{"require_two_factor": true}') WHERE id = $1`,
			tc.Workspace.Workspace.ID)
		require.NoError(t, err)

		rotated(tc, res).Delete(factorsPath+"/"+laptop.id).AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "two_factor_required_by_workspace")
	})
}
