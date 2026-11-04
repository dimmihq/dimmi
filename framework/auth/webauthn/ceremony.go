/*
|--------------------------------------------------------------------------
| Ceremonies
|--------------------------------------------------------------------------
|
| Registering a credential and signing in with one. Both hand the browser
| options as the JSON its credential API expects and keep the challenge
| in state the caller stores on the server, never in the browser, so an
| answer can only be for a challenge this server made.
|
| Signing in checks one credential at a time: the caller names the
| credential it holds, and an answer from any other is unknown here,
| which lets a caller try each of a user's credentials in turn. A sign
| counter that does not move forward, while the authenticator keeps one,
| means a copy of the key exists somewhere and the answer is refused.
|
*/

package webauthn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
)

/*
User is the account a credential belongs to. ID is the opaque handle
authenticators store with the credential, at most 64 bytes and never
personal data; Name and DisplayName help the person pick an account.
*/
type User struct {
	ID          []byte
	Name        string
	DisplayName string
}

/*
Ceremony is the first half of a ceremony: the options to hand the
browser and the state the caller keeps until the answer comes.
*/
type Ceremony struct {
	Options json.RawMessage
	State   json.RawMessage
}

/*
state is what a ceremony keeps between its halves: the library's
session and when it runs out.
*/
type state struct {
	Session   gowebauthn.SessionData `json:"session"`
	ExpiresAt time.Time              `json:"expires_at"`
}

/*
BeginRegistration starts registering a new credential for the user. The
existing credentials are excluded, so an authenticator holding one of
them declines instead of registering twice.
*/
func (rp *RelyingParty) BeginRegistration(user User, existing []Credential) (Ceremony, error) {
	creation, session, err := rp.lib.BeginRegistration(holder{user: user},
		gowebauthn.WithExclusions(descriptors(existing)))
	if err != nil {
		return Ceremony{}, fmt.Errorf("begin registration: %w", err)
	}

	return rp.ceremony(creation.Response, session)
}

/*
FinishRegistration checks the browser's answer to the registration and
returns the new credential. It returns ErrExpired for state that ran
out and ErrInvalidResponse for an answer that does not verify.
*/
func (rp *RelyingParty) FinishRegistration(user User, kept, response json.RawMessage) (Credential, error) {
	session, err := rp.open(kept)
	if err != nil {
		return Credential{}, err
	}

	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		return Credential{}, invalid(err)
	}

	made, err := rp.lib.CreateCredential(holder{user: user}, session, parsed)
	if err != nil {
		return Credential{}, invalid(err)
	}

	return fromLibrary(*made), nil
}

/*
BeginLogin starts a sign-in with one of the allowed credentials of the
user.
*/
func (rp *RelyingParty) BeginLogin(user User, allowed []Credential) (Ceremony, error) {
	assertion, session, err := rp.lib.BeginLogin(holder{user: user, credentials: toLibrary(allowed)})
	if err != nil {
		return Ceremony{}, fmt.Errorf("begin login: %w", err)
	}

	return rp.ceremony(assertion.Response, session)
}

/*
FinishLogin checks the browser's answer against the one credential and
returns the credential as the answer leaves it, with its new sign
counter and flags. It returns ErrUnknownCredential for an answer from
another credential or one the ceremony did not allow, ErrCloned when
the counter did not advance, ErrExpired for state that ran out, and
ErrInvalidResponse for an answer that does not verify.
*/
func (rp *RelyingParty) FinishLogin(user User, held Credential, kept, response json.RawMessage) (Credential, error) {
	session, err := rp.open(kept)
	if err != nil {
		return Credential{}, err
	}

	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return Credential{}, invalid(err)
	}

	if !bytes.Equal(parsed.RawID, held.ID) || !allows(session, held.ID) {
		return Credential{}, ErrUnknownCredential
	}

	session.AllowedCredentialIDs = [][]byte{held.ID}

	owner := holder{user: user, credentials: toLibrary([]Credential{held})}

	checked, err := rp.lib.ValidateLogin(owner, session, parsed)
	if err != nil {
		return Credential{}, invalid(err)
	}

	if checked.Authenticator.CloneWarning {
		return Credential{}, ErrCloned
	}

	return fromLibrary(*checked), nil
}

/*
allows reports whether the ceremony let the credential answer. A
ceremony that named no credentials lets any answer.
*/
func allows(session gowebauthn.SessionData, id []byte) bool {
	if len(session.AllowedCredentialIDs) == 0 {
		return true
	}

	return slices.ContainsFunc(session.AllowedCredentialIDs, func(allowed []byte) bool {
		return bytes.Equal(allowed, id)
	})
}

/*
ceremony encodes the options and the state of a ceremony begun now.
*/
func (rp *RelyingParty) ceremony(options any, session *gowebauthn.SessionData) (Ceremony, error) {
	encoded, err := json.Marshal(options)
	if err != nil {
		return Ceremony{}, fmt.Errorf("encode ceremony options: %w", err)
	}

	kept, err := json.Marshal(state{Session: *session, ExpiresAt: rp.clock.Now().Add(rp.ttl)})
	if err != nil {
		return Ceremony{}, fmt.Errorf("encode ceremony state: %w", err)
	}

	return Ceremony{Options: encoded, State: kept}, nil
}

/*
open returns the session of kept state that has not run out, or
ErrExpired.
*/
func (rp *RelyingParty) open(kept json.RawMessage) (gowebauthn.SessionData, error) {
	var s state
	if err := json.Unmarshal(kept, &s); err != nil {
		return gowebauthn.SessionData{}, fmt.Errorf("%w: unreadable state", ErrExpired)
	}

	if !rp.clock.Now().Before(s.ExpiresAt) {
		return gowebauthn.SessionData{}, ErrExpired
	}

	return s.Session, nil
}
