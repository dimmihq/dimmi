package ses_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/aws/sns"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/local"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/ses"
	"github.com/dimmihq/dimmi/framework/mailbox/drivertest"
)

const (
	topic     = "arn:aws:sns:us-east-1:123456789012:inbound"
	certURL   = "https://sns.us-east-1.amazonaws.com/SimpleNotificationService-abc.pem"
	subscribe = "https://sns.us-east-1.amazonaws.com/?Action=ConfirmSubscription&Token=tok"
)

/*
signer signs messages as SNS does, with a key whose self-signed
certificate stands in for the one SNS publishes.
*/
type signer struct {
	key *rsa.PrivateKey
	pem []byte
}

func newSigner(t *testing.T) signer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "sns.amazonaws.com"},
		NotBefore:    time.Unix(0, 0),
		NotAfter:     time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	return signer{key: key, pem: pemCertificate(der)}
}

/*
post returns the SNS body for the fields, signed over the fields SNS
signs for the message type.
*/
func (s signer) post(t *testing.T, fields map[string]string) []byte {
	t.Helper()

	names := []string{"Message", "MessageId", "Timestamp", "TopicArn", "Type"}
	if fields["Type"] != "Notification" {
		names = []string{"Message", "MessageId", "SubscribeURL", "Timestamp", "Token", "TopicArn", "Type"}
	}

	var text string
	for _, name := range names {
		text += name + "\n" + fields[name] + "\n"
	}

	digest := crypto.SHA256.New()
	_, _ = digest.Write([]byte(text))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, digest.Sum(nil))
	require.NoError(t, err)

	fields["SignatureVersion"] = "2"
	fields["SigningCertURL"] = certURL
	fields["Signature"] = base64.StdEncoding.EncodeToString(signature)

	body, err := json.Marshal(fields)
	require.NoError(t, err)

	return body
}

/*
notification returns the SNS post announcing an SES receipt.
*/
func (s signer) notification(t *testing.T, receipt map[string]any) []byte {
	t.Helper()

	message, err := json.Marshal(receipt)
	require.NoError(t, err)

	return s.post(t, map[string]string{
		"Type": "Notification", "MessageId": "sns-1", "TopicArn": topic, "Message": string(message),
		"Timestamp": "2026-11-02T10:04:01.000Z",
	})
}

func received(d drivertest.Delivery) map[string]any {
	verdict := func(status string) map[string]string { return map[string]string{"status": status} }

	return map[string]any{
		"notificationType": "Received",
		"mail":             map[string]any{"source": d.EnvelopeFrom, "messageId": d.ID},
		"receipt": map[string]any{
			"timestamp":    "2026-11-02T10:04:00.000Z",
			"recipients":   d.Recipients,
			"spfVerdict":   verdict("PASS"),
			"dkimVerdict":  verdict("GRAY"),
			"dmarcVerdict": verdict("FAIL"),
			"dmarcPolicy":  "REJECT",
			"spamVerdict":  verdict("PASS"),
			"virusVerdict": verdict("PROCESSING_FAILED"),
			"action":       map[string]string{"type": "S3", "bucketName": "inbound", "objectKey": "raw/" + d.ID},
		},
	}
}

/*
fixture is a source reading from a local disk, with the fake that
answers its certificate fetches.
*/
type fixture struct {
	source *ses.Source
	disk   filesystem.Filesystem
	fake   *httpclienttest.Fake
	signer signer
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	s := newSigner(t)
	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)
	fake := httpclienttest.New(client).PreventStrayRequests().Stub("GET "+certURL, httpclient.Response{Body: s.pem})

	disks := filesystem.NewManager(filesystem.Config{Disks: map[string]config.Section{
		"ses_inbound": {"driver": "local", "root": t.TempDir()},
	}})
	disks.Extend("local", local.Factory)
	disk, err := disks.Disk("ses_inbound")
	require.NoError(t, err)

	source, err := ses.Factory(client, disks)("ses", config.Section{
		"driver": "ses", "topic_arns": []string{topic}, "max_message_bytes": 4096,
	})
	require.NoError(t, err)

	typed, ok := source.(*ses.Source)
	require.True(t, ok)

	return fixture{source: typed, disk: disk, fake: fake, signer: s}
}

func (f fixture) store(t *testing.T, d drivertest.Delivery) {
	t.Helper()

	require.NoError(t, f.disk.Put(t.Context(), "raw/"+d.ID, d.Raw, filesystem.PutOptions{}))
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	f := newFixture(t)

	return drivertest.Harness{
		Source:   f.source,
		MaxBytes: 4096,
		Payload: func(t *testing.T, d drivertest.Delivery) []byte {
			t.Helper()
			f.store(t, d)

			return f.signer.notification(t, received(d))
		},
		Forged: func(t *testing.T, d drivertest.Delivery) []byte {
			t.Helper()
			f.store(t, d)

			return newSigner(t).notification(t, received(d))
		},
	}
}

func TestSource(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHarness)
}

func pemCertificate(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestVerdicts(t *testing.T) {
	t.Parallel()

	t.Run("hands over the verdicts and where the raw message is", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		d := drivertest.NewDelivery()
		f.store(t, d)

		env, err := f.source.Receive(t.Context(), f.signer.notification(t, received(d)))
		require.NoError(t, err)

		assert.Equal(t, mailbox.Verdicts{
			SPF: mailbox.VerdictPass, DKIM: mailbox.VerdictNone, DMARC: mailbox.VerdictFail,
			Spam: mailbox.VerdictPass, Virus: mailbox.VerdictUnknown, DMARCPolicy: "reject",
		}, env.Verdicts)
		assert.Equal(t, &mailbox.RawRef{Disk: "ses_inbound", Path: "raw/" + d.ID}, env.RawRef)
		assert.Equal(t, time.Date(2026, 11, 2, 10, 4, 0, 0, time.UTC), env.ReceivedAt)
		assert.Equal(t, "ses", f.source.Name())
	})
}

func TestSubscription(t *testing.T) {
	t.Parallel()

	confirmation := func(t *testing.T, f fixture, kind string) []byte {
		t.Helper()

		return f.signer.post(t, map[string]string{
			"Type": kind, "MessageId": "sns-2", "TopicArn": topic, "Message": "confirm",
			"Timestamp": "2026-11-02T10:00:00.000Z", "Token": "tok", "SubscribeURL": subscribe,
		})
	}

	t.Run("confirms a subscription by visiting the signed url", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.fake.Stub("GET "+subscribe, httpclient.Response{Status: http.StatusOK})

		env, err := f.source.Receive(t.Context(), confirmation(t, f, sns.TypeSubscriptionConfirmation))

		require.NoError(t, err)
		assert.Nil(t, env)
		f.fake.AssertSent(t, func(req httpclienttest.SentRequest) bool { return req.URL.String() == subscribe })
	})

	t.Run("reports a confirmation sns refuses", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.fake.Stub("GET "+subscribe, httpclient.Response{Status: http.StatusForbidden})

		_, err := f.source.Receive(t.Context(), confirmation(t, f, sns.TypeSubscriptionConfirmation))

		require.Error(t, err)
	})

	t.Run("reports a confirmation that cannot be sent", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)

		_, err := f.source.Receive(t.Context(), confirmation(t, f, sns.TypeSubscriptionConfirmation))

		require.ErrorIs(t, err, httpclienttest.ErrStrayRequest)
	})

	t.Run("reports the end of the subscription", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)

		_, err := f.source.Receive(t.Context(), confirmation(t, f, sns.TypeUnsubscribeConfirmation))

		require.ErrorIs(t, err, mailbox.ErrUnsubscribed)
	})
}
