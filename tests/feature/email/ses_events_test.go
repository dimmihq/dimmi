package email_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/aws/sns/snstest"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
The topic SES events are accepted from in these tests, and where SNS
asks to confirm a subscription.
*/
const (
	eventsTopic  = "arn:aws:sns:us-east-2:123456789012:dimmi-test-events"
	snsSubscribe = "https://sns.us-east-2.amazonaws.com/?Action=ConfirmSubscription&Token=tok"
)

/*
snsSigner posts to the events webhook what SNS would, signed with a key
whose certificate the application finds at the SNS certificate URL.
*/
type snsSigner struct {
	tc     *tests.TestCase
	signer *snstest.Signer
	http   *httpclienttest.Fake
}

/*
sesCase boots the application accepting SES events from the test
topic, with pause rates on, and returns what signs for that topic.
*/
func sesCase(t *testing.T) (*tests.TestCase, snsSigner) {
	t.Helper()

	tc, _ := emailCase(t, tests.WithEnv("DIMMI_CHANNELS__EMAIL__SENDING__SES__EVENTS_TOPIC_ARN="+eventsTopic))
	judged(tc)

	signer := snstest.New(t)
	fake := httpclienttest.New(tc.App.HTTP).PreventStrayRequests().
		Stub("GET "+snstest.CertURL, httpclient.Response{Body: signer.Certificate()})

	return tc, snsSigner{tc: tc, signer: signer, http: fake}
}

/*
post posts the body to the events webhook and runs what it queues.
*/
func (s snsSigner) post(body []byte) *testkit.Response {
	s.tc.T.Helper()

	res := s.tc.Client().Send("POST", "/webhooks/ses/events", body)
	s.tc.Drain()

	return res
}

/*
notify posts an SES event as a signed notification from the topic.
*/
func (s snsSigner) notify(topic string, event map[string]any) *testkit.Response {
	s.tc.T.Helper()

	message, err := json.Marshal(event)
	require.NoError(s.tc.T, err)

	return s.post(s.signer.Notification(s.tc.T, topic, ids.New().String(), string(message)))
}

/*
sesBounce is the event SES publishes when the address bounced.
*/
func sesBounce(messageID, kind, recipient string) map[string]any {
	return map[string]any{
		"eventType": "Bounce", "mail": map[string]any{"messageId": messageID},
		"bounce": map[string]any{
			"bounceType": kind, "bounceSubType": "General",
			"bouncedRecipients": []map[string]any{{
				"emailAddress": recipient, "action": "failed", "status": "5.1.1",
				"diagnosticCode": "smtp; 550 5.1.1 user unknown",
			}},
		},
	}
}

/*
sesComplaint is the event SES publishes when the address reported the
email as spam, or as wanted.
*/
func sesComplaint(messageID, feedback, recipient string) map[string]any {
	return map[string]any{
		"eventType": "Complaint", "mail": map[string]any{"messageId": messageID},
		"complaint": map[string]any{
			"complaintFeedbackType": feedback,
			"complainedRecipients":  []map[string]any{{"emailAddress": recipient}},
		},
	}
}

/*
sesReply sends a reply to a new customer as SES would have taken it,
and returns the customer, the id SES gave the email and its Message-ID.
*/
func sesReply(t *testing.T, tc *tests.TestCase) (string, string, string) {
	t.Helper()

	address := defaultAddress(tc)
	customer := unique("ses")

	result, err := send(tc, memberReply(tc, startConversation(tc, address, customer), customer))
	require.NoError(t, err)

	providerID := "ses-" + ids.New().String()
	_, err = tc.Operator().Exec(t.Context(),
		`UPDATE outbound_emails SET transport = 'ses', provider_message_id = $1 WHERE message_id_header = $2`,
		providerID, result.ChannelMessageID)
	require.NoError(t, err)

	return customer, providerID, result.ChannelMessageID
}

