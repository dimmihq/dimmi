/*
|--------------------------------------------------------------------------
| Delivery Signatures
|--------------------------------------------------------------------------
|
| Each tool proves a delivery its own way: Linear, GitHub and PagerDuty
| sign the raw body with HMAC-SHA256; Rootly signs a timestamp and the
| body; incident.io signs through Svix, the delivery id, timestamp and
| body; Jira sends a JSON web token signed with the app's secret. Every
| check compares in constant time and refuses an old timestamp, so a
| captured delivery cannot be replayed later.
|
*/

package trackers

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/hashing"
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
replayWindow is how old a signed timestamp may be.
*/
const replayWindow = 5 * time.Minute

/*
Event is one thing a delivery says: an issue or incident that changed,
a comment on one, a new incident, a public update, or a deployment.
*/
type Event struct {
	Kind        string
	ExternalID  string
	Issue       *Issue
	Body        string
	Author      string
	Repository  string
	Environment string
	Succeeded   bool
}

/*
Signed is a delivery as received: its headers and raw body.
*/
type Signed struct {
	Header fhttp.Header
	Body   []byte
}

/*
mac returns the HMAC-SHA256 of the parts with the secret.
*/
func mac(secret []byte, parts ...[]byte) []byte {
	var message []byte
	for _, p := range parts {
		message = append(message, p...)
	}

	return hashing.HMAC(secret, message)
}

/*
Sign returns the hex HMAC-SHA256 of the body, as Linear, GitHub and
PagerDuty sign.
*/
func Sign(secret string, body []byte) string {
	return hex.EncodeToString(mac([]byte(secret), body))
}

/*
sameHex compares a hex signature with the expected bytes.
*/
func sameHex(given string, want []byte) bool {
	raw, err := hex.DecodeString(strings.TrimSpace(given))

	return err == nil && hashing.Equal(raw, want)
}

/*
verifyHex checks a header holding the hex HMAC of the body, after the
prefix.
*/
func verifyHex(s Signed, header, prefix, secret string) error {
	given, ok := strings.CutPrefix(s.Header.Get(header), prefix)
	if secret == "" || !ok || !sameHex(given, mac([]byte(secret), s.Body)) {
		return ErrBadSignature
	}

	return nil
}

/*
fresh reports whether a Unix timestamp in seconds is within the replay
window of now.
*/
func fresh(seconds string, now time.Time) bool {
	n, err := strconv.ParseInt(strings.TrimSpace(seconds), 10, 64)
	if err != nil {
		return false
	}

	age := now.Sub(time.Unix(n, 0))

	return age < replayWindow && age > -replayWindow
}

/*
verifySvix checks an incident.io delivery signed through Svix: the
secret is base64 after whsec_, and any of the space-separated v1
signatures may match.
*/
func verifySvix(s Signed, secret string, now time.Time) error {
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil || secret == "" {
		return ErrBadSignature
	}

	id, ts := s.Header.Get("Webhook-Id"), s.Header.Get("Webhook-Timestamp")
	if id == "" || !fresh(ts, now) {
		return ErrBadSignature
	}

	want := mac(key, []byte(id+"."+ts+"."), s.Body)

	for sig := range strings.FieldsSeq(s.Header.Get("Webhook-Signature")) {
		version, value, _ := strings.Cut(sig, ",")
		raw, err := base64.StdEncoding.DecodeString(value)

		if version == "v1" && err == nil && hashing.Equal(raw, want) {
			return nil
		}
	}

	return ErrBadSignature
}

/*
SvixSignature signs a delivery as Svix does, for tests and fixtures.
*/
func SvixSignature(secret, id, ts string, body []byte) string {
	key, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))

	return "v1," + base64.StdEncoding.EncodeToString(mac(key, []byte(id+"."+ts+"."), body))
}

/*
verifyRootly checks t=<seconds>,v1=<hex> over the timestamp and body.
*/
func verifyRootly(s Signed, secret string, now time.Time) error {
	var ts, sig string

	for part := range strings.SplitSeq(s.Header.Get("X-Rootly-Signature"), ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "=")

		switch k {
		case "t":
			ts = v
		case "v1":
			sig = v
		}
	}

	if secret == "" || !fresh(ts, now) || !sameHex(sig, mac([]byte(secret), []byte(ts), s.Body)) {
		return ErrBadSignature
	}

	return nil
}

/*
RootlySignature signs a delivery as Rootly does, for tests.
*/
func RootlySignature(secret, ts string, body []byte) string {
	return "t=" + ts + ",v1=" + hex.EncodeToString(mac([]byte(secret), []byte(ts), body))
}

/*
verifyPagerDuty checks X-PagerDuty-Signature, which lists one v1
signature per secret the subscription has.
*/
func verifyPagerDuty(s Signed, secret string) error {
	want := mac([]byte(secret), s.Body)

	for sig := range strings.SplitSeq(s.Header.Get("X-PagerDuty-Signature"), ",") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(sig), "v1="); ok && secret != "" && sameHex(value, want) {
			return nil
		}
	}

	return ErrBadSignature
}

/*
verifyJWT checks a Jira delivery's bearer token: HS256 signed with the
app's secret and not expired.
*/
func verifyJWT(s Signed, secret string, now time.Time) error {
	token, ok := strings.CutPrefix(s.Header.Get("Authorization"), "Bearer ")
	if !ok {
		token, ok = strings.CutPrefix(s.Header.Get("Authorization"), "JWT ")
	}

	parts := strings.Split(token, ".")
	if !ok || secret == "" || len(parts) != 3 {
		return ErrBadSignature
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hashing.Equal(sig, mac([]byte(secret), []byte(parts[0]+"."+parts[1]))) {
		return ErrBadSignature
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ErrBadSignature
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}

	if json.Unmarshal(payload, &claims) != nil || (claims.Exp != 0 && now.Unix() > claims.Exp) {
		return ErrBadSignature
	}

	return nil
}

/*
SignJWT makes an HS256 token with the claims, as Jira signs its
deliveries, for tests.
*/
func SignJWT(secret string, claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(body)
	sig := base64.RawURLEncoding.EncodeToString(mac([]byte(secret), []byte(header+"."+payload)))

	return header + "." + payload + "." + sig
}
