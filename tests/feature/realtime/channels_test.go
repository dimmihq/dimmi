package realtime_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
The protocol's error codes for a refused connection or subscription.
*/
const (
	unauthorized     = 101
	unknownChannel   = 102
	permissionDenied = 103
)

func TestRealtimeConnections(t *testing.T) {
	t.Parallel()

	t.Run("opens with the session cookie of a same-origin page", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		url := tc.RealtimeURL()

		c := tc.ListenAs(tc.Owner, url)

		assert.Nil(t, c.Subscribe("workspace:"+public(models.PrefixWorkspace, tc.Workspace.Workspace.ID)).Error)
	})

	t.Run("refuses a cookie sent from another site", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		url := tc.RealtimeURL()
		header := tc.BrowserHeaders(tc.Owner, url)
		header.Set("Sec-Fetch-Site", "cross-site")

		_, refusal := testkit.DialRealtime(t, url, testkit.RealtimeOptions{Header: header})

		require.NotNil(t, refusal)
		assert.Equal(t, unauthorized, refusal.Code)
	})

	t.Run("refuses a connection without credentials or with a bad token", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		url := tc.RealtimeURL()

		_, refusal := testkit.DialRealtime(t, url, testkit.RealtimeOptions{})
		require.NotNil(t, refusal)
		assert.Equal(t, unauthorized, refusal.Code)

		_, refusal = testkit.DialRealtime(t, url, testkit.RealtimeOptions{Token: unknownToken()})
		require.NotNil(t, refusal)
		assert.Equal(t, unauthorized, refusal.Code)
	})

	t.Run("ignores the cookie when a token is sent", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		url := tc.RealtimeURL()
		header := tc.BrowserHeaders(tc.Owner, url)

		_, refusal := testkit.DialRealtime(t, url, testkit.RealtimeOptions{Header: header, Token: unknownToken()})

		require.NotNil(t, refusal)
		assert.Equal(t, unauthorized, refusal.Code)
	})

	t.Run("refuses every connection while the application is still booting", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		booting := *tc.App
		booting.Realtime = nil

		_, err := booting.ResolveRealtime(t.Context(), &fhttp.Request{}, "")

		require.Error(t, err)
	})
}

func TestRealtimeChannels(t *testing.T) {
	t.Parallel()

	t.Run("lets a member follow the workspace, its conversations and themselves", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		c := tc.ListenAs(tc.Owner, tc.RealtimeURL())

		assert.Nil(t, c.Subscribe("workspace:"+public(models.PrefixWorkspace, tc.Workspace.Workspace.ID)).Error)
		assert.Nil(t, c.Subscribe("conversation:"+public(models.PrefixConversation, s.conversation.ID)).Error)
		assert.Nil(t, c.Subscribe("member:"+public(models.PrefixMember, tc.Workspace.Owner.ID)).Error)
	})

	t.Run("refuses a member of another workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		stranger, _ := otherWorkspace(t, tc)
		c := tc.ListenAs(stranger, tc.RealtimeURL())

		for _, channel := range []string{
			"workspace:" + public(models.PrefixWorkspace, tc.Workspace.Workspace.ID),
			"conversation:" + public(models.PrefixConversation, s.conversation.ID),
			"member:" + public(models.PrefixMember, tc.Workspace.Owner.ID),
		} {
			assert.Equal(t, permissionDenied, c.Subscribe(channel).Error.Code, channel)
		}
	})

	t.Run("refuses channels that name nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		c := tc.ListenAs(tc.Owner, tc.RealtimeURL())

		assert.Equal(t, permissionDenied, c.Subscribe("conversation:nonsense").Error.Code)
		assert.Equal(t, permissionDenied, c.Subscribe("member:nonsense").Error.Code)
		assert.Equal(t, permissionDenied, c.Subscribe("workspace:nonsense").Error.Code)
		assert.Equal(t, permissionDenied,
			c.Subscribe("conversation:"+public(models.PrefixConversation, ids.New())).Error.Code)
		assert.Equal(t, unknownChannel, c.Subscribe("lobby").Error.Code)
	})

	t.Run("refuses a conversation outside what the member may see", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		exec(t, tc, "UPDATE conversations SET deleted_at = now() WHERE id = $1",
			s.conversation.ID)
		user, _ := joinAsMember(t, tc)
		channel := "conversation:" + public(models.PrefixConversation, s.conversation.ID)

		member := tc.ListenAs(user, tc.RealtimeURL())
		assert.Equal(t, permissionDenied, member.Subscribe(channel).Error.Code)

		owner := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		assert.Nil(t, owner.Subscribe(channel).Error, "owners see the trash")
	})

	t.Run("refuses a token without conversations:read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		url := tc.RealtimeURL()
		conversation := "conversation:" + public(models.PrefixConversation, s.conversation.ID)
		workspace := "workspace:" + public(models.PrefixWorkspace, tc.Workspace.Workspace.ID)

		blind := tc.ListenWithToken(tc.Workspace.Owner, url, "contacts:read")
		assert.Equal(t, permissionDenied, blind.Subscribe(conversation).Error.Code)
		assert.Equal(t, permissionDenied, blind.Subscribe(workspace).Error.Code)

		reader := tc.ListenWithToken(tc.Workspace.Owner, url, "conversations:read")
		assert.Nil(t, reader.Subscribe(conversation).Error)
		assert.Nil(t, reader.Subscribe(workspace).Error)
		assert.Nil(t, reader.Subscribe("member:"+public(models.PrefixMember, tc.Workspace.Owner.ID)).Error)

		_, other := joinAsMember(t, tc)
		assert.Equal(t, permissionDenied,
			reader.Subscribe("member:"+public(models.PrefixMember, other.ID)).Error.Code)
	})

	t.Run("keeps a contact's channel to the contact's widget", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		channel := "widget_contact:" + public(models.PrefixContact, s.contact.ID)

		member := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		assert.Equal(t, permissionDenied, member.Subscribe(channel).Error.Code)

		widget := testkit.ConnectRealtime(t, widgetNode(t, tc, s.contact), testkit.RealtimeOptions{Token: "visitor"})
		assert.Nil(t, widget.Subscribe(channel).Error)
		assert.Equal(t, permissionDenied,
			widget.Subscribe("workspace:"+public(models.PrefixWorkspace, tc.Workspace.Workspace.ID)).Error.Code)
	})

	t.Run("never lets a client publish", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		c := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		channel := "conversation:" + public(models.PrefixConversation, s.conversation.ID)
		require.Nil(t, c.Subscribe(channel).Error)

		assert.NotNil(t, c.Publish(channel, map[string]string{"event": "typing.started"}))
	})
}
