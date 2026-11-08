/*
|--------------------------------------------------------------------------
| Passes From the Messenger
|--------------------------------------------------------------------------
|
| A visitor the messenger knows, because the team's product signed them
| in or because they proved their email with a code, opens the help
| center already signed in. The messenger asks for a pass, which works
| once and for a minute and travels in the address's fragment, so it
| never reaches a server log; the help center turns it into its own
| cookie. This works on any domain, since nothing is shared but the pass.
|
*/

package helpcenters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ReaderPassTTL is how long a pass from the messenger works.
*/
const ReaderPassTTL = time.Minute

/*
ErrReaderPassInvalid says a pass is unknown, used, too old or for
another help center.
*/
var ErrReaderPassInvalid = fault.New(fault.Unauthenticated, "help_pass_invalid",
	"That link expired. Open the article from the messenger again, or sign in.")

/*
PassFor is who a pass signs in to a help center: the contact the team's
product signed in, or the address a visitor proved, when there is no
such contact.
*/
type PassFor struct {
	Center  models.HelpCenter
	Contact *ids.UUID
	Email   string
}

/*
pass is what is kept of a pass.
*/
type pass struct {
	Center  ids.UUID  `json:"center"`
	Contact *ids.UUID `json:"contact,omitempty"`
	Email   string    `json:"email,omitempty"`
}

/*
IssuePass returns a fresh pass for the reader.
*/
func (r *Readers) IssuePass(ctx context.Context, in PassFor) (string, error) {
	raw, err := json.Marshal(pass{Center: in.Center.ID, Contact: in.Contact, Email: in.Email})
	if err != nil {
		return "", fmt.Errorf("encode help pass: %w", err)
	}

	token := "dmi_hp_" + str.Random(43)
	if err := r.Cache.Set(ctx, passKey(token), raw, ReaderPassTTL); err != nil {
		return "", fmt.Errorf("keep help pass: %w", err)
	}

	return token, nil
}

/*
RedeemPass spends a pass on the help center and returns the session
that remembers its reader.
*/
func (r *Readers) RedeemPass(ctx context.Context, h models.HelpCenter, token string) (ReaderSession, error) {
	key := passKey(token)
	raw, err := r.Cache.Get(ctx, key)
	if errors.Is(err, cache.ErrMiss) {
		return ReaderSession{}, ErrReaderPassInvalid
	}

	if err != nil {
		return ReaderSession{}, fmt.Errorf("load help pass: %w", err)
	}

	var p pass
	if json.Unmarshal(raw, &p) != nil || p.Center != h.ID {
		return ReaderSession{}, ErrReaderPassInvalid
	}

	first, err := r.Cache.Add(ctx, "spent:"+key, []byte("1"), ReaderPassTTL)
	if err != nil {
		return ReaderSession{}, fmt.Errorf("spend help pass: %w", err)
	}

	if !first {
		return ReaderSession{}, ErrReaderPassInvalid
	}

	_ = r.Cache.Delete(ctx, key)
	contact, err := r.passReader(ctx, h, p)
	if err != nil {
		return ReaderSession{}, err
	}

	return r.session(h, contact), nil
}

func (r *Readers) passReader(ctx context.Context, h models.HelpCenter, p pass) (ids.UUID, error) {
	if p.Contact != nil {
		return *p.Contact, nil
	}

	return r.byEmail(ctx, h, p.Email)
}

/*
Known returns who a visitor who proved the address reads as, without
recording anything: false when nobody has it.
*/
func (r *Readers) Known(ctx context.Context, workspace ids.UUID, email string) (ids.UUID, bool) {
	ctx = tenancy.With(ctx, workspace.String())
	var out ids.UUID
	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		found, err := models.New(tx).FindReaderByEmail(ctx, models.FindReaderByEmailParams{
			WorkspaceID: workspace, Email: &email,
		})
		out = found.ID

		return err
	})

	return out, err == nil
}

/*
byEmail returns who a reader who proved the address is: the customer
the team's product named with it, else a lead with it, made when there
is none, its address now verified.
*/
func (r *Readers) byEmail(ctx context.Context, h models.HelpCenter, email string) (ids.UUID, error) {
	ctx = tenancy.With(ctx, h.WorkspaceID.String())
	now := r.Clock.Now()
	var out ids.UUID
	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		found, err := q.FindReaderByEmail(ctx, models.FindReaderByEmailParams{
			WorkspaceID: h.WorkspaceID, Email: &email,
		})
		if err == nil {
			out = found.ID
			return proveLead(ctx, q, found, now)
		}

		if !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find reader: %w", err)
		}

		lead, err := q.CreateContact(ctx, models.CreateContactParams{
			WorkspaceID: h.WorkspaceID, Kind: string(enums.ContactKindLead), Email: &email, EmailVerified: true,
			CustomAttributes: json.RawMessage(`{}`), Now: now,
		})
		if err != nil {
			return fmt.Errorf("create reader: %w", err)
		}

		out = lead.ID

		return r.Events.Dispatch(ctx, tx, appevents.ContactCreated{
			WorkspaceID: h.WorkspaceID, ContactID: lead.ID, ActorID: ids.Nil(),
		})
	})

	return out, err
}

/*
proveLead marks a lead's address verified once its reader proved it. A
customer's address is the team's product's to say.
*/
func proveLead(ctx context.Context, q *models.Queries, c models.Contact, now time.Time) error {
	if c.Kind != string(enums.ContactKindLead) || c.EmailVerified {
		return nil
	}

	_, err := q.ProveContactEmail(ctx, models.ProveContactEmailParams{
		Email: c.Email, Now: now, WorkspaceID: c.WorkspaceID, ID: c.ID,
	})

	return wrapErr("verify reader email", err)
}

func passKey(token string) string {
	return "help:pass:" + hashing.SHA256Hex([]byte(token))
}
