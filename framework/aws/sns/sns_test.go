package sns_test

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
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
)

const (
	topic   = "arn:aws:sns:us-east-1:123456789012:inbound"
	certURL = "https://sns.us-east-1.amazonaws.com/SimpleNotificationService-abc.pem"
)

/*
signer holds a key and a self-signed certificate standing in for the one
SNS publishes.
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

	return signer{key: key, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}

/*
sign signs the text the way SNS does for the signature version.
*/
func (s signer) sign(t *testing.T, version, text string) string {
	t.Helper()

	hash := crypto.SHA256
	if version == "1" {
		hash = crypto.SHA1
	}

	digest := hash.New()
	_, _ = digest.Write([]byte(text))

	signature, err := rsa.SignPKCS1v15(rand.Reader, s.key, hash, digest.Sum(nil))
	require.NoError(t, err)

	return base64.StdEncoding.EncodeToString(signature)
}

func notification(t *testing.T, s signer, version string) sns.Message {
	t.Helper()

	text := "Message\nhello\nMessageId\nm-1\nSubject\nhi\nTimestamp\n2026-11-02T09:00:00.000Z\n" +
		"TopicArn\n" + topic + "\nType\nNotification\n"

	return sns.Message{
		Type: sns.TypeNotification, MessageID: "m-1", TopicARN: topic, Subject: "hi", Message: "hello",
		Timestamp: "2026-11-02T09:00:00.000Z", SignatureVersion: version, SigningCertURL: certURL,
		Signature: s.sign(t, version, text),
	}
}

func confirmation(t *testing.T, s signer) sns.Message {
	t.Helper()

	subscribe := "https://sns.us-east-1.amazonaws.com/?Action=ConfirmSubscription&Token=tok"
	text := "Message\nconfirm\nMessageId\nm-2\nSubscribeURL\n" + subscribe + "\nTimestamp\nnow\n" +
		"Token\ntok\nTopicArn\n" + topic + "\nType\nSubscriptionConfirmation\n"

	return sns.Message{
		Type: sns.TypeSubscriptionConfirmation, MessageID: "m-2", TopicARN: topic, Message: "confirm",
		Timestamp: "now", Token: "tok", SubscribeURL: subscribe, SignatureVersion: "2",
		SigningCertURL: certURL, Signature: s.sign(t, "2", text),
	}
}

/*
verifier returns a verifier whose client finds the signer's certificate
at the SNS certificate URL, and the fake recording those fetches.
*/
func verifier(t *testing.T, s signer) (*sns.Verifier, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)

	fake := httpclienttest.New(client).PreventStrayRequests().Stub("GET "+certURL, httpclient.Response{Body: s.pem})

	return sns.NewVerifier(client), fake
}

