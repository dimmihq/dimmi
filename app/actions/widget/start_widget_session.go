/*
|--------------------------------------------------------------------------
| Start Widget Session
|--------------------------------------------------------------------------
|
| A visitor gets a session only when they first do something, never on a
| page view, so a site's traffic never fills the contact list. An anonymous
| session makes a lead and names it by the session, and hands back a token
| shown this once. A bot that fills the hidden form field gets the same
| answer as anyone, so it learns nothing, but its session is marked and
| everything it starts lands as spam.
|
| A visitor whose host app sent an identity token starts a signed-in
| session instead, and only once the token passes every check; a token
| that fails is refused, never quietly treated as anonymous.
|
*/

package widget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/identity"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
honeypotScore is the bot score of a session whose hidden field was filled.
*/
const honeypotScore = 100

/*
StartWidgetSessionInput is what a visitor's browser sends to start a session,
with where the request came from. Honeypot is the hidden field real visitors
never fill. PreviousToken is the session the browser had, handed over
when the visitor signs in.
*/
type StartWidgetSessionInput struct {
	Resolved      Resolved
	Visitor       Visitor
	UserJWT       string
	PreviousToken string
	Page          *Page
	Pages         []Page
	Locale        string
	Timezone      string
	Context       map[string]any
	Honeypot      string
	SDK           string
}

/*
StartedSession is a new session with its token, shown only now. Reused
says the visitor's signed-in session was extended instead, with no new
token.
*/
type StartedSession struct {
	Session models.WidgetSession
	Contact models.Contact
	Token   string
	Reused  bool
}

/*
StartWidgetSession starts visitors' sessions. AnonymousTTL is how long an
anonymous session lives after the last visit and IdentifiedTTL how long a
signed-in one lives after its last verified token. Identity checks
identity tokens, nil when the install verifies none, Streams closes
what sessions that ended had open and Outcomes counts how starts went.

The workspace's sign-in mode decides what a token does: off ignores it
and starts an anonymous session, required refuses a start without one.

Events: [appevents.ContactCreated], [appevents.ContactUpdated].
*/
type StartWidgetSession struct {
	DB            *database.DB
	Events        *events.Dispatcher
	Clock         clock.Clock
	AnonymousTTL  time.Duration
	IdentifiedTTL time.Duration
	Identity      IdentityVerifier
	Streams       SessionCloser
	Outcomes      OutcomeRecorder
}

/*
OutcomeRecorder counts how session starts went.
*/
type OutcomeRecorder interface {
	/*
		Record counts one outcome of the workspace.
	*/
	Record(workspaceID ids.UUID, outcome string)
}

/*
Handle starts the session the input asks for.
*/
func (a *StartWidgetSession) Handle(ctx context.Context, in StartWidgetSessionInput) (StartedSession, error) {
	out, err := a.start(ctx, in)
	a.record(in.Resolved.Workspace.ID, signsIn(in), err)

	return out, err
}

/*
signsIn reports whether the start signs a customer in: a token was sent
and the workspace reads tokens, or the token is the help center's own
pass, which signs its readers in whatever the workspace does with its
product's tokens.
*/
func signsIn(in StartWidgetSessionInput) bool {
	return in.UserJWT != "" && (in.Resolved.IdentityMode != identity.ModeOff || identity.IsPass(in.UserJWT))
}

/*
start starts the session the mode and input ask for.
*/
func (a *StartWidgetSession) start(ctx context.Context, in StartWidgetSessionInput) (StartedSession, error) {
	if signsIn(in) {
		return a.identified(ctx, in)
	}

	if in.Resolved.IdentityMode == identity.ModeRequired {
		return StartedSession{}, exceptions.ErrIdentityRequired
	}

	if !in.Resolved.Widget.AllowAnonymous {
		return StartedSession{}, exceptions.ErrAnonymousNotAllowed
	}

	return a.anonymous(ctx, in)
}

