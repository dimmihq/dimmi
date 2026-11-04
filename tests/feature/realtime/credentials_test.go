package realtime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
The close codes of a connection whose credential ended: told at once when
it ended, or found out by the connection asking.
*/
const (
	credentialEnded   = 4009
	credentialExpired = 3005
)

/*
tab is a browser's signed-in page with a realtime connection open and
following a conversation: its session as an API client, the connection,
and the headers that opened it.
*/
type tab struct {
	api    *testkit.Client
	socket *testkit.Realtime
	header fhttp.Header
}

/*
where is the realtime endpoint a browser connects to and the seed whose
conversation it follows.
*/
type where struct {
	url  string
	seed seed
}

/*
openTab signs the user in on a new browser and has it follow the seed's
conversation at the realtime endpoint.
*/
func openTab(t *testing.T, tc *tests.TestCase, user models.User, at where) tab {
	t.Helper()

	header := tc.BrowserHeaders(user, at.url)
	socket := testkit.ConnectRealtime(t, at.url, testkit.RealtimeOptions{Header: header})
	require.Nil(t, socket.Subscribe(appbroadcasting.ConversationChannel(at.seed.conversation.ID)).Error)

	return tab{api: tc.Client().WithHeader("Cookie", header.Get("Cookie")), socket: socket, header: header}
}

/*
assertRefusedAgain proves the headers that opened a connection do not
open another.
*/
func assertRefusedAgain(t *testing.T, url string, header fhttp.Header) {
	t.Helper()

	_, refusal := testkit.DialRealtime(t, url, testkit.RealtimeOptions{Header: header})
	require.NotNil(t, refusal)
	assert.Equal(t, unauthorized, refusal.Code)
}

/*
changeConversation changes the conversation as the workspace's owner, so
whoever still follows it hears about it.
*/
func changeConversation(tc *tests.TestCase, s seed) {
	tc.T.Helper()

	owner(tc).PatchJSON("/api/v1/conversations/"+public(models.PrefixConversation, s.conversation.ID),
		testkit.JSON{"priority": "high"}).AssertOK()
}

/*
impatientNode starts another node of the application on the same Valkey
that asks its connections about their credentials every second.
*/
func impatientNode(t *testing.T, tc *tests.TestCase) string {
	t.Helper()

	return nodeWith(t, tc, broadcasting.Config{Resolve: tc.App.ResolveRealtime, Recheck: time.Second})
}