func TestVerify(t *testing.T) {
	t.Parallel()

	s := newSigner(t)
	allow := sns.Allow{TopicARNs: []string{topic}}

	t.Run("accepts messages signed with either signature version", func(t *testing.T) {
		t.Parallel()
		v, _ := verifier(t, s)

		require.NoError(t, v.Verify(t.Context(), notification(t, s, "1"), allow))
		require.NoError(t, v.Verify(t.Context(), notification(t, s, "2"), allow))
		require.NoError(t, v.Verify(t.Context(), confirmation(t, s), allow))
	})

	t.Run("keeps a fetched certificate", func(t *testing.T) {
		t.Parallel()
		v, fake := verifier(t, s)
		require.NoError(t, v.Verify(t.Context(), notification(t, s, "2"), allow))

		fake.Stub("GET "+certURL, httpclient.Response{Status: http.StatusNotFound})

		require.NoError(t, v.Verify(t.Context(), notification(t, s, "1"), allow))
	})

	t.Run("rejects a tampered message", func(t *testing.T) {
		t.Parallel()
		v, _ := verifier(t, s)
		msg := notification(t, s, "2")
		msg.Message = "goodbye"

		require.ErrorIs(t, v.Verify(t.Context(), msg, allow), sns.ErrSignature)
	})

	t.Run("rejects a signature that is not base64", func(t *testing.T) {
		t.Parallel()
		v, _ := verifier(t, s)
		msg := notification(t, s, "2")
		msg.Signature = "%%%"

		require.ErrorIs(t, v.Verify(t.Context(), msg, allow), sns.ErrSignature)
	})

	t.Run("rejects a topic that is not allowed", func(t *testing.T) {
		t.Parallel()
		v, _ := verifier(t, s)

		require.ErrorIs(t, v.Verify(t.Context(), notification(t, s, "2"), sns.Allow{}), sns.ErrTopicNotAllowed)
	})

	t.Run("rejects an unknown signature version or type", func(t *testing.T) {
		t.Parallel()
		v, _ := verifier(t, s)
		badVersion := notification(t, s, "2")
		badVersion.SignatureVersion = "3"
		badType := notification(t, s, "2")
		badType.Type = "Other"

		require.ErrorIs(t, v.Verify(t.Context(), badVersion, allow), sns.ErrInvalidMessage)
		require.ErrorIs(t, v.Verify(t.Context(), badType, allow), sns.ErrInvalidMessage)
	})

	t.Run("rejects certificates not served by sns over https", func(t *testing.T) {
		t.Parallel()
		v, _ := verifier(t, s)

		for _, url := range []string{
			"http://sns.us-east-1.amazonaws.com/cert.pem",
			"https://evil.example.com/cert.pem",
			"https://sns.us-east-1.amazonaws.com.evil.example/cert.pem",
			"https://sns..amazonaws.com/cert.pem",
			"https://sns.us_east.amazonaws.com/cert.pem",
			"https://sns.us-east-1.amazonaws.com:8443/cert.pem",
			"https://sns.us-east-1.amazonaws.com/cert.txt",
			"https://sns.amazonaws.com/cert.pem",
			"://bad",
		} {
			msg := notification(t, s, "2")
			msg.SigningCertURL = url

			require.ErrorIs(t, v.Verify(t.Context(), msg, allow), sns.ErrCertificate, url)
		}
	})

	t.Run("accepts the certificate host of china regions", func(t *testing.T) {
		t.Parallel()
		v, fake := verifier(t, s)
		cnURL := "https://sns.cn-north-1.amazonaws.com.cn/SimpleNotificationService-cn.pem"
		fake.Stub("GET "+cnURL, httpclient.Response{Body: s.pem})
		msg := notification(t, s, "2")
		msg.SigningCertURL = cnURL

		require.NoError(t, v.Verify(t.Context(), msg, allow))
	})

	t.Run("rejects a certificate that cannot be fetched or read", func(t *testing.T) {
		t.Parallel()
		v, fake := verifier(t, s)
		base := "https://sns.us-west-2.amazonaws.com/"
		fake.Stub("GET "+base+"gone.pem", httpclient.Response{Status: http.StatusNotFound}).
			Stub("GET "+base+"junk.pem", httpclient.Response{Body: []byte("junk")}).
			Stub("GET "+base+"bad.pem", httpclient.Response{Body: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE"})})

		for _, name := range []string{"gone.pem", "junk.pem", "bad.pem"} {
			msg := notification(t, s, "2")
			msg.SigningCertURL = base + name

			require.ErrorIs(t, v.Verify(t.Context(), msg, allow), sns.ErrCertificate, name)
		}

		stray := notification(t, s, "2")
		stray.SigningCertURL = base + "stray.pem"
		require.ErrorIs(t, v.Verify(t.Context(), stray, allow), httpclienttest.ErrStrayRequest)
	})
}

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("reads the fields sns posts", func(t *testing.T) {
		t.Parallel()
		body, err := json.Marshal(map[string]string{
			"Type": "Notification", "MessageId": "m-1", "TopicArn": topic, "Subject": "hi",
			"Message": "hello", "Timestamp": "now", "SignatureVersion": "1", "Signature": "c2ln",
			"SigningCertURL": certURL, "UnsubscribeURL": "https://sns.us-east-1.amazonaws.com/?Action=Unsubscribe",
		})
		require.NoError(t, err)

		msg, err := sns.Parse(body)

		require.NoError(t, err)
		assert.Equal(t, sns.Message{
			Type: "Notification", MessageID: "m-1", TopicARN: topic, Subject: "hi", Message: "hello",
			Timestamp: "now", SignatureVersion: "1", Signature: "c2ln", SigningCertURL: certURL,
			UnsubscribeURL: "https://sns.us-east-1.amazonaws.com/?Action=Unsubscribe",
		}, msg)
	})

	t.Run("rejects a body that is not an sns message", func(t *testing.T) {
		t.Parallel()

		_, err := sns.Parse([]byte("not json"))
		require.ErrorIs(t, err, sns.ErrInvalidMessage)
		_, err = sns.Parse([]byte(`{"Type":"Notification"}`))
		require.ErrorIs(t, err, sns.ErrInvalidMessage)
	})
}
