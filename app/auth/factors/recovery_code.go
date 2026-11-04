/*
|--------------------------------------------------------------------------
| Recovery Code Factor
|--------------------------------------------------------------------------
|
| Ten single-use codes for when the authenticator is not at hand. They
| are shown once, when made, and each works once; only their digests are
| kept, sealed. The set stands in for the authenticator and never turns
| two-factor authentication on alone, so it is made when the first real
| factor is activated and removed with the last one.
|
| A used-up set stays on the account with none left, so the settings can
| say so, and drops out of the sign-in challenge.
|
*/

package factors

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/auth/twofactor"
	"github.com/dimmihq/dimmi/framework/encryption"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RecoveryCodeType is the type recovery code sets are stored under.
*/
const RecoveryCodeType = twofactor.RecoveryCodeType

/*
RecoveryCodeCount is the number of codes in a set.
*/
const RecoveryCodeCount = 10

/*
Codes is how many codes a set started with and how many are left.
*/
type Codes struct {
	Total     int `json:"total"`
	Remaining int `json:"remaining"`
}

/*
RecoveryCodes is the recovery code driver. Keys seal the code digests.
*/
type RecoveryCodes struct {
	Keys encryption.KeyManager
}

var _ Factor = (*RecoveryCodes)(nil)

/*
Type returns RecoveryCodeType.
*/
func (d *RecoveryCodes) Type() string {
	return RecoveryCodeType
}

/*
DefaultName returns the name of every set.
*/
func (d *RecoveryCodes) DefaultName() string {
	return "Recovery codes"
}

/*
Capabilities says a set answers the challenge as a stand-in only, and
is listed.
*/
func (d *RecoveryCodes) Capabilities() Capabilities {
	return Capabilities{SecondFactor: true, Backup: true, UserVisible: true}
}

/*
Enroll makes a new set of codes, active at once.
*/
func (d *RecoveryCodes) Enroll(ctx context.Context, in EnrollInput) (Enrollment, error) {
	codes, sealed, err := twofactor.NewRecoveryCodes(bind(d.Keys, in.ID)).Generate(ctx, RecoveryCodeCount)
	if err != nil {
		return Enrollment{}, fmt.Errorf("make recovery codes: %w", err)
	}

	return Enrollment{
		Secret:     sealed,
		Credential: encodeCodes(Codes{Total: RecoveryCodeCount, Remaining: RecoveryCodeCount}),
		Status:     enums.AuthFactorStatusActive,
		Setup:      Setup{Codes: codes},
	}, nil
}

/*
Verify accepts a code still in the set and removes it, so it never
works again.
*/
func (d *RecoveryCodes) Verify(ctx context.Context, factor *models.UserAuthFactor, answer Answer) error {
	codes := twofactor.NewRecoveryCodes(bind(d.Keys, factor.ID))
	cred := twofactor.Credential{ID: factor.ID, Type: factor.Type, Secret: factor.SecretEncrypted}

	if err := codes.Verify(ctx, &cred, answer.Code); err != nil {
		return err
	}

	counts := CodesOf(*factor)
	counts.Remaining = max(counts.Remaining-1, 0)
	factor.SecretEncrypted = cred.Secret
	factor.Credential = encodeCodes(counts)

	return nil
}

/*
CodesOf returns how many codes the set started with and how many are
left, as its credential records them.
*/
func CodesOf(factor models.UserAuthFactor) Codes {
	var c Codes

	_ = json.Unmarshal(factor.Credential, &c)

	return c
}

/*
encodeCodes returns the counts as the set's credential. Two integers
always encode.
*/
func encodeCodes(c Codes) json.RawMessage {
	out, _ := json.Marshal(c)

	return out
}
