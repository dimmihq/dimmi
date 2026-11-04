/*
|--------------------------------------------------------------------------
| Passkey Factor
|--------------------------------------------------------------------------
|
| A passkey is a key pair on the person's device, or synced between
| their devices, that signs a fresh challenge at every sign-in. It is
| bound to this install's domain, so a look-alike site cannot ask for a
| signature it could use here. A user may hold several, one per device,
| each named so they can tell them apart.
|
| Enrolling starts pending, with the browser options and the ceremony's
| state kept on the factor until the device answers; the signed answer
| is what activates it. Only public data is stored: the credential's id,
| its public key and what the device said about it. The id's digest is
| kept apart, so a credential can be found by id alone and never
| registers twice.
|
| A sign counter that does not move forward means the key was copied,
| and that answer is refused as a reused one.
|
*/

package factors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/auth/webauthn"
	"github.com/dimmihq/dimmi/framework/hashing"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
PasskeyType is the type passkeys are stored under.
*/
const PasskeyType = "passkey"

/*
Passkey is the passkey driver. Party is the install's relying party,
nil when the install's address cannot hold passkeys, such as an IP
address; such an install never enables the kind.
*/
type Passkey struct {
	Party *webauthn.RelyingParty
}

var (
	_ Factor    = (*Passkey)(nil)
	_ Activator = (*Passkey)(nil)
	_ Prompter  = (*Passkey)(nil)
)

/*
pendingPasskey is what a pending passkey keeps until the device
answers: the state of its registration ceremony.
*/
type pendingPasskey struct {
	Registration json.RawMessage `json:"registration"`
}

/*
Type returns PasskeyType.
*/
func (d *Passkey) Type() string {
	return PasskeyType
}

/*
DefaultName returns the name a new passkey gets.
*/
func (d *Passkey) DefaultName() string {
	return "Passkey"
}

/*
Capabilities says a passkey answers the challenge, cannot be phished,
may be held several times, and is listed.
*/
func (d *Passkey) Capabilities() Capabilities {
	return Capabilities{SecondFactor: true, PhishingResistant: true, Multiple: true, UserVisible: true}
}

/*
Enroll begins registering a passkey, excluding the ones the user already
holds so a device never registers twice.
*/
func (d *Passkey) Enroll(_ context.Context, in EnrollInput) (Enrollment, error) {
	ceremony, err := d.Party.BeginRegistration(webauthn.User{
		ID: in.User.ID[:], Name: in.User.Email, DisplayName: in.User.Name,
	}, credentials(in.Held))
	if err != nil {
		return Enrollment{}, fmt.Errorf("begin passkey registration: %w", err)
	}

	pending, err := json.Marshal(pendingPasskey{Registration: ceremony.State})
	if err != nil {
		return Enrollment{}, fmt.Errorf("encode passkey registration: %w", err)
	}

	return Enrollment{
		Credential: pending, Status: enums.AuthFactorStatusPending, Setup: Setup{Options: ceremony.Options},
	}, nil
}

/*
Activate checks the device's answer to the registration and keeps the
new credential, its digest and its counter.
*/
func (d *Passkey) Activate(_ context.Context, factor *models.UserAuthFactor, answer Answer) error {
	var pending pendingPasskey
	if err := json.Unmarshal(factor.Credential, &pending); err != nil || pending.Registration == nil {
		return ErrInvalidCode
	}

	owner := webauthn.User{ID: factor.UserID[:]}

	made, err := d.Party.FinishRegistration(owner, pending.Registration, answer.Credential)
	if err != nil {
		return refused(err)
	}

	keep(factor, made)

	return nil
}

/*
Challenge begins a sign-in that any of the offered passkeys may answer.
*/
func (d *Passkey) Challenge(_ context.Context, user models.User, offered []models.UserAuthFactor) (Prompt, error) {
	ceremony, err := d.Party.BeginLogin(webauthn.User{ID: user.ID[:]}, credentials(offered))
	if err != nil {
		return Prompt{}, fmt.Errorf("begin passkey sign-in: %w", err)
	}

	return Prompt{Options: ceremony.Options, State: ceremony.State}, nil
}

/*
Verify checks a signed answer against the passkey and keeps its new
counter and flags. An answer from another of the user's passkeys is
wrong for this one.
*/
func (d *Passkey) Verify(_ context.Context, factor *models.UserAuthFactor, answer Answer) error {
	held, ok := credentialOf(*factor)
	if !ok {
		return ErrInvalidCode
	}

	checked, err := d.Party.FinishLogin(webauthn.User{ID: factor.UserID[:]}, held, answer.State, answer.Credential)
	if err != nil {
		return refused(err)
	}

	keep(factor, checked)

	return nil
}

/*
LookupHash returns the digest a passkey's credential id is found by.
*/
func LookupHash(credentialID []byte) []byte {
	return hashing.SHA256(credentialID)
}

/*
refused maps a failed ceremony to the verification failures: a copied
key is a reused answer, and every other failure a wrong one.
*/
func refused(err error) error {
	switch {
	case errors.Is(err, webauthn.ErrCloned):
		return ErrCodeReused
	case errors.Is(err, webauthn.ErrInvalidResponse), errors.Is(err, webauthn.ErrExpired),
		errors.Is(err, webauthn.ErrUnknownCredential):
		return ErrInvalidCode
	default:
		return err
	}
}

/*
keep stores the credential on the factor, with its digest and counter.
A credential holds only bytes, strings, numbers and flags, so it always
encodes.
*/
func keep(factor *models.UserAuthFactor, c webauthn.Credential) {
	encoded, _ := json.Marshal(c)

	counter := int64(c.SignCount)
	factor.Credential, factor.LookupHash, factor.LastCounter = encoded, LookupHash(c.ID), &counter
}

/*
credentialOf returns the registered credential a passkey stores, and
whether it holds one.
*/
func credentialOf(factor models.UserAuthFactor) (webauthn.Credential, bool) {
	var c webauthn.Credential
	if factor.Type != PasskeyType || json.Unmarshal(factor.Credential, &c) != nil || len(c.ID) == 0 {
		return webauthn.Credential{}, false
	}

	return c, true
}

/*
credentials returns the registered credentials among the factors.
*/
func credentials(held []models.UserAuthFactor) []webauthn.Credential {
	out := []webauthn.Credential{}

	for _, f := range held {
		if c, ok := credentialOf(f); ok {
			out = append(out, c)
		}
	}

	return out
}
