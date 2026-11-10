/*
|--------------------------------------------------------------------------
| Receiving Slack Deliveries
|--------------------------------------------------------------------------
|
| Slack waits three seconds for an answer and retries otherwise, so a
| delivery is only checked, kept and queued here; the work happens in a
| job. Nothing is read before the signature is proven with the signing
| secret of the app it claims to be from: the install's shared app on
| the shared address, the connection's own app on its own address. A
| connection still waiting for its secret only answers Slack's address
| check, which Slack makes before anyone could copy the secret.
|
| A delivery for an app or team nobody connected is answered with
| success and dropped, so Slack stops retrying something that can never
| be ours. On the shared app one event can be visible to several
| installations, both sides of a shared channel being customers, and
| each connected workspace keeps its own copy.
|
*/

package channels

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Delivery is one request from Slack. ConnectionID names the connection
of a connection's own address, nil on the shared one. Body is the raw
body Slack signed; Payload is an interaction's payload field.
*/
type Delivery struct {
	ConnectionID *ids.UUID
	Interaction  bool
	Timestamp    string
	Signature    string
	Body         []byte
	Payload      []byte
}

/*
Answer is what Slack is told: the outcome and, for an address check,
the challenge to echo.
*/
type Answer struct {
	Outcome   Outcome
	Challenge string
}

/*
Outcome is how a delivery went: accepted, signed wrongly or not at all,
sent to an address that leads nowhere, or unreadable.
*/
type Outcome string

/*
The outcomes.
*/
const (
	Accepted   Outcome = "accepted"
	Unsigned   Outcome = "unsigned"
	Unknown    Outcome = "unknown"
	Unreadable Outcome = "unreadable"
)

/*
routed is a connection a delivery is for.
*/
type routed struct {
	workspaceID  ids.UUID
	connectionID ids.UUID
	suspended    bool
}

/*
Receive checks the delivery, keeps it for every connection it is for
and says what to answer Slack.
*/
func (s *Slack) Receive(ctx context.Context, d Delivery) (Answer, error) {
	to, err := s.addressOf(ctx, d.ConnectionID)
	if err != nil || !to.found {
		return Answer{Outcome: Unknown}, err
	}

	if to.secret == "" {
		return s.unverified(d), nil
	}

	own := to.own
	signed := slackdriver.SignedRequest{Timestamp: d.Timestamp, Signature: d.Signature, Body: d.Body}
	if !s.signedBy(to.secret, signed) {
		return Answer{Outcome: Unsigned}, nil
	}

	if d.Interaction {
		return s.receiveInteraction(ctx, own, d.Payload)
	}

	env, ok := readEnvelope(d.Body)
	if !ok {
		return Answer{Outcome: Unreadable}, nil
	}

	if env.Type == slackdriver.EnvelopeURLVerification {
		return Answer{Outcome: Accepted, Challenge: env.Challenge}, nil
	}

	return Answer{Outcome: Accepted}, s.keepEvent(ctx, own, eventDelivery{
		env: env, raw: d.Body, transport: TransportWebhook,
	})
}

/*
signedBy reports whether the request carries a valid signature of the
secret.
*/
func (s *Slack) signedBy(secret string, req slackdriver.SignedRequest) bool {
	return slackdriver.VerifySignature(secret, req, s.Clock.Now()) == nil
}

/*
unverified answers a connection's own address before its secret is
known: Slack's address check, and nothing else.
*/
func (s *Slack) unverified(d Delivery) Answer {
	env, ok := readEnvelope(d.Body)
	if d.Interaction || !ok || env.Type != slackdriver.EnvelopeURLVerification {
		return Answer{Outcome: Unsigned}
	}

	return Answer{Outcome: Accepted, Challenge: env.Challenge}
}

/*
address is where a delivery was sent: the signing secret it must carry,
the connection it belongs to, nil for the shared address, and whether
it leads anywhere at all.
*/
type address struct {
	secret string
	own    *routed
	found  bool
}

