/*
Package snstest signs messages the way Amazon SNS does, so tests can post
to webhooks that only take what SNS signed.
*/
package snstest

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
	"testing"
	"time"

	"github.com/dimmihq/dimmi/framework/aws/sns"
)

/*
CertURL is where a signer's messages say its certificate is. Serve
[Signer.Certificate] there.
*/
const CertURL = "https://sns.us-east-1.amazonaws.com/SimpleNotificationService-test.pem"

/*
Signer holds a key and the self-signed certificate standing in for the
one SNS publishes.
*/
type Signer struct {
	key  *rsa.PrivateKey
	cert []byte
}

/*
New returns a signer with a fresh key.
*/
func New(tb testing.TB) *Signer {
	tb.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		tb.Fatalf("generate sns key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "sns.amazonaws.com"},
		NotBefore: time.Unix(0, 0), NotAfter: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		tb.Fatalf("create sns certificate: %v", err)
	}

	return &Signer{key: key, cert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}

/*
Certificate returns the certificate in PEM, as SNS serves it.
*/
func (s *Signer) Certificate() []byte {
	return s.cert
}

/*
Notification returns the body SNS posts for a notification from the
topic carrying the message.
*/
func (s *Signer) Notification(tb testing.TB, topicARN, messageID, message string) []byte {
	tb.Helper()

	return s.body(tb, map[string]string{
		"Type": sns.TypeNotification, "MessageId": messageID, "TopicArn": topicARN, "Message": message,
		"Timestamp": "2026-11-02T09:00:00.000Z",
	}, "Message", "MessageId", "Timestamp", "TopicArn", "Type")
}

/*
SubscriptionConfirmation returns the body SNS posts to ask an endpoint
to confirm its subscription to the topic at the URL.
*/
func (s *Signer) SubscriptionConfirmation(tb testing.TB, topicARN, subscribeURL string) []byte {
	tb.Helper()

	return s.body(tb, map[string]string{
		"Type": sns.TypeSubscriptionConfirmation, "MessageId": "confirm-1", "TopicArn": topicARN,
		"Message": "confirm", "Timestamp": "2026-11-02T09:00:00.000Z", "Token": "token",
		"SubscribeURL": subscribeURL,
	}, "Message", "MessageId", "SubscribeURL", "Timestamp", "Token", "TopicArn", "Type")
}

/*
body signs the named fields in order, with signature version 2, and
returns the message as JSON.
*/
func (s *Signer) body(tb testing.TB, fields map[string]string, signed ...string) []byte {
	tb.Helper()

	digest := crypto.SHA256.New()
	for _, name := range signed {
		_, _ = digest.Write([]byte(name + "\n" + fields[name] + "\n"))
	}

	signature, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, digest.Sum(nil))
	if err != nil {
		tb.Fatalf("sign sns message: %v", err)
	}

	fields["SignatureVersion"], fields["SigningCertURL"] = "2", CertURL
	fields["Signature"] = base64.StdEncoding.EncodeToString(signature)

	out, err := json.Marshal(fields)
	if err != nil {
		tb.Fatalf("encode sns message: %v", err)
	}

	return out
}
