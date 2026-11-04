/*
|--------------------------------------------------------------------------
| Software Authenticator
|--------------------------------------------------------------------------
|
| Tests need real registrations and real signatures, not a stand-in for
| the server's checks, so this is an authenticator in memory: one P-256
| key, a sign counter, and the same JSON a browser hands the server. It
| answers with "none" attestation, as most passkey providers do, and
| signs exactly what a hardware key would, so every check the server
| makes runs for real.
|
| The authenticator is the browser's side of a process boundary. Nothing
| in it shortcuts the server; a clone shares the key and counter at the
| moment it is made, which is how a copied key behaves.
|
*/

package webauthntest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
)

/*
ErrNoCredential means the options allow none of the authenticator's
credentials, as a browser reports when no authenticator can answer.
*/
var ErrNoCredential = errors.New("webauthntest: no allowed credential")

/*
credentialIDLength is the length of every credential id it makes.
*/
const credentialIDLength = 32

/*
The authenticator data flags it sets: user present, user verified and
attested credential data included.
*/
const (
	flagUserPresent  = 0x01
	flagUserVerified = 0x04
	flagAttested     = 0x40
)

/*
Authenticator is a software authenticator answering for Origin. It holds
one credential once Create has run.
*/
type Authenticator struct {
	Origin  string
	key     *ecdsa.PrivateKey
	id      []byte
	rpID    string
	user    []byte
	counter uint32
}

/*
New returns an authenticator for the origin, with a fresh key and
credential id.
*/
func New(origin string) *Authenticator {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	id := make([]byte, credentialIDLength)
	_, _ = rand.Read(id)

	return &Authenticator{Origin: origin, key: key, id: id}
}

/*
ID returns the credential id.
*/
func (a *Authenticator) ID() []byte {
	return a.id
}

/*
Clone returns an authenticator with the same key, credential and sign
counter, as a copied key would be.
*/
func (a *Authenticator) Clone() *Authenticator {
	c := *a

	return &c
}

/*
options is the JSON options a server handed out, read by the WebAuthn
member names.
*/
type options map[string]any

/*
text returns the string at the path of member names, empty when there
is none.
*/
func (o options) text(path ...string) string {
	var at any = map[string]any(o)

	for _, name := range path {
		m, ok := at.(map[string]any)
		if !ok {
			return ""
		}

		at = m[name]
	}

	s, _ := at.(string)

	return s
}

/*
allowed returns the credential ids the options allow.
*/
func (o options) allowed() []string {
	list, _ := o["allowCredentials"].([]any)
	out := make([]string, 0, len(list))

	for _, item := range list {
		out = append(out, options(asMap(item)).text("id"))
	}

	return out
}

/*
asMap returns the value as a JSON object, nil when it is not one.
*/
func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)

	return m
}

/*
Create registers the credential with the creation options a server
handed out and returns the registration response JSON.
*/
func (a *Authenticator) Create(raw json.RawMessage) (json.RawMessage, error) {
	var opts options
	if err := json.Unmarshal(raw, &opts); err != nil {
		return nil, fmt.Errorf("read creation options: %w", err)
	}

	user, err := decode(opts.text("user", "id"))
	if err != nil {
		return nil, fmt.Errorf("read user handle: %w", err)
	}

	a.rpID, a.user = opts.text("rp", "id"), user

	client := a.clientData("webauthn.create", opts.text("challenge"))

	attestation, err := webauthncbor.Marshal(map[string]any{
		"fmt": "none", "attStmt": map[string]any{}, "authData": a.registrationData(),
	})
	if err != nil {
		return nil, fmt.Errorf("encode attestation: %w", err)
	}

	return json.Marshal(map[string]any{
		"id": encode(a.id), "rawId": encode(a.id), "type": "public-key", "authenticatorAttachment": "platform",
		"clientExtensionResults": map[string]any{},
		"response": map[string]any{
			"clientDataJSON": encode(client), "attestationObject": encode(attestation),
			"transports": []string{"internal"},
		},
	})
}

/*
Get signs in with the request options a server handed out, advancing
the sign counter, and returns the assertion response JSON. It returns
ErrNoCredential when the options allow other credentials only.
*/
func (a *Authenticator) Get(raw json.RawMessage) (json.RawMessage, error) {
	var opts options
	if err := json.Unmarshal(raw, &opts); err != nil {
		return nil, fmt.Errorf("read request options: %w", err)
	}

	allowed := opts.allowed()
	if (len(allowed) > 0 && !slices.Contains(allowed, encode(a.id))) || opts.text("rpId") != a.rpID {
		return nil, ErrNoCredential
	}

	a.counter++

	data := a.header(flagUserPresent | flagUserVerified)
	client := a.clientData("webauthn.get", opts.text("challenge"))
	clientHash := sha256.Sum256(client)
	digest := sha256.Sum256(append(slices.Clone(data), clientHash[:]...))

	signature, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	if err != nil {
		return nil, fmt.Errorf("sign assertion: %w", err)
	}

	return json.Marshal(map[string]any{
		"id": encode(a.id), "rawId": encode(a.id), "type": "public-key", "authenticatorAttachment": "platform",
		"clientExtensionResults": map[string]any{},
		"response": map[string]any{
			"clientDataJSON": encode(client), "authenticatorData": encode(data),
			"signature": encode(signature), "userHandle": encode(a.user),
		},
	})
}

/*
clientData returns the client data a browser collects for the ceremony.
*/
func (a *Authenticator) clientData(kind, challenge string) []byte {
	out, _ := json.Marshal(map[string]any{
		"type": kind, "challenge": challenge, "origin": a.Origin, "crossOrigin": false,
	})

	return out
}

/*
header returns the authenticator data every answer starts with: the
hash of the relying party id, the flags and the sign counter.
*/
func (a *Authenticator) header(flags byte) []byte {
	rp := sha256.Sum256([]byte(a.rpID))
	out := append(slices.Clone(rp[:]), flags)

	return binary.BigEndian.AppendUint32(out, a.counter)
}

/*
registrationData returns the authenticator data of a registration: the
header, an all-zero model id, the credential id and the public key.
*/
func (a *Authenticator) registrationData() []byte {
	out := a.header(flagUserPresent | flagUserVerified | flagAttested)
	out = append(out, make([]byte, 16)...)
	out = binary.BigEndian.AppendUint16(out, credentialIDLength)
	out = append(out, a.id...)

	return append(out, a.publicKey()...)
}

/*
publicKey returns the credential's public key as a COSE key: EC2 on
P-256 for ES256.
*/
func (a *Authenticator) publicKey() []byte {
	pub, _ := a.key.PublicKey.ECDH()
	point := pub.Bytes()

	out, _ := webauthncbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: point[1:33], -3: point[33:]})

	return out
}

/*
encode returns the bytes in unpadded base64url, as WebAuthn JSON
carries them.
*/
func encode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

/*
decode returns the bytes of unpadded base64url.
*/
func decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
