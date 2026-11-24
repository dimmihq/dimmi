package sesevents_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/aws/sesevents"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("reads a bounce a configuration set publishes", func(t *testing.T) {
		t.Parallel()

		event := sesevents.Parse(`{"eventType":"Bounce","mail":{"messageId":"m-1"},"bounce":{"bounceType":"Permanent",
			"bouncedRecipients":[{"emailAddress":"gone@example.com","status":"5.1.1","diagnosticCode":"smtp; 550"}]}}`)

		assert.Equal(t, &sesevents.Event{
			Kind: sesevents.KindBounce, MessageID: "m-1", Permanent: true,
			Recipients: []sesevents.Recipient{{Address: "gone@example.com", Status: "5.1.1", Diagnostic: "smtp; 550"}},
		}, event)
	})

	t.Run("reads a transient bounce and an identity's own notification", func(t *testing.T) {
		t.Parallel()

		event := sesevents.Parse(`{"notificationType":"Bounce","mail":{"messageId":"m-2"},
			"bounce":{"bounceType":"Transient","bouncedRecipients":[{"emailAddress":"full@example.com"}]}}`)

		require.NotNil(t, event)
		assert.False(t, event.Permanent)
		assert.Equal(t, "full@example.com", event.Recipients[0].Address)
	})

	t.Run("reads a complaint and whether it says not spam", func(t *testing.T) {
		t.Parallel()

		abuse := sesevents.Parse(`{"eventType":"Complaint","mail":{"messageId":"m-3"},"complaint":{
			"complaintFeedbackType":"abuse","complainedRecipients":[{"emailAddress":"a@example.com"}]}}`)
		wanted := sesevents.Parse(`{"eventType":"Complaint","mail":{"messageId":"m-3"},"complaint":{
			"complaintFeedbackType":"not-spam","complainedRecipients":[{"emailAddress":"a@example.com"}]}}`)

		assert.Equal(t, &sesevents.Event{
			Kind: sesevents.KindComplaint, MessageID: "m-3",
			Recipients: []sesevents.Recipient{{Address: "a@example.com"}},
		}, abuse)
		assert.True(t, wanted.NotSpam)
	})

	t.Run("yields nothing for other events and for text that is no event", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, sesevents.Parse(`{"eventType":"Delivery","mail":{"messageId":"m-4"}}`))
		assert.Nil(t, sesevents.Parse(`{"eventType":"Bounce","mail":{}}`))
		assert.Nil(t, sesevents.Parse("Successfully validated SNS topic for Amazon SES event publishing."))
	})
}

func TestReaderReceive(t *testing.T) {
	t.Parallel()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)

	events := sesevents.New(client, []string{"arn:aws:sns:us-east-1:123456789012:events"})

	t.Run("rejects a body that is not an sns message", func(t *testing.T) {
		t.Parallel()

		_, err := events.Receive(t.Context(), []byte("not json"))
		require.ErrorIs(t, err, sesevents.ErrRejected)
	})

	t.Run("rejects a topic that is not allowed before fetching anything", func(t *testing.T) {
		t.Parallel()

		_, err := events.Receive(t.Context(), []byte(`{"Type":"Notification","MessageId":"m","Signature":"c2ln",
			"TopicArn":"arn:aws:sns:us-east-1:999999999999:other","Message":"{}"}`))
		require.ErrorIs(t, err, sesevents.ErrRejected)
	})
}
