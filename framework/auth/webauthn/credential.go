/*
|--------------------------------------------------------------------------
| Credentials
|--------------------------------------------------------------------------
|
| A registered credential is public data: its id, its public key, the
| model of authenticator it lives on and what that authenticator said
| about it. Nothing here lets anyone sign in, so it is stored as plain
| JSON beside the account. The sign counter moves with every sign-in and
| is written back each time.
|
*/

package webauthn

import (
	"github.com/go-webauthn/webauthn/protocol"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
)

/*
Credential is a registered public key credential.

ID is the credential id the authenticator chose and PublicKey its COSE
public key. AAGUID names the authenticator model, all zeros when it
does not say. SignCount is the last sign counter seen, zero for
authenticators that keep none. Transports are how the browser may reach
the authenticator, such as "internal" or "usb". Attachment says whether
it is built into the device ("platform") or roams ("cross-platform").
UserVerified says the authenticator checked the person, such as a
fingerprint or PIN. BackupEligible says the credential may be synced to
other devices and BackupState that it is.
*/
type Credential struct {
	ID                []byte   `json:"id"`
	PublicKey         []byte   `json:"public_key"`
	AAGUID            []byte   `json:"aaguid,omitempty"`
	SignCount         uint32   `json:"sign_count"`
	Transports        []string `json:"transports,omitempty"`
	AttestationType   string   `json:"attestation_type,omitempty"`
	AttestationFormat string   `json:"attestation_format,omitempty"`
	Attachment        string   `json:"attachment,omitempty"`
	UserPresent       bool     `json:"user_present"`
	UserVerified      bool     `json:"user_verified"`
	BackupEligible    bool     `json:"backup_eligible"`
	BackupState       bool     `json:"backup_state"`
}

/*
holder is a user with the credentials a ceremony may use, as the
library asks for them.
*/
type holder struct {
	user        User
	credentials []gowebauthn.Credential
}

/*
WebAuthnID returns the user handle.
*/
func (h holder) WebAuthnID() []byte {
	return h.user.ID
}

/*
WebAuthnName returns the account name.
*/
func (h holder) WebAuthnName() string {
	return h.user.Name
}

/*
WebAuthnDisplayName returns the name shown for the account.
*/
func (h holder) WebAuthnDisplayName() string {
	return h.user.DisplayName
}

/*
WebAuthnCredentials returns the credentials the ceremony may use.
*/
func (h holder) WebAuthnCredentials() []gowebauthn.Credential {
	return h.credentials
}

/*
descriptors returns how a browser names the credentials.
*/
func descriptors(held []Credential) []protocol.CredentialDescriptor {
	out := make([]protocol.CredentialDescriptor, 0, len(held))
	for _, c := range toLibrary(held) {
		out = append(out, c.Descriptor())
	}

	return out
}

/*
toLibrary returns the credentials as the library keeps them.
*/
func toLibrary(held []Credential) []gowebauthn.Credential {
	out := make([]gowebauthn.Credential, 0, len(held))

	for _, c := range held {
		transports := make([]protocol.AuthenticatorTransport, 0, len(c.Transports))
		for _, t := range c.Transports {
			transports = append(transports, protocol.AuthenticatorTransport(t))
		}

		out = append(out, gowebauthn.Credential{
			ID:                c.ID,
			PublicKey:         c.PublicKey,
			AttestationType:   c.AttestationType,
			AttestationFormat: c.AttestationFormat,
			Transport:         transports,
			Flags: gowebauthn.CredentialFlags{
				UserPresent: c.UserPresent, UserVerified: c.UserVerified,
				BackupEligible: c.BackupEligible, BackupState: c.BackupState,
			},
			Authenticator: gowebauthn.Authenticator{
				AAGUID: c.AAGUID, SignCount: c.SignCount, Attachment: protocol.AuthenticatorAttachment(c.Attachment),
			},
		})
	}

	return out
}

/*
fromLibrary returns the library's credential as this package keeps it.
*/
func fromLibrary(c gowebauthn.Credential) Credential {
	transports := make([]string, 0, len(c.Transport))
	for _, t := range c.Transport {
		transports = append(transports, string(t))
	}

	return Credential{
		ID:                c.ID,
		PublicKey:         c.PublicKey,
		AAGUID:            c.Authenticator.AAGUID,
		SignCount:         c.Authenticator.SignCount,
		Transports:        transports,
		AttestationType:   c.AttestationType,
		AttestationFormat: c.AttestationFormat,
		Attachment:        string(c.Authenticator.Attachment),
		UserPresent:       c.Flags.UserPresent,
		UserVerified:      c.Flags.UserVerified,
		BackupEligible:    c.Flags.BackupEligible,
		BackupState:       c.Flags.BackupState,
	}
}
