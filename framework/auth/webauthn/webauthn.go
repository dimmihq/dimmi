/*
|--------------------------------------------------------------------------
| Relying Party
|--------------------------------------------------------------------------
|
| A site that accepts public key credentials is a relying party. It names
| itself by a domain, the authenticator binds every credential to that
| domain, and every answer must come from one of the site's own origins.
| That binding is what makes a passkey impossible to phish: a look-alike
| site gets nothing it can replay here.
|
| A ceremony has two halves. The first hands the browser options with a
| fresh random challenge and returns state the caller keeps on the
| server; the second checks the browser's answer against that state. The
| state lives only a few minutes, measured on the caller's clock, and
| proves nothing once it has been used.
|
*/

package webauthn

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
Failures of a relying party and its ceremonies.

ErrInvalidConfig means the relying party cannot work, such as a domain
that is an IP address. ErrInvalidResponse means the browser's answer
does not verify: a wrong challenge, origin, domain or signature.
ErrExpired means the ceremony's state ran out or is unreadable.
ErrUnknownCredential means the answer came from another credential than
the one checked. ErrCloned means the credential's sign counter did not
advance, so a copy of its key may be in use.
*/
var (
	ErrInvalidConfig     = errors.New("webauthn: invalid relying party")
	ErrInvalidResponse   = errors.New("webauthn: invalid response")
	ErrExpired           = errors.New("webauthn: ceremony expired")
	ErrUnknownCredential = errors.New("webauthn: unknown credential")
	ErrCloned            = errors.New("webauthn: sign counter did not advance")
)

/*
DefaultTTL is how long a ceremony may be finished when the
configuration names no other time.
*/
const DefaultTTL = 5 * time.Minute

/*
Config describes the relying party. ID is its domain, such as
"example.com", without scheme or port. Name is shown by some
authenticators. Origins are the full origins answers may come from,
such as "https://example.com". TTL is how long a ceremony may be
finished, DefaultTTL when zero, and Clock measures it, the system clock
when nil.
*/
type Config struct {
	ID      string
	Name    string
	Origins []string
	TTL     time.Duration
	Clock   clock.Clock
}

/*
RelyingParty runs registration and sign-in ceremonies.
*/
type RelyingParty struct {
	lib   *gowebauthn.WebAuthn
	ttl   time.Duration
	clock clock.Clock
}

/*
New returns the relying party of the configuration, or ErrInvalidConfig
when it cannot work. Credentials are asked to verify their user when
they can and to be discoverable when they can, and no attestation is
requested, so any authenticator may register.
*/
func New(cfg Config) (*RelyingParty, error) {
	if cfg.ID == "" || cfg.Name == "" || len(cfg.Origins) == 0 {
		return nil, fmt.Errorf("%w: id, name and origins are required", ErrInvalidConfig)
	}

	if err := protocol.ValidateRPID(cfg.ID); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidConfig, err.Error())
	}

	required := false

	lib, err := gowebauthn.New(&gowebauthn.Config{
		RPID:                  cfg.ID,
		RPDisplayName:         cfg.Name,
		RPOrigins:             cfg.Origins,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementPreferred,
			RequireResidentKey: &required,
			UserVerification:   protocol.VerificationPreferred,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidConfig, err.Error())
	}

	rp := &RelyingParty{lib: lib, ttl: cfg.TTL, clock: cfg.Clock}
	if rp.ttl <= 0 {
		rp.ttl = DefaultTTL
	}

	if rp.clock == nil {
		rp.clock = clock.System()
	}

	return rp, nil
}

/*
invalid returns ErrInvalidResponse with what the library found wrong.
*/
func invalid(err error) error {
	var detailed *protocol.Error
	if errors.As(err, &detailed) && detailed.DevInfo != "" {
		return fmt.Errorf("%w: %s: %s", ErrInvalidResponse, detailed.Details, detailed.DevInfo)
	}

	return fmt.Errorf("%w: %s", ErrInvalidResponse, err.Error())
}
