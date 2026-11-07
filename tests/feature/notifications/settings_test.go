package notifications_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"
)

const pushEndpoint = "https://fcm.googleapis.com/fcm/send/device-1"

/*
subscribe registers the owner's browser for push.
*/
func (d *desk) subscribe() {
	d.t.Helper()
	k := testkit.BrowserPushKeys(d.t)
	d.Owner.PostJSON("/api/v1/push-subscriptions", testkit.JSON{
		"endpoint": pushEndpoint, "keys": testkit.JSON{"p256dh": k.P256dh, "auth": k.Auth},
		"device_label": "Laptop",
	}).AssertCreated().AssertJSONPath("data.platform", "google").AssertJSONMissing("data.keys")
}

func TestNotificationSettings(t *testing.T) {
	t.Parallel()

	t.Run("lists every trigger by channel with its default", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		res := d.Owner.GetJSON("/api/v1/notification-preferences").AssertOK()
		res.AssertJSONPath("data.channels.0", "in_app").AssertJSONPath("data.settings.show_previews", true)
		assert.NotEmpty(t, res.String("data.triggers.0.label"))
		d.Owner.PatchJSON("/api/v1/notification-preferences", testkit.JSON{"preferences": []testkit.JSON{
			{"trigger": "nope", "channel": "email", "enabled": false},
		}}).AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "preference_unknown")
	})

	t.Run("checks quiet hours, time zones and pauses", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.Owner.PatchJSON("/api/v1/notification-settings", testkit.JSON{
			"quiet_hours_enabled": true, "quiet_hours_start": "21:00", "quiet_hours_end": "07:30",
			"timezone": "Europe/Paris", "quiet_hours_bypass": []string{"mentioned"},
		}).AssertOK().AssertJSONPath("data.quiet_hours_start", "21:00").AssertJSONPath("data.timezone", "Europe/Paris")
		d.Owner.PatchJSON("/api/v1/notification-settings", testkit.JSON{"quiet_hours_start": "25:00"}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
		d.Owner.PatchJSON("/api/v1/notification-settings", testkit.JSON{"timezone": "Mars/Base"}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
		d.Owner.PatchJSON("/api/v1/notification-settings", testkit.JSON{"quiet_hours_bypass": []string{"nope"}}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
	})

	t.Run("pushes to an enabled browser and forgets it once the push service says it is gone", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		fake := httpclienttest.New(d.TC.App.HTTP)
		fake.Stub("POST "+pushEndpoint, httpclient.Response{Status: fhttp.StatusCreated})
		d.Owner.GetJSON("/api/v1/push-config").AssertOK()
		d.subscribe()
		d.subscribe()
		d.Owner.GetJSON("/api/v1/push-subscriptions").AssertOK().AssertJSONPath("data.0.device_label", "Laptop")
		d.TC.Table("push_subscriptions").AssertCount(1, nil)

		d.visitor().start("Help!")
		d.TC.DrainLater()
		fake.AssertSent(t, func(r httpclienttest.SentRequest) bool { return r.URL.String() == pushEndpoint })

		fake.Stub("POST "+pushEndpoint, httpclient.Response{Status: fhttp.StatusGone})
		d.visitor().start("Again")
		d.TC.DrainLater()
		d.TC.Table("push_subscriptions").AssertCount(0, nil)
	})

	t.Run("holds back push and email while the member is looking", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		fake := httpclienttest.New(d.TC.App.HTTP)
		fake.Stub("POST "+pushEndpoint, httpclient.Response{Status: fhttp.StatusCreated})
		d.subscribe()
		d.Owner.PostJSON("/api/v1/notification-focus", testkit.JSON{"visible": true}).AssertNoContent()
		d.visitor().start("Hi")
		require.Empty(t, d.alerts(d.TC.Owner.Email))
		assert.Empty(t, fake.Sent())
		assert.Len(t, filter(bell(t, d.Owner), "new_conversation"), 1)
	})

	t.Run("refuses a subscription that is not a push service and removes a device", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		k := testkit.BrowserPushKeys(t)
		d.Owner.PostJSON("/api/v1/push-subscriptions", testkit.JSON{
			"endpoint": "https://evil.example.com/hook", "keys": testkit.JSON{"p256dh": k.P256dh, "auth": k.Auth},
		}).AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "push_endpoint_invalid")
		d.Owner.PostJSON("/api/v1/push-subscriptions", testkit.JSON{
			"endpoint": pushEndpoint, "keys": testkit.JSON{"p256dh": "short", "auth": k.Auth},
		}).AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "push_keys_invalid")
		d.subscribe()
		id := d.Owner.GetJSON("/api/v1/push-subscriptions").AssertOK().String("data.0.id")
		d.Owner.Delete("/api/v1/push-subscriptions/" + id).AssertNoContent()
		d.Owner.Delete("/api/v1/push-subscriptions/" + id).AssertNotFound()
	})

	t.Run("sends a test to the channels that can reach the member", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.Owner.PostJSON("/api/v1/notification-tests", testkit.JSON{"channel": "push"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "channel_unreachable")
		d.Owner.PostJSON("/api/v1/notification-tests", testkit.JSON{"channel": "email"}).AssertNoContent()
		assert.Len(t, d.alerts(d.TC.Owner.Email), 1)
		d.Owner.PostJSON("/api/v1/notification-tests", testkit.JSON{"channel": "in_app"}).AssertNoContent()
	})
}
