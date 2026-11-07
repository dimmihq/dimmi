/*
|--------------------------------------------------------------------------
| Identity Token Checker
|--------------------------------------------------------------------------
|
| A developer wiring sign-in pastes a token in the dashboard and reads
| what the messenger would make of it: valid or not, why, the slip that
| explains a failure, the claims, and whether it would sign in a customer
| dimmi already knows or make a new one. The check is dry: no nonce is
| spent, nothing is noted, no contact or session is touched, and the
| token is never stored or logged. The signature part is never echoed.
|
*/

package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Checker explains tokens to developers.
*/
type Checker struct {
	Verifier *Verifier
	IDs      *ids.Registry
}

/*
Preview says what a valid token would do: reuse the customer it names,
or create them.
*/
type Preview struct {
	Action    string
	ContactID *ids.UUID
}

/*
Validation is what the checker found. Code, Message and Hint explain a
failure and Mistake the slip behind it; Header and Claims are what the
token says, shown only to the member who pasted it. Warnings name
things that work but likely not as meant.
*/
type Validation struct {
	Valid    bool
	Code     string
	Message  string
	Hint     string
	Mistake  *Mistake
	Header   map[string]any
	Claims   map[string]any
	Preview  *Preview
	Warnings []Mistake
}

/*
CheckInput is a token to check, and optionally the public id of the
messenger it is meant for.
*/
type CheckInput struct {
	Membership authz.Membership
	Token      string
	AppID      string
}

/*
Check verifies the token dry and explains the outcome.
*/
func (c *Checker) Check(ctx context.Context, in CheckInput) (Validation, error) {
	if err := RequireRead(in.Membership); err != nil {
		return Validation{}, err
	}
	ws := in.Membership.Workspace.ID
	p, _ := readToken(strings.TrimPrefix(strings.TrimSpace(in.Token), "Bearer "))
	out := Validation{Header: headerOf(p), Claims: p.claims}
	verified, err := c.Verifier.Check(ctx, Token{WorkspaceID: ws, AppID: in.AppID, Value: in.Token})
	if err != nil {
		var f *fault.Error
		if !errors.As(err, &f) {
			return Validation{}, err
		}
		out.Code, out.Message, out.Hint = f.Code(), f.Message(), hintOf(f)
		out.Mistake = c.diagnose(ctx, ws, failed{token: in.Token, read: p, code: f.Code()})
		return out, nil
	}
	out.Valid = true
	if _, ok := p.claims["email"]; ok && p.claims["email_verified"] == nil {
		out.Warnings = append(out.Warnings, *mistake("email_not_flagged"))
	}
	out.Preview, err = c.preview(ctx, ws, verified.Profile.ExternalID)
	return out, err
}

/*
preview finds whether the customer the token names already exists.
*/
func (c *Checker) preview(ctx context.Context, ws ids.UUID, externalID string) (*Preview, error) {
	out := &Preview{Action: "create"}
	err := c.Verifier.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		row, err := models.New(tx).FindContactByExternalID(ctx, models.FindContactByExternalIDParams{
			WorkspaceID: ws, ExternalID: &externalID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find customer: %w", err)
		}
		out.Action, out.ContactID = "reuse", &row.ID
		return nil
	})
	return out, err
}

/*
headerOf is the token's header as shown: its algorithm and key id.
*/
func headerOf(p payload) map[string]any {
	if p.header.Algorithm == "" && p.header.KeyID == "" {
		return nil
	}
	out := map[string]any{"alg": p.header.Algorithm}
	if p.header.KeyID != "" {
		out["kid"] = p.header.KeyID
	}
	return out
}

func hintOf(f *fault.Error) string {
	hint, _ := f.Meta()["hint"].(string)
	return hint
}
