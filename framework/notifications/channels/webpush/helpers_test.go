package webpush_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/notifications/channels/webpush"
)

/*
device is a browser subscribed to push, holding the private key its
messages are encrypted for.
*/
type device struct {
	sub  webpush.Subscription
	key  *ecdh.PrivateKey
	auth []byte
}

/*
newDevice subscribes a device at the endpoint.
*/
func newDevice(t *testing.T, endpoint string) device {
	t.Helper()

	key, err := ecdh.P256().GenerateKey(rand.Reader)
	require.NoError(t, err)

	auth := make([]byte, 16)
	_, err = rand.Read(auth)
	require.NoError(t, err)

	return device{
		sub: webpush.Subscription{Endpoint: endpoint, Keys: webpush.Keys{
			Auth:   base64.RawURLEncoding.EncodeToString(auth),
			P256dh: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		}},
		key:  key,
		auth: auth,
	}
}

/*
decrypt reads a message the way the browser does, by the aes128gcm
content coding of RFC 8291, and returns the plaintext.
*/
func (d device) decrypt(t *testing.T, body []byte) []byte {
	t.Helper()

	salt, recordSize, idLength := body[:16], binary.BigEndian.Uint32(body[16:20]), int(body[20])
	require.Equal(t, uint32(4096), recordSize)

	serverKey, err := ecdh.P256().NewPublicKey(body[21 : 21+idLength])
	require.NoError(t, err)

	secret, err := d.key.ECDH(serverKey)
	require.NoError(t, err)

	info := append([]byte("WebPush: info\x00"), d.key.PublicKey().Bytes()...)
	info = append(info, serverKey.Bytes()...)
	ikm, err := hkdf.Key(sha256.New, secret, d.auth, string(info), 32)
	require.NoError(t, err)

	cek, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	require.NoError(t, err)
	nonce, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	require.NoError(t, err)

	block, err := aes.NewCipher(cek)
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)

	padded, err := gcm.Open(nil, nonce, body[21+idLength:], nil)
	require.NoError(t, err)

	padded = bytes.TrimRight(padded, "\x00")
	require.Equal(t, byte(2), padded[len(padded)-1])

	return padded[:len(padded)-1]
}

/*
claims reads the VAPID token's claims and public key from an
Authorization header.
*/
func claims(t *testing.T, header string) (map[string]any, string) {
	t.Helper()

	token, key, ok := strings.Cut(strings.TrimPrefix(header, "vapid t="), ", k=")
	require.True(t, ok, header)

	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)

	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)

	var c map[string]any
	require.NoError(t, json.Unmarshal(raw, &c))

	return c, key
}

type member struct {
	route any
}

func (member) NotifiableType() string { return "member" }

func (member) NotifiableID() string { return "m1" }

func (m member) RouteFor(string) any { return m.route }

type alert struct {
	Text string
	TTL  time.Duration
	Fail bool
}

func (alert) Type() string { return "alert" }

func (alert) Via(notifications.Notifiable) []string { return []string{"webpush"} }

func (a alert) ToWebPush(context.Context, notifications.Delivery) (webpush.Message, error) {
	if a.Fail {
		return webpush.Message{}, errors.New("payload too big to trim")
	}

	return webpush.Message{Payload: []byte(a.Text), TTL: a.TTL, Urgency: webpush.UrgencyHigh, Topic: "c1"}, nil
}

/*
silent has no push rendering.
*/
type silent struct{}

func (silent) Type() string { return "silent" }

func (silent) Via(notifications.Notifiable) []string { return []string{"webpush"} }

/*
goneList records the subscriptions reported gone.
*/
type goneList struct {
	mu   sync.Mutex
	subs []webpush.Subscription
	err  error
}

func (g *goneList) report(_ context.Context, _ notifications.Delivery, sub webpush.Subscription) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.subs = append(g.subs, sub)

	return g.err
}

/*
fixture is a manager with a webpush channel posting through a fake
transport.
*/
type fixture struct {
	manager *notifications.Manager
	fake    *httpclienttest.Fake
	client  *httpclient.Client
	clock   *clock.Fake
	cfg     webpush.Config
}

/*
newFixture builds the channel with fresh VAPID keys, reporting gone
subscriptions to gone when it is not nil.
*/
func newFixture(t *testing.T, subject string, gone webpush.GoneFunc) fixture {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)

	private, public, err := webpush.GenerateKeys()
	require.NoError(t, err)

	clk := clock.NewFake(time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC))

	m, err := notifications.New(notifications.Config{Channels: map[string]config.Section{"webpush": {
		"driver": "webpush", "vapid_public_key": public, "vapid_private_key": private, "subject": subject,
	}}}, nil)
	require.NoError(t, err)
	m.Extend("webpush", webpush.Factory(client, clk, gone, webpush.Defaults{}))

	return fixture{
		manager: m,
		fake:    httpclienttest.New(client),
		client:  client,
		clock:   clk,
		cfg:     webpush.Config{VAPIDPublicKey: public, VAPIDPrivateKey: private, Subject: subject},
	}
}

/*
channel builds a channel on the fixture's client and keys.
*/
func (f fixture) channel(t *testing.T) *webpush.Channel {
	t.Helper()

	channel, err := webpush.New(f.cfg, f.client, f.clock, nil)
	require.NoError(t, err)

	return channel
}