func TestRealtimeCredentials(t *testing.T) {
	t.Parallel()

	t.Run("closes the socket when its browser signs out, and refuses that cookie after", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "api")
		user, _ := joinAsMember(t, tc)
		url := tc.RealtimeURL()
		browser := openTab(t, tc, user, where{url, s})

		browser.api.Delete("/api/v1/sessions/current").AssertNoContent()

		assert.Equal(t, credentialEnded, browser.socket.Closed())
		assertRefusedAgain(t, url, browser.header)
	})

	t.Run("closes only the socket of a session ended from another device", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "api")
		user, _ := joinAsMember(t, tc)
		url := tc.RealtimeURL()
		laptop, phone := openTab(t, tc, user, where{url, s}), openTab(t, tc, user, where{url, s})
		list := phone.api.GetJSON("/api/v1/sessions").AssertOK()
		other := list.String(`data.#(current==false).id`)

		phone.api.Delete("/api/v1/sessions/" + other).AssertNoContent()

		assert.Equal(t, credentialEnded, laptop.socket.Closed())
		assertRefusedAgain(t, url, laptop.header)
		changeConversation(tc, s)
		assert.NotEmpty(t, phone.socket.Publication().Event)
	})

	t.Run("closes the sockets of every other session when signing out everywhere else", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "api")
		user, _ := joinAsMember(t, tc)
		url := tc.RealtimeURL()
		laptop, phone := openTab(t, tc, user, where{url, s}), openTab(t, tc, user, where{url, s})

		phone.api.Delete("/api/v1/sessions").AssertNoContent()

		assert.Equal(t, credentialEnded, laptop.socket.Closed())
		changeConversation(tc, s)
		assert.NotEmpty(t, phone.socket.Publication().Event)
	})

	t.Run("closes only the socket of a revoked token", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "api")
		user, member := joinAsMember(t, tc)
		url := tc.RealtimeURL()
		tok, plain := tc.MintToken(member, "*")
		script := testkit.ConnectRealtime(t, url, testkit.RealtimeOptions{Token: plain})
		require.Nil(t, script.Subscribe(appbroadcasting.ConversationChannel(s.conversation.ID)).Error)
		browser := openTab(t, tc, user, where{url, s})

		browser.api.WithHeader("Dimmi-Workspace", workspaceID(tc)).
			Delete("/api/v1/tokens/" + public(models.PrefixPersonalToken, tok.ID)).AssertNoContent()

		assert.Equal(t, credentialEnded, script.Closed())
		_, refusal := testkit.DialRealtime(t, url, testkit.RealtimeOptions{Token: plain})
		require.NotNil(t, refusal)
		assert.Equal(t, unauthorized, refusal.Code)
		changeConversation(tc, s)
		assert.NotEmpty(t, browser.socket.Publication().Event)
	})

	t.Run("closes every socket of the user when their password is reset", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "api")
		user, member := joinAsMember(t, tc)
		url := tc.RealtimeURL()
		browser := openTab(t, tc, user, where{url, s})
		script := tc.ListenWithToken(member, url, "*")
		fake := tc.Mail()
		tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": user.Email}).
			AssertStatus(fhttp.StatusAccepted)
		sent, _ := tests.LastSent[appmail.ResetPasswordMail](tc, fake)
		link := tc.Fragment(sent.URL)

		tc.Client().PatchJSON("/api/v1/password-resets/"+link.Get("id"), testkit.JSON{
			"token": link.Get("token"), "password": "grace-hopper-1906",
		}).AssertOK()

		assert.Equal(t, credentialEnded, browser.socket.Closed())
		assert.Equal(t, credentialEnded, script.Closed())
		assertRefusedAgain(t, url, browser.header)
	})

	t.Run("closes a stranger's socket when the owner of the inbox takes the account back", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "api")
		url := tc.RealtimeURL()
		user, _ := joinAsMember(t, tc)
		exec(t, tc, "UPDATE users SET email_verified_at = NULL WHERE id = $1", user.ID)
		stranger := openTab(t, tc, user, where{url, s})
		fake := tc.Mail()
		tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": user.Email}).
			AssertStatus(fhttp.StatusAccepted)
		sent, _ := tests.LastSent[appmail.MagicLinkMail](tc, fake)

		tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "magic_link", "token": tc.Fragment(sent.URL).Get("token"),
		}).AssertCreated()

		assert.Equal(t, credentialEnded, stranger.socket.Closed())
		assertRefusedAgain(t, url, stranger.header)
	})

	t.Run("closes the socket on another node when its session ends here", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "api")
		user, _ := joinAsMember(t, tc)
		browser := openTab(t, tc, user, where{secondNode(t, tc, tc.App.ResolveRealtime), s})

		browser.api.Delete("/api/v1/sessions/current").AssertNoContent()

		assert.Equal(t, credentialEnded, browser.socket.Closed())
	})

	t.Run("closes a socket trying to subscribe once its session is gone without notice", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "api")
		user, _ := joinAsMember(t, tc)
		url := tc.RealtimeURL()
		socket := tc.ListenAs(user, url)
		exec(t, tc, "DELETE FROM sessions WHERE user_id = $1", user.ID)

		assert.Equal(t, credentialExpired,
			socket.ClosedBySubscribing(appbroadcasting.ConversationChannel(s.conversation.ID)))
	})

	t.Run("closes a socket trying to subscribe once its token is revoked without notice", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "api")
		_, member := joinAsMember(t, tc)
		socket := tc.ListenWithToken(member, tc.RealtimeURL(), "*")
		exec(t, tc, "UPDATE personal_access_tokens SET revoked_at = now() WHERE owner_id = $1", member.ID)

		assert.Equal(t, credentialExpired,
			socket.ClosedBySubscribing(appbroadcasting.ConversationChannel(s.conversation.ID)))
	})

	t.Run("finds out on its own when a session is gone without notice", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, _ := joinAsMember(t, tc)
		socket := tc.ListenAs(user, impatientNode(t, tc))
		exec(t, tc, "DELETE FROM sessions WHERE user_id = $1", user.ID)

		assert.Equal(t, credentialExpired, socket.Closed())
	})

	t.Run("finds out on its own when a token runs out", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "api")
		_, member := joinAsMember(t, tc)
		socket := tc.ListenWithToken(member, impatientNode(t, tc), "*")
		require.Nil(t, socket.Subscribe(appbroadcasting.ConversationChannel(s.conversation.ID)).Error)
		exec(t, tc, "UPDATE personal_access_tokens SET expires_at = $1 WHERE owner_id = $2",
			tests.Epoch().Add(time.Hour), member.ID)
		tc.Clock.Advance(2 * time.Hour)

		assert.Equal(t, credentialExpired, socket.Closed())
	})

	t.Run("finds out on its own when a token's member is deleted with their account", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := joinAsMember(t, tc)
		socket := tc.ListenWithToken(member, impatientNode(t, tc), "*")
		require.NoError(t, factories.DeleteAccount(t.Context(), tc.App, user))

		assert.Equal(t, credentialExpired, socket.Closed())
	})

	t.Run("keeps a socket whose credential still works through its checks", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "api")
		user, _ := joinAsMember(t, tc)
		browser := openTab(t, tc, user, where{impatientNode(t, tc), s})

		assert.Never(t, func() bool { return false }, 2500*time.Millisecond, 100*time.Millisecond)

		changeConversation(tc, s)
		assert.NotEmpty(t, browser.socket.Publication().Event)
	})

	t.Run("a workspace limit reached while listening drops that workspace but keeps the socket", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)
		c := tc.ListenAs(tc.Owner, impatientNode(t, tc))
		strict := appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID)
		require.Nil(t, c.Subscribe(strict).Error)

		_, err = tc.App.DB.Exec(t.Context(), `UPDATE workspaces SET settings =
			settings || '{"security": {"session_max_age_minutes": 60}}' WHERE id = $1`, tc.Workspace.Workspace.ID)
		require.NoError(t, err)
		tc.Clock.Advance(2 * time.Hour)

		require.NotNil(t, c.Subscribe(strict).Error)
		require.Nil(t, c.Subscribe(appbroadcasting.WorkspaceChannel(other.Workspace.ID)).Error)
	})
}
