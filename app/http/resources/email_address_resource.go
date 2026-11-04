/*
|--------------------------------------------------------------------------
| Email Address Resources
|--------------------------------------------------------------------------
|
| An email address as the settings page shows it: the public address,
| the forwarding address to point it at, how replies leave and are
| signed, and whether mail has arrived yet. A forwarding confirmation a
| provider sent is shown so an admin can finish setting the forward up.
| Each address carries the steps for setting up forwarding in the mail
| services people use, with its own forwarding address filled in.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
previewTail shows the shape of a reply token address without being
one.
*/
const previewTail = "k7f3x9q2mz8b4c1d"

/*
EmailAddress is one connected email address.
*/
type EmailAddress struct {
	ID                     string                      `json:"id"`
	Object                 string                      `json:"object"`
	Address                string                      `json:"address"`
	DisplayName            string                      `json:"display_name"`
	ChannelConnection      ConnectionRef               `json:"channel_connection"`
	Inbox                  InboxRef                    `json:"inbox"`
	ForwardingAddress      string                      `json:"forwarding_address"`
	ReplyToMode            string                      `json:"reply_to_mode"`
	ReplyToPreview         *string                     `json:"reply_to_preview"`
	SendingIdentity        SendingIdentity             `json:"sending_identity"`
	SendingState           string                      `json:"sending_state"`
	SendsAs                string                      `json:"sends_as"`
	SendingBlockedReason   *string                     `json:"sending_blocked_reason"`
	FromNameStyle          string                      `json:"from_name_style"`
	IsDefault              bool                        `json:"is_default"`
	Settings               models.EmailAddressSettings `json:"settings"`
	ForwardingConfirmation json.RawMessage             `json:"forwarding_confirmation"`
	InboundVerifiedAt      *time.Time                  `json:"inbound_verified_at"`
	LastInboundAt          *time.Time                  `json:"last_inbound_at"`
	LastOutboundAt         *time.Time                  `json:"last_outbound_at"`
	SetupGuides            []SetupGuide                `json:"setup_guides"`
	CreatedAt              time.Time                   `json:"created_at"`
	UpdatedAt              time.Time                   `json:"updated_at"`
}

/*
ConnectionRef names the channel connection behind an address.
*/
type ConnectionRef struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Status string `json:"status"`
}

/*
InboxRef names the inbox an address feeds.
*/
type InboxRef struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Name   string `json:"name"`
}

/*
SendingIdentity is how replies from the address leave: the From line,
whether a verified domain of the workspace's own sends them, and the
install's provider.
*/
type SendingIdentity struct {
	From              string `json:"from"`
	DomainStatus      string `json:"domain_status"`
	UsesDefaultSender bool   `json:"uses_default_sender"`
	Provider          string `json:"provider"`
}

/*
EmailAddressRow is an address with the connection and inbox behind it.
*/
type EmailAddressRow struct {
	Address          models.EmailAddress
	InboxID          string
	InboxName        string
	ConnectionStatus string
}

/*
NewEmailAddress shapes an address. Provider is the install's
customer-stream transport.
*/
func NewEmailAddress(row EmailAddressRow, provider string) EmailAddress {
	a := row.Address

	out := EmailAddress{
		ID: models.PublicID(models.PrefixEmailAddress, a.ID), Object: "email_address", Address: a.Address,
		DisplayName: a.DisplayName,
		ChannelConnection: ConnectionRef{
			ID: models.PublicID(models.PrefixChannelConnection, a.ChannelConnectionID), Object: "channel_connection",
			Status: row.ConnectionStatus,
		},
		Inbox:             InboxRef{ID: row.InboxID, Object: "inbox", Name: row.InboxName},
		ForwardingAddress: a.ForwardingAddress, ReplyToMode: a.ReplyToMode, FromNameStyle: a.FromNameStyle,
		SendingIdentity: SendingIdentity{
			From: `"` + a.DisplayName + `" <` + a.ForwardingAddress + `>`, DomainStatus: "unverified",
			UsesDefaultSender: true, Provider: provider,
		},
		SendingState: "default_sender", SendsAs: `"` + a.DisplayName + `" <` + a.ForwardingAddress + `>`,
		IsDefault: a.IsDefault, Settings: models.ReadEmailAddressSettings(a.Settings),
		ForwardingConfirmation: confirmation(a.ForwardingConfirmation), InboundVerifiedAt: a.InboundVerifiedAt,
		LastInboundAt: a.LastInboundAt, LastOutboundAt: a.LastOutboundAt,
		SetupGuides: setupGuides(a.Address, a.ForwardingAddress), CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}

	if a.ReplyToMode == "token" {
		local, domain := split(a.ForwardingAddress)
		preview := local + "+" + previewTail + "@" + domain
		out.ReplyToPreview = &preview
	}

	return out
}

/*
NewEmailAddresses shapes a list of addresses, in the order given.
*/
func NewEmailAddresses(rows []models.ListEmailAddressesRow, provider string) []EmailAddress {
	out := make([]EmailAddress, 0, len(rows))
	for _, row := range rows {
		out = append(out, NewEmailAddress(EmailAddressRow{
			Address: row.EmailAddress, InboxID: models.PublicID(models.PrefixInbox, row.InboxID),
			InboxName: row.InboxName, ConnectionStatus: row.ConnectionStatus,
		}, provider))
	}

	return out
}

/*
confirmation returns a stored forwarding confirmation, or JSON null.
*/
func confirmation(stored json.RawMessage) json.RawMessage {
	if len(stored) == 0 {
		return json.RawMessage(`null`)
	}

	return stored
}

/*
split returns the local part and domain of an address.
*/
func split(address string) (string, string) {
	for i := len(address) - 1; i >= 0; i-- {
		if address[i] == '@' {
			return address[:i], address[i+1:]
		}
	}

	return address, ""
}
