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
| Signing a visitor in with their identity token belongs to identity
| verification; until an install has it, a token is refused rather than
| silently treated as anonymous.
|
*/

package widget

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/ids"

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
never fill.
*/
type StartWidgetSessionInput struct {
	Resolved Resolved
	Visitor  Visitor
	UserJWT  string
	Page     *Page
	Pages    []Page
	Locale   string
	Timezone string
	Context  map[string]any
	Honeypot string
	SDK      string
}

/*
StartedSession is a new session with its token, shown only now.
*/
type StartedSession struct {
	Session models.WidgetSession
	Contact models.Contact
	Token   string
}

/*
StartWidgetSession starts visitors' sessions. AnonymousTTL is how long an
anonymous session lives after the last visit.

Events: [appevents.ContactCreated].
*/
type StartWidgetSession struct {
	DB           *database.DB
	Events       *events.Dispatcher
	Clock        clock.Clock
	AnonymousTTL time.Duration
}

/*
Handle starts the session the input asks for.
*/
func (a *StartWidgetSession) Handle(ctx context.Context, in StartWidgetSessionInput) (StartedSession, error) {
	if in.UserJWT != "" {
		return StartedSession{}, exceptions.ErrIdentityNotConfigured
	}

	if !in.Resolved.Widget.AllowAnonymous {
		return StartedSession{}, exceptions.ErrAnonymousNotAllowed
	}

	return a.anonymous(ctx, in)
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