/*
record counts the start: signed in, anonymous, or the code it was
refused with.
*/
func (a *StartWidgetSession) record(workspaceID ids.UUID, signingIn bool, err error) {
	if a.Outcomes == nil {
		return
	}

	var f *fault.Error

	switch {
	case err == nil && signingIn:
		a.Outcomes.Record(workspaceID, identity.OutcomeVerified)
	case err == nil:
		a.Outcomes.Record(workspaceID, identity.OutcomeAnonymous)
	case errors.As(err, &f):
		a.Outcomes.Record(workspaceID, f.Code())
	}
}

/*
anonymous makes a lead and its session in one transaction.
*/
func (a *StartWidgetSession) anonymous(ctx context.Context, in StartWidgetSessionInput) (StartedSession, error) {
	now := a.Clock.Now()
	token, hash := NewSessionToken()
	out := StartedSession{Token: token}

	params, err := sessionParams(in, now)
	if err != nil {
		return out, err
	}

	params.TokenHash, params.ExpiresAt = hash, now.Add(a.AnonymousTTL)

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		out.Contact, err = q.CreateContact(ctx, models.CreateContactParams{
			WorkspaceID: in.Resolved.Workspace.ID, Kind: string(enums.ContactKindLead),
			CustomAttributes: json.RawMessage(`{}`), Now: now,
		})
		if err != nil {
			return fmt.Errorf("create lead: %w", err)
		}

		params.ContactID = out.Contact.ID

		out.Session, err = q.CreateWidgetSession(ctx, params)
		if err != nil {
			return fmt.Errorf("create widget session: %w", err)
		}

		_, err = q.CreateContactIdentity(ctx, models.CreateContactIdentityParams{
			WorkspaceID: out.Contact.WorkspaceID, ContactID: out.Contact.ID,
			Type:  string(enums.IdentityTypeWidgetVisitor),
			Value: models.PublicID(models.PrefixWidgetSession, out.Session.ID), Now: now,
		})
		if err != nil {
			return fmt.Errorf("name lead by its session: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.ContactCreated{
			WorkspaceID: out.Contact.WorkspaceID, ContactID: out.Contact.ID, ActorID: ids.Nil(),
		})
	})

	return out, err
}

/*
sessionParams returns the new session's row from what the browser sent,
cleaned, without its contact, token or expiry.
*/
func sessionParams(in StartWidgetSessionInput, now time.Time) (models.CreateWidgetSessionParams, error) {
	pages := cleanPages(in.Pages, now)

	var current json.RawMessage

	if in.Page != nil {
		if page, ok := cleanPage(*in.Page, now); ok {
			pages = pushPage(pages, page)
			current, _ = json.Marshal(page)
		}
	}

	recent, err := json.Marshal(pages)
	if err != nil {
		return models.CreateWidgetSessionParams{}, fmt.Errorf("encode recent pages: %w", err)
	}

	p := models.CreateWidgetSessionParams{
		WorkspaceID: in.Resolved.Workspace.ID, WidgetSettingsID: in.Resolved.Widget.ID,
		Kind: string(enums.WidgetSessionAnonymous), CurrentPage: current, RecentPages: recent,
		Context: cleanContext(json.RawMessage(`{}`), in.Context), UserAgent: cleanUserAgent(in.Visitor.UserAgent),
		IpPrefix: IPPrefix(in.Visitor.IP), Locale: validLocale(in.Locale), Timezone: validTimezone(in.Timezone),
		Origin: optionalText(in.Visitor.Origin), Now: now,
	}

	if in.Honeypot != "" {
		p.BotScore = honeypotScore
	}

	return p, nil
}

/*
validLocale returns the language tag in canonical form, nil when it is none.
*/
func validLocale(raw string) *string {
	locale, ok := i18n.Canonical(raw)
	if !ok || raw == "" {
		return nil
	}

	return &locale
}

/*
validTimezone returns the time zone name when the install knows it.
*/
func validTimezone(raw string) *string {
	if raw == "" || raw == "Local" {
		return nil
	}

	if _, err := time.LoadLocation(raw); err != nil {
		return nil
	}

	return &raw
}

/*
optionalText returns nil for empty text.
*/
func optionalText(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
