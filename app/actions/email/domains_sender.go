/*
|--------------------------------------------------------------------------
| Sending Identity
|--------------------------------------------------------------------------
|
| An unproven address never sends. A reply leaves from the customer's own
| address only while the workspace holds that exact domain verified;
| otherwise it leaves from the address's forwarding address on our
| domain, with replies routed back through the reply token. A degraded
| domain falls back at once, since unsigned mail from the customer's own
| domain would fail DMARC and land in spam. A relay we sign for signs
| each reply with the domain's own key.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/integrations/email/dns"
	"github.com/dimmihq/dimmi/app/models"
)

/*
How an address sends: from its own domain, or from the default sender.
*/
const (
	SendingOwnDomain     = "own_domain"
	SendingDefaultSender = "default_sender"
)

/*
Why an address that should send from its own domain does not, or why
it does not send at all.
*/
const (
	BlockedDomainDegraded = "domain_degraded"
	BlockedSendingPaused  = "sending_paused"
)

/*
Sender is how one address sends now: the address mail leaves from, its
state, why it is held back, and the key a relay signs with.
*/
type Sender struct {
	Email         string
	State         string
	BlockedReason string
	Signing       *mail.SigningKey
}

/*
SendingStates is what decides how every address of a workspace sends:
the state of each of its domains, and whether its sending is paused.
*/
type SendingStates struct {
	Domains map[string]string
	Paused  bool
}

/*
For returns how the address sends, without its signing key.
*/
func (s SendingStates) For(address, forwarding string) Sender {
	sender := Sender{Email: forwarding, State: SendingDefaultSender}
	if s.Paused {
		sender.BlockedReason = BlockedSendingPaused
	}

	if strings.EqualFold(address, forwarding) {
		return sender
	}

	switch s.Domains[domainPart(address)] {
	case DomainVerified:
		sender.Email, sender.State = address, SendingOwnDomain
	case DomainDegraded:
		if sender.BlockedReason == "" {
			sender.BlockedReason = BlockedDomainDegraded
		}
	}

	return sender
}

/*
domainPart returns the lower case domain of an address.
*/
func domainPart(address string) string {
	_, domain, _ := strings.Cut(address, "@")

	return strings.ToLower(domain)
}

/*
SendingStates reads how the workspace's addresses send.
*/
func (d *Domains) SendingStates(ctx context.Context, q *models.Queries, ws ids.UUID) (SendingStates, error) {
	rows, err := q.ListEmailDomainStates(ctx, ws)
	if err != nil {
		return SendingStates{}, fmt.Errorf("list email domain states: %w", err)
	}

	states := SendingStates{Domains: make(map[string]string, len(rows))}
	for _, row := range rows {
		states.Domains[row.Domain] = row.Status
	}

	profile, err := q.FindEmailSendingProfile(ctx, ws)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return SendingStates{}, fmt.Errorf("find email sending profile: %w", err)
	}

	states.Paused = err == nil && profile.Status == "paused"

	return states, nil
}

/*
WorkspaceSendingStates reads how the workspace's addresses send, in a
transaction of its own.
*/
func (d *Domains) WorkspaceSendingStates(ctx context.Context, ws ids.UUID) (SendingStates, error) {
	var states SendingStates

	err := d.DB.Transaction(inWorkspace(ctx, ws), func(ctx context.Context, tx *database.Tx) error {
		var err error

		states, err = d.SendingStates(ctx, models.New(tx), ws)

		return err
	})

	return states, err
}

/*
ResolveSender returns how one address sends a reply now, with the key a
relay signs it with.
*/
func (d *Domains) ResolveSender(
	ctx context.Context, q *models.Queries, ws ids.UUID, address models.EmailAddress,
) (Sender, error) {
	states, err := d.SendingStates(ctx, q, ws)
	if err != nil {
		return Sender{}, err
	}

	sender := states.For(address.Address, address.ForwardingAddress)
	if sender.State != SendingOwnDomain || d.Driver != dns.DriverSMTP {
		return sender, nil
	}

	row, err := q.FindVerifiedEmailDomain(ctx, models.FindVerifiedEmailDomainParams{
		WorkspaceID: ws, Domain: domainPart(address.Address),
	})
	if err != nil {
		return Sender{}, fmt.Errorf("find verified email domain: %w", err)
	}

	signed, err := d.withKey(ctx, row, sender)
	if err != nil || signed.Signing != nil {
		return signed, err
	}

	fallback := Sender{Email: address.ForwardingAddress, State: SendingDefaultSender}
	fallback.BlockedReason = sender.BlockedReason

	return fallback, nil
}

/*
withKey adds the domain's signing key to the sender. A domain set up for
another driver has none, and is returned without one.
*/
func (d *Domains) withKey(ctx context.Context, row models.EmailDomain, sender Sender) (Sender, error) {
	if row.Driver != dns.DriverSMTP || row.DkimSelector == nil {
		return sender, nil
	}

	private, err := d.openKey(ctx, row, row.DkimSelector, row.DkimPrivateKeyEncrypted)
	if err != nil {
		return Sender{}, err
	}

	sender.Signing = &mail.SigningKey{Domain: row.Domain, Selector: *row.DkimSelector, PrivateKey: private}

	return sender, nil
}