/*
addressOf returns what the address the delivery was sent to expects.
*/
func (s *Slack) addressOf(ctx context.Context, connectionID *ids.UUID) (address, error) {
	if connectionID == nil {
		if s.Shared == nil || s.Shared.SigningSecret == "" {
			return address{}, nil
		}

		return address{secret: s.Shared.SigningSecret, found: true}, nil
	}

	found, err := models.New(s.DB).RouteChannelConnectionID(ctx, models.RouteChannelConnectionIDParams{
		ConnectionType: slackdriver.Type, ConnectionID: *connectionID,
	})
	if err != nil {
		return address{}, ignoreMissing(err)
	}

	r := &routed{workspaceID: found.WorkspaceID, connectionID: found.ChannelConnectionID, suspended: found.Suspended}

	conn, err := s.connection(tenancy.With(ctx, r.workspaceID.String()), r.workspaceID, r.connectionID)
	if err != nil {
		return address{}, err
	}

	secrets, err := s.Secrets.Open(ctx, conn)
	if err != nil {
		return address{}, err
	}

	return address{secret: secrets.SigningSecret, own: r, found: true}, nil
}

/*
receiveInteraction keeps an interaction for the connection it is for.
*/
func (s *Slack) receiveInteraction(ctx context.Context, own *routed, payload []byte) (Answer, error) {
	in, ok := readInteraction(payload)
	if !ok {
		return Answer{Outcome: Unreadable}, nil
	}

	return Answer{Outcome: Accepted}, s.keepInteraction(ctx, own, in, payload)
}

/*
eventDelivery is an Events API delivery as it came, and how it arrived.
*/
type eventDelivery struct {
	env       slackdriver.Envelope
	raw       []byte
	transport string
}

/*
keepEvent keeps an Events API delivery for every connection it is for:
the one named, or every installation of the shared app it is visible
to.
*/
func (s *Slack) keepEvent(ctx context.Context, own *routed, d eventDelivery) error {
	env := d.env
	if env.Type != slackdriver.EnvelopeEventCallback || env.EventID == "" {
		return nil
	}

	ev, ok := readEvent(env.Event)
	if !ok {
		return nil
	}

	targets, err := s.targets(ctx, own, env.APIAppID, teamsOf(env))
	if err != nil {
		return err
	}

	for _, target := range targets {
		err := s.keep(ctx, target, stored{
			transport: d.transport, dedupeKey: env.EventID, serialKey: serialKeyOf(ev), kind: ev.Type, payload: d.raw,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

/*
keepInteraction keeps an interaction for the connection it is for.
*/
func (s *Slack) keepInteraction(ctx context.Context, own *routed, in slackdriver.Interaction, raw []byte) error {
	targets, err := s.targets(ctx, own, in.APIAppID, []string{in.TeamID()})
	if err != nil {
		return err
	}

	for _, target := range targets {
		err := s.keep(ctx, target, stored{
			transport: "interaction", dedupeKey: in.DedupeKey(), kind: in.Type, payload: raw,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

/*
targets returns the connections a delivery is for: the address's own,
or on the shared app every connected installation among the teams.
Message shortcuts arrive without the app's id; their signature already
proves they come from the shared app, so the team alone routes them.
*/
func (s *Slack) targets(ctx context.Context, own *routed, appID string, teams []string) ([]routed, error) {
	if own != nil {
		return []routed{*own}, nil
	}

	if s.Shared == nil || (s.Shared.AppID != "" && appID != "" && appID != s.Shared.AppID) {
		return nil, nil
	}

	appID = cmp.Or(appID, s.Shared.AppID)

	var out []routed

	for _, team := range teams {
		found, err := models.New(s.DB).RouteChannelConnection(ctx, models.RouteChannelConnectionParams{
			ConnectionType: slackdriver.Type, ProviderKey: slackdriver.ExternalID(appID, team),
		})
		if errors.Is(err, database.ErrNotFound) {
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("route slack delivery: %w", err)
		}

		out = append(out, routed{
			workspaceID: found.WorkspaceID, connectionID: found.ChannelConnectionID, suspended: found.Suspended,
		})
	}

	return out, nil
}

/*
teamsOf returns the teams an event is visible to, each once: the team
it was delivered for and every installation it lists.
*/
func teamsOf(env slackdriver.Envelope) []string {
	teams := []string{env.TeamID}

	for _, auth := range env.Authorizations {
		if auth.TeamID != "" && !slices.Contains(teams, auth.TeamID) {
			teams = append(teams, auth.TeamID)
		}
	}

	return teams
}

/*
serialKeyOf returns the thread a message event belongs to, so events of
one thread are processed one at a time, in order; empty for other
events.
*/
func serialKeyOf(ev slackdriver.Event) string {
	if ev.Type != "message" {
		return ""
	}

	root := ev.RootTS()
	if ev.Message != nil && ev.Message.ThreadTS != "" {
		root = ev.Message.ThreadTS
	}

	return ev.ChannelID() + ":" + root
}
