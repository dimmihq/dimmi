/*
|--------------------------------------------------------------------------
| Changelog Subscribers
|--------------------------------------------------------------------------
|
| People ask to hear about new entries with their email. An address
| someone typed only starts getting mail once its owner confirms from
| the mail sent to it, so nobody can subscribe a stranger; a reader
| signed in with that address is subscribed at once. Asking again never
| says whether the address was already known, and a confirmation goes
| out at most once every ten minutes.
|
| Links carry the subscriber and a keyed hash of its nonce. Confirming
| and unsubscribing happen on a POST, never on the GET a link scanner
| makes. Unsubscribing draws a new nonce, so every old link stops
| working.
|
*/

package changelog

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	fmail "github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of subscribing.
*/
var (
	ErrEmailInvalid = fault.New(fault.Invalid, "email_invalid", "Enter a valid email address.").OnField("email")
	ErrLinkInvalid  = fault.New(fault.NotFound, "subscription_link_invalid",
		"This link no longer works. Subscribe again from the changelog.")
	ErrSubscriberNotFound = fault.New(fault.NotFound, "changelog_subscriber_not_found",
		"No subscriber of this workspace has this id.")
)

/*
resendAfter is how long a confirmation must wait before another goes to
the same address.
*/
const resendAfter = 10 * time.Minute

/*
Purposes a subscriber's link serves.
*/
const (
	PurposeConfirm     = "confirm"
	PurposeUnsubscribe = "unsubscribe"
)

/*
Subscribers manages who hears about a changelog. Key signs their links
and Addresses says where the help center lives.
*/
type Subscribers struct {
	DB        *database.DB
	Clock     clock.Clock
	Mail      *fmail.Manager
	Key       []byte
	Addresses helpcenters.Addresses
}

/*
SubscribeInput is an address asking for updates. Verified says a reader
signed in with it, so it needs no confirming; Contact is that reader.
*/
type SubscribeInput struct {
	Center   models.HelpCenter
	Email    string
	Verified bool
	Contact  *ids.UUID
}

/*
Subscribe takes an address, active at once when verified, otherwise
pending with a confirmation on its way.
*/
func (s *Subscribers) Subscribe(ctx context.Context, in SubscribeInput) (models.ChangelogSubscriber, error) {
	address, err := fmail.ParseAddress(strings.TrimSpace(in.Email))
	if err != nil || len(address.Email) > 320 {
		return models.ChangelogSubscriber{}, ErrEmailInvalid
	}
	email, now := strings.ToLower(address.Email), s.Clock.Now()
	var out models.ChangelogSubscriber
	scoped := tenancy.With(ctx, in.Center.WorkspaceID.String())
	err = s.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		before, err := q.FindChangelogSubscriberByEmail(ctx, models.FindChangelogSubscriberByEmailParams{
			WorkspaceID: in.Center.WorkspaceID, HelpCenterID: in.Center.ID, Email: email,
		})
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find changelog subscriber: %w", err)
		}
		mailNow := !in.Verified && before.Status != "active" && !recentlySent(before, now)
		params := models.UpsertChangelogSubscriberParams{
			WorkspaceID: in.Center.WorkspaceID, HelpCenterID: in.Center.ID, Email: email, ContactID: in.Contact,
			Status: "pending", Nonce: str.Random(24), Now: now,
		}
		if in.Verified {
			params.Status, params.ConfirmedAt = "active", &now
		}
		if mailNow {
			params.SentAt = &now
		}
		if out, err = q.UpsertChangelogSubscriber(ctx, params); err != nil {
			return fmt.Errorf("keep changelog subscriber: %w", err)
		}
		if !mailNow || out.Status == "active" {
			return nil
		}
		return s.Mail.To(fmail.Address{Email: email}).QueueTx(ctx, tx, appmail.ChangelogConfirmMail{
			HelpCenter: in.Center.Name, Brand: brandOf(in.Center), Link: s.Link(in.Center, out, PurposeConfirm),
		})
	})
	return out, err
}

func recentlySent(sub models.ChangelogSubscriber, now time.Time) bool {
	return sub.ConfirmationSentAt != nil && now.Sub(*sub.ConfirmationSentAt) < resendAfter
}

/*
Link is the page a subscriber's link opens for the purpose.
*/
func (s *Subscribers) Link(h models.HelpCenter, sub models.ChangelogSubscriber, purpose string) string {
	return s.Addresses.Root(h) + "/changelog/subscribers/" + sub.ID.String() + "/" + purpose + "/" +
		s.token(sub, purpose)
}

/*
token is the keyed hash a subscriber's link carries for the purpose.
*/
func (s *Subscribers) token(sub models.ChangelogSubscriber, purpose string) string {
	mac := hashing.HMAC(s.Key, []byte(sub.ID.String()+"|"+sub.Nonce+"|"+purpose))
	return base64.RawURLEncoding.EncodeToString(mac)[:32]
}

/*
LinkRequest is a subscriber's link as it came back.
*/
type LinkRequest struct {
	Center  models.HelpCenter
	ID      ids.UUID
	Purpose string
	Token   string
}