func TestSESEvents(t *testing.T) {
	t.Parallel()

	t.Run("a permanent bounce stops mail to the address for good, fails the reply and is counted", func(t *testing.T) {
		t.Parallel()
		tc, sns := sesCase(t)
		customer, providerID, _ := sesReply(t, tc)

		sns.notify(eventsTopic, sesBounce(providerID, "Permanent", customer)).AssertOK()

		tc.Table("outbound_emails").AssertHas(map[string]any{"provider_message_id": providerID, "status": "bounced"})
		tc.Table("email_suppressions").AssertHas(map[string]any{
			"email": customer, "reason": "hard_bounce", "source": "ses", "expires_at": nil,
		})
		tc.Table("messages").AssertCount(1, map[string]any{"delivery_status": "failed"})
		tc.Table("email_sending_stats").AssertCount(1, map[string]any{"sent": 1, "hard_bounced": 1, "complained": 0})
	})

	t.Run("a transient bounce stops mail to the address for a while and is not counted", func(t *testing.T) {
		t.Parallel()
		tc, sns := sesCase(t)
		customer, providerID, _ := sesReply(t, tc)

		sns.notify(eventsTopic, sesBounce(providerID, "Transient", customer)).AssertOK()

		tc.Table("outbound_emails").AssertHas(map[string]any{"provider_message_id": providerID, "status": "bounced"})
		tc.Table("email_suppressions").AssertHas(map[string]any{
			"email": customer, "reason": "soft_bounce", "source": "ses",
		})
		tc.Table("email_suppressions").AssertMissing(map[string]any{"email": customer, "expires_at": nil})
		tc.Table("messages").AssertCount(1, map[string]any{"delivery_status": "failed"})
		tc.Table("email_sending_stats").AssertCount(1, map[string]any{"sent": 1, "hard_bounced": 0, "complained": 0})
	})

	t.Run("a complaint stops all mail to the complainer and is counted, unless it says not spam", func(t *testing.T) {
		t.Parallel()
		tc, sns := sesCase(t)
		customer, providerID, _ := sesReply(t, tc)

		sns.notify(eventsTopic, sesComplaint(providerID, "not-spam", customer)).AssertOK()
		tc.Table("email_suppressions").AssertCount(0)

		sns.notify(eventsTopic, sesComplaint(providerID, "abuse", customer)).AssertOK()

		tc.Table("outbound_emails").AssertHas(map[string]any{"provider_message_id": providerID, "status": "complained"})
		tc.Table("email_suppressions").AssertHas(map[string]any{
			"email": customer, "reason": "complaint", "source": "ses",
		})
		tc.Table("email_sending_stats").AssertCount(1, map[string]any{"sent": 1, "hard_bounced": 0, "complained": 1})
	})

	t.Run("a replayed notification, or one already reported by mail, is counted once", func(t *testing.T) {
		t.Parallel()
		tc, sns := sesCase(t)
		customer, providerID, _ := sesReply(t, tc)

		sns.notify(eventsTopic, sesBounce(providerID, "Permanent", customer)).AssertOK()
		sns.notify(eventsTopic, sesBounce(providerID, "Permanent", customer)).AssertOK()
		sns.notify(eventsTopic, sesBounce(providerID, "Transient", customer)).AssertOK()

		tc.Table("email_sending_stats").AssertCount(1, map[string]any{"sent": 1, "hard_bounced": 1})
		tc.Table("email_suppressions").AssertCount(1, map[string]any{"email": customer, "reason": "hard_bounce"})

		address := defaultAddress(tc)
		mailed, mailedID, original := sesReply(t, tc)
		deliver(tc, fixture(t, "bounce_dsn.eml", map[string]string{
			"TO": address.ForwardingAddress, "FORWARDING": address.ForwardingAddress, "RECIPIENT": mailed,
			"ORIGINAL": original,
		}), "")
		tc.Table("email_sending_stats").AssertCount(1, map[string]any{"sent": 2, "hard_bounced": 2})

		sns.notify(eventsTopic, sesBounce(mailedID, "Permanent", mailed)).AssertOK()

		tc.Table("email_sending_stats").AssertCount(1, map[string]any{"sent": 2, "hard_bounced": 2})
		tc.Table("email_suppressions").AssertHas(map[string]any{"email": mailed, "source": "dsn"})
	})

	t.Run("refuses a bad signature, another topic and a post that is not from sns", func(t *testing.T) {
		t.Parallel()
		tc, sns := sesCase(t)
		customer, providerID, _ := sesReply(t, tc)

		message, err := json.Marshal(sesBounce(providerID, "Permanent", customer))
		require.NoError(t, err)

		signed := string(sns.signer.Notification(t, eventsTopic, "m-1", string(message)))
		tampered := strings.Replace(signed, "Permanent", "Transient", 1)
		require.NotEqual(t, signed, tampered)

		sns.post([]byte(tampered)).AssertStatus(401)
		sns.notify("arn:aws:sns:us-east-2:999999999999:someone-else", sesBounce(providerID, "Permanent", customer)).
			AssertStatus(401)
		tc.Client().Send("POST", "/webhooks/ses/events", message).AssertStatus(401)

		tc.Table("email_suppressions").AssertCount(0)
		tc.Table("outbound_emails").AssertHas(map[string]any{"provider_message_id": providerID, "status": "sent"})
		tc.Table("email_sending_stats").AssertCount(1, map[string]any{"hard_bounced": 0})
	})

	t.Run("only touches addresses the email went to, and drops events about mail it never sent", func(t *testing.T) {
		t.Parallel()
		tc, sns := sesCase(t)
		_, providerID, _ := sesReply(t, tc)

		sns.notify(eventsTopic, sesBounce(providerID, "Permanent", unique("stranger"))).AssertOK()
		sns.notify(eventsTopic, sesBounce("ses-unknown", "Permanent", unique("nobody"))).AssertOK()
		delivered := map[string]any{"eventType": "Delivery", "mail": map[string]any{"messageId": providerID}}
		sns.notify(eventsTopic, delivered).AssertOK()

		tc.Table("email_suppressions").AssertCount(0)
		tc.Table("outbound_emails").AssertHas(map[string]any{"provider_message_id": providerID, "status": "sent"})
	})

	t.Run("pauses bulk email when a bounce takes the workspace past the rate", func(t *testing.T) {
		t.Parallel()
		tc, sns := sesCase(t)
		ws := tc.Workspace.Workspace.ID
		customer, providerID, _ := sesReply(t, tc)

		_, err := tc.Operator().Exec(t.Context(),
			`UPDATE email_sending_stats SET sent = 150, hard_bounced = 7 WHERE workspace_id = $1`, ws)
		require.NoError(t, err)

		sns.notify(eventsTopic, sesBounce(providerID, "Permanent", customer)).AssertOK()

		tc.Table("email_sending_profiles").AssertCount(1, map[string]any{
			"workspace_id": ws, "status": "paused", "paused_reason": "bounce_rate",
		})
	})

	t.Run("confirms the subscription sns asks about", func(t *testing.T) {
		t.Parallel()
		_, sns := sesCase(t)
		sns.http.Stub("GET "+snsSubscribe, httpclient.Response{Body: []byte("ok")})

		sns.post(sns.signer.SubscriptionConfirmation(t, eventsTopic, snsSubscribe)).AssertOK()

		sns.http.AssertSent(t, func(r httpclienttest.SentRequest) bool { return r.URL.String() == snsSubscribe })
	})

	t.Run("answers not found on an install that named no events topic", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)

		tc.Client().Send("POST", "/webhooks/ses/events", []byte(`{}`)).AssertNotFound()
	})
}

func TestSESConfigurationSet(t *testing.T) {
	t.Parallel()

	t.Run("customer mail carries the configured set, and the mailer's own when none is", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, tests.WithEnv("DIMMI_CHANNELS__EMAIL__SENDING__SES__CONFIGURATION_SET=dimmi-events"))
		outbox := tc.Mail()
		customer := unique("set")

		_, err := send(tc, memberReply(tc, startConversation(tc, defaultAddress(tc), customer), customer))
		require.NoError(t, err)

		sent := outbox.Sent()
		require.NotEmpty(t, sent)
		assert.Equal(t, "dimmi-events", sent[len(sent)-1].Message.ConfigurationSet)

		plain, _ := emailCase(t)
		box := plain.Mail()
		_, err = send(plain, memberReply(plain, startConversation(plain, defaultAddress(plain), customer), customer))
		require.NoError(t, err)

		sent = box.Sent()
		require.NotEmpty(t, sent)
		assert.Empty(t, sent[len(sent)-1].Message.ConfigurationSet)
	})
}
