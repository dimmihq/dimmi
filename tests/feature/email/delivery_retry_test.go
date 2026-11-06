package email_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

func TestRepliesThatWereNotDelivered(t *testing.T) {
	t.Parallel()

	t.Run("shows why a reply failed and sends it again on request", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer := unique("blocked")
		conversation := startConversation(tc, address, customer)
		ws := conversation.WorkspaceID
		query(tc, ws, func(ctx context.Context, q *models.Queries) (struct{}, error) {
			return struct{}{}, q.SuppressEmail(ctx, models.SuppressEmailParams{
				WorkspaceID: ws, Email: customer, Reason: "hard_bounce", Scope: "all", Source: "dsn",
				Details: []byte(`{}`), Now: tc.Clock.Now(),
			})
		})
		path := "/api/v1/conversations/" + models.PublicID(models.PrefixConversation, conversation.ID)
		live := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		require.Nil(t, live.Subscribe(appbroadcasting.ConversationChannel(conversation.ID)).Error)

		reply := tc.ActingAsOwner().PostJSON(path+"/replies", testkit.JSON{"body": "Here is your refund."}).
			AssertCreated().String("data.id")
		tc.Drain()

		failed := publicationOf(live, "message.delivery_updated")
		var entry resources.TimelineEntry
		require.NoError(t, json.Unmarshal(failed.Data, &entry))
		assert.Equal(t, "failed", *entry.DeliveryStatus)
		assert.Equal(t, "The recipient's address bounced or complained before.", *entry.DeliveryError)

		query(tc, ws, func(ctx context.Context, q *models.Queries) (struct{}, error) {
			return struct{}{}, q.LiftBounceSuppressions(ctx, models.LiftBounceSuppressionsParams{
				WorkspaceID: ws, Email: customer,
			})
		})
		tc.ActingAsOwner().PostJSON(path+"/messages/"+reply+"/retry", nil).AssertOK().
			AssertJSONPath("data.delivery_status", "pending").AssertJSONPath("data.delivery_error", nil)
		tc.Drain()

		require.Len(t, received(t, customer), 1)
		tc.ActingAsOwner().PostJSON(path+"/messages/"+reply+"/retry", nil).AssertStatus(409).
			AssertJSONPath("code", "message_not_failed")
	})

	t.Run("sends a bounced reply again once a teammate asks", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer := unique("again")
		conversation := startConversation(tc, address, customer)
		path := "/api/v1/conversations/" + models.PublicID(models.PrefixConversation, conversation.ID)
		reply := tc.ActingAsOwner().PostJSON(path+"/replies", testkit.JSON{"body": "Here is your refund."}).
			AssertCreated().String("data.id")
		tc.Drain()
		sent := parsed(t, received(t, customer)[0])

		deliver(tc, []byte("From: Mail Delivery System <MAILER-DAEMON@mx.example.com>\r\nTo: "+
			address.ForwardingAddress+"\r\nSubject: Message not delivered\r\nX-Failed-Recipients: "+customer+
			"\r\nContent-Type: text/plain\r\n\r\nYour message could not be delivered.\r\n\r\n"+
			"Message-ID: <"+sent.MessageID+">\r\n"), "")
		tc.ActingAsOwner().GetJSON(path+"/messages").AssertJSONPath("data.1.delivery_status", "failed")

		tc.ActingAsOwner().PostJSON(path+"/messages/"+reply+"/retry", nil).AssertOK()
		tc.Drain()

		assert.Len(t, received(t, customer), 2)
		tc.ActingAsOwner().GetJSON(path+"/messages").AssertJSONPath("data.1.delivery_status", "sent")
	})
}

/*
publicationOf returns the next publication of the event, passing over
the others the same change sent first.
*/
func publicationOf(live *testkit.Realtime, event string) testkit.RealtimePublication {
	for range 10 {
		if p := live.Publication(); p.Event == event {
			return p
		}
	}

	return testkit.RealtimePublication{}
}