/*
Check returns the subscriber a link names when it is still good.
*/
func (s *Subscribers) Check(ctx context.Context, in LinkRequest) (models.ChangelogSubscriber, error) {
	var out models.ChangelogSubscriber
	err := s.DB.Transaction(scope(ctx, in), func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = s.checked(ctx, models.New(tx), in)
		return err
	})
	return out, err
}

func (s *Subscribers) checked(ctx context.Context, q *models.Queries, in LinkRequest,
) (models.ChangelogSubscriber, error) {
	sub, err := q.FindChangelogSubscriber(ctx, models.FindChangelogSubscriberParams{
		WorkspaceID: in.Center.WorkspaceID, ID: in.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return sub, ErrLinkInvalid
	}
	if err != nil {
		return sub, fmt.Errorf("find changelog subscriber: %w", err)
	}
	if sub.HelpCenterID != in.Center.ID || !hashing.Equal([]byte(s.token(sub, in.Purpose)), []byte(in.Token)) {
		return sub, ErrLinkInvalid
	}
	return sub, nil
}

/*
Confirm makes the subscriber a link names active.
*/
func (s *Subscribers) Confirm(ctx context.Context, in LinkRequest) (models.ChangelogSubscriber, error) {
	in.Purpose = PurposeConfirm
	var out models.ChangelogSubscriber
	err := s.DB.Transaction(scope(ctx, in), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		sub, err := s.checked(ctx, q, in)
		if err != nil {
			return err
		}
		out, err = q.ConfirmChangelogSubscriber(ctx, models.ConfirmChangelogSubscriberParams{
			Now: s.Clock.Now(), WorkspaceID: sub.WorkspaceID, ID: sub.ID,
		})
		if err != nil {
			return fmt.Errorf("confirm changelog subscriber: %w", err)
		}
		return nil
	})
	return out, err
}

/*
Unsubscribe stops the mail to the subscriber a link names.
*/
func (s *Subscribers) Unsubscribe(ctx context.Context, in LinkRequest) (models.ChangelogSubscriber, error) {
	in.Purpose = PurposeUnsubscribe
	var out models.ChangelogSubscriber
	err := s.DB.Transaction(scope(ctx, in), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		sub, err := s.checked(ctx, q, in)
		if err != nil {
			return err
		}
		out, err = q.UnsubscribeChangelogSubscriber(ctx, models.UnsubscribeChangelogSubscriberParams{
			Now: new(s.Clock.Now()), Nonce: str.Random(24), WorkspaceID: sub.WorkspaceID, ID: sub.ID,
		})
		if err != nil {
			return fmt.Errorf("unsubscribe changelog subscriber: %w", err)
		}
		return nil
	})
	return out, err
}

/*
SubscriberFilter narrows a page of a changelog's subscribers.
*/
type SubscriberFilter struct {
	HelpCenterID ids.UUID
	Status       string
	Query        string
	Limit        int
	Offset       int
}

/*
List returns a page of a changelog's subscribers and how many match.
*/
func (s *Subscribers) List(ctx context.Context, m authz.Membership, f SubscriberFilter,
) ([]models.ChangelogSubscriber, int64, error) {
	if !m.Can("changelog:read") {
		return nil, 0, exceptions.ErrForbidden
	}
	ws, status, query := m.Workspace.ID, nonEmpty(f.Status), nonEmpty(strings.TrimSpace(f.Query))
	var (
		out   []models.ChangelogSubscriber
		total int64
	)
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		var err error
		if out, err = q.ListChangelogSubscribers(ctx, models.ListChangelogSubscribersParams{
			WorkspaceID: ws, HelpCenterID: f.HelpCenterID, Status: status, Query: query,
			RowLimit: int32Of(f.Limit), RowOffset: int32Of(f.Offset),
		}); err != nil {
			return fmt.Errorf("list changelog subscribers: %w", err)
		}
		if total, err = q.CountChangelogSubscribers(ctx, models.CountChangelogSubscribersParams{
			WorkspaceID: ws, HelpCenterID: f.HelpCenterID, Status: status, Query: query,
		}); err != nil {
			return fmt.Errorf("count changelog subscribers: %w", err)
		}
		return nil
	})
	return out, total, err
}

/*
Delete forgets a subscriber.
*/
func (s *Subscribers) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("changelog:write") {
		return exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := q.FindChangelogSubscriber(ctx, models.FindChangelogSubscriberParams{
			WorkspaceID: ws, ID: id,
		}); err != nil {
			return notFound(err, ErrSubscriberNotFound)
		}
		if err := q.DeleteChangelogSubscriber(ctx, models.DeleteChangelogSubscriberParams{
			WorkspaceID: ws, ID: id,
		}); err != nil {
			return fmt.Errorf("delete changelog subscriber: %w", err)
		}
		return nil
	})
}

/*
brandOf is the help center's brand color, with the default when it has
none.
*/
func brandOf(h models.HelpCenter) string {
	if c := helpcenters.SettingsOf(h).PrimaryColor; c != "" {
		return c
	}
	return "#10b981"
}

/*
scope runs a link's work inside the help center's workspace.
*/
func scope(ctx context.Context, in LinkRequest) context.Context {
	return tenancy.With(ctx, in.Center.WorkspaceID.String())
}
