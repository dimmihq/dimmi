/*
|--------------------------------------------------------------------------
| Push Subscriptions
|--------------------------------------------------------------------------
|
| A browser that allows push hands the application a subscription: the
| push service URL to post to, and the keys messages are encrypted for.
| Its JSON form is the one the browser produces, so the application can
| store what it received and pass it back unchanged.
|
| A subscription is checked before anything is sent, so a malformed one
| fails as a permanent error instead of as a refused request.
|
*/

package webpush

import (
	"crypto/ecdh"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/notifications"
)

/*
Subscription is one browser's push subscription: the endpoint messages
are posted to and the keys they are encrypted for.
*/
type Subscription struct {
	Endpoint string `json:"endpoint"`
	Keys     Keys   `json:"keys"`
}

/*
Keys are a subscription's keys, base64url encoded as the browser gives
them: the authentication secret and the P-256 public key.
*/
type Keys struct {
	Auth   string `json:"auth"`
	P256dh string `json:"p256dh"`
}

/*
authSecretSize is the length of a subscription's authentication secret.
*/
const authSecretSize = 16

/*
validate checks that the endpoint is an https URL and the keys decode
to a 16 byte secret and a P-256 public key.
*/
func (s Subscription) validate() error {
	endpoint, err := url.Parse(s.Endpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
		return fmt.Errorf("%w: endpoint is not an https url", ErrInvalidSubscription)
	}

	auth, err := decodeKey(s.Keys.Auth)
	if err != nil || len(auth) != authSecretSize {
		return fmt.Errorf("%w: auth is not a 16 byte secret", ErrInvalidSubscription)
	}

	public, err := decodeKey(s.Keys.P256dh)
	if err != nil {
		return fmt.Errorf("%w: p256dh: %w", ErrInvalidSubscription, err)
	}

	if _, err := ecdh.P256().NewPublicKey(public); err != nil {
		return fmt.Errorf("%w: p256dh: %w", ErrInvalidSubscription, err)
	}

	return nil
}

/*
decodeKey reads base64 in the URL or the standard alphabet, padded or
not, since stored subscriptions come in either.
*/
func decodeKey(key string) ([]byte, error) {
	trimmed := strings.TrimRight(key, "=")

	decoded, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err == nil {
		return decoded, nil
	}

	decoded, err = base64.RawStdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}

	return decoded, nil
}

/*
subscriptions reads a route: one subscription or a slice of them.
*/
func subscriptions(route any) ([]Subscription, error) {
	switch route := route.(type) {
	case nil:
		return nil, nil
	case Subscription:
		return []Subscription{route}, nil
	case []Subscription:
		return route, nil
	default:
		return nil, fmt.Errorf("webpush route %T: %w", route, notifications.ErrInvalidRoute)
	}
}
