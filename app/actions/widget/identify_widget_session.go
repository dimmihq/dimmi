/*
|--------------------------------------------------------------------------
| Signed-In Widget Sessions
|--------------------------------------------------------------------------
|
| A messenger on a page whose visitor is signed in sends the identity
| token its host app minted. A token that fails changes nothing at all:
| the visitor keeps whatever session they had and the developer reads why.
| A good token starts a signed-in session for the customer it names; an
| anonymous session the visitor already had is handed over, its lead
| becoming the customer, and then ends. A session already signed in as
| the same customer is simply extended. Refreshing needs a token for the
| very customer the session belongs to, so a signed-in session never
| changes hands.
|
*/

package widget

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/identity"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
IdentityVerifier checks the identity tokens visitors' messengers send.
*/
type IdentityVerifier interface {
	/*
		Verify checks a token and returns who it names, or a fault safe
		to show the developer.
	*/
	Verify(ctx context.Context, t identity.Token) (identity.Verified, error)
}

/*
identified starts a signed-in session from a verified token, or extends
the one the visitor already has for the same customer.
*/
func (a *StartWidgetSession) identified(ctx context.Context, in StartWidgetSessionInput) (StartedSession, error) {
	if a.Identity == nil {
		return StartedSession{}, exceptions.ErrIdentityNotConfigured
	}
	verified, err := a.Identity.Verify(ctx, identity.Token{
		WorkspaceID: in.Resolved.Workspace.ID, AppID: models.PublicID(models.PrefixWidget, in.Resolved.Widget.ID),
		Value: in.UserJWT,
	})
	if err != nil {
		return StartedSession{}, err
	}
	previous, ok := a.previous(ctx, in)
	if ok && previous.Kind == string(enums.WidgetSessionIdentified) && sameCustomer(previous, verified) {
		return a.extend(ctx, previous, verified)
	}
	sign := signing{in: in, verified: verified}
	if ok {
		sign.previous = &previous
	}
	if ok && previous.Kind == string(enums.WidgetSessionAnonymous) {
		sign.lead = &previous.ContactID
	}
	return a.signIn(ctx, sign)
}

/*
signing is one sign-in: what the browser sent, the verified token, the
session the visitor had before, and the lead that session belonged to
when it was anonymous.
*/
type signing struct {
	in       StartWidgetSessionInput
	verified identity.Verified
	previous *models.WidgetSession
	lead     *ids.UUID
}

/*
signIn makes the customer and their new session in one transaction,
ending the session the visitor had before.
*/
func (a *StartWidgetSession) signIn(ctx context.Context, sign signing) (StartedSession, error) {
	now := a.Clock.Now()
	token, hash := NewSessionToken()
	out := StartedSession{Token: token}
	params, err := sessionParams(sign.in, now)
	if err != nil {
		return out, err
	}
	params.TokenHash, params.ExpiresAt = hash, now.Add(a.IdentifiedTTL)
	params.Kind = string(enums.WidgetSessionIdentified)
	var ended []EndedSession
	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		who, err := a.identifyLead(ctx, tx, sign)
		if err != nil {
			return err
		}
		ended, out.Contact, params.ContactID = endedOf(who.Ended), who.Contact, who.Contact.ID
		created, err := q.CreateWidgetSession(ctx, params)
		if err != nil {
			return fmt.Errorf("create widget session: %w", err)
		}
		if out.Session, err = a.verify(ctx, q, created, sign.verified); err != nil {
			return err
		}
		if sign.previous == nil {
			return nil
		}
		ended = append(ended, EndedSession{ID: sign.previous.ID, ContactID: sign.previous.ContactID})
		return endPrevious(ctx, q, *sign.previous, now)
	})
	if err != nil {
		return StartedSession{}, err
	}
	return out, a.close(ctx, ended)
}

/*
identifyLead finds or makes the customer, turning the visitor's
anonymous lead into them when there was one.
*/
func (a *StartWidgetSession) identifyLead(ctx context.Context, tx *database.Tx, sign signing,
) (identity.Identified, error) {
	ws := sign.in.Resolved.Workspace.ID
	var lead *models.Contact
	if sign.lead != nil {
		found, err := models.New(tx).FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: *sign.lead})
		if err != nil {
			return identity.Identified{}, fmt.Errorf("find lead: %w", err)
		}
		lead = &found
	}
	return identity.Identify(ctx, tx, a.Events, identity.Visitor{
		WorkspaceID: ws, Profile: sign.verified.Profile, Lead: lead, Now: a.Clock.Now(),
	})
}

/*
endPrevious ends the session the visitor had before signing in.
*/
func endPrevious(ctx context.Context, q *models.Queries, previous models.WidgetSession, now time.Time) error {
	_, err := q.RevokeWidgetSession(ctx, models.RevokeWidgetSessionParams{
		Now: now, Reason: string(enums.WidgetRevokeIdentitySwitch), WorkspaceID: previous.WorkspaceID,
		ID: previous.ID,
	})
	if err != nil {
		return fmt.Errorf("end previous session: %w", err)
	}
	return nil
}

/*
extend refreshes a signed-in session the visitor already has with the
same customer's fresh token.
*/
func (a *StartWidgetSession) extend(ctx context.Context, s models.WidgetSession, verified identity.Verified,
) (StartedSession, error) {
	out := StartedSession{Reused: true}
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out.Session, out.Contact, err = a.reverify(ctx, tx, s, verified)
		return err
	})
	return out, err
}

/*
Refresh proves a signed-in session's customer again with a fresh token,
extending it and writing what the token says about them. A token for
anyone else is refused, and the session stays as it was.
*/
func (a *StartWidgetSession) Refresh(ctx context.Context, auth Authenticated, token string) (SessionView, error) {
	if a.Identity == nil {
		return SessionView{}, exceptions.ErrIdentityNotConfigured
	}
	verified, err := a.Identity.Verify(ctx, identity.Token{
		WorkspaceID: auth.Session.WorkspaceID, AppID: models.PublicID(models.PrefixWidget, auth.Resolved.Widget.ID),
		Value: token,
	})
	if err != nil {
		return SessionView{}, err
	}
	if auth.Session.Kind != string(enums.WidgetSessionIdentified) || !sameCustomer(auth.Session, verified) {
		return SessionView{}, exceptions.ErrIdentityMismatch
	}
	out := SessionView{}
	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out.Session, out.Contact, err = a.reverify(ctx, tx, auth.Session, verified)
		return err
	})
	return out, err
}

/*
reverify writes the token's claims over the customer and extends the
session.
*/
func (a *StartWidgetSession) reverify(ctx context.Context, tx *database.Tx, s models.WidgetSession,
	verified identity.Verified,
) (models.WidgetSession, models.Contact, error) {
	now := a.Clock.Now()
	who, err := identity.Identify(ctx, tx, a.Events, identity.Visitor{
		WorkspaceID: s.WorkspaceID, Profile: verified.Profile, Now: now,
	})
	if err != nil {
		return s, who.Contact, err
	}
	session, err := a.verify(ctx, models.New(tx), s, verified)
	return session, who.Contact, err
}

/*
verify marks the session signed in by the verified token and moves its
expiry on.
*/
func (a *StartWidgetSession) verify(ctx context.Context, q *models.Queries, s models.WidgetSession,
	verified identity.Verified,
) (models.WidgetSession, error) {
	now := a.Clock.Now()
	out, err := q.VerifyWidgetSession(ctx, models.VerifyWidgetSessionParams{
		IdentitySecretID: &verified.SecretID, ExternalID: &verified.Profile.ExternalID, Now: now,
		ExpiresAt: now.Add(a.IdentifiedTTL), WorkspaceID: s.WorkspaceID, ID: s.ID,
	})
	if err != nil {
		return s, fmt.Errorf("verify widget session: %w", err)
	}
	return out, nil
}

/*
previous returns the session the visitor's earlier token names, only
when it is live and on the same widget; anything else is ignored.
*/
func (a *StartWidgetSession) previous(ctx context.Context, in StartWidgetSessionInput) (models.WidgetSession, bool) {
	if !WellFormedSessionToken(in.PreviousToken) {
		return models.WidgetSession{}, false
	}
	row, err := models.New(a.DB).ResolveWidgetSession(ctx, HashSessionToken(in.PreviousToken))
	if err != nil || row.WorkspaceID != in.Resolved.Workspace.ID {
		return models.WidgetSession{}, false
	}
	var s models.WidgetSession
	scoped := tenancy.With(ctx, row.WorkspaceID.String())
	err = a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		var err error
		s, err = models.New(tx).FindWidgetSession(ctx, models.FindWidgetSessionParams(row))
		return err
	})
	live := err == nil && s.RevokedAt == nil && a.Clock.Now().Before(s.ExpiresAt)
	if !live || s.WidgetSettingsID != in.Resolved.Widget.ID {
		return models.WidgetSession{}, false
	}
	return s, true
}

/*
close disconnects the streams of sessions that ended.
*/
func (a *StartWidgetSession) close(ctx context.Context, ended []EndedSession) error {
	if len(ended) == 0 || a.Streams == nil {
		return nil
	}
	return a.Streams.CloseSessions(ctx, ended)
}

/*
sameCustomer reports whether the session belongs to the customer the
token names.
*/
func sameCustomer(s models.WidgetSession, verified identity.Verified) bool {
	return s.ExternalID != nil && *s.ExternalID == verified.Profile.ExternalID
}

func endedOf(in []identity.Ended) []EndedSession {
	out := make([]EndedSession, len(in))
	for i, e := range in {
		out[i] = EndedSession{ID: e.SessionID, ContactID: e.ContactID}
	}
	return out
}

/*
IdentityStreams lets the identity actions close the streams of sessions
a revoked secret ended.
*/
type IdentityStreams struct {
	Streams SessionCloser
}

/*
CloseSessions closes the sessions' streams.
*/
func (s IdentityStreams) CloseSessions(ctx context.Context, ended []identity.Ended) error {
	if s.Streams == nil {
		return nil
	}
	return s.Streams.CloseSessions(ctx, endedOf(ended))
}
